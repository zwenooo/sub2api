package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	openAIAuthImportMaxBytes           = 2 << 20
	openAIAuthImportDefaultConcurrency = 3
	openAIAuthImportDefaultPriority    = 50
	openAIAuthImportSource             = "auth_json"
)

type OpenAIAuthImportResult struct {
	AccountCreated int                     `json:"account_created"`
	AccountFailed  int                     `json:"account_failed"`
	Errors         []OpenAIAuthImportError `json:"errors,omitempty"`
}

type OpenAIAuthImportError struct {
	Index   int    `json:"index"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

type openAIAuthImportRequest struct {
	Items        []json.RawMessage `json:"items"`
	GroupIDs     []int64           `json:"group_ids"`
	NameTemplate string            `json:"name_template"`
}

type openAIAuthImportOptions struct {
	GroupIDs     []int64
	NameTemplate string
}

var openAIAuthImportSupportedTemplateTokens = map[string]struct{}{
	"index":              {},
	"email":              {},
	"account_id":         {},
	"chatgpt_account_id": {},
	"chatgpt_user_id":    {},
	"organization_id":    {},
	"plan_type":          {},
	"client_id":          {},
}

// ImportOpenAIAuthJSON imports OpenAI auth.json payloads from a raw JSON array.
// POST /api/v1/admin/accounts/openai-auths/import
func (h *AccountHandler) ImportOpenAIAuthJSON(c *gin.Context) {
	raw, err := readOpenAIAuthImportPayload(c.Request.Body)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	items, options, err := decodeOpenAIAuthImportRequest(raw)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_openai_auth_json", map[string]any{
		"payload": string(raw),
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importOpenAIAuthItems(ctx, items, options)
	})
}

// ImportOpenAIAuthFile imports OpenAI auth.json payloads from an uploaded JSON file.
// POST /api/v1/admin/accounts/openai-auths/import-file
func (h *AccountHandler) ImportOpenAIAuthFile(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.BadRequest(c, "failed to open file: "+err.Error())
		return
	}
	defer file.Close()

	raw, err := readOpenAIAuthImportPayload(file)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	items, err := decodeOpenAIAuthImportItems(raw)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	options, err := parseOpenAIAuthImportFormOptions(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_openai_auth_file", map[string]any{
		"filename":      fileHeader.Filename,
		"payload":       string(raw),
		"group_ids":     options.GroupIDs,
		"name_template": options.NameTemplate,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importOpenAIAuthItems(ctx, items, options)
	})
}

func (h *AccountHandler) importOpenAIAuthItems(ctx context.Context, items []json.RawMessage, options openAIAuthImportOptions) (OpenAIAuthImportResult, error) {
	result := OpenAIAuthImportResult{}

	for i, rawItem := range items {
		var payload map[string]any
		if err := json.Unmarshal(rawItem, &payload); err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, OpenAIAuthImportError{
				Index:   i + 1,
				Message: "invalid auth item: " + err.Error(),
			})
			continue
		}
		if payload == nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, OpenAIAuthImportError{
				Index:   i + 1,
				Message: "auth item must be a JSON object",
			})
			continue
		}

		accountInput, accountName, err := buildOpenAIAuthImportAccountInput(ctx, payload, i, options, h.resolveOpenAIAuthImportPlanType)
		if err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, OpenAIAuthImportError{
				Index:   i + 1,
				Name:    accountName,
				Message: err.Error(),
			})
			continue
		}

		if _, err := h.adminService.CreateAccount(ctx, accountInput); err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, OpenAIAuthImportError{
				Index:   i + 1,
				Name:    accountInput.Name,
				Message: err.Error(),
			})
			continue
		}

		result.AccountCreated++
	}

	return result, nil
}

type openAIAuthImportPlanTypeResolver func(ctx context.Context, accessToken string, accountIDHint string) (string, string, error)

func buildOpenAIAuthImportAccountInput(ctx context.Context, payload map[string]any, index int, options openAIAuthImportOptions, planTypeResolver openAIAuthImportPlanTypeResolver) (*service.CreateAccountInput, string, error) {
	refreshToken := openAIAuthImportString(payload, "refresh_token")
	if refreshToken == "" {
		return nil, "", errors.New("refresh_token is required")
	}

	idToken := openAIAuthImportString(payload, "id_token")
	accessToken := openAIAuthImportString(payload, "access_token")
	livePlanAccessToken := accessToken
	if accessToken == "" && idToken != "" {
		// codex-service-go treats id_token as a usable bearer fallback when access_token is absent.
		accessToken = idToken
	}
	if accessToken == "" {
		return nil, "", errors.New("access_token or id_token is required")
	}

	credentials := map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	}

	if idToken != "" {
		credentials["id_token"] = idToken
	}
	if clientID := openAIAuthImportString(payload, "client_id"); clientID != "" {
		credentials["client_id"] = clientID
	}
	if email := openAIAuthImportString(payload, "email"); email != "" {
		credentials["email"] = email
	}
	if planType := openAIAuthImportString(payload, "plan_type"); planType != "" {
		credentials["plan_type"] = planType
	}
	if organizationID := openAIAuthImportString(payload, "organization_id"); organizationID != "" {
		credentials["organization_id"] = organizationID
	}
	if chatGPTUserID := openAIAuthImportString(payload, "chatgpt_user_id"); chatGPTUserID != "" {
		credentials["chatgpt_user_id"] = chatGPTUserID
	}

	chatGPTAccountID := openAIAuthImportString(payload, "chatgpt_account_id")
	if chatGPTAccountID == "" {
		chatGPTAccountID = openAIAuthImportString(payload, "account_id")
	}
	if chatGPTAccountID != "" {
		credentials["chatgpt_account_id"] = chatGPTAccountID
	}

	dataAccount := DataAccount{
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeOAuth,
		Credentials: credentials,
	}
	enrichCredentialsFromIDToken(&dataAccount)
	enrichOpenAIAuthImportCredentialsFromResolver(ctx, &dataAccount, livePlanAccessToken, planTypeResolver)

	accountName := buildOpenAIAuthImportAccountName(dataAccount.Credentials, index)
	if template := strings.TrimSpace(options.NameTemplate); template != "" {
		rendered, err := renderOpenAIAuthImportNameTemplate(template, payload, dataAccount.Credentials, index+1)
		if err != nil {
			return nil, accountName, err
		}
		accountName = rendered
	}

	return &service.CreateAccountInput{
		Name:                 accountName,
		Platform:             service.PlatformOpenAI,
		Type:                 service.AccountTypeOAuth,
		Credentials:          dataAccount.Credentials,
		Extra:                map[string]any{"import_source": openAIAuthImportSource},
		Concurrency:          openAIAuthImportDefaultConcurrency,
		Priority:             openAIAuthImportDefaultPriority,
		GroupIDs:             cloneOpenAIRTImportGroupIDs(options.GroupIDs),
		SkipDefaultGroupBind: true,
	}, accountName, nil
}

func (h *AccountHandler) resolveOpenAIAuthImportPlanType(ctx context.Context, accessToken string, accountIDHint string) (string, string, error) {
	if h == nil || h.openaiOAuthService == nil {
		return "", "", nil
	}
	return h.openaiOAuthService.ResolvePlanTypeFromAccessToken(ctx, accessToken, "", accountIDHint)
}

func enrichOpenAIAuthImportCredentialsFromResolver(ctx context.Context, item *DataAccount, accessToken string, planTypeResolver openAIAuthImportPlanTypeResolver) {
	if item == nil || item.Credentials == nil || planTypeResolver == nil {
		return
	}

	if strings.TrimSpace(accessToken) == "" {
		return
	}

	accountIDHint, _ := item.Credentials["chatgpt_account_id"].(string)
	resolvedPlanType, resolvedAccountID, err := planTypeResolver(ctx, accessToken, accountIDHint)
	if err != nil {
		slog.Warn("openai_auth_import_plan_type_resolve_failed", "account", item.Name, "error", err, "chatgpt_account_id", accountIDHint)
		return
	}

	if strings.TrimSpace(resolvedPlanType) != "" {
		item.Credentials["plan_type"] = strings.TrimSpace(resolvedPlanType)
	}
	if existingAccountID, _ := item.Credentials["chatgpt_account_id"].(string); strings.TrimSpace(existingAccountID) == "" && strings.TrimSpace(resolvedAccountID) != "" {
		item.Credentials["chatgpt_account_id"] = strings.TrimSpace(resolvedAccountID)
	}
}

func buildOpenAIAuthImportAccountName(credentials map[string]any, index int) string {
	email := openAIAuthImportValueString(credentials["email"])
	organizationID := openAIAuthImportValueString(credentials["organization_id"])
	chatGPTAccountID := openAIAuthImportValueString(credentials["chatgpt_account_id"])

	switch {
	case email != "" && organizationID != "":
		return fmt.Sprintf("%s (%s)", email, organizationID)
	case email != "":
		return email
	case chatGPTAccountID != "" && organizationID != "":
		return fmt.Sprintf("openai-%s (%s)", chatGPTAccountID, organizationID)
	case chatGPTAccountID != "":
		return "openai-" + chatGPTAccountID
	default:
		return fmt.Sprintf("openai-auth-import-%d", index+1)
	}
}

func decodeOpenAIAuthImportItems(raw []byte) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("payload must be a JSON array: %w", err)
	}
	if len(items) == 0 {
		return nil, errors.New("payload must contain at least one auth item")
	}
	return items, nil
}

func decodeOpenAIAuthImportRequest(raw []byte) ([]json.RawMessage, openAIAuthImportOptions, error) {
	if items, err := decodeOpenAIAuthImportItems(raw); err == nil {
		return items, openAIAuthImportOptions{}, nil
	}

	var req openAIAuthImportRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, openAIAuthImportOptions{}, fmt.Errorf("payload must be a JSON array or object: %w", err)
	}

	items, err := decodeOpenAIAuthImportItemsFromRequest(req.Items)
	if err != nil {
		return nil, openAIAuthImportOptions{}, err
	}

	options, err := normalizeOpenAIAuthImportOptions(req.GroupIDs, req.NameTemplate)
	if err != nil {
		return nil, openAIAuthImportOptions{}, err
	}
	return items, options, nil
}

func decodeOpenAIAuthImportItemsFromRequest(items []json.RawMessage) ([]json.RawMessage, error) {
	if len(items) == 0 {
		return nil, errors.New("items must contain at least one auth item")
	}
	return items, nil
}

func parseOpenAIAuthImportFormOptions(c *gin.Context) (openAIAuthImportOptions, error) {
	groupIDs, err := parseOpenAIAuthImportGroupIDs(c.PostForm("group_ids"))
	if err != nil {
		return openAIAuthImportOptions{}, err
	}
	return normalizeOpenAIAuthImportOptions(groupIDs, c.PostForm("name_template"))
}

func normalizeOpenAIAuthImportOptions(groupIDs []int64, nameTemplate string) (openAIAuthImportOptions, error) {
	trimmedTemplate := strings.TrimSpace(nameTemplate)
	if trimmedTemplate != "" {
		if err := validateOpenAIAuthImportNameTemplate(trimmedTemplate); err != nil {
			return openAIAuthImportOptions{}, err
		}
	}
	return openAIAuthImportOptions{
		GroupIDs:     normalizeOpenAIRTImportGroupIDs(groupIDs),
		NameTemplate: trimmedTemplate,
	}, nil
}

func parseOpenAIAuthImportGroupIDs(raw string) ([]int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	var ids []int64
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &ids); err != nil {
			return nil, fmt.Errorf("group_ids must be a JSON array of integers: %w", err)
		}
		return ids, nil
	}

	parts := strings.Split(trimmed, ",")
	ids = make([]int64, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("group_ids must contain integers: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func validateOpenAIAuthImportNameTemplate(template string) error {
	inPlaceholder := false
	start := -1
	for i, r := range template {
		switch r {
		case '{':
			if inPlaceholder {
				return errors.New("name_template contains nested '{'")
			}
			inPlaceholder = true
			start = i
		case '}':
			if !inPlaceholder {
				return errors.New("name_template contains unmatched '}'")
			}
			token := strings.TrimSpace(template[start+1 : i])
			if token == "" {
				return errors.New("name_template contains empty placeholder")
			}
			if _, ok := openAIAuthImportSupportedTemplateTokens[token]; !ok {
				return fmt.Errorf("name_template contains unsupported placeholder {%s}", token)
			}
			inPlaceholder = false
			start = -1
		}
	}
	if inPlaceholder {
		return errors.New("name_template contains unmatched '{'")
	}
	return nil
}

func renderOpenAIAuthImportNameTemplate(template string, payload map[string]any, credentials map[string]any, seq int) (string, error) {
	values := buildOpenAIAuthImportTemplateValues(payload, credentials, seq)

	var builder strings.Builder
	inPlaceholder := false
	start := -1
	for i, r := range template {
		switch r {
		case '{':
			if inPlaceholder {
				return "", errors.New("name_template contains nested '{'")
			}
			inPlaceholder = true
			start = i
		case '}':
			if !inPlaceholder {
				return "", errors.New("name_template contains unmatched '}'")
			}
			token := strings.TrimSpace(template[start+1 : i])
			value, ok := values[token]
			if !ok {
				return "", fmt.Errorf("name_template contains unsupported placeholder {%s}", token)
			}
			if value == "" {
				return "", fmt.Errorf("name_template placeholder {%s} is empty in auth item", token)
			}
			builder.WriteString(value)
			inPlaceholder = false
			start = -1
		default:
			if !inPlaceholder {
				builder.WriteRune(r)
			}
		}
	}

	if inPlaceholder {
		return "", errors.New("name_template contains unmatched '{'")
	}

	rendered := strings.TrimSpace(builder.String())
	if rendered == "" {
		return "", errors.New("name_template generated an empty account name")
	}
	return rendered, nil
}

func buildOpenAIAuthImportTemplateValues(payload map[string]any, credentials map[string]any, seq int) map[string]string {
	accountID := openAIAuthImportString(payload, "account_id")
	chatGPTAccountID := openAIAuthImportValueString(credentials["chatgpt_account_id"])
	if chatGPTAccountID == "" {
		chatGPTAccountID = openAIAuthImportString(payload, "chatgpt_account_id")
	}
	if accountID == "" {
		accountID = chatGPTAccountID
	}
	if chatGPTAccountID == "" {
		chatGPTAccountID = accountID
	}

	return map[string]string{
		"index":              strconv.Itoa(seq),
		"email":              openAIAuthImportValueString(credentials["email"]),
		"account_id":         accountID,
		"chatgpt_account_id": chatGPTAccountID,
		"chatgpt_user_id":    openAIAuthImportValueString(credentials["chatgpt_user_id"]),
		"organization_id":    openAIAuthImportValueString(credentials["organization_id"]),
		"plan_type":          openAIAuthImportValueString(credentials["plan_type"]),
		"client_id":          openAIAuthImportValueString(credentials["client_id"]),
	}
}

func readOpenAIAuthImportPayload(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, errors.New("payload is required")
	}

	raw, err := io.ReadAll(io.LimitReader(reader, openAIAuthImportMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}
	if len(raw) > openAIAuthImportMaxBytes {
		return nil, fmt.Errorf("payload exceeds %d bytes", openAIAuthImportMaxBytes)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("payload is required")
	}

	return raw, nil
}

func openAIAuthImportString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	if tokens, ok := payload["tokens"].(map[string]any); ok {
		if value := openAIAuthImportValueString(tokens[key]); value != "" {
			return value
		}
	}
	return openAIAuthImportValueString(payload[key])
}

func openAIAuthImportValueString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

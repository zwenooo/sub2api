package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	AccountCreated int                      `json:"account_created"`
	AccountFailed  int                      `json:"account_failed"`
	Errors         []OpenAIAuthImportError  `json:"errors,omitempty"`
}

type OpenAIAuthImportError struct {
	Index   int    `json:"index"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// ImportOpenAIAuthJSON imports OpenAI auth.json payloads from a raw JSON array.
// POST /api/v1/admin/accounts/openai-auths/import
func (h *AccountHandler) ImportOpenAIAuthJSON(c *gin.Context) {
	raw, err := readOpenAIAuthImportPayload(c.Request.Body)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	items, err := decodeOpenAIAuthImportItems(raw)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_openai_auth_json", map[string]any{
		"payload": string(raw),
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importOpenAIAuthItems(ctx, items)
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

	executeAdminIdempotentJSON(c, "admin.accounts.import_openai_auth_file", map[string]any{
		"filename": fileHeader.Filename,
		"payload":  string(raw),
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importOpenAIAuthItems(ctx, items)
	})
}

func (h *AccountHandler) importOpenAIAuthItems(ctx context.Context, items []json.RawMessage) (OpenAIAuthImportResult, error) {
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

		accountInput, accountName, err := buildOpenAIAuthImportAccountInput(payload, i)
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

func buildOpenAIAuthImportAccountInput(payload map[string]any, index int) (*service.CreateAccountInput, string, error) {
	refreshToken := openAIAuthImportString(payload, "refresh_token")
	if refreshToken == "" {
		return nil, "", errors.New("refresh_token is required")
	}

	idToken := openAIAuthImportString(payload, "id_token")
	accessToken := openAIAuthImportString(payload, "access_token")
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

	accountName := buildOpenAIAuthImportAccountName(dataAccount.Credentials, index)

	return &service.CreateAccountInput{
		Name:                 accountName,
		Platform:             service.PlatformOpenAI,
		Type:                 service.AccountTypeOAuth,
		Credentials:          dataAccount.Credentials,
		Extra:                map[string]any{"import_source": openAIAuthImportSource},
		Concurrency:          openAIAuthImportDefaultConcurrency,
		Priority:             openAIAuthImportDefaultPriority,
		SkipDefaultGroupBind: true,
	}, accountName, nil
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

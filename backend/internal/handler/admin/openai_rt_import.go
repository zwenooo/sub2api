package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	openAIRTImportSource           = "api_refresh_token"
	openAIRTImportNameModeEmail    = "email"
	openAIRTImportNameModeIndex    = "index"
	openAIRTImportNameModeTemplate = "template"
)

type OpenAIRTImportResult struct {
	AccountCreated int                       `json:"account_created"`
	AccountFailed  int                       `json:"account_failed"`
	AutoRefreshed  int                       `json:"auto_refreshed,omitempty"`
	Errors         []OpenAIAuthImportError   `json:"errors,omitempty"`
	Warnings       []OpenAIAuthImportWarning `json:"warnings,omitempty"`
}

type OpenAIAuthImportWarning struct {
	Index   int    `json:"index"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

type openAIRTImportRequest struct {
	Items                 []json.RawMessage `json:"items"`
	Notes                 *string           `json:"notes"`
	GroupIDs              []int64           `json:"group_ids"`
	ProxyID               *int64            `json:"proxy_id"`
	Concurrency           int               `json:"concurrency"`
	Priority              int               `json:"priority"`
	RateMultiplier        *float64          `json:"rate_multiplier"`
	LoadFactor            *int              `json:"load_factor"`
	ExpiresAt             *int64            `json:"expires_at"`
	AutoPauseOnExpired    *bool             `json:"auto_pause_on_expired"`
	OpenAIPassthrough     *bool             `json:"openai_passthrough"`
	OpenAIWSMode          string            `json:"openai_ws_mode"`
	SkipDefaultGroupBind  *bool             `json:"skip_default_group_bind"`
	AutoRefreshAfterImport bool             `json:"auto_refresh_after_import"`
	NameMode              string            `json:"name_mode"`
	NamePrefix            string            `json:"name_prefix"`
	NameSuffix            string            `json:"name_suffix"`
	NameStartIndex        int               `json:"name_start_index"`
	NameTemplate          string            `json:"name_template"`
}

type openAIRTImportItem struct {
	RefreshToken string
	ClientID     string
	Name         string
	Notes        *string
	GroupIDs     []int64
	ProxyID      *int64
}

type openAIRTImportPlan struct {
	Items  []openAIRTImportItem
	Config openAIRTImportConfig
}

type openAIRTImportConfig struct {
	Notes                  *string
	GroupIDs               []int64
	ProxyID                *int64
	Concurrency            int
	Priority               int
	RateMultiplier         *float64
	LoadFactor             *int
	ExpiresAt              *int64
	AutoPauseOnExpired     bool
	OpenAIPassthrough      bool
	OpenAIWSMode           string
	SkipDefaultGroupBind   bool
	AutoRefreshAfterImport bool
	NameMode               string
	NamePrefix             string
	NameSuffix             string
	NameStartIndex         int
	NameTemplate           string
}

// ImportOpenAIRTAccounts imports OpenAI OAuth accounts from refresh tokens.
// POST /api/v1/admin/accounts/openai-rt-import
func (h *AccountHandler) ImportOpenAIRTAccounts(c *gin.Context) {
	if h.openaiOAuthService == nil {
		response.InternalError(c, "OpenAI OAuth service not available")
		return
	}

	raw, err := readOpenAIAuthImportPayload(c.Request.Body)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	plan, err := decodeOpenAIRTImportRequest(raw)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_openai_rt", map[string]any{
		"payload": string(raw),
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importOpenAIRTItems(ctx, plan)
	})
}

func decodeOpenAIRTImportRequest(raw []byte) (*openAIRTImportPlan, error) {
	var req openAIRTImportRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("payload must be a JSON object: %w", err)
	}
	if len(req.Items) == 0 {
		return nil, errors.New("items must contain at least one refresh token")
	}

	config, err := normalizeOpenAIRTImportConfig(&req)
	if err != nil {
		return nil, err
	}

	items := make([]openAIRTImportItem, 0, len(req.Items))
	for i, rawItem := range req.Items {
		item, err := decodeOpenAIRTImportItem(rawItem)
		if err != nil {
			return nil, fmt.Errorf("items[%d]: %w", i, err)
		}
		items = append(items, item)
	}

	return &openAIRTImportPlan{
		Items:  items,
		Config: config,
	}, nil
}

func normalizeOpenAIRTImportConfig(req *openAIRTImportRequest) (openAIRTImportConfig, error) {
	config := openAIRTImportConfig{
		Notes:                  trimStringPtr(req.Notes),
		GroupIDs:               normalizeOpenAIRTImportGroupIDs(req.GroupIDs),
		ProxyID:                req.ProxyID,
		Concurrency:            openAIAuthImportDefaultConcurrency,
		Priority:               openAIAuthImportDefaultPriority,
		ExpiresAt:              req.ExpiresAt,
		AutoPauseOnExpired:     true,
		OpenAIPassthrough:      true,
		OpenAIWSMode:           service.OpenAIWSIngressModePassthrough,
		SkipDefaultGroupBind:   true,
		AutoRefreshAfterImport: req.AutoRefreshAfterImport,
		NameMode:               openAIRTImportNameModeEmail,
		NamePrefix:             strings.TrimSpace(req.NamePrefix),
		NameSuffix:             strings.TrimSpace(req.NameSuffix),
		NameStartIndex:         1,
		NameTemplate:           strings.TrimSpace(req.NameTemplate),
	}

	if req.Concurrency < 0 {
		return config, errors.New("concurrency must be >= 0")
	}
	if req.Concurrency > 0 {
		config.Concurrency = req.Concurrency
	}

	if req.Priority < 0 {
		return config, errors.New("priority must be >= 0")
	}
	if req.Priority > 0 {
		config.Priority = req.Priority
	}

	if req.RateMultiplier != nil {
		if *req.RateMultiplier < 0 {
			return config, errors.New("rate_multiplier must be >= 0")
		}
		config.RateMultiplier = req.RateMultiplier
	}

	if req.LoadFactor != nil {
		if *req.LoadFactor < 0 {
			return config, errors.New("load_factor must be >= 0")
		}
		config.LoadFactor = req.LoadFactor
	}

	if req.AutoPauseOnExpired != nil {
		config.AutoPauseOnExpired = *req.AutoPauseOnExpired
	}
	if req.OpenAIPassthrough != nil {
		config.OpenAIPassthrough = *req.OpenAIPassthrough
	}
	if req.SkipDefaultGroupBind != nil {
		config.SkipDefaultGroupBind = *req.SkipDefaultGroupBind
	}

	if normalized := normalizeOpenAIRTImportNameMode(req.NameMode); normalized != "" {
		config.NameMode = normalized
	} else if strings.TrimSpace(req.NameMode) != "" {
		return config, fmt.Errorf("invalid name_mode %q, allowed values: email, index, template", strings.TrimSpace(req.NameMode))
	}

	if req.NameStartIndex < 0 {
		return config, errors.New("name_start_index must be >= 0")
	}
	if req.NameStartIndex > 0 {
		config.NameStartIndex = req.NameStartIndex
	}

	if config.NameMode == openAIRTImportNameModeTemplate && config.NameTemplate == "" {
		return config, errors.New("name_template is required when name_mode=template")
	}

	if normalized := normalizeOpenAIRTImportWSMode(req.OpenAIWSMode); normalized != "" {
		config.OpenAIWSMode = normalized
	} else if strings.TrimSpace(req.OpenAIWSMode) != "" {
		return config, fmt.Errorf("invalid openai_ws_mode %q, allowed values: off, ctx_pool, passthrough, shared, dedicated", strings.TrimSpace(req.OpenAIWSMode))
	}

	return config, nil
}

func decodeOpenAIRTImportItem(raw json.RawMessage) (openAIRTImportItem, error) {
	type itemPayload struct {
		RefreshToken string  `json:"refresh_token"`
		RT           string  `json:"rt"`
		ClientID     string  `json:"client_id"`
		Name         string  `json:"name"`
		Notes        *string `json:"notes"`
		GroupIDs     []int64 `json:"group_ids"`
		ProxyID      *int64  `json:"proxy_id"`
	}

	var simple string
	if err := json.Unmarshal(raw, &simple); err == nil {
		refreshToken := strings.TrimSpace(simple)
		if refreshToken == "" {
			return openAIRTImportItem{}, errors.New("refresh token string cannot be empty")
		}
		return openAIRTImportItem{RefreshToken: refreshToken}, nil
	}

	var payload itemPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return openAIRTImportItem{}, fmt.Errorf("item must be a string or object: %w", err)
	}

	refreshToken := strings.TrimSpace(payload.RefreshToken)
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(payload.RT)
	}
	if refreshToken == "" {
		return openAIRTImportItem{}, errors.New("refresh_token is required")
	}

	return openAIRTImportItem{
		RefreshToken: refreshToken,
		ClientID:     strings.TrimSpace(payload.ClientID),
		Name:         strings.TrimSpace(payload.Name),
		Notes:        trimStringPtr(payload.Notes),
		GroupIDs:     normalizeOpenAIRTImportGroupIDs(payload.GroupIDs),
		ProxyID:      payload.ProxyID,
	}, nil
}

func (h *AccountHandler) importOpenAIRTItems(ctx context.Context, plan *openAIRTImportPlan) (OpenAIRTImportResult, error) {
	result := OpenAIRTImportResult{}
	if plan == nil {
		return result, errors.New("import plan is required")
	}

	for i, item := range plan.Items {
		displayName := item.Name
		if displayName == "" {
			displayName = fmt.Sprintf("item-%d", i+1)
		}
		proxyID := resolveOpenAIRTImportProxyID(item, plan.Config)
		tokenInfo, err := h.openaiOAuthService.RefreshTokenByProxyID(ctx, item.RefreshToken, proxyID, item.ClientID)
		if err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, OpenAIAuthImportError{
				Index:   i + 1,
				Name:    displayName,
				Message: err.Error(),
			})
			continue
		}

		credentials := h.openaiOAuthService.BuildAccountCredentials(tokenInfo)
		if strings.TrimSpace(openAIAuthImportValueString(credentials["refresh_token"])) == "" {
			credentials["refresh_token"] = item.RefreshToken
		}
		if strings.TrimSpace(openAIAuthImportValueString(credentials["client_id"])) == "" && item.ClientID != "" {
			credentials["client_id"] = item.ClientID
		}

		accountName := buildOpenAIRTImportAccountName(credentials, plan.Config, item, i)
		input := &service.CreateAccountInput{
			Name:                 accountName,
			Notes:                resolveOpenAIRTImportNotes(item, plan.Config),
			Platform:             service.PlatformOpenAI,
			Type:                 service.AccountTypeOAuth,
			Credentials:          credentials,
			Extra:                buildOpenAIRTImportExtra(plan.Config),
			ProxyID:              proxyID,
			Concurrency:          plan.Config.Concurrency,
			Priority:             plan.Config.Priority,
			RateMultiplier:       plan.Config.RateMultiplier,
			LoadFactor:           plan.Config.LoadFactor,
			GroupIDs:             resolveOpenAIRTImportGroupIDs(item, plan.Config),
			ExpiresAt:            plan.Config.ExpiresAt,
			AutoPauseOnExpired:   boolPtr(plan.Config.AutoPauseOnExpired),
			SkipDefaultGroupBind: plan.Config.SkipDefaultGroupBind,
		}

		account, err := h.adminService.CreateAccount(ctx, input)
		if err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, OpenAIAuthImportError{
				Index:   i + 1,
				Name:    accountName,
				Message: err.Error(),
			})
			continue
		}

		result.AccountCreated++
		if account != nil {
			h.adminService.EnsureOpenAIPrivacy(ctx, account)
		}

		if !plan.Config.AutoRefreshAfterImport || account == nil {
			continue
		}

		_, warning, err := h.refreshSingleAccount(ctx, account)
		if err != nil {
			result.Warnings = append(result.Warnings, OpenAIAuthImportWarning{
				Index:   i + 1,
				Name:    accountName,
				Message: "auto refresh after import failed: " + err.Error(),
			})
			continue
		}
		result.AutoRefreshed++
		if warning != "" {
			result.Warnings = append(result.Warnings, OpenAIAuthImportWarning{
				Index:   i + 1,
				Name:    accountName,
				Message: warning,
			})
		}
	}

	return result, nil
}

func buildOpenAIRTImportExtra(config openAIRTImportConfig) map[string]any {
	return map[string]any{
		"import_source":                               openAIRTImportSource,
		"openai_passthrough":                          config.OpenAIPassthrough,
		"openai_oauth_responses_websockets_v2_mode":   config.OpenAIWSMode,
		"openai_oauth_responses_websockets_v2_enabled": config.OpenAIWSMode != service.OpenAIWSIngressModeOff,
	}
}

func buildOpenAIRTImportAccountName(credentials map[string]any, config openAIRTImportConfig, item openAIRTImportItem, index int) string {
	if item.Name != "" {
		return item.Name
	}

	switch config.NameMode {
	case openAIRTImportNameModeIndex:
		seq := config.NameStartIndex + index
		if config.NamePrefix == "" && config.NameSuffix == "" {
			return fmt.Sprintf("openai-rt-import-%d", seq)
		}
		return config.NamePrefix + strconv.Itoa(seq) + config.NameSuffix
	case openAIRTImportNameModeTemplate:
		if rendered := renderOpenAIRTImportNameTemplate(config.NameTemplate, credentials, config.NameStartIndex+index); rendered != "" {
			return rendered
		}
	}

	baseName := buildOpenAIAuthImportAccountName(credentials, index)
	if config.NamePrefix == "" && config.NameSuffix == "" {
		return baseName
	}
	return config.NamePrefix + baseName + config.NameSuffix
}

func renderOpenAIRTImportNameTemplate(template string, credentials map[string]any, seq int) string {
	accountID := openAIAuthImportValueString(credentials["chatgpt_account_id"])
	if accountID == "" {
		accountID = openAIAuthImportValueString(credentials["account_id"])
	}

	replacer := strings.NewReplacer(
		"{index}", strconv.Itoa(seq),
		"{email}", openAIAuthImportValueString(credentials["email"]),
		"{account_id}", accountID,
		"{chatgpt_account_id}", openAIAuthImportValueString(credentials["chatgpt_account_id"]),
		"{chatgpt_user_id}", openAIAuthImportValueString(credentials["chatgpt_user_id"]),
		"{organization_id}", openAIAuthImportValueString(credentials["organization_id"]),
		"{plan_type}", openAIAuthImportValueString(credentials["plan_type"]),
		"{client_id}", openAIAuthImportValueString(credentials["client_id"]),
	)

	return strings.TrimSpace(replacer.Replace(template))
}

func resolveOpenAIRTImportNotes(item openAIRTImportItem, config openAIRTImportConfig) *string {
	if item.Notes != nil {
		return item.Notes
	}
	return config.Notes
}

func resolveOpenAIRTImportGroupIDs(item openAIRTImportItem, config openAIRTImportConfig) []int64 {
	if len(item.GroupIDs) > 0 {
		return cloneOpenAIRTImportGroupIDs(item.GroupIDs)
	}
	return cloneOpenAIRTImportGroupIDs(config.GroupIDs)
}

func resolveOpenAIRTImportProxyID(item openAIRTImportItem, config openAIRTImportConfig) *int64 {
	if item.ProxyID != nil {
		return item.ProxyID
	}
	return config.ProxyID
}

func cloneOpenAIRTImportGroupIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	out := make([]int64, len(ids))
	copy(out, ids)
	return out
}

func normalizeOpenAIRTImportGroupIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeOpenAIRTImportNameMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", openAIRTImportNameModeEmail:
		return openAIRTImportNameModeEmail
	case openAIRTImportNameModeIndex:
		return openAIRTImportNameModeIndex
	case openAIRTImportNameModeTemplate:
		return openAIRTImportNameModeTemplate
	default:
		return ""
	}
}

func normalizeOpenAIRTImportWSMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "":
		return service.OpenAIWSIngressModePassthrough
	case service.OpenAIWSIngressModeOff:
		return service.OpenAIWSIngressModeOff
	case service.OpenAIWSIngressModeCtxPool:
		return service.OpenAIWSIngressModeCtxPool
	case service.OpenAIWSIngressModePassthrough:
		return service.OpenAIWSIngressModePassthrough
	case service.OpenAIWSIngressModeShared, service.OpenAIWSIngressModeDedicated:
		return service.OpenAIWSIngressModeCtxPool
	default:
		return ""
	}
}

func trimStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func boolPtr(value bool) *bool {
	return &value
}

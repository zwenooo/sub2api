package admin

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type AccountRuleHandler struct {
	accountRuleService *service.AccountRuleService
	opsService         *service.OpsService
	accountRepo        service.AccountRepository
}

func NewAccountRuleHandler(
	accountRuleService *service.AccountRuleService,
	opsService *service.OpsService,
	accountRepo service.AccountRepository,
) *AccountRuleHandler {
	return &AccountRuleHandler{
		accountRuleService: accountRuleService,
		opsService:         opsService,
		accountRepo:        accountRepo,
	}
}

type upsertAccountRuleScopeRequest struct {
	Platform    string   `json:"platform"`
	AccountType string   `json:"account_type"`
	Enabled     *bool    `json:"enabled"`
	ModelSet    []string `json:"model_set"`
	Description string   `json:"description"`
}

type updateAccountRuleSettingsRequest struct {
	ForwardMaxAttempts int `json:"forward_max_attempts"`
}

type upsertAccountRuleRequest struct {
	Name            string   `json:"name"`
	Enabled         *bool    `json:"enabled"`
	Priority        *int     `json:"priority"`
	StatusCodes     []int    `json:"status_codes"`
	Keywords        []string `json:"keywords"`
	MatchMode       string   `json:"match_mode"`
	ActionDisable   *bool    `json:"action_disable"`
	ActionFailover  *bool    `json:"action_failover"`
	ActionDelete    *bool    `json:"action_delete"`
	ActionOverride  *bool    `json:"action_override"`
	PassthroughCode *bool    `json:"passthrough_code"`
	ResponseCode    *int     `json:"response_code"`
	PassthroughBody *bool    `json:"passthrough_body"`
	CustomMessage   *string  `json:"custom_message"`
	SkipMonitoring  *bool    `json:"skip_monitoring"`
	Description     string   `json:"description"`
	SampleResponse  string   `json:"sample_response"`
}

func (h *AccountRuleHandler) GetCatalog(c *gin.Context) {
	catalog, err := h.accountRuleService.ListCatalog(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, catalog)
}

func (h *AccountRuleHandler) CreateScope(c *gin.Context) {
	var req upsertAccountRuleScopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	scope := &service.AccountRuleScope{
		Platform:    req.Platform,
		AccountType: req.AccountType,
		Enabled:     true,
		ModelSet:    req.ModelSet,
		Description: req.Description,
	}
	if req.Enabled != nil {
		scope.Enabled = *req.Enabled
	}

	created, err := h.accountRuleService.CreateScope(c.Request.Context(), scope)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, created)
}

func (h *AccountRuleHandler) UpdateScope(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid scope id")
		return
	}

	var req upsertAccountRuleScopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	existing, err := h.accountRuleService.GetScopeByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if existing == nil {
		response.NotFound(c, "Scope not found")
		return
	}

	scope := &service.AccountRuleScope{
		ID:          existing.ID,
		Platform:    existing.Platform,
		AccountType: existing.AccountType,
		Enabled:     existing.Enabled,
		ModelSet:    req.ModelSet,
		Description: req.Description,
	}
	if req.Enabled != nil {
		scope.Enabled = *req.Enabled
	}
	if req.ModelSet == nil {
		scope.ModelSet = existing.ModelSet
	}
	if strings.TrimSpace(req.Description) == "" {
		scope.Description = existing.Description
	}

	updated, updateErr := h.accountRuleService.UpdateScope(c.Request.Context(), scope)
	if updateErr != nil {
		response.BadRequest(c, updateErr.Error())
		return
	}
	if updated == nil {
		response.NotFound(c, "Scope not found")
		return
	}
	response.Success(c, updated)
}

func (h *AccountRuleHandler) DeleteScope(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid scope id")
		return
	}
	if err := h.accountRuleService.DeleteScope(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *AccountRuleHandler) CreateRule(c *gin.Context) {
	scopeID, err := strconv.ParseInt(c.Param("scopeId"), 10, 64)
	if err != nil || scopeID <= 0 {
		response.BadRequest(c, "Invalid scope id")
		return
	}

	var req upsertAccountRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	rule := buildAccountRuleFromRequest(scopeID, req)
	created, createErr := h.accountRuleService.CreateRule(c.Request.Context(), rule)
	if createErr != nil {
		response.BadRequest(c, createErr.Error())
		return
	}
	response.Success(c, created)
}

func (h *AccountRuleHandler) UpdateRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid rule id")
		return
	}

	existing, err := h.accountRuleService.GetRuleByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if existing == nil {
		response.NotFound(c, "Rule not found")
		return
	}

	var req upsertAccountRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	rule := buildAccountRuleFromRequest(existing.ScopeID, req)
	rule.ID = existing.ID
	rule.ScopeID = existing.ScopeID
	if req.Name == "" {
		rule.Name = existing.Name
	}
	if req.Enabled == nil {
		rule.Enabled = existing.Enabled
	}
	if req.Priority == nil {
		rule.Priority = existing.Priority
	}
	if req.StatusCodes == nil {
		rule.StatusCodes = existing.StatusCodes
	}
	if req.Keywords == nil {
		rule.Keywords = existing.Keywords
	}
	if strings.TrimSpace(req.MatchMode) == "" {
		rule.MatchMode = existing.MatchMode
	}
	if req.ActionDisable == nil {
		rule.ActionDisable = existing.ActionDisable
	}
	if req.ActionFailover == nil {
		rule.ActionFailover = existing.ActionFailover
	}
	if req.ActionDelete == nil {
		rule.ActionDelete = existing.ActionDelete
	}
	if req.ActionOverride == nil {
		rule.ActionOverride = existing.ActionOverride
	}
	if req.PassthroughCode == nil {
		rule.PassthroughCode = existing.PassthroughCode
	}
	if req.ResponseCode == nil {
		rule.ResponseCode = existing.ResponseCode
	}
	if req.PassthroughBody == nil {
		rule.PassthroughBody = existing.PassthroughBody
	}
	if req.CustomMessage == nil {
		rule.CustomMessage = existing.CustomMessage
	}
	if req.SkipMonitoring == nil {
		rule.SkipMonitoring = existing.SkipMonitoring
	}
	if strings.TrimSpace(req.Description) == "" {
		rule.Description = existing.Description
	}
	if strings.TrimSpace(req.SampleResponse) == "" {
		rule.SampleResponse = existing.SampleResponse
	}

	updated, updateErr := h.accountRuleService.UpdateRule(c.Request.Context(), rule)
	if updateErr != nil {
		response.BadRequest(c, updateErr.Error())
		return
	}
	if updated == nil {
		response.NotFound(c, "Rule not found")
		return
	}
	response.Success(c, updated)
}

func (h *AccountRuleHandler) DeleteRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid rule id")
		return
	}
	if err := h.accountRuleService.DeleteRule(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *AccountRuleHandler) GetSettings(c *gin.Context) {
	settings, err := h.accountRuleService.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *AccountRuleHandler) UpdateSettings(c *gin.Context) {
	var req updateAccountRuleSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	settings, err := h.accountRuleService.UpdateSettings(c.Request.Context(), &service.AccountRuleSettings{
		ForwardMaxAttempts: req.ForwardMaxAttempts,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, settings)
}

func (h *AccountRuleHandler) GetOpsDraft(c *gin.Context) {
	source := strings.ToLower(strings.TrimSpace(c.Query("source")))
	id, err := strconv.ParseInt(strings.TrimSpace(c.Query("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid draft source id")
		return
	}
	if source != "request-error" && source != "upstream-error" {
		response.BadRequest(c, "Unsupported draft source")
		return
	}

	detail, err := h.opsService.GetErrorLogByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if detail == nil {
		response.NotFound(c, "Error log not found")
		return
	}

	draft, buildErr := h.buildDraftFromOpsDetail(c, detail, source)
	if buildErr != nil {
		response.BadRequest(c, buildErr.Error())
		return
	}
	response.Success(c, draft)
}

func buildAccountRuleFromRequest(scopeID int64, req upsertAccountRuleRequest) *service.AccountRuleErrorRule {
	rule := &service.AccountRuleErrorRule{
		ScopeID:         scopeID,
		Name:            req.Name,
		Enabled:         true,
		Priority:        100,
		StatusCodes:     req.StatusCodes,
		Keywords:        req.Keywords,
		MatchMode:       req.MatchMode,
		ActionFailover:  true,
		PassthroughCode: true,
		PassthroughBody: true,
		Description:     req.Description,
		SampleResponse:  req.SampleResponse,
		ResponseCode:    req.ResponseCode,
		CustomMessage:   req.CustomMessage,
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.ActionDisable != nil {
		rule.ActionDisable = *req.ActionDisable
	}
	if req.ActionFailover != nil {
		rule.ActionFailover = *req.ActionFailover
	}
	if req.ActionDelete != nil {
		rule.ActionDelete = *req.ActionDelete
	}
	if req.ActionOverride != nil {
		rule.ActionOverride = *req.ActionOverride
	}
	if req.PassthroughCode != nil {
		rule.PassthroughCode = *req.PassthroughCode
	}
	if req.PassthroughBody != nil {
		rule.PassthroughBody = *req.PassthroughBody
	}
	if req.SkipMonitoring != nil {
		rule.SkipMonitoring = *req.SkipMonitoring
	}
	return rule
}

func (h *AccountRuleHandler) buildDraftFromOpsDetail(c *gin.Context, detail *service.OpsErrorLogDetail, source string) (*service.AccountRuleDraft, error) {
	if detail == nil {
		return nil, fmt.Errorf("empty error detail")
	}

	platform := strings.TrimSpace(detail.Platform)
	accountType := ""
	var matchedScopeID *int64
	var accountID *int64
	accountName := strings.TrimSpace(detail.AccountName)

	if detail.AccountID != nil && *detail.AccountID > 0 {
		accountID = detail.AccountID
		if h.accountRepo != nil {
			account, err := h.accountRepo.GetByID(c.Request.Context(), *detail.AccountID)
			if err == nil && account != nil {
				platform = account.Platform
				accountType = account.Type
				if strings.TrimSpace(accountName) == "" {
					accountName = account.Name
				}
			}
		}
	}

	if h.accountRuleService != nil {
		scopeID, err := h.accountRuleService.FindScopeIDByKey(c.Request.Context(), platform, accountType)
		if err == nil && scopeID != nil {
			matchedScopeID = scopeID
		} else if err == nil {
			scopeID, fallbackErr := h.accountRuleService.FindScopeIDByKey(c.Request.Context(), platform, "")
			if fallbackErr == nil && scopeID != nil {
				matchedScopeID = scopeID
			}
		}
	}

	statusCode := detail.StatusCode
	if detail.UpstreamStatusCode != nil && *detail.UpstreamStatusCode > 0 {
		statusCode = *detail.UpstreamStatusCode
	}

	bestMessage := firstNonEmpty(
		detail.UpstreamErrorMessage,
		detail.Message,
	)
	bestSample := firstNonEmpty(
		detail.UpstreamErrorDetail,
		detail.ErrorBody,
		detail.UpstreamErrorMessage,
		detail.Message,
	)

	keywords := []string{}
	if trimmed := trimForDraft(bestMessage, 240); trimmed != "" {
		keywords = append(keywords, trimmed)
	}

	ruleName := fmt.Sprintf("%s %d", strings.ToUpper(platform), statusCode)
	if accountType != "" {
		ruleName = fmt.Sprintf("%s %d (%s)", strings.ToUpper(platform), statusCode, accountType)
	}
	if source == "request-error" {
		ruleName = ruleName + " request"
	}

	return &service.AccountRuleDraft{
		Platform:       platform,
		AccountType:    accountType,
		MatchedScopeID: matchedScopeID,
		AccountID:      accountID,
		AccountName:    accountName,
		Rule: &service.AccountRuleErrorRule{
			Name:            ruleName,
			Enabled:         true,
			Priority:        100,
			StatusCodes:     []int{statusCode},
			Keywords:        keywords,
			MatchMode:       service.AccountRuleMatchModeAny,
			ActionFailover:  true,
			ActionDisable:   false,
			ActionDelete:    false,
			ActionOverride:  false,
			PassthroughCode: true,
			PassthroughBody: true,
			Description:     fmt.Sprintf("Drafted from %s #%d", source, detail.ID),
			SampleResponse:  trimForDraft(bestSample, 4000),
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func trimForDraft(value string, maxLen int) string {
	trimmed := strings.TrimSpace(value)
	if maxLen > 0 && len(trimmed) > maxLen {
		return trimmed[:maxLen]
	}
	return trimmed
}

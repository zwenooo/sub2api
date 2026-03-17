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

type upsertAccountRuleBindingRequest struct {
	Platform          string `json:"platform"`
	BusinessType      string `json:"business_type"`
	Enabled           *bool  `json:"enabled"`
	ModelCollectionID *int64 `json:"model_collection_id"`
	ErrorCollectionID *int64 `json:"error_collection_id"`
	Description       string `json:"description"`
}

type upsertAccountRuleModelCollectionRequest struct {
	Name        string   `json:"name"`
	Models      []string `json:"models"`
	Description string   `json:"description"`
}

type upsertAccountRuleErrorCollectionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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

func (h *AccountRuleHandler) CreateBinding(c *gin.Context) {
	var req upsertAccountRuleBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	binding := &service.AccountRuleBinding{
		Platform:          req.Platform,
		BusinessType:      req.BusinessType,
		Enabled:           true,
		ModelCollectionID: req.ModelCollectionID,
		ErrorCollectionID: req.ErrorCollectionID,
		Description:       req.Description,
	}
	if req.Enabled != nil {
		binding.Enabled = *req.Enabled
	}

	created, err := h.accountRuleService.CreateBinding(c.Request.Context(), binding)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, created)
}

func (h *AccountRuleHandler) UpdateBinding(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid binding id")
		return
	}

	var req upsertAccountRuleBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	existing, err := h.accountRuleService.GetBindingByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if existing == nil {
		response.NotFound(c, "Binding not found")
		return
	}

	binding := &service.AccountRuleBinding{
		ID:                existing.ID,
		Platform:          existing.Platform,
		BusinessType:      existing.BusinessType,
		Enabled:           existing.Enabled,
		ModelCollectionID: req.ModelCollectionID,
		ErrorCollectionID: req.ErrorCollectionID,
		Description:       req.Description,
	}
	if req.Enabled != nil {
		binding.Enabled = *req.Enabled
	}
	if strings.TrimSpace(req.Platform) != "" {
		binding.Platform = req.Platform
	}
	binding.BusinessType = req.BusinessType

	updated, updateErr := h.accountRuleService.UpdateBinding(c.Request.Context(), binding)
	if updateErr != nil {
		response.BadRequest(c, updateErr.Error())
		return
	}
	if updated == nil {
		response.NotFound(c, "Binding not found")
		return
	}
	response.Success(c, updated)
}

func (h *AccountRuleHandler) DeleteBinding(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid binding id")
		return
	}
	if err := h.accountRuleService.DeleteBinding(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *AccountRuleHandler) CreateModelCollection(c *gin.Context) {
	var req upsertAccountRuleModelCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	collection := &service.AccountRuleModelCollection{
		Name:        req.Name,
		Models:      req.Models,
		Description: req.Description,
	}
	created, err := h.accountRuleService.CreateModelCollection(c.Request.Context(), collection)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, created)
}

func (h *AccountRuleHandler) UpdateModelCollection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid model collection id")
		return
	}

	var req upsertAccountRuleModelCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	existing, err := h.accountRuleService.GetModelCollectionByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if existing == nil {
		response.NotFound(c, "Model collection not found")
		return
	}

	collection := &service.AccountRuleModelCollection{
		ID:          existing.ID,
		Name:        req.Name,
		Models:      req.Models,
		Description: req.Description,
	}
	if strings.TrimSpace(collection.Name) == "" {
		collection.Name = existing.Name
	}
	if collection.Models == nil {
		collection.Models = existing.Models
	}

	updated, updateErr := h.accountRuleService.UpdateModelCollection(c.Request.Context(), collection)
	if updateErr != nil {
		response.BadRequest(c, updateErr.Error())
		return
	}
	if updated == nil {
		response.NotFound(c, "Model collection not found")
		return
	}
	response.Success(c, updated)
}

func (h *AccountRuleHandler) DeleteModelCollection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid model collection id")
		return
	}
	if err := h.accountRuleService.DeleteModelCollection(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *AccountRuleHandler) CreateErrorCollection(c *gin.Context) {
	var req upsertAccountRuleErrorCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	collection := &service.AccountRuleErrorCollection{
		Name:        req.Name,
		Description: req.Description,
	}
	created, err := h.accountRuleService.CreateErrorCollection(c.Request.Context(), collection)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, created)
}

func (h *AccountRuleHandler) UpdateErrorCollection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid error collection id")
		return
	}

	var req upsertAccountRuleErrorCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	existing, err := h.accountRuleService.GetErrorCollectionByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if existing == nil {
		response.NotFound(c, "Error collection not found")
		return
	}

	collection := &service.AccountRuleErrorCollection{
		ID:          existing.ID,
		Name:        req.Name,
		Description: req.Description,
		Rules:       existing.Rules,
	}
	if strings.TrimSpace(collection.Name) == "" {
		collection.Name = existing.Name
	}

	updated, updateErr := h.accountRuleService.UpdateErrorCollection(c.Request.Context(), collection)
	if updateErr != nil {
		response.BadRequest(c, updateErr.Error())
		return
	}
	if updated == nil {
		response.NotFound(c, "Error collection not found")
		return
	}
	response.Success(c, updated)
}

func (h *AccountRuleHandler) DeleteErrorCollection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid error collection id")
		return
	}
	if err := h.accountRuleService.DeleteErrorCollection(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *AccountRuleHandler) CreateRule(c *gin.Context) {
	errorCollectionID, err := strconv.ParseInt(c.Param("collectionId"), 10, 64)
	if err != nil || errorCollectionID <= 0 {
		response.BadRequest(c, "Invalid error collection id")
		return
	}

	var req upsertAccountRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	rule := buildAccountRuleFromRequest(errorCollectionID, req)
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

	rule := buildAccountRuleFromRequest(existing.ErrorCollectionID, req)
	rule.ID = existing.ID
	rule.ErrorCollectionID = existing.ErrorCollectionID
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
	if req.PassthroughCode != nil && *req.PassthroughCode {
		rule.ResponseCode = nil
	} else if req.ResponseCode == nil {
		rule.ResponseCode = existing.ResponseCode
	}
	if req.PassthroughBody == nil {
		rule.PassthroughBody = existing.PassthroughBody
	}
	if req.PassthroughBody != nil && *req.PassthroughBody {
		rule.CustomMessage = nil
	} else if req.CustomMessage == nil {
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

func buildAccountRuleFromRequest(errorCollectionID int64, req upsertAccountRuleRequest) *service.AccountRuleErrorRule {
	rule := &service.AccountRuleErrorRule{
		ErrorCollectionID: errorCollectionID,
		Name:              req.Name,
		Enabled:           true,
		Priority:          100,
		StatusCodes:       req.StatusCodes,
		Keywords:          req.Keywords,
		MatchMode:         req.MatchMode,
		ActionFailover:    true,
		PassthroughCode:   true,
		PassthroughBody:   true,
		Description:       req.Description,
		SampleResponse:    req.SampleResponse,
		ResponseCode:      req.ResponseCode,
		CustomMessage:     req.CustomMessage,
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
	businessType := ""
	var matchedBindingID *int64
	var matchedErrorCollectionID *int64
	var accountID *int64
	accountName := strings.TrimSpace(detail.AccountName)

	if detail.AccountID != nil && *detail.AccountID > 0 {
		accountID = detail.AccountID
		if h.accountRepo != nil {
			account, err := h.accountRepo.GetByID(c.Request.Context(), *detail.AccountID)
			if err == nil && account != nil {
				platform = account.Platform
				businessType = account.AccountRuleScopeType()
				if strings.TrimSpace(accountName) == "" {
					accountName = account.Name
				}
			}
		}
	}

	if h.accountRuleService != nil {
		binding, err := h.accountRuleService.FindEffectiveBinding(c.Request.Context(), platform, businessType)
		if err == nil && binding != nil {
			id := binding.ID
			matchedBindingID = &id
			matchedErrorCollectionID = binding.ErrorCollectionID
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
	if businessType != "" {
		ruleName = fmt.Sprintf("%s %d (%s)", strings.ToUpper(platform), statusCode, businessType)
	}
	if source == "request-error" {
		ruleName = ruleName + " request"
	}

	return &service.AccountRuleDraft{
		Platform:                 platform,
		BusinessType:             businessType,
		MatchedBindingID:         matchedBindingID,
		MatchedErrorCollectionID: matchedErrorCollectionID,
		AccountID:                accountID,
		AccountName:              accountName,
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

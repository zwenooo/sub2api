package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func resolveAccountRuleFailoverResponse(
	c *gin.Context,
	accountRuleService *service.AccountRuleService,
	platform string,
	statusCode int,
	responseBody []byte,
	defaultMessage string,
) (resolvedStatus int, errType string, errMessage string, matched bool) {
	if c == nil || accountRuleService == nil || strings.TrimSpace(platform) == "" {
		return 0, "", "", false
	}

	scopeType := ""
	if c.Request != nil {
		if v, ok := c.Request.Context().Value(ctxkey.AccountScopeType).(string); ok {
			scopeType = strings.TrimSpace(v)
		}
	}

	match := accountRuleService.MatchRuntimeRule(&service.Account{
		Platform:      platform,
		RuleScopeType: scopeType,
	}, statusCode, responseBody)
	if match == nil || match.Rule == nil || !match.Rule.ActionOverride {
		return 0, "", "", false
	}

	resolvedStatus = statusCode
	if !match.Rule.PassthroughCode && match.Rule.ResponseCode != nil {
		resolvedStatus = *match.Rule.ResponseCode
	}

	errMessage = strings.TrimSpace(service.ExtractUpstreamErrorMessage(responseBody))
	if !match.Rule.PassthroughBody && match.Rule.CustomMessage != nil {
		errMessage = strings.TrimSpace(*match.Rule.CustomMessage)
	}
	if errMessage == "" {
		errMessage = defaultMessage
	}
	if match.Rule.SkipMonitoring {
		c.Set(service.OpsSkipPassthroughKey, true)
	}

	return resolvedStatus, "upstream_error", errMessage, true
}

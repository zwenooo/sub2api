package service

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

const accountRuleServiceContextKey = "account_rule_service"

func BindAccountRuleService(c *gin.Context, svc *AccountRuleService) {
	if c == nil || svc == nil {
		return
	}
	c.Set(accountRuleServiceContextKey, svc)
}

func getBoundAccountRuleService(c *gin.Context) *AccountRuleService {
	if c == nil {
		return nil
	}
	v, ok := c.Get(accountRuleServiceContextKey)
	if !ok {
		return nil
	}
	svc, ok := v.(*AccountRuleService)
	if !ok {
		return nil
	}
	return svc
}

func applyBoundAccountRule(ctx context.Context, c *gin.Context, account *Account, statusCode int, headers http.Header, responseBody []byte) AccountRuleActionResult {
	result := AccountRuleActionResult{}
	svc := getBoundAccountRuleService(c)
	if svc == nil || account == nil {
		return result
	}
	match := svc.MatchRuntimeRule(account, statusCode, responseBody)
	if match == nil {
		return result
	}
	result = svc.ApplyMatchedRule(ctx, account, match, statusCode, headers, responseBody)
	if result.SkipMonitoring && c != nil {
		c.Set(OpsSkipPassthroughKey, true)
	}
	return result
}

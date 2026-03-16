package service

import "github.com/gin-gonic/gin"

const errorPassthroughServiceContextKey = "error_passthrough_service"

// BindErrorPassthroughService 将错误透传服务绑定到请求上下文，供 service 层在非 failover 场景下复用规则。
func BindErrorPassthroughService(c *gin.Context, svc *ErrorPassthroughService) {
	if c == nil || svc == nil {
		return
	}
	c.Set(errorPassthroughServiceContextKey, svc)
}

func getBoundErrorPassthroughService(c *gin.Context) *ErrorPassthroughService {
	if c == nil {
		return nil
	}
	v, ok := c.Get(errorPassthroughServiceContextKey)
	if !ok {
		return nil
	}
	svc, ok := v.(*ErrorPassthroughService)
	if !ok {
		return nil
	}
	return svc
}

// applyErrorPassthroughRule 按规则改写错误响应；未命中时返回默认响应参数。
func applyErrorPassthroughRule(
	c *gin.Context,
	account *Account,
	upstreamStatus int,
	responseBody []byte,
	defaultStatus int,
	defaultErrType string,
	defaultErrMsg string,
) (status int, errType string, errMsg string, matched bool) {
	status = defaultStatus
	errType = defaultErrType
	errMsg = defaultErrMsg

	if scopedSvc := getBoundAccountRuleService(c); scopedSvc != nil && account != nil {
		match := scopedSvc.MatchRuntimeRule(account, upstreamStatus, responseBody)
		if match != nil && match.Rule != nil && match.Rule.ActionOverride {
			status = upstreamStatus
			if !match.Rule.PassthroughCode && match.Rule.ResponseCode != nil {
				status = *match.Rule.ResponseCode
			}

			errMsg = ExtractUpstreamErrorMessage(responseBody)
			if !match.Rule.PassthroughBody && match.Rule.CustomMessage != nil {
				errMsg = *match.Rule.CustomMessage
			}
			if match.Rule.SkipMonitoring {
				c.Set(OpsSkipPassthroughKey, true)
			}
			errType = "upstream_error"
			return status, errType, errMsg, true
		}
	}

	svc := getBoundErrorPassthroughService(c)
	if svc == nil {
		return status, errType, errMsg, false
	}

	platform := ""
	if account != nil {
		platform = account.Platform
	}
	rule := svc.MatchRule(platform, upstreamStatus, responseBody)
	if rule == nil {
		return status, errType, errMsg, false
	}

	status = upstreamStatus
	if !rule.PassthroughCode && rule.ResponseCode != nil {
		status = *rule.ResponseCode
	}

	errMsg = ExtractUpstreamErrorMessage(responseBody)
	if !rule.PassthroughBody && rule.CustomMessage != nil {
		errMsg = *rule.CustomMessage
	}

	// 命中 skip_monitoring 时在 context 中标记，供 ops_error_logger 跳过记录。
	if rule.SkipMonitoring {
		c.Set(OpsSkipPassthroughKey, true)
	}

	// 与现有 failover 场景保持一致：命中规则时统一返回 upstream_error。
	errType = "upstream_error"
	return status, errType, errMsg, true
}

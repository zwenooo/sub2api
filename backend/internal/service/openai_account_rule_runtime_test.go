package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIAccountRuleActionRepoStub struct {
	stubOpenAIAccountRepo
	setErrorCalls int
	lastSetError  struct {
		id  int64
		msg string
	}
}

func (r *openAIAccountRuleActionRepoStub) SetError(_ context.Context, id int64, errorMsg string) error {
	r.setErrorCalls++
	r.lastSetError.id = id
	r.lastSetError.msg = errorMsg
	return nil
}

func newOpenAIRuntimeAccountRuleServiceForTest(
	accountRepo AccountRepository,
	statusCodes []int,
	keywords []string,
	actionFailover bool,
) *AccountRuleService {
	errorCollectionID := int64(1)
	rule := &AccountRuleErrorRule{
		ID:                1,
		ErrorCollectionID: errorCollectionID,
		Name:              "openai-runtime-rule",
		Enabled:           true,
		Priority:          1,
		StatusCodes:       append([]int(nil), statusCodes...),
		Keywords:          append([]string(nil), keywords...),
		MatchMode:         AccountRuleMatchModeAll,
		ActionDisable:     true,
		ActionFailover:    actionFailover,
	}
	rule.Normalize()

	cachedRule := &cachedAccountRule{
		rule:          rule,
		statusCodeSet: make(map[int]struct{}, len(rule.StatusCodes)),
		lowerKeywords: make([]string, 0, len(rule.Keywords)),
	}
	for _, code := range rule.StatusCodes {
		cachedRule.statusCodeSet[code] = struct{}{}
	}
	for _, keyword := range rule.Keywords {
		cachedRule.lowerKeywords = append(cachedRule.lowerKeywords, strings.ToLower(keyword))
	}

	binding := &AccountRuleBinding{
		ID:                1,
		Platform:          PlatformOpenAI,
		Enabled:           true,
		ErrorCollectionID: &errorCollectionID,
	}
	binding.Normalize()

	return &AccountRuleService{
		accountRepo: accountRepo,
		cache: &accountRuleCacheSnapshot{
			loadedAt: time.Now(),
			bindingsByKey: map[string]*cachedAccountRuleBinding{
				accountRuleBindingKey(PlatformOpenAI, ""): {
					binding: binding,
					rules:   []*cachedAccountRule{cachedRule},
				},
			},
		},
	}
}

func TestOpenAIGatewayService_HandleCompatUpstreamErrorWithFailover_AppliesRuleDisable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)

	account := Account{
		ID:          6101,
		Name:        "openai-compat-rule",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"pool_mode": true},
		Status:      StatusActive,
		Schedulable: true,
	}
	repo := &openAIAccountRuleActionRepoStub{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}},
	}
	ruleService := newOpenAIRuntimeAccountRuleServiceForTest(repo, []int{http.StatusTooManyRequests}, []string{"usage limit"}, true)
	svc := &OpenAIGatewayService{accountRuleService: ruleService}

	respBody := []byte(`{"error":{"type":"rate_limit_error","message":"The usage limit has been reached"}}`)
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"x-request-id": []string{"req_compat_rule"}},
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	nonFailoverCalls := 0
	_, err := svc.handleCompatUpstreamErrorWithFailover(
		context.Background(),
		resp,
		c,
		&account,
		respBody,
		func(_ *http.Response, _ *gin.Context, _ *Account) (*OpenAIForwardResult, error) {
			nonFailoverCalls++
			return nil, errors.New("unexpected non-failover handler call")
		},
	)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Zero(t, nonFailoverCalls)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, defaultAccountRuleForwardMaxAttempts, failoverErr.MaxSwitchesOverride)
	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, account.ID, repo.lastSetError.id)
	require.Contains(t, strings.ToLower(repo.lastSetError.msg), "usage limit")
}

func TestOpenAIGatewayService_HandleErrorResponse_RuleFailoverSkipsSameAccountRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)

	account := &Account{
		ID:          6103,
		Name:        "openai-http-rule",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"pool_mode": true},
		Status:      StatusActive,
		Schedulable: true,
	}
	repo := &openAIAccountRuleActionRepoStub{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{*account}},
	}
	svc := &OpenAIGatewayService{
		accountRuleService: newOpenAIRuntimeAccountRuleServiceForTest(
			repo,
			[]int{http.StatusTooManyRequests},
			[]string{"usage limit"},
			true,
		),
	}

	respBody := []byte(`{"error":{"type":"rate_limit_error","message":"The usage limit has been reached"}}`)
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"x-request-id": []string{"req_http_rule"}},
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	_, err := svc.handleErrorResponse(context.Background(), resp, c, account, nil)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, defaultAccountRuleForwardMaxAttempts, failoverErr.MaxSwitchesOverride)
	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, account.ID, repo.lastSetError.id)
	require.Contains(t, strings.ToLower(repo.lastSetError.msg), "usage limit")
}

func TestOpenAIGatewayService_HandleCompatErrorResponse_RuleFailoverSkipsSameAccountRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)

	account := &Account{
		ID:          6104,
		Name:        "openai-compat-direct-rule",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"pool_mode": true},
		Status:      StatusActive,
		Schedulable: true,
	}
	repo := &openAIAccountRuleActionRepoStub{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{*account}},
	}
	svc := &OpenAIGatewayService{
		accountRuleService: newOpenAIRuntimeAccountRuleServiceForTest(
			repo,
			[]int{http.StatusTooManyRequests},
			[]string{"usage limit"},
			true,
		),
	}

	respBody := []byte(`{"error":{"type":"rate_limit_error","message":"The usage limit has been reached"}}`)
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"x-request-id": []string{"req_compat_direct_rule"}},
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	writeCalls := 0
	_, err := svc.handleCompatErrorResponse(
		resp,
		c,
		account,
		func(_ *gin.Context, _ int, _, _ string) {
			writeCalls++
		},
	)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Zero(t, writeCalls)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, defaultAccountRuleForwardMaxAttempts, failoverErr.MaxSwitchesOverride)
	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, account.ID, repo.lastSetError.id)
	require.Contains(t, strings.ToLower(repo.lastSetError.msg), "usage limit")
}

func TestOpenAIGatewayService_PersistOpenAIWSErrorSignals_AppliesRuleDisable(t *testing.T) {
	account := Account{
		ID:          6102,
		Name:        "openai-ws-rule",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
	}
	repo := &openAIAccountRuleActionRepoStub{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}},
	}
	svc := &OpenAIGatewayService{
		accountRuleService: newOpenAIRuntimeAccountRuleServiceForTest(
			repo,
			[]int{http.StatusTooManyRequests},
			[]string{"usage limit"},
			false,
		),
	}

	message := []byte(`{"type":"error","error":{"code":"rate_limit_exceeded","type":"usage_limit_reached","message":"The usage limit has been reached"}}`)
	result := svc.persistOpenAIWSErrorSignals(
		context.Background(),
		nil,
		&account,
		http.Header{},
		message,
		"rate_limit_exceeded",
		"usage_limit_reached",
		"The usage limit has been reached",
	)

	require.True(t, result.Matched)
	require.False(t, result.ShouldFailover)
	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, account.ID, repo.lastSetError.id)
	require.Contains(t, strings.ToLower(repo.lastSetError.msg), "usage limit")
}

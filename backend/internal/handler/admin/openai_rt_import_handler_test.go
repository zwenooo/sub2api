package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIRTImportResponse struct {
	Code int                 `json:"code"`
	Data OpenAIRTImportResult `json:"data"`
}

type openAIRTImportOAuthClientStub struct {
	responses map[string]*openai.TokenResponse
	err       error
}

func (s *openAIRTImportOAuthClientStub) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*openai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *openAIRTImportOAuthClientStub) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*openai.TokenResponse, error) {
	return s.RefreshTokenWithClientID(ctx, refreshToken, proxyURL, "")
}

func (s *openAIRTImportOAuthClientStub) RefreshTokenWithClientID(ctx context.Context, refreshToken, proxyURL string, clientID string) (*openai.TokenResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	if resp, ok := s.responses[refreshToken]; ok && resp != nil {
		cloned := *resp
		return &cloned, nil
	}
	return &openai.TokenResponse{
		AccessToken: "at-" + refreshToken,
		ExpiresIn:   3600,
	}, nil
}

func setupOpenAIRTImportRouter(oauthClient service.OpenAIOAuthClient) (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()
	openaiSvc := service.NewOpenAIOAuthService(nil, oauthClient)

	h := NewAccountHandler(
		adminSvc,
		nil,
		openaiSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	router.POST("/api/v1/admin/accounts/openai-rt-import", h.ImportOpenAIRTAccounts)
	return router, adminSvc
}

func TestImportOpenAIRTAccountsWithBatchDefaults(t *testing.T) {
	router, adminSvc := setupOpenAIRTImportRouter(&openAIRTImportOAuthClientStub{
		responses: map[string]*openai.TokenResponse{
			"rt-1": {AccessToken: "at-1", ExpiresIn: 3600},
			"rt-2": {AccessToken: "at-2", ExpiresIn: 3600},
		},
	})

	payload := map[string]any{
		"items":            []any{"rt-1", "rt-2"},
		"group_ids":        []int64{11, 12},
		"notes":            "batch note",
		"name_mode":        "index",
		"name_prefix":      "oai-rt-",
		"name_start_index": 10,
	}

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-rt-import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "rt-import-test-1")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp openAIRTImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 2, resp.Data.AccountCreated)
	require.Equal(t, 0, resp.Data.AccountFailed)

	require.Len(t, adminSvc.createdAccounts, 2)

	first := adminSvc.createdAccounts[0]
	require.Equal(t, "oai-rt-10", first.Name)
	require.Equal(t, service.PlatformOpenAI, first.Platform)
	require.Equal(t, service.AccountTypeOAuth, first.Type)
	require.True(t, first.SkipDefaultGroupBind)
	require.Equal(t, []int64{11, 12}, first.GroupIDs)
	require.NotNil(t, first.Notes)
	require.Equal(t, "batch note", *first.Notes)
	require.NotNil(t, first.AutoPauseOnExpired)
	require.True(t, *first.AutoPauseOnExpired)
	require.Equal(t, "at-1", first.Credentials["access_token"])
	require.Equal(t, "rt-1", first.Credentials["refresh_token"])
	require.Equal(t, openAIRTImportSource, first.Extra["import_source"])
	require.Equal(t, true, first.Extra["openai_passthrough"])
	require.Equal(t, service.OpenAIWSIngressModePassthrough, first.Extra["openai_oauth_responses_websockets_v2_mode"])
	require.Equal(t, true, first.Extra["openai_oauth_responses_websockets_v2_enabled"])

	second := adminSvc.createdAccounts[1]
	require.Equal(t, "oai-rt-11", second.Name)
	require.Equal(t, "at-2", second.Credentials["access_token"])
	require.Equal(t, "rt-2", second.Credentials["refresh_token"])
}

func TestImportOpenAIRTAccountsWithPerItemOverrides(t *testing.T) {
	router, adminSvc := setupOpenAIRTImportRouter(&openAIRTImportOAuthClientStub{
		responses: map[string]*openai.TokenResponse{
			"rt-3": {AccessToken: "at-3", RefreshToken: "rt-3-new", ExpiresIn: 7200},
		},
	})

	payload := map[string]any{
		"items": []any{
			map[string]any{
				"rt":        "rt-3",
				"client_id": "client-3",
				"name":      "custom-name",
				"group_ids": []int64{21},
				"notes":     "item-note",
			},
		},
		"group_ids":               []int64{11, 12},
		"notes":                   "batch note",
		"concurrency":             5,
		"priority":                80,
		"openai_passthrough":      false,
		"openai_ws_mode":          "off",
		"auto_pause_on_expired":   false,
		"skip_default_group_bind": false,
	}

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-rt-import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "rt-import-test-2")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp openAIRTImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 1, resp.Data.AccountCreated)
	require.Equal(t, 0, resp.Data.AccountFailed)

	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.Equal(t, "custom-name", created.Name)
	require.Equal(t, 5, created.Concurrency)
	require.Equal(t, 80, created.Priority)
	require.False(t, created.SkipDefaultGroupBind)
	require.NotNil(t, created.AutoPauseOnExpired)
	require.False(t, *created.AutoPauseOnExpired)
	require.NotNil(t, created.Notes)
	require.Equal(t, "item-note", *created.Notes)
	require.Equal(t, []int64{21}, created.GroupIDs)
	require.Equal(t, "at-3", created.Credentials["access_token"])
	require.Equal(t, "rt-3-new", created.Credentials["refresh_token"])
	require.Equal(t, "client-3", created.Credentials["client_id"])
	require.Equal(t, false, created.Extra["openai_passthrough"])
	require.Equal(t, service.OpenAIWSIngressModeOff, created.Extra["openai_oauth_responses_websockets_v2_mode"])
	require.Equal(t, false, created.Extra["openai_oauth_responses_websockets_v2_enabled"])
}

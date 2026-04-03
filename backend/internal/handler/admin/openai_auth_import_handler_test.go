package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIAuthImportResponse struct {
	Code int                    `json:"code"`
	Data OpenAIAuthImportResult `json:"data"`
}

type openAIAuthImportOAuthClientStub struct {
	responses    map[string]*openai.TokenResponse
	err          error
	refreshCalls int
	lastClientID string
}

func (s *openAIAuthImportOAuthClientStub) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*openai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *openAIAuthImportOAuthClientStub) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*openai.TokenResponse, error) {
	return s.RefreshTokenWithClientID(ctx, refreshToken, proxyURL, "")
}

func (s *openAIAuthImportOAuthClientStub) RefreshTokenWithClientID(ctx context.Context, refreshToken, proxyURL string, clientID string) (*openai.TokenResponse, error) {
	s.refreshCalls++
	s.lastClientID = clientID
	if s.err != nil {
		return nil, s.err
	}
	if resp, ok := s.responses[refreshToken]; ok && resp != nil {
		cloned := *resp
		return &cloned, nil
	}
	return &openai.TokenResponse{
		AccessToken: "at-" + refreshToken,
		IDToken:     buildOpenAIAuthImportIDTokenForTest(nil, "acct-"+refreshToken, "free", refreshToken+"@example.com"),
		ExpiresIn:   3600,
	}, nil
}

func setupOpenAIAuthImportRouter(oauthClient service.OpenAIOAuthClient) (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()
	var openaiSvc *service.OpenAIOAuthService
	if oauthClient != nil {
		openaiSvc = service.NewOpenAIOAuthService(nil, oauthClient)
	}

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

	router.POST("/api/v1/admin/accounts/openai-auths/import", h.ImportOpenAIAuthJSON)
	router.POST("/api/v1/admin/accounts/openai-auths/import-file", h.ImportOpenAIAuthFile)
	return router, adminSvc
}

func TestImportOpenAIAuthJSONCreatesOpenAIOAuthAccount(t *testing.T) {
	router, adminSvc := setupOpenAIAuthImportRouter(nil)

	body := []map[string]any{
		{
			"tokens": map[string]any{
				"access_token":  "at-1",
				"refresh_token": "rt-1",
				"id_token":      "header.payload.signature",
				"account_id":    "acct-1",
			},
			"client_id": "client-1",
			"email":     "user@example.com",
		},
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp openAIAuthImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 1, resp.Data.AccountCreated)
	require.Equal(t, 0, resp.Data.AccountFailed)

	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.Equal(t, service.PlatformOpenAI, created.Platform)
	require.Equal(t, service.AccountTypeOAuth, created.Type)
	require.True(t, created.SkipDefaultGroupBind)
	require.Equal(t, "user@example.com", created.Name)
	require.Equal(t, "auth_json", created.Extra["import_source"])
	require.Equal(t, "at-1", created.Credentials["access_token"])
	require.Equal(t, "rt-1", created.Credentials["refresh_token"])
	require.Equal(t, "client-1", created.Credentials["client_id"])
	require.Equal(t, "acct-1", created.Credentials["chatgpt_account_id"])
}

func TestImportOpenAIAuthFileUsesIDTokenAsAccessTokenFallback(t *testing.T) {
	router, adminSvc := setupOpenAIAuthImportRouter(nil)

	payload := `[{"tokens":{"refresh_token":"rt-2","id_token":"jwt-2","account_id":"acct-2"}}]`

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "auth.json")
	require.NoError(t, err)
	_, err = part.Write([]byte(payload))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import-file", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp openAIAuthImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 1, resp.Data.AccountCreated)
	require.Equal(t, 0, resp.Data.AccountFailed)

	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.Equal(t, "jwt-2", created.Credentials["access_token"])
	require.Equal(t, "jwt-2", created.Credentials["id_token"])
	require.Equal(t, "acct-2", created.Credentials["chatgpt_account_id"])
	require.Equal(t, "openai-acct-2", created.Name)
}

func TestImportOpenAIAuthJSONSupportsGroupIDsAndNameTemplate(t *testing.T) {
	router, adminSvc := setupOpenAIAuthImportRouter(nil)

	body := map[string]any{
		"items": []map[string]any{
			{
				"tokens": map[string]any{
					"access_token":  "at-3",
					"refresh_token": "rt-3",
					"account_id":    "acct-3",
				},
				"email":     "plus@example.com",
				"plan_type": "plus",
				"client_id": "client-3",
			},
		},
		"group_ids":     []int64{11, 12},
		"name_template": "{plan_type}-{email}",
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp openAIAuthImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 1, resp.Data.AccountCreated)
	require.Equal(t, 0, resp.Data.AccountFailed)

	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.Equal(t, "plus-plus@example.com", created.Name)
	require.Equal(t, []int64{11, 12}, created.GroupIDs)
	require.True(t, created.SkipDefaultGroupBind)
}

func TestImportOpenAIAuthFileSupportsGroupIDsAndNameTemplate(t *testing.T) {
	router, adminSvc := setupOpenAIAuthImportRouter(nil)

	payload := `[{"tokens":{"refresh_token":"rt-4","access_token":"at-4","account_id":"acct-4"},"email":"pro@example.com","plan_type":"pro"}]`

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "auth.json")
	require.NoError(t, err)
	_, err = part.Write([]byte(payload))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("group_ids", "[21]"))
	require.NoError(t, writer.WriteField("name_template", "{index}-{account_id}"))
	require.NoError(t, writer.Close())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import-file", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp openAIAuthImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 1, resp.Data.AccountCreated)
	require.Equal(t, 0, resp.Data.AccountFailed)

	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.Equal(t, "1-acct-4", created.Name)
	require.Equal(t, []int64{21}, created.GroupIDs)
}

func TestImportOpenAIAuthJSONSupportsAccountOptions(t *testing.T) {
	router, adminSvc := setupOpenAIAuthImportRouter(nil)

	body := map[string]any{
		"items": []map[string]any{
			{
				"tokens": map[string]any{
					"access_token":  "at-opts",
					"refresh_token": "rt-opts",
					"account_id":    "acct-opts",
				},
			},
		},
		"proxy_id":              9,
		"auto_pause_on_expired": false,
		"openai_passthrough":    true,
		"openai_ws_mode":        "off",
		"codex_cli_only":        true,
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.NotNil(t, created.ProxyID)
	require.EqualValues(t, 9, *created.ProxyID)
	require.NotNil(t, created.AutoPauseOnExpired)
	require.False(t, *created.AutoPauseOnExpired)
	require.Equal(t, true, created.Extra["openai_passthrough"])
	require.Equal(t, service.OpenAIWSIngressModeOff, created.Extra["openai_oauth_responses_websockets_v2_mode"])
	require.Equal(t, false, created.Extra["openai_oauth_responses_websockets_v2_enabled"])
	require.Equal(t, true, created.Extra["codex_cli_only"])
}

func TestImportOpenAIAuthFileSupportsAccountOptions(t *testing.T) {
	router, adminSvc := setupOpenAIAuthImportRouter(nil)

	payload := `[{"tokens":{"refresh_token":"rt-file-opts","access_token":"at-file-opts","account_id":"acct-file-opts"}}]`

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "auth.json")
	require.NoError(t, err)
	_, err = part.Write([]byte(payload))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("proxy_id", "15"))
	require.NoError(t, writer.WriteField("auto_pause_on_expired", "true"))
	require.NoError(t, writer.WriteField("openai_passthrough", "false"))
	require.NoError(t, writer.WriteField("openai_ws_mode", "passthrough"))
	require.NoError(t, writer.WriteField("codex_cli_only", "true"))
	require.NoError(t, writer.Close())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import-file", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.NotNil(t, created.ProxyID)
	require.EqualValues(t, 15, *created.ProxyID)
	require.NotNil(t, created.AutoPauseOnExpired)
	require.True(t, *created.AutoPauseOnExpired)
	require.Equal(t, service.OpenAIWSIngressModePassthrough, created.Extra["openai_oauth_responses_websockets_v2_mode"])
	require.Equal(t, true, created.Extra["openai_oauth_responses_websockets_v2_enabled"])
	require.Equal(t, true, created.Extra["codex_cli_only"])
	_, hasPassthrough := created.Extra["openai_passthrough"]
	require.False(t, hasPassthrough)
}

func TestImportOpenAIAuthJSONRejectsUnsupportedTemplatePlaceholder(t *testing.T) {
	router, _ := setupOpenAIAuthImportRouter(nil)

	body := map[string]any{
		"items": []map[string]any{
			{
				"tokens": map[string]any{
					"access_token":  "at-5",
					"refresh_token": "rt-5",
					"account_id":    "acct-5",
				},
			},
		},
		"name_template": "{unknown}",
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported placeholder")
}

func TestImportOpenAIAuthJSONFailsWhenTemplateFieldMissing(t *testing.T) {
	router, adminSvc := setupOpenAIAuthImportRouter(nil)

	body := map[string]any{
		"items": []map[string]any{
			{
				"tokens": map[string]any{
					"access_token":  "at-6",
					"refresh_token": "rt-6",
					"account_id":    "acct-6",
				},
				"plan_type": "plus",
			},
		},
		"name_template": "{plan_type}-{email}",
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp openAIAuthImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 0, resp.Data.AccountCreated)
	require.Equal(t, 1, resp.Data.AccountFailed)
	require.Len(t, resp.Data.Errors, 1)
	require.Contains(t, resp.Data.Errors[0].Message, "{email}")
	require.Empty(t, adminSvc.createdAccounts)
}

func TestBuildOpenAIAuthImportAccountInputPrefersIDTokenPlanType(t *testing.T) {
	payload := map[string]any{
		"tokens": map[string]any{
			"access_token":  "at-live",
			"refresh_token": "rt-live",
			"account_id":    "acct-live",
			"id_token":      buildOpenAIAuthImportIDTokenForTest(t, "acct-live", "plus", "plus@example.com"),
		},
		"email":     "plus@example.com",
		"plan_type": "free",
	}

	input, accountName, err := buildOpenAIAuthImportAccountInput(context.Background(), payload, 0, openAIAuthImportOptions{NameTemplate: "{plan_type}-{email}"}, nil)
	require.NoError(t, err)
	require.NotNil(t, input)
	require.Equal(t, "plus-plus@example.com", accountName)
	require.Equal(t, "plus-plus@example.com", input.Name)
	require.Equal(t, "plus", input.Credentials["plan_type"])
	require.Equal(t, "acct-live", input.Credentials["chatgpt_account_id"])
}

func TestImportOpenAIAuthJSONRefreshesTokensWhenEnabled(t *testing.T) {
	oauthClient := &openAIAuthImportOAuthClientStub{
		responses: map[string]*openai.TokenResponse{
			"rt-refresh": {
				AccessToken:  "at-fresh",
				RefreshToken: "rt-fresh",
				IDToken:      buildOpenAIAuthImportIDTokenForTest(t, "acct-fresh", "plus", "fresh@example.com"),
				ExpiresIn:    3600,
			},
		},
	}
	router, adminSvc := setupOpenAIAuthImportRouter(oauthClient)

	body := map[string]any{
		"items": []map[string]any{
			{
				"tokens": map[string]any{
					"access_token":  "at-stale",
					"refresh_token": "rt-refresh",
					"id_token":      buildOpenAIAuthImportIDTokenForTest(t, "acct-stale", "free", "stale@example.com"),
					"account_id":    "acct-stale",
				},
				"client_id": "client-refresh",
			},
		},
		"refresh_before_import": true,
		"name_template":         "{plan_type}-{email}",
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp openAIAuthImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Data.AccountCreated)
	require.Equal(t, 0, resp.Data.AccountFailed)
	require.Equal(t, 1, oauthClient.refreshCalls)
	require.Equal(t, "client-refresh", oauthClient.lastClientID)

	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.Equal(t, "at-fresh", created.Credentials["access_token"])
	require.Equal(t, "rt-fresh", created.Credentials["refresh_token"])
	require.Equal(t, "fresh@example.com", created.Credentials["email"])
	require.Equal(t, "plus", created.Credentials["plan_type"])
	require.Equal(t, "acct-fresh", created.Credentials["chatgpt_account_id"])
	require.Equal(t, "plus-fresh@example.com", created.Name)
}

func TestImportOpenAIAuthJSONDoesNotRefreshByDefault(t *testing.T) {
	oauthClient := &openAIAuthImportOAuthClientStub{
		responses: map[string]*openai.TokenResponse{
			"rt-refresh": {
				AccessToken:  "at-fresh",
				RefreshToken: "rt-fresh",
				IDToken:      buildOpenAIAuthImportIDTokenForTest(t, "acct-fresh", "plus", "fresh@example.com"),
				ExpiresIn:    3600,
			},
		},
	}
	router, adminSvc := setupOpenAIAuthImportRouter(oauthClient)

	body := []map[string]any{
		{
			"tokens": map[string]any{
				"access_token":  "at-stale",
				"refresh_token": "rt-refresh",
				"id_token":      buildOpenAIAuthImportIDTokenForTest(t, "acct-stale", "free", "stale@example.com"),
				"account_id":    "acct-stale",
			},
		},
	}

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, 0, oauthClient.refreshCalls)
	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.Equal(t, "at-stale", created.Credentials["access_token"])
	require.Equal(t, "free", created.Credentials["plan_type"])
	require.Equal(t, "stale@example.com", created.Credentials["email"])
}

func TestImportOpenAIAuthFileRefreshesTokensWhenEnabled(t *testing.T) {
	oauthClient := &openAIAuthImportOAuthClientStub{
		responses: map[string]*openai.TokenResponse{
			"rt-file": {
				AccessToken:  "at-file-fresh",
				RefreshToken: "rt-file-fresh",
				IDToken:      buildOpenAIAuthImportIDTokenForTest(t, "acct-file-fresh", "team", "file@example.com"),
				ExpiresIn:    3600,
			},
		},
	}
	router, adminSvc := setupOpenAIAuthImportRouter(oauthClient)

	payload := `[{"tokens":{"access_token":"at-file-stale","refresh_token":"rt-file","id_token":"` + buildOpenAIAuthImportIDTokenForTest(t, "acct-file-stale", "free", "stale-file@example.com") + `","account_id":"acct-file-stale"},"client_id":"client-file"}]`

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "auth.json")
	require.NoError(t, err)
	_, err = part.Write([]byte(payload))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("refresh_before_import", "true"))
	require.NoError(t, writer.WriteField("name_template", "{plan_type}-{email}"))
	require.NoError(t, writer.Close())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-auths/import-file", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Equal(t, 1, oauthClient.refreshCalls)
	require.Len(t, adminSvc.createdAccounts, 1)
	created := adminSvc.createdAccounts[0]
	require.Equal(t, "team-file@example.com", created.Name)
	require.Equal(t, "team", created.Credentials["plan_type"])
	require.Equal(t, "file@example.com", created.Credentials["email"])
	require.Equal(t, "at-file-fresh", created.Credentials["access_token"])
}

func buildOpenAIAuthImportIDTokenForTest(t *testing.T, accountID, planType, email string) string {
	if t != nil {
		t.Helper()
	}

	claims := map[string]any{
		"email": email,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": accountID,
			"chatgpt_plan_type":  planType,
		},
	}
	payload, err := json.Marshal(claims)
	if t != nil {
		require.NoError(t, err)
	} else if err != nil {
		panic(err)
	}

	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

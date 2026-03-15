package admin

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIAuthImportResponse struct {
	Code int                    `json:"code"`
	Data OpenAIAuthImportResult `json:"data"`
}

func setupOpenAIAuthImportRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()

	h := NewAccountHandler(
		adminSvc,
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
		nil,
		nil,
	)

	router.POST("/api/v1/admin/accounts/openai-auths/import", h.ImportOpenAIAuthJSON)
	router.POST("/api/v1/admin/accounts/openai-auths/import-file", h.ImportOpenAIAuthFile)
	return router, adminSvc
}

func TestImportOpenAIAuthJSONCreatesOpenAIOAuthAccount(t *testing.T) {
	router, adminSvc := setupOpenAIAuthImportRouter()

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
	router, adminSvc := setupOpenAIAuthImportRouter()

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

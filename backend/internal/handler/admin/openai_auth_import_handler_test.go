package admin

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestImportOpenAIAuthJSONSupportsGroupIDsAndNameTemplate(t *testing.T) {
	router, adminSvc := setupOpenAIAuthImportRouter()

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
	router, adminSvc := setupOpenAIAuthImportRouter()

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

func TestImportOpenAIAuthJSONRejectsUnsupportedTemplatePlaceholder(t *testing.T) {
	router, _ := setupOpenAIAuthImportRouter()

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
	router, adminSvc := setupOpenAIAuthImportRouter()

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

	input, accountName, err := buildOpenAIAuthImportAccountInput(payload, 0, openAIAuthImportOptions{NameTemplate: "{plan_type}-{email}"})
	require.NoError(t, err)
	require.NotNil(t, input)
	require.Equal(t, "plus-plus@example.com", accountName)
	require.Equal(t, "plus-plus@example.com", input.Name)
	require.Equal(t, "plus", input.Credentials["plan_type"])
	require.Equal(t, "acct-live", input.Credentials["chatgpt_account_id"])
}

func buildOpenAIAuthImportIDTokenForTest(t *testing.T, accountID, planType, email string) string {
	t.Helper()

	claims := map[string]any{
		"email": email,
		"exp":   time.Now().Add(time.Hour).Unix(),
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": accountID,
			"chatgpt_plan_type":  planType,
		},
	}
	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

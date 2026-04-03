package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

type openaiOAuthClientPlanStub struct {
	exchangeResp *openai.TokenResponse
	refreshResp  *openai.TokenResponse
}

func (s *openaiOAuthClientPlanStub) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*openai.TokenResponse, error) {
	if s.exchangeResp == nil {
		return nil, errors.New("not implemented")
	}
	return s.exchangeResp, nil
}

func (s *openaiOAuthClientPlanStub) RefreshToken(ctx context.Context, refreshToken, proxyURL string) (*openai.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *openaiOAuthClientPlanStub) RefreshTokenWithClientID(ctx context.Context, refreshToken, proxyURL string, clientID string) (*openai.TokenResponse, error) {
	if s.refreshResp == nil {
		return nil, errors.New("not implemented")
	}
	return s.refreshResp, nil
}

func TestOpenAIOAuthService_ExchangeCode_PrefersAccountCheckPlanType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer at-exchange", r.Header.Get("Authorization"))
		require.Equal(t, "acct-exchange", r.Header.Get("chatgpt-account-id"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"account_ordering":["acct-exchange"],
			"accounts":{
				"default":{"account":{"account_id":"acct-exchange"},"entitlement":{"subscription_plan":"chatgptplusplan"}},
				"acct-exchange":{"account":{"account_id":"acct-exchange"},"entitlement":{"subscription_plan":"chatgptplusplan"}}
			}
		}`))
	}))
	defer server.Close()

	origin := openAIAccountsCheckURL
	openAIAccountsCheckURL = server.URL
	defer func() { openAIAccountsCheckURL = origin }()

	client := &openaiOAuthClientPlanStub{
		exchangeResp: &openai.TokenResponse{
			AccessToken:  "at-exchange",
			RefreshToken: "rt-exchange",
			IDToken:      buildOpenAIIDTokenForTest(t, "acct-exchange", "free", "plus@example.com"),
			ExpiresIn:    3600,
		},
	}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()

	svc.sessionStore.Set("sid", &openai.OAuthSession{
		State:        "expected-state",
		CodeVerifier: "verifier",
		ClientID:     openai.ClientID,
		RedirectURI:  openai.DefaultRedirectURI,
		CreatedAt:    time.Now(),
	})

	info, err := svc.ExchangeCode(context.Background(), &OpenAIExchangeCodeInput{
		SessionID: "sid",
		Code:      "auth-code",
		State:     "expected-state",
	})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "plus", info.PlanType)
	require.Equal(t, "acct-exchange", info.ChatGPTAccountID)
	require.Equal(t, "plus@example.com", info.Email)
}

func TestOpenAIOAuthService_RefreshTokenWithClientID_PrefersAccountCheckPlanType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer at-refresh", r.Header.Get("Authorization"))
		require.Equal(t, "acct-refresh", r.Header.Get("chatgpt-account-id"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"accounts":{
				"default":{"account":{"account_id":"acct-refresh"},"entitlement":{"subscription_plan":"chatgptteamplan"}}
			}
		}`))
	}))
	defer server.Close()

	origin := openAIAccountsCheckURL
	openAIAccountsCheckURL = server.URL
	defer func() { openAIAccountsCheckURL = origin }()

	client := &openaiOAuthClientPlanStub{
		refreshResp: &openai.TokenResponse{
			AccessToken:  "at-refresh",
			RefreshToken: "rt-refresh",
			IDToken:      buildOpenAIIDTokenForTest(t, "acct-refresh", "free", "team@example.com"),
			ExpiresIn:    1800,
		},
	}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()

	info, err := svc.RefreshTokenWithClientID(context.Background(), "rt-refresh", "", "client-refresh")
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "team", info.PlanType)
	require.Equal(t, "acct-refresh", info.ChatGPTAccountID)
}

func TestOpenAIOAuthService_RefreshAccountToken_UsesExistingAccountIDHintForPlanType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer at-account-refresh", r.Header.Get("Authorization"))
		require.Equal(t, "acct-account-refresh", r.Header.Get("chatgpt-account-id"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"accounts":{
				"acct-account-refresh":{"account":{"account_id":"acct-account-refresh"},"entitlement":{"subscription_plan":"chatgptplusplan"}}
			},
			"account_ordering":["acct-account-refresh"]
		}`))
	}))
	defer server.Close()

	origin := openAIAccountsCheckURL
	openAIAccountsCheckURL = server.URL
	defer func() { openAIAccountsCheckURL = origin }()

	client := &openaiOAuthClientPlanStub{
		refreshResp: &openai.TokenResponse{
			AccessToken:  "at-account-refresh",
			RefreshToken: "rt-account-refresh",
			ExpiresIn:    900,
		},
	}
	svc := NewOpenAIOAuthService(nil, client)
	defer svc.Stop()

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token":      "rt-account-refresh",
			"client_id":          "client-account-refresh",
			"chatgpt_account_id": "acct-account-refresh",
		},
	}

	info, err := svc.RefreshAccountToken(context.Background(), account)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "plus", info.PlanType)
	require.Equal(t, "acct-account-refresh", info.ChatGPTAccountID)
}

func buildOpenAIIDTokenForTest(t *testing.T, accountID, planType, email string) string {
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

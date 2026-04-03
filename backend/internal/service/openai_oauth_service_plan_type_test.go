package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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

func TestOpenAIOAuthService_ExchangeCode_UsesIDTokenPlanType(t *testing.T) {
	client := &openaiOAuthClientPlanStub{
		exchangeResp: &openai.TokenResponse{
			AccessToken:  "at-exchange",
			RefreshToken: "rt-exchange",
			IDToken:      buildOpenAIIDTokenForTest(t, "acct-exchange", "plus", "plus@example.com"),
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

func TestOpenAIOAuthService_RefreshTokenWithClientID_UsesIDTokenPlanType(t *testing.T) {
	client := &openaiOAuthClientPlanStub{
		refreshResp: &openai.TokenResponse{
			AccessToken:  "at-refresh",
			RefreshToken: "rt-refresh",
			IDToken:      buildOpenAIIDTokenForTest(t, "acct-refresh", "team", "team@example.com"),
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

func TestOpenAIOAuthService_RefreshAccountToken_UsesRefreshedIDTokenPlanType(t *testing.T) {
	client := &openaiOAuthClientPlanStub{
		refreshResp: &openai.TokenResponse{
			AccessToken:  "at-account-refresh",
			RefreshToken: "rt-account-refresh",
			IDToken:      buildOpenAIIDTokenForTest(t, "acct-account-refresh", "pro", "pro@example.com"),
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
	require.Equal(t, "pro", info.PlanType)
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

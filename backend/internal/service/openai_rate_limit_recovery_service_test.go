//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type openAIRateLimitRecoveryRepo struct {
	stubOpenAIAccountRepo
}

func (r *openAIRateLimitRecoveryRepo) ListWithFilters(_ context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64) ([]Account, *pagination.PaginationResult, error) {
	_ = platform
	_ = accountType
	_ = status
	_ = search
	_ = groupID
	start := (params.Page - 1) * params.PageSize
	if start >= len(r.accounts) {
		return nil, &pagination.PaginationResult{Total: int64(len(r.accounts)), Page: params.Page, PageSize: params.PageSize}, nil
	}
	end := start + params.PageSize
	if end > len(r.accounts) {
		end = len(r.accounts)
	}
	items := make([]Account, end-start)
	copy(items, r.accounts[start:end])
	return items, &pagination.PaginationResult{Total: int64(len(r.accounts)), Page: params.Page, PageSize: params.PageSize}, nil
}

type blockingRecoveryHTTPUpstream struct {
	startCh chan struct{}
}

func (u *blockingRecoveryHTTPUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return nil, errors.New("unexpected Do call")
}

func (u *blockingRecoveryHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ bool) (*http.Response, error) {
	select {
	case u.startCh <- struct{}{}:
	default:
	}
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestOpenAIRateLimitRecoveryService_StopCancelsActiveRecoveryWorkers(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(30 * time.Minute)
	account := Account{
		ID:          90210,
		Name:        "openai-recovery-cancel",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
		RateLimitedAt: func() *time.Time {
			ts := time.Now().Add(-time.Minute)
			return &ts
		}(),
		RateLimitResetAt: &resetAt,
	}

	repo := &openAIRateLimitRecoveryRepo{
		stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}},
	}
	upstream := &blockingRecoveryHTTPUpstream{startCh: make(chan struct{}, 1)}
	accountTest := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
	}
	svc := NewOpenAIRateLimitRecoveryService(repo, accountTest, &RateLimitService{accountRepo: repo}, nil)

	roundCtx, roundCancel := context.WithCancel(svc.runCtx)
	defer roundCancel()
	roundDone := make(chan struct{})
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()
		svc.runRecoveryRound(roundCtx, time.Now(), &OpenAIRateLimitRecoverySettings{
			Enabled:              true,
			TestModel:            "gpt-5.1",
			CheckIntervalMinutes: 10,
		})
		close(roundDone)
	}()

	select {
	case <-upstream.startCh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected recovery worker to start upstream probe")
	}

	stopped := make(chan struct{})
	go func() {
		svc.Stop()
		close(stopped)
	}()

	select {
	case <-roundDone:
	case <-time.After(2 * time.Second):
		t.Fatal("recovery round should exit promptly after service stop")
	}

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop should return promptly after canceling active recovery workers")
	}
}

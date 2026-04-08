//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProgressToAccountRiskCandidateIgnoresEmptyProgress(t *testing.T) {
	require.Nil(t, progressToAccountRiskCandidate(nil))
	require.Nil(t, progressToAccountRiskCandidate(&UsageProgress{
		Utilization:      0,
		RemainingSeconds: 0,
	}))
}

func TestResolveAccountRiskCandidateAnthropicUsesPassiveSevenDay(t *testing.T) {
	now := time.Now()
	sessionReset := now.Add(45 * time.Minute)
	sevenDayReset := now.Add(36 * time.Hour)

	svc := &AccountUsageService{}
	account := &Account{
		Platform:         PlatformAnthropic,
		Type:             AccountTypeOAuth,
		SessionWindowEnd: &sessionReset,
		Extra: map[string]any{
			"session_window_utilization":   0.35,
			"passive_usage_7d_utilization": 0.92,
			"passive_usage_7d_reset":       float64(sevenDayReset.Unix()),
		},
	}

	candidate, isRateLimited := svc.resolveAccountRiskCandidate(account, now)
	require.False(t, isRateLimited)
	require.NotNil(t, candidate)
	require.InDelta(t, 92, candidate.Utilization, 0.001)
	require.NotNil(t, candidate.ResetAt)
	require.WithinDuration(t, sevenDayReset, *candidate.ResetAt, time.Second)
}

func TestResolveAccountRiskCandidateAnthropicWithoutSnapshotReturnsUnknown(t *testing.T) {
	svc := &AccountUsageService{}
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeSetupToken,
		Extra:    map[string]any{},
	}

	candidate, isRateLimited := svc.resolveAccountRiskCandidate(account, time.Now())
	require.False(t, isRateLimited)
	require.Nil(t, candidate)
}

// 数据正常（窗口都未过期）的 OpenAI OAuth 账号应按 5h/7d 的 max 落桶。
// 注意这里**没有**设置 codex_usage_updated_at —— 该字段在生产环境只在低频
// snapshot 写入路径中被设置，绝大多数账号不会带它。风险概览不应该因为它的
// 缺失而把账号一刀切到 Unknown。
func TestResolveAccountRiskCandidateOpenAIClassifiesWithoutUpdatedAt(t *testing.T) {
	now := time.Now()
	fiveHourReset := now.Add(90 * time.Minute)
	sevenDayReset := now.Add(36 * time.Hour)

	svc := &AccountUsageService{}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent": 85.0,
			"codex_5h_reset_at":     fiveHourReset.Format(time.RFC3339),
			"codex_7d_used_percent": 40.0,
			"codex_7d_reset_at":     sevenDayReset.Format(time.RFC3339),
		},
	}

	candidate, isRateLimited := svc.resolveAccountRiskCandidate(account, now)
	require.False(t, isRateLimited)
	require.NotNil(t, candidate)
	require.InDelta(t, 85, candidate.Utilization, 0.001)
	require.NotNil(t, candidate.ResetAt)
	require.WithinDuration(t, fiveHourReset, *candidate.ResetAt, 2*time.Second)
}

// 过期窗口（ResetsAt 在过去）必须被 codexProgressToRiskCandidate 丢弃；
// 7d 窗口仍然有效时，账号应该按 7d 的数据落桶，而不是被 5h 的 0% 骗到 below_50。
func TestResolveAccountRiskCandidateOpenAIDiscardsExpiredWindow(t *testing.T) {
	now := time.Now()
	expiredReset := now.Add(-2 * time.Hour).Format(time.RFC3339)
	sevenDayReset := now.Add(48 * time.Hour)

	svc := &AccountUsageService{}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent": 92.0, // 已被 buildCodexUsageProgressFromExtra 归零
			"codex_5h_reset_at":     expiredReset,
			"codex_7d_used_percent": 55.0,
			"codex_7d_reset_at":     sevenDayReset.Format(time.RFC3339),
		},
	}

	candidate, isRateLimited := svc.resolveAccountRiskCandidate(account, now)
	require.False(t, isRateLimited)
	require.NotNil(t, candidate)
	require.InDelta(t, 55, candidate.Utilization, 0.001)
	require.NotNil(t, candidate.ResetAt)
	require.WithinDuration(t, sevenDayReset, *candidate.ResetAt, 2*time.Second)
}

// 两个窗口都过期 → 全部被 codexProgressToRiskCandidate 丢弃 → Unknown。
// 防止 buildCodexUsageProgressFromExtra 的 "归零但保留 ResetsAt" 行为把僵尸账号
// 算成 0% utilization 的 "安全账号"。
func TestResolveAccountRiskCandidateOpenAIAllWindowsExpiredReturnsUnknown(t *testing.T) {
	now := time.Now()
	expiredReset := now.Add(-1 * time.Hour).Format(time.RFC3339)

	svc := &AccountUsageService{}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent": 97.0,
			"codex_5h_reset_at":     expiredReset,
			"codex_7d_used_percent": 98.0,
			"codex_7d_reset_at":     expiredReset,
		},
	}

	candidate, isRateLimited := svc.resolveAccountRiskCandidate(account, now)
	require.False(t, isRateLimited)
	require.Nil(t, candidate)
}

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

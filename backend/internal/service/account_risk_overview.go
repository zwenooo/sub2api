package service

import (
	"context"
	"fmt"
	"time"
)

type AccountRiskOverviewFilter struct {
	Platform    string
	AccountType string
	Search      string
	GroupID     int64
	PrivacyMode string
}

type AccountRiskOverviewSummary struct {
	TotalAccounts           int64 `json:"total_accounts"`
	SupportedAccounts       int64 `json:"supported_accounts"`
	ChartedAccounts         int64 `json:"charted_accounts"`
	ExcludedAccounts        int64 `json:"excluded_accounts"`
	UnknownAccounts         int64 `json:"unknown_accounts"`
	HighRiskAccounts        int64 `json:"high_risk_accounts"`
	RateLimitedAccounts     int64 `json:"rate_limited_accounts"`
	RecoveryTrackedAccounts int64 `json:"recovery_tracked_accounts"`
}

type AccountRiskOverviewBucket struct {
	BucketKey string `json:"bucket_key"`
	Count     int64  `json:"count"`
}

type AccountRiskOverview struct {
	GeneratedAt     time.Time                   `json:"generated_at"`
	Summary         AccountRiskOverviewSummary  `json:"summary"`
	RiskBuckets     []AccountRiskOverviewBucket `json:"risk_buckets"`
	RecoveryBuckets []AccountRiskOverviewBucket `json:"recovery_buckets"`
}

type accountRiskCandidate struct {
	Utilization float64
	ResetAt     *time.Time
}

var accountRiskBucketOrder = []string{
	"below_50",
	"50_60",
	"60_70",
	"70_80",
	"80_90",
	"90_100",
	"rate_limited",
}

var accountRecoveryBucketOrder = []string{
	"under_30m",
	"under_1h",
	"under_3h",
	"under_6h",
	"under_12h",
	"under_24h",
	"under_3d",
	"under_7d",
	"over_7d",
}

func (s *AccountUsageService) GetRiskOverview(ctx context.Context, filter AccountRiskOverviewFilter) (*AccountRiskOverview, error) {
	if s == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("account repo unavailable")
	}

	now := time.Now()
	accounts, err := s.accountRepo.ListAllWithFilters(
		ctx,
		filter.Platform,
		filter.AccountType,
		"",
		filter.Search,
		filter.GroupID,
		filter.PrivacyMode,
	)
	if err != nil {
		return nil, fmt.Errorf("list risk overview accounts failed: %w", err)
	}

	riskCounts := make(map[string]int64, len(accountRiskBucketOrder))
	recoveryCounts := make(map[string]int64, len(accountRecoveryBucketOrder))
	summary := AccountRiskOverviewSummary{}

	for i := range accounts {
		account := &accounts[i]
		summary.TotalAccounts++

		if !supportsAccountRiskOverview(account, now) {
			summary.ExcludedAccounts++
			continue
		}

		summary.SupportedAccounts++
		candidate, isRateLimited := s.resolveAccountRiskCandidate(account, now)
		if candidate == nil {
			summary.UnknownAccounts++
			continue
		}

		summary.ChartedAccounts++
		if isRateLimited {
			riskCounts["rate_limited"]++
			summary.RateLimitedAccounts++
			if key := classifyAccountRecoveryBucket(candidate.ResetAt, now); key != "" {
				recoveryCounts[key]++
				summary.RecoveryTrackedAccounts++
			}
			continue
		}

		riskBucketKey := classifyAccountRiskBucket(candidate.Utilization)
		riskCounts[riskBucketKey]++
		if isHighRiskAccountBucket(riskBucketKey) {
			summary.HighRiskAccounts++
			if key := classifyAccountRecoveryBucket(candidate.ResetAt, now); key != "" {
				recoveryCounts[key]++
				summary.RecoveryTrackedAccounts++
			}
		}
	}

	return &AccountRiskOverview{
		GeneratedAt:     now.UTC(),
		Summary:         summary,
		RiskBuckets:     buildAccountRiskOverviewBuckets(accountRiskBucketOrder, riskCounts),
		RecoveryBuckets: buildAccountRiskOverviewBuckets(accountRecoveryBucketOrder, recoveryCounts),
	}, nil
}

func supportsAccountRiskOverview(account *Account, now time.Time) bool {
	if account == nil || account.Status != StatusActive || !account.Schedulable {
		return false
	}
	if account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
		return false
	}

	switch account.Platform {
	case PlatformOpenAI:
		return account.Type == AccountTypeOAuth
	case PlatformAnthropic:
		return account.Type == AccountTypeOAuth || account.Type == AccountTypeSetupToken
	default:
		return false
	}
}

func (s *AccountUsageService) resolveAccountRiskCandidate(account *Account, now time.Time) (*accountRiskCandidate, bool) {
	if account == nil {
		return nil, false
	}
	if account.IsRateLimited() {
		return &accountRiskCandidate{
			Utilization: 100,
			ResetAt:     account.RateLimitResetAt,
		}, true
	}

	switch account.Platform {
	case PlatformOpenAI:
		return dominantAccountRiskCandidate(
			progressToAccountRiskCandidate(buildCodexUsageProgressFromExtra(account.Extra, "5h", now)),
			progressToAccountRiskCandidate(buildCodexUsageProgressFromExtra(account.Extra, "7d", now)),
		), false
	case PlatformAnthropic:
		usage := s.estimateSetupTokenUsage(account)
		var fiveHour *UsageProgress
		if usage != nil {
			fiveHour = usage.FiveHour
		}
		return dominantAccountRiskCandidate(
			progressToAccountRiskCandidate(fiveHour),
			progressToAccountRiskCandidate(buildAnthropicPassiveSevenDayProgress(account, now)),
		), false
	default:
		return nil, false
	}
}

func progressToAccountRiskCandidate(progress *UsageProgress) *accountRiskCandidate {
	if progress == nil || !hasRiskSnapshot(progress) {
		return nil
	}
	return &accountRiskCandidate{
		Utilization: progress.Utilization,
		ResetAt:     progress.ResetsAt,
	}
}

func hasRiskSnapshot(progress *UsageProgress) bool {
	if progress == nil {
		return false
	}
	if progress.ResetsAt != nil {
		return true
	}
	if progress.RemainingSeconds > 0 {
		return true
	}
	return progress.Utilization > 0
}

func buildAnthropicPassiveSevenDayProgress(account *Account, now time.Time) *UsageProgress {
	if account == nil {
		return nil
	}

	util7d := parseExtraFloat64(account.Extra["passive_usage_7d_utilization"])
	reset7dRaw := parseExtraFloat64(account.Extra["passive_usage_7d_reset"])
	if util7d <= 0 && reset7dRaw <= 0 {
		return nil
	}

	var resetAt *time.Time
	var remaining int
	if reset7dRaw > 0 {
		t := time.Unix(int64(reset7dRaw), 0)
		resetAt = &t
		remaining = int(t.Sub(now).Seconds())
		if remaining < 0 {
			remaining = 0
		}
	}

	return &UsageProgress{
		Utilization:      util7d * 100,
		ResetsAt:         resetAt,
		RemainingSeconds: remaining,
	}
}

func dominantAccountRiskCandidate(candidates ...*accountRiskCandidate) *accountRiskCandidate {
	var best *accountRiskCandidate
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if best == nil {
			best = candidate
			continue
		}
		if candidate.Utilization > best.Utilization {
			best = candidate
			continue
		}
		if candidate.Utilization == best.Utilization && accountRiskResetAtAfter(candidate.ResetAt, best.ResetAt) {
			best = candidate
		}
	}
	return best
}

func accountRiskResetAtAfter(left, right *time.Time) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	return left.After(*right)
}

func classifyAccountRiskBucket(utilization float64) string {
	switch {
	case utilization < 50:
		return "below_50"
	case utilization < 60:
		return "50_60"
	case utilization < 70:
		return "60_70"
	case utilization < 80:
		return "70_80"
	case utilization < 90:
		return "80_90"
	default:
		return "90_100"
	}
}

func isHighRiskAccountBucket(bucketKey string) bool {
	return bucketKey == "80_90" || bucketKey == "90_100"
}

func classifyAccountRecoveryBucket(resetAt *time.Time, now time.Time) string {
	if resetAt == nil {
		return ""
	}

	diff := resetAt.Sub(now)
	switch {
	case diff <= 30*time.Minute:
		return "under_30m"
	case diff <= time.Hour:
		return "under_1h"
	case diff <= 3*time.Hour:
		return "under_3h"
	case diff <= 6*time.Hour:
		return "under_6h"
	case diff <= 12*time.Hour:
		return "under_12h"
	case diff <= 24*time.Hour:
		return "under_24h"
	case diff <= 3*24*time.Hour:
		return "under_3d"
	case diff <= 7*24*time.Hour:
		return "under_7d"
	default:
		return "over_7d"
	}
}

func buildAccountRiskOverviewBuckets(order []string, counts map[string]int64) []AccountRiskOverviewBucket {
	buckets := make([]AccountRiskOverviewBucket, 0, len(order))
	for _, key := range order {
		buckets = append(buckets, AccountRiskOverviewBucket{
			BucketKey: key,
			Count:     counts[key],
		})
	}
	return buckets
}

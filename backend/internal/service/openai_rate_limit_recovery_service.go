package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	openAIRateLimitRecoveryTick           = time.Minute
	openAIRateLimitRecoveryPageSize       = 100
	openAIRateLimitRecoveryMaxWorkers     = 5
	openAIRateLimitRecoveryAccountTimeout = 90 * time.Second
	openAIRateLimitRecoveryRoundTimeout   = 15 * time.Minute
)

// OpenAIRateLimitRecoveryService periodically probes selected OpenAI accounts
// and optionally clears recoverable runtime state after a successful upstream test.
type OpenAIRateLimitRecoveryService struct {
	accountRepo    AccountRepository
	accountTest    *AccountTestService
	rateLimitSvc   *RateLimitService
	settingService *SettingService

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	wg        sync.WaitGroup
	runCtx    context.Context
	cancelRun context.CancelFunc

	running         atomic.Bool
	lastRunUnixNano atomic.Int64
}

func NewOpenAIRateLimitRecoveryService(
	accountRepo AccountRepository,
	accountTest *AccountTestService,
	rateLimitSvc *RateLimitService,
	settingService *SettingService,
) *OpenAIRateLimitRecoveryService {
	runCtx, cancelRun := context.WithCancel(context.Background())
	return &OpenAIRateLimitRecoveryService{
		accountRepo:    accountRepo,
		accountTest:    accountTest,
		rateLimitSvc:   rateLimitSvc,
		settingService: settingService,
		stopCh:         make(chan struct{}),
		runCtx:         runCtx,
		cancelRun:      cancelRun,
	}
}

func (s *OpenAIRateLimitRecoveryService) Start() {
	if s == nil || s.accountRepo == nil || s.accountTest == nil || s.rateLimitSvc == nil || s.settingService == nil {
		return
	}
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go s.loop()
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] started (tick=%s)", openAIRateLimitRecoveryTick)
	})
}

func (s *OpenAIRateLimitRecoveryService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cancelRun != nil {
			s.cancelRun()
		}
		close(s.stopCh)
		s.wg.Wait()
	})
}

func (s *OpenAIRateLimitRecoveryService) loop() {
	defer s.wg.Done()

	ticker := time.NewTicker(openAIRateLimitRecoveryTick)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-s.stopCh:
			return
		}
	}
}

func (s *OpenAIRateLimitRecoveryService) tick() {
	if s == nil || !s.running.CompareAndSwap(false, true) {
		return
	}
	defer s.running.Store(false)

	ctx, cancel := context.WithTimeout(s.runCtx, openAIRateLimitRecoveryRoundTimeout)
	defer cancel()

	settings, err := s.settingService.GetOpenAIRateLimitRecoverySettings(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] load settings failed: %v", err)
		return
	}
	if settings == nil || !settings.Enabled {
		return
	}

	interval := time.Duration(settings.CheckIntervalMinutes) * time.Minute
	now := time.Now()
	lastRun := time.Unix(0, s.lastRunUnixNano.Load())
	if !lastRun.IsZero() && now.Sub(lastRun) < interval {
		return
	}
	s.lastRunUnixNano.Store(now.UnixNano())

	s.runRecoveryRound(ctx, now, settings)
}

func (s *OpenAIRateLimitRecoveryService) runRecoveryRound(ctx context.Context, now time.Time, settings *OpenAIRateLimitRecoverySettings) {
	targetStatusesInput := settings.TargetStatuses
	if targetStatusesInput == nil {
		targetStatusesInput = append([]string(nil), DefaultOpenAIRateLimitRecoverySettings().TargetStatuses...)
	}
	targetStatuses, err := normalizeOpenAIProbeTargetStatuses(targetStatusesInput)
	if err != nil {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] invalid target statuses: %v", err)
		return
	}

	accounts, err := s.listRecoverableAccounts(ctx, now, targetStatuses)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] list accounts failed: %v", err)
		return
	}
	if len(accounts) == 0 {
		return
	}

	logger.LegacyPrintf(
		"service.openai_rate_limit_recovery",
		"[OpenAIRateLimitRecovery] round start accounts=%d model=%s interval=%dm statuses=%v auto_recover=%t",
		len(accounts),
		settings.TestModel,
		settings.CheckIntervalMinutes,
		targetStatuses,
		settings.AutoRecover,
	)

	sem := make(chan struct{}, openAIRateLimitRecoveryMaxWorkers)
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var recoveredCount atomic.Int64
	var failedCount atomic.Int64

	for i := range accounts {
		account := accounts[i]
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}
		wg.Add(1)
		go func(acc Account) {
			defer wg.Done()
			defer func() { <-sem }()

			success, recovered := s.runRecoveryCheck(ctx, &acc, settings.TestModel, settings.AutoRecover)
			if success {
				successCount.Add(1)
				if recovered {
					recoveredCount.Add(1)
				}
				return
			}
			failedCount.Add(1)
		}(account)
	}

	wg.Wait()

	logger.LegacyPrintf(
		"service.openai_rate_limit_recovery",
		"[OpenAIRateLimitRecovery] round finished accounts=%d success=%d recovered=%d failed=%d",
		len(accounts),
		successCount.Load(),
		recoveredCount.Load(),
		failedCount.Load(),
	)
}

func (s *OpenAIRateLimitRecoveryService) listRecoverableAccounts(ctx context.Context, now time.Time, targetStatuses []string) ([]Account, error) {
	accounts := make([]Account, 0)
	for page := 1; ; page++ {
		items, _, err := s.accountRepo.ListWithFilters(
			ctx,
			pagination.PaginationParams{Page: page, PageSize: openAIRateLimitRecoveryPageSize},
			PlatformOpenAI,
			"",
			"",
			"",
			0,
			"",
		)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}

		for i := range items {
			account := items[i]
			if !accountMatchesOpenAIProbeTargetStatuses(&account, targetStatuses, now) {
				continue
			}
			accounts = append(accounts, account)
		}

		if len(items) < openAIRateLimitRecoveryPageSize {
			break
		}
	}

	return accounts, nil
}

func (s *OpenAIRateLimitRecoveryService) runRecoveryCheck(parent context.Context, account *Account, testModel string, autoRecover bool) (bool, bool) {
	if s == nil || account == nil {
		return false, false
	}
	if parent.Err() != nil {
		return false, false
	}

	ctx, cancel := context.WithTimeout(parent, openAIRateLimitRecoveryAccountTimeout)
	defer cancel()

	result, err := s.accountTest.RunTestBackground(ctx, account.ID, testModel)
	if err != nil {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d test failed: %v", account.ID, err)
		return false, false
	}
	if result == nil || result.Status != "success" {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d test unsuccessful: status=%s err=%s", account.ID, safeScheduledTestStatus(result), safeScheduledTestError(result))
		return false, false
	}
	if !autoRecover {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d probe succeeded without auto recovery", account.ID)
		return true, false
	}

	// 探针成功但 7d 限额仍超标时不恢复，避免 5h 恢复后误解除限流
	freshAccount, reloadErr := s.accountRepo.GetByID(ctx, account.ID)
	if reloadErr == nil && freshAccount != nil && accountHasStored7dUtilizationExceeded(freshAccount) {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d probe succeeded but 7d utilization still exceeded, skipping recovery", account.ID)
		return true, false
	}

	recovery, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, account.ID)
	if err != nil {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d clear state failed: %v", account.ID, err)
		return true, false
	}

	logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d cleared recoverable runtime state after successful probe", account.ID)
	_ = recovery
	return true, true
}

func accountMatchesOpenAIProbeTargetStatuses(account *Account, targetStatuses []string, now time.Time) bool {
	if account == nil {
		return false
	}
	for _, status := range targetStatuses {
		if accountMatchesOpenAIProbeStatus(account, status, now) {
			return true
		}
	}
	return false
}

func accountMatchesOpenAIProbeStatus(account *Account, status string, now time.Time) bool {
	if account == nil {
		return false
	}
	switch status {
	case StatusActive:
		if account.Status != StatusActive || !account.Schedulable {
			return false
		}
		if account.AutoPauseOnExpired && account.ExpiresAt != nil && !now.Before(*account.ExpiresAt) {
			return false
		}
		if account.OverloadUntil != nil && now.Before(*account.OverloadUntil) {
			return false
		}
		if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
			return false
		}
		return !accountHasActiveRuntimeRateLimit(account, now)
	case "rate_limited":
		return accountHasActiveRuntimeRateLimit(account, now)
	case "temp_unschedulable":
		return account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil)
	default:
		return account.Status == status
	}
}

func accountHasActiveRuntimeRateLimit(account *Account, now time.Time) bool {
	if account == nil {
		return false
	}
	if account.RateLimitResetAt != nil && now.Before(*account.RateLimitResetAt) {
		return true
	}
	return account.HasAnyActiveModelRateLimitAt(now)
}

// accountHasStored7dUtilizationExceeded 检查账号 extras 中存储的 7d utilization
// 是否仍 >= 100%，且对应的 7d reset 时间尚未到达。
func accountHasStored7dUtilizationExceeded(account *Account) bool {
	if account == nil || len(account.Extra) == 0 {
		return false
	}
	raw, ok := account.Extra["passive_usage_7d_utilization"]
	if !ok || raw == nil {
		return false
	}
	var util float64
	switch v := raw.(type) {
	case float64:
		util = v
	case int64:
		util = float64(v)
	case int:
		util = float64(v)
	default:
		return false
	}
	if util < 1.0-1e-9 {
		return false
	}
	// 7d utilization >= 100%，再检查 reset 时间是否仍在未来
	rawReset, ok := account.Extra["passive_usage_7d_reset"]
	if !ok || rawReset == nil {
		return true
	}
	var resetTs int64
	switch v := rawReset.(type) {
	case float64:
		resetTs = int64(v)
	case int64:
		resetTs = v
	case int:
		resetTs = int64(v)
	default:
		return true
	}
	if resetTs > 1e11 {
		resetTs = resetTs / 1000
	}
	return time.Now().Before(time.Unix(resetTs, 0))
}

func safeScheduledTestStatus(result *ScheduledTestResult) string {
	if result == nil {
		return ""
	}
	return result.Status
}

func safeScheduledTestError(result *ScheduledTestResult) string {
	if result == nil {
		return ""
	}
	return result.ErrorMessage
}

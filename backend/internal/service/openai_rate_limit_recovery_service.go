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

// OpenAIRateLimitRecoveryService periodically re-tests rate-limited OpenAI accounts
// and clears their runtime rate-limit state once a real upstream test succeeds.
type OpenAIRateLimitRecoveryService struct {
	accountRepo    AccountRepository
	accountTest    *AccountTestService
	rateLimitSvc   *RateLimitService
	settingService *SettingService

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	wg        sync.WaitGroup

	running         atomic.Bool
	lastRunUnixNano atomic.Int64
}

func NewOpenAIRateLimitRecoveryService(
	accountRepo AccountRepository,
	accountTest *AccountTestService,
	rateLimitSvc *RateLimitService,
	settingService *SettingService,
) *OpenAIRateLimitRecoveryService {
	return &OpenAIRateLimitRecoveryService{
		accountRepo:    accountRepo,
		accountTest:    accountTest,
		rateLimitSvc:   rateLimitSvc,
		settingService: settingService,
		stopCh:         make(chan struct{}),
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

	ctx, cancel := context.WithTimeout(context.Background(), openAIRateLimitRecoveryRoundTimeout)
	defer cancel()

	settings, err := s.settingService.GetOpenAIRateLimitRecoverySettings(ctx)
	if err != nil {
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
	accounts, err := s.listRecoverableAccounts(ctx, now)
	if err != nil {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] list accounts failed: %v", err)
		return
	}
	if len(accounts) == 0 {
		return
	}

	logger.LegacyPrintf(
		"service.openai_rate_limit_recovery",
		"[OpenAIRateLimitRecovery] round start accounts=%d model=%s interval=%dm",
		len(accounts),
		settings.TestModel,
		settings.CheckIntervalMinutes,
	)

	sem := make(chan struct{}, openAIRateLimitRecoveryMaxWorkers)
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var clearedCount atomic.Int64
	var failedCount atomic.Int64

	for i := range accounts {
		account := accounts[i]
		sem <- struct{}{}
		wg.Add(1)
		go func(acc Account) {
			defer wg.Done()
			defer func() { <-sem }()

			if s.runRecoveryCheck(ctx, &acc, settings.TestModel) {
				successCount.Add(1)
				clearedCount.Add(1)
				return
			}
			failedCount.Add(1)
		}(account)
	}

	wg.Wait()

	logger.LegacyPrintf(
		"service.openai_rate_limit_recovery",
		"[OpenAIRateLimitRecovery] round finished accounts=%d cleared=%d failed=%d",
		len(accounts),
		clearedCount.Load(),
		failedCount.Load(),
	)
}

func (s *OpenAIRateLimitRecoveryService) listRecoverableAccounts(ctx context.Context, now time.Time) ([]Account, error) {
	accounts := make([]Account, 0)
	for page := 1; ; page++ {
		items, _, err := s.accountRepo.ListWithFilters(
			ctx,
			pagination.PaginationParams{Page: page, PageSize: openAIRateLimitRecoveryPageSize},
			PlatformOpenAI,
			"",
			"rate_limited",
			"",
			0,
		)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}

		for i := range items {
			account := items[i]
			if !account.IsActive() || !account.Schedulable {
				continue
			}
			if !account.IsOpenAIOAuth() && !account.IsOpenAIApiKey() {
				continue
			}
			if account.RateLimitResetAt == nil || !now.Before(*account.RateLimitResetAt) {
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

func (s *OpenAIRateLimitRecoveryService) runRecoveryCheck(parent context.Context, account *Account, testModel string) bool {
	if s == nil || account == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(parent, openAIRateLimitRecoveryAccountTimeout)
	defer cancel()

	result, err := s.accountTest.RunTestBackground(ctx, account.ID, testModel)
	if err != nil {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d test failed: %v", account.ID, err)
		return false
	}
	if result == nil || result.Status != "success" {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d test unsuccessful: status=%s err=%s", account.ID, safeScheduledTestStatus(result), safeScheduledTestError(result))
		return false
	}

	recovery, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, account.ID)
	if err != nil {
		logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d clear state failed: %v", account.ID, err)
		return false
	}

	logger.LegacyPrintf("service.openai_rate_limit_recovery", "[OpenAIRateLimitRecovery] account=%d cleared rate limit after successful self-test", account.ID)
	_ = recovery
	return true
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

package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AccountRuleRepository interface {
	ListScopes(ctx context.Context) ([]*AccountRuleScope, error)
	GetScopeByID(ctx context.Context, id int64) (*AccountRuleScope, error)
	CreateScope(ctx context.Context, scope *AccountRuleScope) (*AccountRuleScope, error)
	UpdateScope(ctx context.Context, scope *AccountRuleScope) (*AccountRuleScope, error)
	DeleteScope(ctx context.Context, id int64) error

	GetRuleByID(ctx context.Context, id int64) (*AccountRuleErrorRule, error)
	CreateRule(ctx context.Context, rule *AccountRuleErrorRule) (*AccountRuleErrorRule, error)
	UpdateRule(ctx context.Context, rule *AccountRuleErrorRule) (*AccountRuleErrorRule, error)
	DeleteRule(ctx context.Context, id int64) error

	ListObservedScopes(ctx context.Context) ([]*AccountRuleObservedScope, error)
}

type cachedAccountRuleScope struct {
	scope    *AccountRuleScope
	modelSet []string
	rules    []*cachedAccountRule
}

type cachedAccountRule struct {
	rule          *AccountRuleErrorRule
	statusCodeSet map[int]struct{}
	lowerKeywords []string
}

type accountRuleCacheSnapshot struct {
	loadedAt    time.Time
	scopesByKey map[string]*cachedAccountRuleScope
}

const accountRuleCacheTTL = 30 * time.Second

type AccountRuleService struct {
	repo           AccountRuleRepository
	accountRepo    AccountRepository
	settingService *SettingService

	cacheMu sync.RWMutex
	cache   *accountRuleCacheSnapshot
}

func NewAccountRuleService(
	repo AccountRuleRepository,
	accountRepo AccountRepository,
	settingService *SettingService,
) *AccountRuleService {
	svc := &AccountRuleService{
		repo:           repo,
		accountRepo:    accountRepo,
		settingService: settingService,
	}
	if err := svc.refreshCache(context.Background()); err != nil {
		slog.Warn("account_rule_cache_init_failed", "error", err)
	}
	return svc
}

func (s *AccountRuleService) ListCatalog(ctx context.Context) (*AccountRuleCatalog, error) {
	scopes, err := s.repo.ListScopes(ctx)
	if err != nil {
		return nil, err
	}
	observed, err := s.repo.ListObservedScopes(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	for _, scope := range scopes {
		scope.Normalize()
		for _, rule := range scope.Rules {
			rule.Normalize()
		}
	}

	return &AccountRuleCatalog{
		Scopes:         scopes,
		ObservedScopes: synthesizePlatformObservedScopes(observed),
		Settings:       *settings,
	}, nil
}

func synthesizePlatformObservedScopes(observed []*AccountRuleObservedScope) []*AccountRuleObservedScope {
	if len(observed) == 0 {
		return []*AccountRuleObservedScope{}
	}
	type key struct {
		platform string
		scopeTyp string
	}
	items := make([]*AccountRuleObservedScope, 0, len(observed)*2)
	seen := make(map[key]struct{}, len(observed)*2)
	platformCounts := make(map[string]int64)

	for _, item := range observed {
		if item == nil {
			continue
		}
		item.Platform = normalizeAccountRulePlatform(item.Platform)
		item.AccountType = normalizeAccountRuleType(item.AccountType)
		if item.Platform == "" {
			continue
		}
		platformCounts[item.Platform] += item.AccountCount
		k := key{platform: item.Platform, scopeTyp: item.AccountType}
		if _, exists := seen[k]; exists {
			continue
		}
		seen[k] = struct{}{}
		items = append(items, &AccountRuleObservedScope{
			Platform:     item.Platform,
			AccountType:  item.AccountType,
			AccountCount: item.AccountCount,
		})
	}

	for platform, count := range platformCounts {
		k := key{platform: platform, scopeTyp: ""}
		if _, exists := seen[k]; exists {
			continue
		}
		items = append(items, &AccountRuleObservedScope{
			Platform:     platform,
			AccountType:  "",
			AccountCount: count,
		})
	}

	sortObservedScopes(items)
	return items
}

func sortObservedScopes(items []*AccountRuleObservedScope) {
	sort.Slice(items, func(i, j int) bool {
		a := items[i]
		b := items[j]
		if a.Platform != b.Platform {
			return a.Platform < b.Platform
		}
		if a.AccountType == "" && b.AccountType != "" {
			return true
		}
		if a.AccountType != "" && b.AccountType == "" {
			return false
		}
		return a.AccountType < b.AccountType
	})
}

func (s *AccountRuleService) GetScopeByID(ctx context.Context, id int64) (*AccountRuleScope, error) {
	scope, err := s.repo.GetScopeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if scope != nil {
		scope.Normalize()
		for _, rule := range scope.Rules {
			rule.Normalize()
		}
	}
	return scope, nil
}

func (s *AccountRuleService) CreateScope(ctx context.Context, scope *AccountRuleScope) (*AccountRuleScope, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return created, nil
}

func (s *AccountRuleService) UpdateScope(ctx context.Context, scope *AccountRuleScope) (*AccountRuleScope, error) {
	if scope == nil || scope.ID <= 0 {
		return nil, fmt.Errorf("scope id is required")
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return updated, nil
}

func (s *AccountRuleService) DeleteScope(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("scope id is required")
	}
	if err := s.repo.DeleteScope(ctx, id); err != nil {
		return err
	}
	_ = s.refreshCache(context.Background())
	return nil
}

func (s *AccountRuleService) CreateRule(ctx context.Context, rule *AccountRuleErrorRule) (*AccountRuleErrorRule, error) {
	if rule == nil || rule.ScopeID <= 0 {
		return nil, fmt.Errorf("scope_id is required")
	}
	if _, err := s.repo.GetScopeByID(ctx, rule.ScopeID); err != nil {
		return nil, err
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateRule(ctx, rule)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return created, nil
}

func (s *AccountRuleService) UpdateRule(ctx context.Context, rule *AccountRuleErrorRule) (*AccountRuleErrorRule, error) {
	if rule == nil || rule.ID <= 0 {
		return nil, fmt.Errorf("rule id is required")
	}
	existing, err := s.repo.GetRuleByID(ctx, rule.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("rule not found")
	}
	if rule.ScopeID == 0 {
		rule.ScopeID = existing.ScopeID
	}
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateRule(ctx, rule)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return updated, nil
}

func (s *AccountRuleService) DeleteRule(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("rule id is required")
	}
	if err := s.repo.DeleteRule(ctx, id); err != nil {
		return err
	}
	_ = s.refreshCache(context.Background())
	return nil
}

func (s *AccountRuleService) GetRuleByID(ctx context.Context, id int64) (*AccountRuleErrorRule, error) {
	rule, err := s.repo.GetRuleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rule != nil {
		rule.Normalize()
	}
	return rule, nil
}

func (s *AccountRuleService) GetSettings(ctx context.Context) (*AccountRuleSettings, error) {
	if s.settingService == nil || s.settingService.settingRepo == nil {
		return &AccountRuleSettings{ForwardMaxAttempts: defaultAccountRuleForwardMaxAttempts}, nil
	}
	raw, err := s.settingService.settingRepo.GetValue(ctx, SettingKeyAccountRuleForwardMaxAttempts)
	if err != nil || strings.TrimSpace(raw) == "" {
		return &AccountRuleSettings{ForwardMaxAttempts: defaultAccountRuleForwardMaxAttempts}, nil
	}
	value, parseErr := strconv.Atoi(strings.TrimSpace(raw))
	if parseErr != nil || value <= 0 {
		return &AccountRuleSettings{ForwardMaxAttempts: defaultAccountRuleForwardMaxAttempts}, nil
	}
	return &AccountRuleSettings{ForwardMaxAttempts: value}, nil
}

func (s *AccountRuleService) UpdateSettings(ctx context.Context, settings *AccountRuleSettings) (*AccountRuleSettings, error) {
	if settings == nil || settings.ForwardMaxAttempts <= 0 {
		return nil, fmt.Errorf("forward_max_attempts must be greater than 0")
	}
	if s.settingService == nil || s.settingService.settingRepo == nil {
		return nil, fmt.Errorf("setting service unavailable")
	}
	if err := s.settingService.settingRepo.Set(ctx, SettingKeyAccountRuleForwardMaxAttempts, strconv.Itoa(settings.ForwardMaxAttempts)); err != nil {
		return nil, err
	}
	return &AccountRuleSettings{ForwardMaxAttempts: settings.ForwardMaxAttempts}, nil
}

func (s *AccountRuleService) MaxForwardAttempts(ctx context.Context) int {
	settings, err := s.GetSettings(ctx)
	if err != nil || settings == nil || settings.ForwardMaxAttempts <= 0 {
		return defaultAccountRuleForwardMaxAttempts
	}
	return settings.ForwardMaxAttempts
}

func (s *AccountRuleService) MatchRuntimeRule(account *Account, statusCode int, body []byte) *AccountRuleMatch {
	if account == nil {
		return nil
	}
	snapshot := s.getCacheSnapshot(context.Background())
	if snapshot == nil || len(snapshot.scopesByKey) == 0 {
		return nil
	}

	platform := normalizeAccountRulePlatform(account.Platform)
	accountType := normalizeAccountRuleType(account.Type)
	if platform == "" {
		return nil
	}

	keys := []string{accountRuleScopeKey(platform, accountType)}
	if accountType != "" {
		keys = append(keys, accountRuleScopeKey(platform, ""))
	}

	bodyLower := ""
	bodyLowerReady := false
	for _, key := range keys {
		scope := snapshot.scopesByKey[key]
		if scope == nil || scope.scope == nil || !scope.scope.Enabled {
			continue
		}
		for _, rule := range scope.rules {
			if rule == nil || rule.rule == nil || !rule.rule.Enabled {
				continue
			}
			if matchAccountRule(rule, statusCode, body, &bodyLower, &bodyLowerReady) {
				return &AccountRuleMatch{
					Scope: scope.scope,
					Rule:  rule.rule,
				}
			}
		}
	}
	return nil
}

func matchAccountRule(rule *cachedAccountRule, statusCode int, body []byte, bodyLower *string, bodyLowerReady *bool) bool {
	codeMatched := len(rule.statusCodeSet) == 0
	if len(rule.statusCodeSet) > 0 {
		_, codeMatched = rule.statusCodeSet[statusCode]
	}

	keywordMatched := len(rule.lowerKeywords) == 0
	if len(rule.lowerKeywords) > 0 {
		if !*bodyLowerReady {
			lower := strings.ToLower(string(body))
			*bodyLower = lower
			*bodyLowerReady = true
		}
		keywordMatched = containsAnyKeyword(*bodyLower, rule.lowerKeywords)
	}

	if rule.rule.MatchMode == AccountRuleMatchModeAll {
		return codeMatched && keywordMatched
	}
	return codeMatched || keywordMatched
}

func containsAnyKeyword(body string, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(body, keyword) {
			return true
		}
	}
	return false
}

func (s *AccountRuleService) ApplyMatchedRule(ctx context.Context, account *Account, match *AccountRuleMatch, statusCode int, headers http.Header, responseBody []byte) AccountRuleActionResult {
	result := AccountRuleActionResult{}
	if account == nil || match == nil || match.Rule == nil {
		return result
	}

	result.Matched = true
	result.Rule = match.Rule
	result.SkipMonitoring = match.Rule.SkipMonitoring
	result.MaxForwardAttempts = s.MaxForwardAttempts(ctx)
	result.ShouldFailover = match.Rule.ActionFailover

	msg := strings.TrimSpace(extractUpstreamErrorMessage(responseBody))
	msg = sanitizeUpstreamErrorMessage(msg)
	if msg == "" {
		msg = fmt.Sprintf("status=%d", statusCode)
	}
	reason := fmt.Sprintf("account rule matched: %s (%s/%s): %s", match.Rule.Name, account.Platform, account.Type, msg)

	if s.accountRepo == nil {
		return result
	}
	if match.Rule.ActionDelete {
		if err := s.accountRepo.Delete(ctx, account.ID); err != nil {
			slog.Warn("account_rule_delete_failed", "account_id", account.ID, "rule_id", match.Rule.ID, "error", err)
		}
	}
	if match.Rule.ActionDisable {
		if err := s.accountRepo.SetError(ctx, account.ID, reason); err != nil {
			slog.Warn("account_rule_disable_failed", "account_id", account.ID, "rule_id", match.Rule.ID, "error", err)
		}
	}
	return result
}

func (s *AccountRuleService) ResolveScopedModelSet(account *Account) []string {
	if account == nil {
		return nil
	}
	snapshot := s.getCacheSnapshot(context.Background())
	if snapshot == nil {
		return nil
	}
	platform := normalizeAccountRulePlatform(account.Platform)
	accountType := normalizeAccountRuleType(account.Type)
	if platform == "" {
		return nil
	}

	if accountType != "" {
		if scope := snapshot.scopesByKey[accountRuleScopeKey(platform, accountType)]; scope != nil && scope.scope != nil && scope.scope.Enabled && len(scope.modelSet) > 0 {
			return append([]string(nil), scope.modelSet...)
		}
	}
	if scope := snapshot.scopesByKey[accountRuleScopeKey(platform, "")]; scope != nil && scope.scope != nil && scope.scope.Enabled && len(scope.modelSet) > 0 {
		return append([]string(nil), scope.modelSet...)
	}
	return nil
}

func (s *AccountRuleService) IsModelAllowedByScope(account *Account, requestedModel string) (bool, bool) {
	modelSet := s.ResolveScopedModelSet(account)
	if len(modelSet) == 0 {
		return false, false
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return true, true
	}
	for _, candidate := range modelSet {
		if candidate == requestedModel || matchWildcard(strings.ToLower(candidate), strings.ToLower(requestedModel)) {
			return true, true
		}
	}
	return false, true
}

func (s *AccountRuleService) CollectAvailableModels(accounts []Account) ([]string, bool) {
	modelSet := make(map[string]struct{})
	hasAny := false
	for i := range accounts {
		account := &accounts[i]
		if account == nil {
			continue
		}
		if account.HasExplicitModelMapping() {
			for model := range account.GetModelMapping() {
				modelSet[model] = struct{}{}
				hasAny = true
			}
			continue
		}
		scopeModels := s.ResolveScopedModelSet(account)
		if len(scopeModels) == 0 {
			continue
		}
		for _, model := range scopeModels {
			modelSet[model] = struct{}{}
			hasAny = true
		}
	}
	if !hasAny {
		return nil, false
	}
	out := make([]string, 0, len(modelSet))
	for model := range modelSet {
		out = append(out, model)
	}
	sort.Strings(out)
	return out, true
}

func (s *AccountRuleService) FindScopeIDByKey(ctx context.Context, platform, accountType string) (*int64, error) {
	scopes, err := s.repo.ListScopes(ctx)
	if err != nil {
		return nil, err
	}
	platform = normalizeAccountRulePlatform(platform)
	accountType = normalizeAccountRuleType(accountType)
	for _, scope := range scopes {
		if scope == nil {
			continue
		}
		if normalizeAccountRulePlatform(scope.Platform) == platform && normalizeAccountRuleType(scope.AccountType) == accountType {
			id := scope.ID
			return &id, nil
		}
	}
	return nil, nil
}

func (s *AccountRuleService) getCacheSnapshot(ctx context.Context) *accountRuleCacheSnapshot {
	s.cacheMu.RLock()
	snapshot := s.cache
	s.cacheMu.RUnlock()
	if snapshot != nil && time.Since(snapshot.loadedAt) < accountRuleCacheTTL {
		return snapshot
	}
	if err := s.refreshCache(ctx); err != nil {
		slog.Warn("account_rule_cache_refresh_failed", "error", err)
	}
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return s.cache
}

func (s *AccountRuleService) refreshCache(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	scopes, err := s.repo.ListScopes(ctx)
	if err != nil {
		return err
	}

	scopesByKey := make(map[string]*cachedAccountRuleScope, len(scopes))
	for _, scope := range scopes {
		if scope == nil {
			continue
		}
		scope.Normalize()
		cachedScope := &cachedAccountRuleScope{
			scope:    scope,
			modelSet: append([]string(nil), scope.ModelSet...),
			rules:    make([]*cachedAccountRule, 0, len(scope.Rules)),
		}
		for _, rule := range scope.Rules {
			if rule == nil {
				continue
			}
			rule.Normalize()
			cachedRule := &cachedAccountRule{
				rule:          rule,
				statusCodeSet: make(map[int]struct{}, len(rule.StatusCodes)),
				lowerKeywords: make([]string, 0, len(rule.Keywords)),
			}
			for _, code := range rule.StatusCodes {
				cachedRule.statusCodeSet[code] = struct{}{}
			}
			for _, keyword := range rule.Keywords {
				cachedRule.lowerKeywords = append(cachedRule.lowerKeywords, strings.ToLower(keyword))
			}
			cachedScope.rules = append(cachedScope.rules, cachedRule)
		}
		scopesByKey[accountRuleScopeKey(scope.Platform, scope.AccountType)] = cachedScope
	}

	s.cacheMu.Lock()
	s.cache = &accountRuleCacheSnapshot{
		loadedAt:    time.Now(),
		scopesByKey: scopesByKey,
	}
	s.cacheMu.Unlock()
	return nil
}

func accountRuleScopeKey(platform, accountType string) string {
	return normalizeAccountRulePlatform(platform) + "|" + normalizeAccountRuleType(accountType)
}

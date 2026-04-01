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
	ListBindings(ctx context.Context) ([]*AccountRuleBinding, error)
	GetBindingByID(ctx context.Context, id int64) (*AccountRuleBinding, error)
	CreateBinding(ctx context.Context, binding *AccountRuleBinding) (*AccountRuleBinding, error)
	UpdateBinding(ctx context.Context, binding *AccountRuleBinding) (*AccountRuleBinding, error)
	DeleteBinding(ctx context.Context, id int64) error

	ListModelCollections(ctx context.Context) ([]*AccountRuleModelCollection, error)
	GetModelCollectionByID(ctx context.Context, id int64) (*AccountRuleModelCollection, error)
	CreateModelCollection(ctx context.Context, collection *AccountRuleModelCollection) (*AccountRuleModelCollection, error)
	UpdateModelCollection(ctx context.Context, collection *AccountRuleModelCollection) (*AccountRuleModelCollection, error)
	DeleteModelCollection(ctx context.Context, id int64) error

	ListErrorCollections(ctx context.Context) ([]*AccountRuleErrorCollection, error)
	GetErrorCollectionByID(ctx context.Context, id int64) (*AccountRuleErrorCollection, error)
	CreateErrorCollection(ctx context.Context, collection *AccountRuleErrorCollection) (*AccountRuleErrorCollection, error)
	UpdateErrorCollection(ctx context.Context, collection *AccountRuleErrorCollection) (*AccountRuleErrorCollection, error)
	DeleteErrorCollection(ctx context.Context, id int64) error

	GetRuleByID(ctx context.Context, id int64) (*AccountRuleErrorRule, error)
	CreateRule(ctx context.Context, rule *AccountRuleErrorRule) (*AccountRuleErrorRule, error)
	UpdateRule(ctx context.Context, rule *AccountRuleErrorRule) (*AccountRuleErrorRule, error)
	DeleteRule(ctx context.Context, id int64) error

	ListObservedBindings(ctx context.Context) ([]*AccountRuleObservedBinding, error)
}

type cachedAccountRuleBinding struct {
	binding  *AccountRuleBinding
	modelSet []string
	rules    []*cachedAccountRule
}

type cachedAccountRule struct {
	rule          *AccountRuleErrorRule
	statusCodeSet map[int]struct{}
	lowerKeywords []string
}

type accountRuleCacheSnapshot struct {
	loadedAt      time.Time
	bindingsByKey map[string]*cachedAccountRuleBinding
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
	bindings, err := s.repo.ListBindings(ctx)
	if err != nil {
		return nil, err
	}
	modelCollections, err := s.repo.ListModelCollections(ctx)
	if err != nil {
		return nil, err
	}
	errorCollections, err := s.repo.ListErrorCollections(ctx)
	if err != nil {
		return nil, err
	}
	observed, err := s.repo.ListObservedBindings(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	for _, binding := range bindings {
		binding.Normalize()
	}
	for _, collection := range modelCollections {
		collection.Normalize()
	}
	for _, collection := range errorCollections {
		collection.Normalize()
		for _, rule := range collection.Rules {
			rule.Normalize()
		}
	}

	return &AccountRuleCatalog{
		Bindings:         bindings,
		ModelCollections: modelCollections,
		ErrorCollections: errorCollections,
		ObservedBindings: synthesizePlatformObservedBindings(observed),
		Settings:         *settings,
	}, nil
}

func synthesizePlatformObservedBindings(observed []*AccountRuleObservedBinding) []*AccountRuleObservedBinding {
	if len(observed) == 0 {
		return []*AccountRuleObservedBinding{}
	}
	type key struct {
		platform     string
		businessType string
	}
	items := make([]*AccountRuleObservedBinding, 0, len(observed)*2)
	itemByKey := make(map[key]*AccountRuleObservedBinding, len(observed)*2)
	platformCounts := make(map[string]int64)

	for _, item := range observed {
		if item == nil {
			continue
		}
		item.Platform = normalizeAccountRulePlatform(item.Platform)
		item.BusinessType = normalizeAccountRuleType(item.BusinessType)
		if item.Platform == "" {
			continue
		}
		platformCounts[item.Platform] += item.AccountCount
		k := key{platform: item.Platform, businessType: item.BusinessType}
		if existing, exists := itemByKey[k]; exists {
			existing.AccountCount += item.AccountCount
			continue
		}
		copied := &AccountRuleObservedBinding{
			Platform:     item.Platform,
			BusinessType: item.BusinessType,
			AccountCount: item.AccountCount,
		}
		itemByKey[k] = copied
		items = append(items, copied)
	}

	for platform, count := range platformCounts {
		k := key{platform: platform, businessType: ""}
		if existing, exists := itemByKey[k]; exists {
			existing.AccountCount = count
			continue
		}
		copied := &AccountRuleObservedBinding{
			Platform:     platform,
			BusinessType: "",
			AccountCount: count,
		}
		itemByKey[k] = copied
		items = append(items, copied)
	}

	sortObservedBindings(items)
	return items
}

func sortObservedBindings(items []*AccountRuleObservedBinding) {
	sort.Slice(items, func(i, j int) bool {
		a := items[i]
		b := items[j]
		if a.Platform != b.Platform {
			return a.Platform < b.Platform
		}
		if a.BusinessType == "" && b.BusinessType != "" {
			return true
		}
		if a.BusinessType != "" && b.BusinessType == "" {
			return false
		}
		return a.BusinessType < b.BusinessType
	})
}

func (s *AccountRuleService) GetBindingByID(ctx context.Context, id int64) (*AccountRuleBinding, error) {
	binding, err := s.repo.GetBindingByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if binding != nil {
		binding.Normalize()
	}
	return binding, nil
}

func (s *AccountRuleService) CreateBinding(ctx context.Context, binding *AccountRuleBinding) (*AccountRuleBinding, error) {
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateBinding(ctx, binding)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return created, nil
}

func (s *AccountRuleService) UpdateBinding(ctx context.Context, binding *AccountRuleBinding) (*AccountRuleBinding, error) {
	if binding == nil || binding.ID <= 0 {
		return nil, fmt.Errorf("binding id is required")
	}
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateBinding(ctx, binding)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return updated, nil
}

func (s *AccountRuleService) DeleteBinding(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("binding id is required")
	}
	if err := s.repo.DeleteBinding(ctx, id); err != nil {
		return err
	}
	_ = s.refreshCache(context.Background())
	return nil
}

func (s *AccountRuleService) GetModelCollectionByID(ctx context.Context, id int64) (*AccountRuleModelCollection, error) {
	collection, err := s.repo.GetModelCollectionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if collection != nil {
		collection.Normalize()
	}
	return collection, nil
}

func (s *AccountRuleService) CreateModelCollection(ctx context.Context, collection *AccountRuleModelCollection) (*AccountRuleModelCollection, error) {
	if err := collection.Validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateModelCollection(ctx, collection)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return created, nil
}

func (s *AccountRuleService) UpdateModelCollection(ctx context.Context, collection *AccountRuleModelCollection) (*AccountRuleModelCollection, error) {
	if collection == nil || collection.ID <= 0 {
		return nil, fmt.Errorf("model collection id is required")
	}
	if err := collection.Validate(); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateModelCollection(ctx, collection)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return updated, nil
}

func (s *AccountRuleService) DeleteModelCollection(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("model collection id is required")
	}
	if err := s.repo.DeleteModelCollection(ctx, id); err != nil {
		return err
	}
	_ = s.refreshCache(context.Background())
	return nil
}

func (s *AccountRuleService) GetErrorCollectionByID(ctx context.Context, id int64) (*AccountRuleErrorCollection, error) {
	collection, err := s.repo.GetErrorCollectionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if collection != nil {
		collection.Normalize()
		for _, rule := range collection.Rules {
			rule.Normalize()
		}
	}
	return collection, nil
}

func (s *AccountRuleService) CreateErrorCollection(ctx context.Context, collection *AccountRuleErrorCollection) (*AccountRuleErrorCollection, error) {
	if err := collection.Validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateErrorCollection(ctx, collection)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return created, nil
}

func (s *AccountRuleService) UpdateErrorCollection(ctx context.Context, collection *AccountRuleErrorCollection) (*AccountRuleErrorCollection, error) {
	if collection == nil || collection.ID <= 0 {
		return nil, fmt.Errorf("error collection id is required")
	}
	if err := collection.Validate(); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateErrorCollection(ctx, collection)
	if err != nil {
		return nil, err
	}
	_ = s.refreshCache(context.Background())
	return updated, nil
}

func (s *AccountRuleService) DeleteErrorCollection(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("error collection id is required")
	}
	if err := s.repo.DeleteErrorCollection(ctx, id); err != nil {
		return err
	}
	_ = s.refreshCache(context.Background())
	return nil
}

func (s *AccountRuleService) CreateRule(ctx context.Context, rule *AccountRuleErrorRule) (*AccountRuleErrorRule, error) {
	if rule == nil || rule.ErrorCollectionID <= 0 {
		return nil, fmt.Errorf("error_collection_id is required")
	}
	if _, err := s.repo.GetErrorCollectionByID(ctx, rule.ErrorCollectionID); err != nil {
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
	if rule.ErrorCollectionID == 0 {
		rule.ErrorCollectionID = existing.ErrorCollectionID
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

func defaultAccountRuleSettings() *AccountRuleSettings {
	return &AccountRuleSettings{
		ForwardMaxAttempts: defaultAccountRuleForwardMaxAttempts,
		FailoverOn429:      true,
	}
}

func (s *AccountRuleService) GetSettings(ctx context.Context) (*AccountRuleSettings, error) {
	if s.settingService == nil || s.settingService.settingRepo == nil {
		return defaultAccountRuleSettings(), nil
	}

	settings := defaultAccountRuleSettings()

	rawAttempts, err := s.settingService.settingRepo.GetValue(ctx, SettingKeyAccountRuleForwardMaxAttempts)
	if err == nil && strings.TrimSpace(rawAttempts) != "" {
		if value, parseErr := strconv.Atoi(strings.TrimSpace(rawAttempts)); parseErr == nil && value > 0 {
			settings.ForwardMaxAttempts = value
		}
	}

	rawFailover, err := s.settingService.settingRepo.GetValue(ctx, SettingKeyAccountRuleFailoverOn429Enabled)
	if err == nil && strings.TrimSpace(rawFailover) != "" {
		if value, parseErr := strconv.ParseBool(strings.TrimSpace(rawFailover)); parseErr == nil {
			settings.FailoverOn429 = value
		}
	}

	return settings, nil
}

func (s *AccountRuleService) UpdateSettings(ctx context.Context, settings *AccountRuleSettings) (*AccountRuleSettings, error) {
	if settings == nil || settings.ForwardMaxAttempts <= 0 {
		return nil, fmt.Errorf("forward_max_attempts must be greater than 0")
	}
	if s.settingService == nil || s.settingService.settingRepo == nil {
		return nil, fmt.Errorf("setting service unavailable")
	}
	if err := s.settingService.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyAccountRuleForwardMaxAttempts:   strconv.Itoa(settings.ForwardMaxAttempts),
		SettingKeyAccountRuleFailoverOn429Enabled: strconv.FormatBool(settings.FailoverOn429),
	}); err != nil {
		return nil, err
	}
	return &AccountRuleSettings{
		ForwardMaxAttempts: settings.ForwardMaxAttempts,
		FailoverOn429:      settings.FailoverOn429,
	}, nil
}

func (s *AccountRuleService) MaxForwardAttempts(ctx context.Context) int {
	settings, err := s.GetSettings(ctx)
	if err != nil || settings == nil || settings.ForwardMaxAttempts <= 0 {
		return defaultAccountRuleForwardMaxAttempts
	}
	return settings.ForwardMaxAttempts
}

func (s *AccountRuleService) ShouldFailoverOn429(ctx context.Context) bool {
	settings, err := s.GetSettings(ctx)
	if err != nil || settings == nil {
		return true
	}
	return settings.FailoverOn429
}

func (s *AccountRuleService) MatchRuntimeRule(account *Account, statusCode int, body []byte) *AccountRuleMatch {
	if account == nil {
		return nil
	}
	snapshot := s.getCacheSnapshot(context.Background())
	if snapshot == nil || len(snapshot.bindingsByKey) == 0 {
		return nil
	}

	platform := normalizeAccountRulePlatform(account.Platform)
	businessType := account.AccountRuleScopeType()
	if platform == "" {
		return nil
	}

	keys := []string{accountRuleBindingKey(platform, businessType)}
	if businessType != "" {
		keys = append(keys, accountRuleBindingKey(platform, ""))
	}

	bodyLower := ""
	bodyLowerReady := false
	for _, key := range keys {
		binding := snapshot.bindingsByKey[key]
		if binding == nil || binding.binding == nil || !binding.binding.Enabled {
			continue
		}
		for _, rule := range binding.rules {
			if rule == nil || rule.rule == nil || !rule.rule.Enabled {
				continue
			}
			if matchAccountRule(rule, statusCode, body, &bodyLower, &bodyLowerReady) {
				return &AccountRuleMatch{
					Binding: binding.binding,
					Rule:    rule.rule,
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
	businessType := account.AccountRuleScopeType()
	if businessType == "" {
		businessType = "platform"
	}
	reason := fmt.Sprintf("account rule matched: %s (%s/%s): %s", match.Rule.Name, account.Platform, businessType, msg)

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
	businessType := account.AccountRuleScopeType()
	if platform == "" {
		return nil
	}

	if businessType != "" {
		if binding := snapshot.bindingsByKey[accountRuleBindingKey(platform, businessType)]; binding != nil && binding.binding != nil && binding.binding.Enabled && len(binding.modelSet) > 0 {
			return append([]string(nil), binding.modelSet...)
		}
	}
	if binding := snapshot.bindingsByKey[accountRuleBindingKey(platform, "")]; binding != nil && binding.binding != nil && binding.binding.Enabled && len(binding.modelSet) > 0 {
		return append([]string(nil), binding.modelSet...)
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

func (s *AccountRuleService) FindBindingByKey(ctx context.Context, platform, businessType string) (*AccountRuleBinding, error) {
	snapshot := s.getCacheSnapshot(ctx)
	if snapshot == nil {
		return nil, nil
	}
	binding := snapshot.bindingsByKey[accountRuleBindingKey(platform, businessType)]
	if binding == nil || binding.binding == nil {
		return nil, nil
	}
	copied := *binding.binding
	return &copied, nil
}

func (s *AccountRuleService) FindEffectiveBinding(ctx context.Context, platform, businessType string) (*AccountRuleBinding, error) {
	binding, err := s.FindBindingByKey(ctx, platform, businessType)
	if err != nil || binding != nil {
		return binding, err
	}
	if normalizeAccountRuleType(businessType) == "" {
		return nil, nil
	}
	return s.FindBindingByKey(ctx, platform, "")
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
	bindings, err := s.repo.ListBindings(ctx)
	if err != nil {
		return err
	}
	modelCollections, err := s.repo.ListModelCollections(ctx)
	if err != nil {
		return err
	}
	errorCollections, err := s.repo.ListErrorCollections(ctx)
	if err != nil {
		return err
	}

	modelsByCollectionID := make(map[int64][]string, len(modelCollections))
	for _, collection := range modelCollections {
		if collection == nil {
			continue
		}
		collection.Normalize()
		modelsByCollectionID[collection.ID] = append([]string(nil), collection.Models...)
	}

	rulesByCollectionID := make(map[int64][]*cachedAccountRule, len(errorCollections))
	for _, collection := range errorCollections {
		if collection == nil {
			continue
		}
		collection.Normalize()
		cachedRules := make([]*cachedAccountRule, 0, len(collection.Rules))
		for _, rule := range collection.Rules {
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
			cachedRules = append(cachedRules, cachedRule)
		}
		rulesByCollectionID[collection.ID] = cachedRules
	}

	bindingsByKey := make(map[string]*cachedAccountRuleBinding, len(bindings))
	for _, binding := range bindings {
		if binding == nil {
			continue
		}
		binding.Normalize()
		cachedBinding := &cachedAccountRuleBinding{
			binding: binding,
		}
		if binding.ModelCollectionID != nil {
			cachedBinding.modelSet = append([]string(nil), modelsByCollectionID[*binding.ModelCollectionID]...)
		}
		if binding.ErrorCollectionID != nil {
			cachedBinding.rules = append([]*cachedAccountRule(nil), rulesByCollectionID[*binding.ErrorCollectionID]...)
		}
		bindingsByKey[accountRuleBindingKey(binding.Platform, binding.BusinessType)] = cachedBinding
	}

	s.cacheMu.Lock()
	s.cache = &accountRuleCacheSnapshot{
		loadedAt:      time.Now(),
		bindingsByKey: bindingsByKey,
	}
	s.cacheMu.Unlock()
	return nil
}

func accountRuleBindingKey(platform, businessType string) string {
	return normalizeAccountRulePlatform(platform) + "|" + normalizeAccountRuleType(businessType)
}

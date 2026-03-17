package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type accountRuleRepository struct {
	db *sql.DB
}

func NewAccountRuleRepository(db *sql.DB) service.AccountRuleRepository {
	return &accountRuleRepository{db: db}
}

func (r *accountRuleRepository) ListScopes(ctx context.Context) ([]*service.AccountRuleScope, error) {
	scopeRows, err := r.db.QueryContext(ctx, `
SELECT id, platform, account_type, enabled, model_set, COALESCE(description, ''), created_at, updated_at
FROM account_rule_scopes
ORDER BY platform ASC, account_type ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = scopeRows.Close() }()

	scopes := make([]*service.AccountRuleScope, 0)
	scopeByID := make(map[int64]*service.AccountRuleScope)
	scopeIDs := make([]int64, 0)

	for scopeRows.Next() {
		scope, err := scanAccountRuleScope(scopeRows)
		if err != nil {
			return nil, err
		}
		scope.Rules = []*service.AccountRuleErrorRule{}
		scopes = append(scopes, scope)
		scopeByID[scope.ID] = scope
		scopeIDs = append(scopeIDs, scope.ID)
	}
	if err := scopeRows.Err(); err != nil {
		return nil, err
	}
	if len(scopeIDs) == 0 {
		return scopes, nil
	}

	ruleRows, err := r.db.QueryContext(ctx, `
SELECT id, scope_id, name, enabled, priority, status_codes, keywords, match_mode,
       action_disable, action_failover, action_delete, action_override,
       passthrough_code, response_code, passthrough_body, custom_message,
       skip_monitoring, COALESCE(description, ''), COALESCE(sample_response, ''),
       created_at, updated_at
FROM account_rule_error_rules
WHERE scope_id = ANY($1)
ORDER BY scope_id ASC, priority ASC, id ASC`, pq.Array(scopeIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = ruleRows.Close() }()

	for ruleRows.Next() {
		rule, err := scanAccountRuleErrorRule(ruleRows)
		if err != nil {
			return nil, err
		}
		scope := scopeByID[rule.ScopeID]
		if scope == nil {
			continue
		}
		scope.Rules = append(scope.Rules, rule)
	}
	if err := ruleRows.Err(); err != nil {
		return nil, err
	}

	return scopes, nil
}

func (r *accountRuleRepository) GetScopeByID(ctx context.Context, id int64) (*service.AccountRuleScope, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, platform, account_type, enabled, model_set, COALESCE(description, ''), created_at, updated_at
FROM account_rule_scopes
WHERE id = $1`, id)
	scope, err := scanAccountRuleScope(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	ruleRows, err := r.db.QueryContext(ctx, `
SELECT id, scope_id, name, enabled, priority, status_codes, keywords, match_mode,
       action_disable, action_failover, action_delete, action_override,
       passthrough_code, response_code, passthrough_body, custom_message,
       skip_monitoring, COALESCE(description, ''), COALESCE(sample_response, ''),
       created_at, updated_at
FROM account_rule_error_rules
WHERE scope_id = $1
ORDER BY priority ASC, id ASC`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ruleRows.Close() }()

	scope.Rules = make([]*service.AccountRuleErrorRule, 0)
	for ruleRows.Next() {
		rule, err := scanAccountRuleErrorRule(ruleRows)
		if err != nil {
			return nil, err
		}
		scope.Rules = append(scope.Rules, rule)
	}
	if err := ruleRows.Err(); err != nil {
		return nil, err
	}

	return scope, nil
}

func (r *accountRuleRepository) CreateScope(ctx context.Context, scope *service.AccountRuleScope) (*service.AccountRuleScope, error) {
	modelSet, err := marshalJSONString(scope.ModelSet)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
INSERT INTO account_rule_scopes (platform, account_type, enabled, model_set, description, created_at, updated_at)
VALUES ($1, $2, $3, $4::jsonb, $5, NOW(), NOW())
ON CONFLICT (platform, account_type)
DO UPDATE SET
  enabled = EXCLUDED.enabled,
  model_set = EXCLUDED.model_set,
  description = EXCLUDED.description,
  updated_at = NOW()
RETURNING id, platform, account_type, enabled, model_set, COALESCE(description, ''), created_at, updated_at`,
		scope.Platform,
		scope.AccountType,
		scope.Enabled,
		modelSet,
		nullIfEmpty(scope.Description),
	)
	created, scanErr := scanAccountRuleScope(row)
	if scanErr != nil {
		return nil, scanErr
	}
	created.Rules = []*service.AccountRuleErrorRule{}
	return created, nil
}

func (r *accountRuleRepository) UpdateScope(ctx context.Context, scope *service.AccountRuleScope) (*service.AccountRuleScope, error) {
	modelSet, err := marshalJSONString(scope.ModelSet)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
UPDATE account_rule_scopes
SET platform = $2,
    account_type = $3,
    enabled = $4,
    model_set = $5::jsonb,
    description = $6,
    updated_at = NOW()
WHERE id = $1
RETURNING id, platform, account_type, enabled, model_set, COALESCE(description, ''), created_at, updated_at`,
		scope.ID,
		scope.Platform,
		scope.AccountType,
		scope.Enabled,
		modelSet,
		nullIfEmpty(scope.Description),
	)
	updated, scanErr := scanAccountRuleScope(row)
	if scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return nil, nil
		}
		return nil, scanErr
	}
	updated.Rules = scope.Rules
	return updated, nil
}

func (r *accountRuleRepository) DeleteScope(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM account_rule_scopes WHERE id = $1`, id)
	return err
}

func (r *accountRuleRepository) GetRuleByID(ctx context.Context, id int64) (*service.AccountRuleErrorRule, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, scope_id, name, enabled, priority, status_codes, keywords, match_mode,
       action_disable, action_failover, action_delete, action_override,
       passthrough_code, response_code, passthrough_body, custom_message,
       skip_monitoring, COALESCE(description, ''), COALESCE(sample_response, ''),
       created_at, updated_at
FROM account_rule_error_rules
WHERE id = $1`, id)
	rule, err := scanAccountRuleErrorRule(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return rule, nil
}

func (r *accountRuleRepository) CreateRule(ctx context.Context, rule *service.AccountRuleErrorRule) (*service.AccountRuleErrorRule, error) {
	statusCodes, err := marshalJSONString(rule.StatusCodes)
	if err != nil {
		return nil, err
	}
	keywords, err := marshalJSONString(rule.Keywords)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
INSERT INTO account_rule_error_rules (
  scope_id, name, enabled, priority, status_codes, keywords, match_mode,
  action_disable, action_failover, action_delete, action_override,
  passthrough_code, response_code, passthrough_body, custom_message,
  skip_monitoring, description, sample_response, created_at, updated_at
)
VALUES (
  $1, $2, $3, $4, $5::jsonb, $6::jsonb, $7,
  $8, $9, $10, $11,
  $12, $13, $14, $15,
  $16, $17, $18, NOW(), NOW()
)
RETURNING id, scope_id, name, enabled, priority, status_codes, keywords, match_mode,
          action_disable, action_failover, action_delete, action_override,
          passthrough_code, response_code, passthrough_body, custom_message,
          skip_monitoring, COALESCE(description, ''), COALESCE(sample_response, ''),
          created_at, updated_at`,
		rule.ScopeID,
		rule.Name,
		rule.Enabled,
		rule.Priority,
		statusCodes,
		keywords,
		rule.MatchMode,
		rule.ActionDisable,
		rule.ActionFailover,
		rule.ActionDelete,
		rule.ActionOverride,
		rule.PassthroughCode,
		rule.ResponseCode,
		rule.PassthroughBody,
		nullIfBlankPtr(rule.CustomMessage),
		rule.SkipMonitoring,
		nullIfEmpty(rule.Description),
		nullIfEmpty(rule.SampleResponse),
	)
	return scanAccountRuleErrorRule(row)
}

func (r *accountRuleRepository) UpdateRule(ctx context.Context, rule *service.AccountRuleErrorRule) (*service.AccountRuleErrorRule, error) {
	statusCodes, err := marshalJSONString(rule.StatusCodes)
	if err != nil {
		return nil, err
	}
	keywords, err := marshalJSONString(rule.Keywords)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
UPDATE account_rule_error_rules
SET name = $2,
    enabled = $3,
    priority = $4,
    status_codes = $5::jsonb,
    keywords = $6::jsonb,
    match_mode = $7,
    action_disable = $8,
    action_failover = $9,
    action_delete = $10,
    action_override = $11,
    passthrough_code = $12,
    response_code = $13,
    passthrough_body = $14,
    custom_message = $15,
    skip_monitoring = $16,
    description = $17,
    sample_response = $18,
    updated_at = NOW()
WHERE id = $1
RETURNING id, scope_id, name, enabled, priority, status_codes, keywords, match_mode,
          action_disable, action_failover, action_delete, action_override,
          passthrough_code, response_code, passthrough_body, custom_message,
          skip_monitoring, COALESCE(description, ''), COALESCE(sample_response, ''),
          created_at, updated_at`,
		rule.ID,
		rule.Name,
		rule.Enabled,
		rule.Priority,
		statusCodes,
		keywords,
		rule.MatchMode,
		rule.ActionDisable,
		rule.ActionFailover,
		rule.ActionDelete,
		rule.ActionOverride,
		rule.PassthroughCode,
		rule.ResponseCode,
		rule.PassthroughBody,
		nullIfBlankPtr(rule.CustomMessage),
		rule.SkipMonitoring,
		nullIfEmpty(rule.Description),
		nullIfEmpty(rule.SampleResponse),
	)
	updated, scanErr := scanAccountRuleErrorRule(row)
	if scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return nil, nil
		}
		return nil, scanErr
	}
	return updated, nil
}

func (r *accountRuleRepository) DeleteRule(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM account_rule_error_rules WHERE id = $1`, id)
	return err
}

func (r *accountRuleRepository) ListObservedScopes(ctx context.Context) ([]*service.AccountRuleObservedScope, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT platform,
       COALESCE(NULLIF(type, ''), ''),
       COALESCE(credentials, '{}'::jsonb),
       COALESCE(extra, '{}'::jsonb)
FROM accounts
ORDER BY platform ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	type key struct {
		platform    string
		accountType string
	}
	counts := make(map[key]*service.AccountRuleObservedScope)
	for rows.Next() {
		var (
			platform        string
			accountType     string
			credentialsJSON []byte
			extraJSON       []byte
		)
		if err := rows.Scan(&platform, &accountType, &credentialsJSON, &extraJSON); err != nil {
			return nil, err
		}

		account := &service.Account{
			Platform: strings.TrimSpace(platform),
			Type:     strings.TrimSpace(accountType),
		}
		if account.Credentials, err = unmarshalObservedAccountJSONMap(credentialsJSON); err != nil {
			return nil, fmt.Errorf("unmarshal observed scope credentials: %w", err)
		}
		if account.Extra, err = unmarshalObservedAccountJSONMap(extraJSON); err != nil {
			return nil, fmt.Errorf("unmarshal observed scope extra: %w", err)
		}

		k := key{
			platform:    strings.ToLower(strings.TrimSpace(account.Platform)),
			accountType: account.AccountRuleScopeType(),
		}
		if k.platform == "" {
			continue
		}
		if existing, ok := counts[k]; ok {
			existing.AccountCount++
			continue
		}
		counts[k] = &service.AccountRuleObservedScope{
			Platform:     k.platform,
			AccountType:  k.accountType,
			AccountCount: 1,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items := make([]*service.AccountRuleObservedScope, 0, len(counts))
	for _, item := range counts {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Platform != items[j].Platform {
			return items[i].Platform < items[j].Platform
		}
		return items[i].AccountType < items[j].AccountType
	})
	return items, nil
}

func unmarshalObservedAccountJSONMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type accountRuleScopeScanner interface {
	Scan(dest ...any) error
}

func scanAccountRuleScope(scanner accountRuleScopeScanner) (*service.AccountRuleScope, error) {
	var (
		scope        service.AccountRuleScope
		modelSetJSON []byte
	)
	if err := scanner.Scan(
		&scope.ID,
		&scope.Platform,
		&scope.AccountType,
		&scope.Enabled,
		&modelSetJSON,
		&scope.Description,
		&scope.CreatedAt,
		&scope.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(modelSetJSON, &scope.ModelSet); err != nil {
		return nil, fmt.Errorf("unmarshal scope model_set: %w", err)
	}
	if scope.ModelSet == nil {
		scope.ModelSet = []string{}
	}
	return &scope, nil
}

type accountRuleErrorRuleScanner interface {
	Scan(dest ...any) error
}

func scanAccountRuleErrorRule(scanner accountRuleErrorRuleScanner) (*service.AccountRuleErrorRule, error) {
	var (
		rule            service.AccountRuleErrorRule
		statusCodesJSON []byte
		keywordsJSON    []byte
		customMessage   sql.NullString
		responseCode    sql.NullInt64
	)
	if err := scanner.Scan(
		&rule.ID,
		&rule.ScopeID,
		&rule.Name,
		&rule.Enabled,
		&rule.Priority,
		&statusCodesJSON,
		&keywordsJSON,
		&rule.MatchMode,
		&rule.ActionDisable,
		&rule.ActionFailover,
		&rule.ActionDelete,
		&rule.ActionOverride,
		&rule.PassthroughCode,
		&responseCode,
		&rule.PassthroughBody,
		&customMessage,
		&rule.SkipMonitoring,
		&rule.Description,
		&rule.SampleResponse,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(statusCodesJSON, &rule.StatusCodes); err != nil {
		return nil, fmt.Errorf("unmarshal rule status_codes: %w", err)
	}
	if err := json.Unmarshal(keywordsJSON, &rule.Keywords); err != nil {
		return nil, fmt.Errorf("unmarshal rule keywords: %w", err)
	}
	if responseCode.Valid {
		v := int(responseCode.Int64)
		rule.ResponseCode = &v
	}
	if customMessage.Valid {
		msg := customMessage.String
		rule.CustomMessage = &msg
	}
	if rule.StatusCodes == nil {
		rule.StatusCodes = []int{}
	}
	if rule.Keywords == nil {
		rule.Keywords = []string{}
	}
	return &rule, nil
}

func marshalJSONString(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.TrimSpace(v)
}

func nullIfBlankPtr(v *string) any {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

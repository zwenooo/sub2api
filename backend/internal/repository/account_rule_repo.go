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

func (r *accountRuleRepository) ListBindings(ctx context.Context) ([]*service.AccountRuleBinding, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, platform, business_type, enabled, model_collection_id, error_collection_id,
       COALESCE(description, ''), created_at, updated_at
FROM account_rule_collection_bindings
ORDER BY platform ASC, business_type ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*service.AccountRuleBinding, 0)
	for rows.Next() {
		item, scanErr := scanAccountRuleBinding(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *accountRuleRepository) GetBindingByID(ctx context.Context, id int64) (*service.AccountRuleBinding, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, platform, business_type, enabled, model_collection_id, error_collection_id,
       COALESCE(description, ''), created_at, updated_at
FROM account_rule_collection_bindings
WHERE id = $1`, id)
	item, err := scanAccountRuleBinding(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (r *accountRuleRepository) CreateBinding(ctx context.Context, binding *service.AccountRuleBinding) (*service.AccountRuleBinding, error) {
	row := r.db.QueryRowContext(ctx, `
INSERT INTO account_rule_collection_bindings (
  platform, business_type, enabled, model_collection_id, error_collection_id, description, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (platform, business_type)
DO UPDATE SET
  enabled = EXCLUDED.enabled,
  model_collection_id = EXCLUDED.model_collection_id,
  error_collection_id = EXCLUDED.error_collection_id,
  description = EXCLUDED.description,
  updated_at = NOW()
RETURNING id, platform, business_type, enabled, model_collection_id, error_collection_id,
          COALESCE(description, ''), created_at, updated_at`,
		binding.Platform,
		binding.BusinessType,
		binding.Enabled,
		binding.ModelCollectionID,
		binding.ErrorCollectionID,
		nullIfEmpty(binding.Description),
	)
	return scanAccountRuleBinding(row)
}

func (r *accountRuleRepository) UpdateBinding(ctx context.Context, binding *service.AccountRuleBinding) (*service.AccountRuleBinding, error) {
	row := r.db.QueryRowContext(ctx, `
UPDATE account_rule_collection_bindings
SET platform = $2,
    business_type = $3,
    enabled = $4,
    model_collection_id = $5,
    error_collection_id = $6,
    description = $7,
    updated_at = NOW()
WHERE id = $1
RETURNING id, platform, business_type, enabled, model_collection_id, error_collection_id,
          COALESCE(description, ''), created_at, updated_at`,
		binding.ID,
		binding.Platform,
		binding.BusinessType,
		binding.Enabled,
		binding.ModelCollectionID,
		binding.ErrorCollectionID,
		nullIfEmpty(binding.Description),
	)
	item, err := scanAccountRuleBinding(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (r *accountRuleRepository) DeleteBinding(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM account_rule_collection_bindings WHERE id = $1`, id)
	return err
}

func (r *accountRuleRepository) ListModelCollections(ctx context.Context) ([]*service.AccountRuleModelCollection, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, models, COALESCE(description, ''), created_at, updated_at
FROM account_rule_model_collections
ORDER BY name ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*service.AccountRuleModelCollection, 0)
	for rows.Next() {
		item, scanErr := scanAccountRuleModelCollection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *accountRuleRepository) GetModelCollectionByID(ctx context.Context, id int64) (*service.AccountRuleModelCollection, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, models, COALESCE(description, ''), created_at, updated_at
FROM account_rule_model_collections
WHERE id = $1`, id)
	item, err := scanAccountRuleModelCollection(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (r *accountRuleRepository) CreateModelCollection(ctx context.Context, collection *service.AccountRuleModelCollection) (*service.AccountRuleModelCollection, error) {
	models, err := marshalJSONString(collection.Models)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
INSERT INTO account_rule_model_collections (name, models, description, created_at, updated_at)
VALUES ($1, $2::jsonb, $3, NOW(), NOW())
RETURNING id, name, models, COALESCE(description, ''), created_at, updated_at`,
		collection.Name,
		models,
		nullIfEmpty(collection.Description),
	)
	return scanAccountRuleModelCollection(row)
}

func (r *accountRuleRepository) UpdateModelCollection(ctx context.Context, collection *service.AccountRuleModelCollection) (*service.AccountRuleModelCollection, error) {
	models, err := marshalJSONString(collection.Models)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
UPDATE account_rule_model_collections
SET name = $2,
    models = $3::jsonb,
    description = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, models, COALESCE(description, ''), created_at, updated_at`,
		collection.ID,
		collection.Name,
		models,
		nullIfEmpty(collection.Description),
	)
	item, scanErr := scanAccountRuleModelCollection(row)
	if scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return nil, nil
		}
		return nil, scanErr
	}
	return item, nil
}

func (r *accountRuleRepository) DeleteModelCollection(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM account_rule_model_collections WHERE id = $1`, id)
	return err
}

func (r *accountRuleRepository) ListErrorCollections(ctx context.Context) ([]*service.AccountRuleErrorCollection, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, COALESCE(description, ''), created_at, updated_at
FROM account_rule_error_collections
ORDER BY name ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*service.AccountRuleErrorCollection, 0)
	collectionByID := make(map[int64]*service.AccountRuleErrorCollection)
	collectionIDs := make([]int64, 0)
	for rows.Next() {
		item, scanErr := scanAccountRuleErrorCollection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		item.Rules = []*service.AccountRuleErrorRule{}
		items = append(items, item)
		collectionByID[item.ID] = item
		collectionIDs = append(collectionIDs, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(collectionIDs) == 0 {
		return items, nil
	}

	ruleRows, err := r.db.QueryContext(ctx, `
SELECT id, error_collection_id, name, enabled, priority, status_codes, keywords, match_mode,
       action_disable, action_failover, action_delete, action_override,
       passthrough_code, response_code, passthrough_body, custom_message,
       skip_monitoring, COALESCE(description, ''), COALESCE(sample_response, ''),
       created_at, updated_at
FROM account_rule_error_collection_rules
WHERE error_collection_id = ANY($1)
ORDER BY error_collection_id ASC, priority ASC, id ASC`, pq.Array(collectionIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = ruleRows.Close() }()

	for ruleRows.Next() {
		rule, scanErr := scanAccountRuleErrorRule(ruleRows)
		if scanErr != nil {
			return nil, scanErr
		}
		collection := collectionByID[rule.ErrorCollectionID]
		if collection == nil {
			continue
		}
		collection.Rules = append(collection.Rules, rule)
	}
	if err := ruleRows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *accountRuleRepository) GetErrorCollectionByID(ctx context.Context, id int64) (*service.AccountRuleErrorCollection, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, COALESCE(description, ''), created_at, updated_at
FROM account_rule_error_collections
WHERE id = $1`, id)
	collection, err := scanAccountRuleErrorCollection(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	ruleRows, err := r.db.QueryContext(ctx, `
SELECT id, error_collection_id, name, enabled, priority, status_codes, keywords, match_mode,
       action_disable, action_failover, action_delete, action_override,
       passthrough_code, response_code, passthrough_body, custom_message,
       skip_monitoring, COALESCE(description, ''), COALESCE(sample_response, ''),
       created_at, updated_at
FROM account_rule_error_collection_rules
WHERE error_collection_id = $1
ORDER BY priority ASC, id ASC`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ruleRows.Close() }()

	collection.Rules = make([]*service.AccountRuleErrorRule, 0)
	for ruleRows.Next() {
		rule, scanErr := scanAccountRuleErrorRule(ruleRows)
		if scanErr != nil {
			return nil, scanErr
		}
		collection.Rules = append(collection.Rules, rule)
	}
	if err := ruleRows.Err(); err != nil {
		return nil, err
	}
	return collection, nil
}

func (r *accountRuleRepository) CreateErrorCollection(ctx context.Context, collection *service.AccountRuleErrorCollection) (*service.AccountRuleErrorCollection, error) {
	row := r.db.QueryRowContext(ctx, `
INSERT INTO account_rule_error_collections (name, description, created_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		collection.Name,
		nullIfEmpty(collection.Description),
	)
	item, err := scanAccountRuleErrorCollection(row)
	if err != nil {
		return nil, err
	}
	item.Rules = []*service.AccountRuleErrorRule{}
	return item, nil
}

func (r *accountRuleRepository) UpdateErrorCollection(ctx context.Context, collection *service.AccountRuleErrorCollection) (*service.AccountRuleErrorCollection, error) {
	row := r.db.QueryRowContext(ctx, `
UPDATE account_rule_error_collections
SET name = $2,
    description = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, COALESCE(description, ''), created_at, updated_at`,
		collection.ID,
		collection.Name,
		nullIfEmpty(collection.Description),
	)
	item, scanErr := scanAccountRuleErrorCollection(row)
	if scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return nil, nil
		}
		return nil, scanErr
	}
	item.Rules = collection.Rules
	return item, nil
}

func (r *accountRuleRepository) DeleteErrorCollection(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM account_rule_error_collections WHERE id = $1`, id)
	return err
}

func (r *accountRuleRepository) GetRuleByID(ctx context.Context, id int64) (*service.AccountRuleErrorRule, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, error_collection_id, name, enabled, priority, status_codes, keywords, match_mode,
       action_disable, action_failover, action_delete, action_override,
       passthrough_code, response_code, passthrough_body, custom_message,
       skip_monitoring, COALESCE(description, ''), COALESCE(sample_response, ''),
       created_at, updated_at
FROM account_rule_error_collection_rules
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
INSERT INTO account_rule_error_collection_rules (
  error_collection_id, name, enabled, priority, status_codes, keywords, match_mode,
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
RETURNING id, error_collection_id, name, enabled, priority, status_codes, keywords, match_mode,
          action_disable, action_failover, action_delete, action_override,
          passthrough_code, response_code, passthrough_body, custom_message,
          skip_monitoring, COALESCE(description, ''), COALESCE(sample_response, ''),
          created_at, updated_at`,
		rule.ErrorCollectionID,
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
UPDATE account_rule_error_collection_rules
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
RETURNING id, error_collection_id, name, enabled, priority, status_codes, keywords, match_mode,
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
	_, err := r.db.ExecContext(ctx, `DELETE FROM account_rule_error_collection_rules WHERE id = $1`, id)
	return err
}

func (r *accountRuleRepository) ListObservedBindings(ctx context.Context) ([]*service.AccountRuleObservedBinding, error) {
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
		platform     string
		businessType string
	}
	counts := make(map[key]*service.AccountRuleObservedBinding)
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
			return nil, fmt.Errorf("unmarshal observed binding credentials: %w", err)
		}
		if account.Extra, err = unmarshalObservedAccountJSONMap(extraJSON); err != nil {
			return nil, fmt.Errorf("unmarshal observed binding extra: %w", err)
		}

		k := key{
			platform:     strings.ToLower(strings.TrimSpace(account.Platform)),
			businessType: account.AccountRuleScopeType(),
		}
		if k.platform == "" {
			continue
		}
		if existing, ok := counts[k]; ok {
			existing.AccountCount++
			continue
		}
		counts[k] = &service.AccountRuleObservedBinding{
			Platform:     k.platform,
			BusinessType: k.businessType,
			AccountCount: 1,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items := make([]*service.AccountRuleObservedBinding, 0, len(counts))
	for _, item := range counts {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Platform != items[j].Platform {
			return items[i].Platform < items[j].Platform
		}
		return items[i].BusinessType < items[j].BusinessType
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

type accountRuleBindingScanner interface {
	Scan(dest ...any) error
}

func scanAccountRuleBinding(scanner accountRuleBindingScanner) (*service.AccountRuleBinding, error) {
	var (
		item              service.AccountRuleBinding
		modelCollectionID sql.NullInt64
		errorCollectionID sql.NullInt64
	)
	if err := scanner.Scan(
		&item.ID,
		&item.Platform,
		&item.BusinessType,
		&item.Enabled,
		&modelCollectionID,
		&errorCollectionID,
		&item.Description,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if modelCollectionID.Valid {
		v := modelCollectionID.Int64
		item.ModelCollectionID = &v
	}
	if errorCollectionID.Valid {
		v := errorCollectionID.Int64
		item.ErrorCollectionID = &v
	}
	return &item, nil
}

type accountRuleModelCollectionScanner interface {
	Scan(dest ...any) error
}

func scanAccountRuleModelCollection(scanner accountRuleModelCollectionScanner) (*service.AccountRuleModelCollection, error) {
	var (
		item       service.AccountRuleModelCollection
		modelsJSON []byte
	)
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&modelsJSON,
		&item.Description,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(modelsJSON, &item.Models); err != nil {
		return nil, fmt.Errorf("unmarshal model collection models: %w", err)
	}
	if item.Models == nil {
		item.Models = []string{}
	}
	return &item, nil
}

type accountRuleErrorCollectionScanner interface {
	Scan(dest ...any) error
}

func scanAccountRuleErrorCollection(scanner accountRuleErrorCollectionScanner) (*service.AccountRuleErrorCollection, error) {
	var item service.AccountRuleErrorCollection
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Description,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
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
		&rule.ErrorCollectionID,
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

CREATE TABLE IF NOT EXISTS account_rule_scopes (
    id BIGSERIAL PRIMARY KEY,
    platform TEXT NOT NULL,
    account_type TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    model_set JSONB NOT NULL DEFAULT '[]'::jsonb,
    description TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_rule_scopes_platform_type_unique
    ON account_rule_scopes (platform, account_type);

CREATE INDEX IF NOT EXISTS idx_account_rule_scopes_platform
    ON account_rule_scopes (platform);

CREATE TABLE IF NOT EXISTS account_rule_error_rules (
    id BIGSERIAL PRIMARY KEY,
    scope_id BIGINT NOT NULL REFERENCES account_rule_scopes(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INTEGER NOT NULL DEFAULT 100,
    status_codes JSONB NOT NULL DEFAULT '[]'::jsonb,
    keywords JSONB NOT NULL DEFAULT '[]'::jsonb,
    match_mode TEXT NOT NULL DEFAULT 'any',
    action_disable BOOLEAN NOT NULL DEFAULT FALSE,
    action_failover BOOLEAN NOT NULL DEFAULT FALSE,
    action_delete BOOLEAN NOT NULL DEFAULT FALSE,
    action_override BOOLEAN NOT NULL DEFAULT FALSE,
    passthrough_code BOOLEAN NOT NULL DEFAULT TRUE,
    response_code INTEGER NULL,
    passthrough_body BOOLEAN NOT NULL DEFAULT TRUE,
    custom_message TEXT NULL,
    skip_monitoring BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NULL,
    sample_response TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_rule_error_rules_scope_priority
    ON account_rule_error_rules (scope_id, priority, id);

CREATE INDEX IF NOT EXISTS idx_account_rule_error_rules_enabled
    ON account_rule_error_rules (enabled);

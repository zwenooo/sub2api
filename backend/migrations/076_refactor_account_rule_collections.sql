CREATE TABLE IF NOT EXISTS account_rule_model_collections (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    models JSONB NOT NULL DEFAULT '[]'::jsonb,
    description TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_rule_model_collections_name_unique
    ON account_rule_model_collections (LOWER(name));

CREATE TABLE IF NOT EXISTS account_rule_error_collections (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_rule_error_collections_name_unique
    ON account_rule_error_collections (LOWER(name));

CREATE TABLE IF NOT EXISTS account_rule_collection_bindings (
    id BIGSERIAL PRIMARY KEY,
    platform TEXT NOT NULL,
    business_type TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    model_collection_id BIGINT NULL REFERENCES account_rule_model_collections(id) ON DELETE SET NULL,
    error_collection_id BIGINT NULL REFERENCES account_rule_error_collections(id) ON DELETE SET NULL,
    description TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_account_rule_collection_bindings_platform_type_unique
    ON account_rule_collection_bindings (platform, business_type);

CREATE INDEX IF NOT EXISTS idx_account_rule_collection_bindings_platform
    ON account_rule_collection_bindings (platform);

CREATE TABLE IF NOT EXISTS account_rule_error_collection_rules (
    id BIGSERIAL PRIMARY KEY,
    error_collection_id BIGINT NOT NULL REFERENCES account_rule_error_collections(id) ON DELETE CASCADE,
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

CREATE INDEX IF NOT EXISTS idx_account_rule_error_collection_rules_collection_priority
    ON account_rule_error_collection_rules (error_collection_id, priority, id);

CREATE INDEX IF NOT EXISTS idx_account_rule_error_collection_rules_enabled
    ON account_rule_error_collection_rules (enabled);

INSERT INTO account_rule_model_collections (name, models, description, created_at, updated_at)
SELECT
    CASE
        WHEN COALESCE(NULLIF(account_type, ''), '') = '' THEN platform
        ELSE platform || ' / ' || account_type
    END AS name,
    model_set,
    NULLIF(COALESCE(description, ''), ''),
    created_at,
    updated_at
FROM account_rule_scopes
WHERE jsonb_typeof(model_set) = 'array'
  AND jsonb_array_length(model_set) > 0;

INSERT INTO account_rule_error_collections (name, description, created_at, updated_at)
SELECT
    CASE
        WHEN COALESCE(NULLIF(s.account_type, ''), '') = '' THEN s.platform
        ELSE s.platform || ' / ' || s.account_type
    END AS name,
    NULLIF(COALESCE(s.description, ''), ''),
    s.created_at,
    s.updated_at
FROM account_rule_scopes s
WHERE EXISTS (
    SELECT 1
    FROM account_rule_error_rules r
    WHERE r.scope_id = s.id
);

INSERT INTO account_rule_collection_bindings (
    platform,
    business_type,
    enabled,
    model_collection_id,
    error_collection_id,
    description,
    created_at,
    updated_at
)
SELECT
    s.platform,
    s.account_type,
    s.enabled,
    mc.id,
    ec.id,
    NULLIF(COALESCE(s.description, ''), ''),
    s.created_at,
    s.updated_at
FROM account_rule_scopes s
LEFT JOIN account_rule_model_collections mc
    ON mc.name = CASE
        WHEN COALESCE(NULLIF(s.account_type, ''), '') = '' THEN s.platform
        ELSE s.platform || ' / ' || s.account_type
    END
LEFT JOIN account_rule_error_collections ec
    ON ec.name = CASE
        WHEN COALESCE(NULLIF(s.account_type, ''), '') = '' THEN s.platform
        ELSE s.platform || ' / ' || s.account_type
    END;

INSERT INTO account_rule_error_collection_rules (
    error_collection_id,
    name,
    enabled,
    priority,
    status_codes,
    keywords,
    match_mode,
    action_disable,
    action_failover,
    action_delete,
    action_override,
    passthrough_code,
    response_code,
    passthrough_body,
    custom_message,
    skip_monitoring,
    description,
    sample_response,
    created_at,
    updated_at
)
SELECT
    b.error_collection_id,
    r.name,
    r.enabled,
    r.priority,
    r.status_codes,
    r.keywords,
    r.match_mode,
    r.action_disable,
    r.action_failover,
    r.action_delete,
    r.action_override,
    r.passthrough_code,
    r.response_code,
    r.passthrough_body,
    r.custom_message,
    r.skip_monitoring,
    r.description,
    r.sample_response,
    r.created_at,
    r.updated_at
FROM account_rule_error_rules r
JOIN account_rule_scopes s
    ON s.id = r.scope_id
JOIN account_rule_collection_bindings b
    ON b.platform = s.platform
   AND b.business_type = s.account_type
WHERE b.error_collection_id IS NOT NULL;

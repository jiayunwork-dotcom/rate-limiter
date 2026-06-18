CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS rate_limit_rules (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::VARCHAR,
    name VARCHAR(255) NOT NULL,
    api_path VARCHAR(512) NOT NULL DEFAULT '*',
    method VARCHAR(16) NOT NULL DEFAULT '*',
    algorithm VARCHAR(32) NOT NULL DEFAULT 'token_bucket',
    enabled BOOLEAN NOT NULL DEFAULT true,
    version BIGINT NOT NULL DEFAULT 1,
    limit_count BIGINT NOT NULL DEFAULT 100,
    window_seconds BIGINT NOT NULL DEFAULT 60,
    dimensions JSONB NOT NULL DEFAULT '{}',
    token_bucket_config JSONB,
    leaky_bucket_config JSONB,
    shaping_config JSONB,
    gray_release_config JSONB,
    config_json JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rate_limit_rules_enabled ON rate_limit_rules(enabled);
CREATE INDEX idx_rate_limit_rules_api_path ON rate_limit_rules(api_path);
CREATE INDEX idx_rate_limit_rules_algorithm ON rate_limit_rules(algorithm);

CREATE TABLE IF NOT EXISTS rule_versions (
    id BIGSERIAL PRIMARY KEY,
    rule_id VARCHAR(64) NOT NULL REFERENCES rate_limit_rules(id) ON DELETE CASCADE,
    version BIGINT NOT NULL,
    config_json JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_rule_versions_rule_id ON rule_versions(rule_id, version DESC);

CREATE TABLE IF NOT EXISTS quota_configs (
    id BIGSERIAL PRIMARY KEY,
    quota_level VARCHAR(16) NOT NULL,
    identifier VARCHAR(256) NOT NULL,
    limit_count BIGINT NOT NULL,
    window_seconds BIGINT NOT NULL DEFAULT 60,
    inherit_from BOOLEAN NOT NULL DEFAULT true,
    override_value BIGINT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(quota_level, identifier)
);

CREATE TABLE IF NOT EXISTS rate_limit_events (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    request_id VARCHAR(128),
    allowed BOOLEAN NOT NULL,
    rule_id VARCHAR(64),
    rule_name VARCHAR(255),
    dimensions JSONB,
    limit_count BIGINT,
    remaining BIGINT,
    api_path VARCHAR(512),
    method VARCHAR(16),
    user_id VARCHAR(128),
    tenant_id VARCHAR(128),
    client_ip VARCHAR(64),
    triggered_level VARCHAR(16),
    mode VARCHAR(16)
);

CREATE INDEX idx_rate_limit_events_timestamp ON rate_limit_events(timestamp DESC);
CREATE INDEX idx_rate_limit_events_tenant ON rate_limit_events(tenant_id, timestamp DESC);
CREATE INDEX idx_rate_limit_events_api ON rate_limit_events(api_path, timestamp DESC);
CREATE INDEX idx_rate_limit_events_allowed ON rate_limit_events(allowed, timestamp DESC);
CREATE INDEX idx_rate_limit_events_rule ON rate_limit_events(rule_id, timestamp DESC);

CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::VARCHAR,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_users (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::VARCHAR,
    tenant_id VARCHAR(64) REFERENCES tenants(id),
    name VARCHAR(255),
    email VARCHAR(255),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_api_users_tenant ON api_users(tenant_id);

CREATE TABLE IF NOT EXISTS api_endpoints (
    id BIGSERIAL PRIMARY KEY,
    path VARCHAR(512) NOT NULL,
    method VARCHAR(16) NOT NULL,
    description TEXT,
    tenant_id VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(path, method)
);

CREATE TABLE IF NOT EXISTS adaptive_configs (
    id BIGSERIAL PRIMARY KEY,
    component VARCHAR(64) NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    target_p99_latency_ms BIGINT NOT NULL DEFAULT 200,
    error_rate_threshold DOUBLE PRECISION NOT NULL DEFAULT 0.05,
    min_coefficient DOUBLE PRECISION NOT NULL DEFAULT 0.3,
    max_coefficient DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    tightening_ratio DOUBLE PRECISION NOT NULL DEFAULT 0.7,
    recovery_interval_sec BIGINT NOT NULL DEFAULT 10,
    recovery_step_percent DOUBLE PRECISION NOT NULL DEFAULT 10,
    stable_period_sec BIGINT NOT NULL DEFAULT 30,
    pid_kp DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    pid_ki DOUBLE PRECISION NOT NULL DEFAULT 0.1,
    pid_kd DOUBLE PRECISION NOT NULL DEFAULT 0.2,
    pid_setpoint DOUBLE PRECISION NOT NULL DEFAULT 200,
    pid_output_min DOUBLE PRECISION NOT NULL DEFAULT -30,
    pid_output_max DOUBLE PRECISION NOT NULL DEFAULT 30,
    manual_override_coeff DOUBLE PRECISION,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(128) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    role VARCHAR(32) NOT NULL DEFAULT 'viewer',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMP WITH TIME ZONE
);

CREATE OR REPLACE FUNCTION update_timestamp_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_update_rate_limit_rules_timestamp
BEFORE UPDATE ON rate_limit_rules
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

CREATE TRIGGER trg_update_quota_configs_timestamp
BEFORE UPDATE ON quota_configs
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

CREATE TRIGGER trg_update_tenants_timestamp
BEFORE UPDATE ON tenants
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

CREATE TRIGGER trg_update_api_users_timestamp
BEFORE UPDATE ON api_users
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

CREATE TRIGGER trg_update_adaptive_configs_timestamp
BEFORE UPDATE ON adaptive_configs
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

CREATE TABLE IF NOT EXISTS rule_templates (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::VARCHAR,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    algorithm VARCHAR(32) NOT NULL DEFAULT 'token_bucket',
    limit_count BIGINT NOT NULL DEFAULT 100,
    window_seconds BIGINT NOT NULL DEFAULT 60,
    token_bucket_config JSONB,
    leaky_bucket_config JSONB,
    shaping_config JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rule_templates_algorithm ON rule_templates(algorithm);
CREATE INDEX idx_rule_templates_name ON rule_templates(name);

CREATE TRIGGER trg_update_rule_templates_timestamp
BEFORE UPDATE ON rule_templates
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

INSERT INTO tenants (id, name, description) VALUES
('tenant-demo-1', 'Demo Tenant 1', 'First demo tenant for testing'),
('tenant-demo-2', 'Demo Tenant 2', 'Second demo tenant for testing')
ON CONFLICT DO NOTHING;

INSERT INTO admin_users (username, password_hash, email, role) VALUES
('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'admin@ratelimiter.local', 'admin')
ON CONFLICT DO NOTHING;

INSERT INTO adaptive_configs (component) VALUES ('global') ON CONFLICT DO NOTHING;

INSERT INTO rate_limit_rules (id, name, api_path, method, algorithm, limit_count, window_seconds, config_json, token_bucket_config, shaping_config) VALUES
(
    'api-login-protect',
    'Login API Protection',
    '/api/auth/login',
    'POST',
    'sliding_window',
    5,
    60,
    '{}',
    NULL,
    '{"enabled": true, "max_queue_depth": 20, "max_wait_ms": 5000, "priority_enabled": false}'
),
(
    'tenant-wide-api',
    'Per-tenant API Limit',
    '*',
    '*',
    'token_bucket',
    10000,
    60,
    '{}',
    '{"refill_rate": 167, "capacity": 10000, "tokens_per_req": 1}',
    '{"enabled": true, "max_queue_depth": 200, "max_wait_ms": 3000, "priority_enabled": true}'
)
ON CONFLICT DO NOTHING;

INSERT INTO rule_templates (name, description, algorithm, limit_count, window_seconds, token_bucket_config, shaping_config) VALUES
(
    '标准API限流模板',
    '适用于普通API接口的限流配置，使用令牌桶算法，支持突发流量',
    'token_bucket',
    1000,
    60,
    '{"refill_rate": 16, "capacity": 1000, "tokens_per_req": 1}',
    '{"enabled": true, "max_queue_depth": 100, "max_wait_ms": 2000, "priority_enabled": false}'
),
(
    '登录接口防护模板',
    '针对登录接口的严格限流配置，防止暴力破解，使用滑动窗口算法',
    'sliding_window',
    5,
    60,
    NULL,
    '{"enabled": true, "max_queue_depth": 10, "max_wait_ms": 5000, "priority_enabled": false}'
),
(
    '高并发读接口模板',
    '适用于高并发读接口，较大的限流阈值，支持优先级排队',
    'token_bucket',
    10000,
    60,
    '{"refill_rate": 167, "capacity": 10000, "tokens_per_req": 1}',
    '{"enabled": true, "max_queue_depth": 500, "max_wait_ms": 3000, "priority_enabled": true}'
),
(
    '严格写操作模板',
    '针对写入/修改操作的严格限流，固定窗口算法，直接拒绝无队列',
    'fixed_window',
    100,
    60,
    NULL,
    '{"enabled": false, "max_queue_depth": 0, "max_wait_ms": 0, "priority_enabled": false}'
)
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS alert_rules (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::VARCHAR,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    severity VARCHAR(16) NOT NULL DEFAULT 'warning',
    enabled BOOLEAN NOT NULL DEFAULT true,
    trigger_type VARCHAR(32) NOT NULL,
    trigger_config JSONB NOT NULL DEFAULT '{}',
    scope_type VARCHAR(16) NOT NULL DEFAULT 'global',
    scope_value VARCHAR(512),
    notification_channels JSONB NOT NULL DEFAULT '[]',
    silent_period_seconds BIGINT NOT NULL DEFAULT 300,
    retention_hours BIGINT NOT NULL DEFAULT 168,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alert_rules_enabled ON alert_rules(enabled);
CREATE INDEX idx_alert_rules_trigger_type ON alert_rules(trigger_type);
CREATE INDEX idx_alert_rules_scope ON alert_rules(scope_type, scope_value);

CREATE TABLE IF NOT EXISTS alert_events (
    id BIGSERIAL PRIMARY KEY,
    alert_rule_id VARCHAR(64) NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    rule_name VARCHAR(255) NOT NULL,
    severity VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'firing',
    dimension_type VARCHAR(32) NOT NULL,
    dimension_value VARCHAR(512) NOT NULL,
    current_value DOUBLE PRECISION NOT NULL,
    threshold_value DOUBLE PRECISION NOT NULL,
    trigger_snapshot JSONB NOT NULL DEFAULT '{}',
    acknowledged_by VARCHAR(128),
    acknowledged_at TIMESTAMP WITH TIME ZONE,
    resolved_at TIMESTAMP WITH TIME ZONE,
    expired_at TIMESTAMP WITH TIME ZONE,
    firing_started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_firing_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alert_events_status ON alert_events(status);
CREATE INDEX idx_alert_events_severity ON alert_events(severity);
CREATE INDEX idx_alert_events_rule ON alert_events(alert_rule_id, created_at DESC);
CREATE INDEX idx_alert_events_dimension ON alert_events(dimension_type, dimension_value, created_at DESC);
CREATE INDEX idx_alert_events_created ON alert_events(created_at DESC);
CREATE INDEX idx_alert_events_status_created ON alert_events(status, created_at DESC);

CREATE TRIGGER trg_update_alert_rules_timestamp
BEFORE UPDATE ON alert_rules
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

CREATE TRIGGER trg_update_alert_events_timestamp
BEFORE UPDATE ON alert_events
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

INSERT INTO alert_rules (id, name, description, severity, trigger_type, trigger_config, scope_type, scope_value, silent_period_seconds) VALUES
(
    'rule-api-high-reject',
    'API高拒绝率告警',
    '当某个API的拒绝次数在60秒内超过100次时触发告警',
    'critical',
    'threshold',
    '{"windowSeconds": 60, "threshold": 100, "metric": "reject_count"}',
    'global',
    NULL,
    300
),
(
    'rule-tenant-reject-rate',
    '租户拒绝率告警',
    '当某个租户的拒绝率在5分钟内超过20%时触发告警',
    'warning',
    'rate',
    '{"windowSeconds": 300, "thresholdPercent": 20, "metric": "reject_rate"}',
    'global',
    NULL,
    600
)
ON CONFLICT DO NOTHING;

ALTER TABLE alert_events ADD COLUMN IF NOT EXISTS suppressed BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE alert_events ADD COLUMN IF NOT EXISTS suppressed_by_rule_id VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_alert_events_suppressed ON alert_events(suppressed);

CREATE TABLE IF NOT EXISTS alert_aggregation_rules (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::VARCHAR,
    name VARCHAR(255) NOT NULL,
    dimension_type VARCHAR(32) NOT NULL,
    window_seconds BIGINT NOT NULL DEFAULT 300,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_aggregation_rules_enabled ON alert_aggregation_rules(enabled);

CREATE TRIGGER trg_update_alert_aggregation_rules_timestamp
BEFORE UPDATE ON alert_aggregation_rules
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

CREATE TABLE IF NOT EXISTS alert_suppression_rules (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4()::VARCHAR,
    name VARCHAR(255) NOT NULL,
    source_severity VARCHAR(16) NOT NULL,
    source_status VARCHAR(16) NOT NULL,
    source_rule_id VARCHAR(64),
    target_severity VARCHAR(16) NOT NULL,
    target_dimension_type VARCHAR(32),
    match_dimension_fields VARCHAR(512) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_suppression_rules_enabled ON alert_suppression_rules(enabled);

CREATE TRIGGER trg_update_alert_suppression_rules_timestamp
BEFORE UPDATE ON alert_suppression_rules
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

CREATE TABLE IF NOT EXISTS alert_aggregation_groups (
    id BIGSERIAL PRIMARY KEY,
    aggregation_rule_id VARCHAR(64) NOT NULL,
    dimension_type VARCHAR(32) NOT NULL,
    dimension_value VARCHAR(512) NOT NULL,
    trigger_count BIGINT NOT NULL DEFAULT 1,
    first_triggered_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_triggered_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    window_ends_at TIMESTAMP WITH TIME ZONE NOT NULL,
    severity VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'firing',
    unique_values JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_aggregation_groups_rule_dim ON alert_aggregation_groups(aggregation_rule_id, dimension_type, dimension_value, status);
CREATE INDEX IF NOT EXISTS idx_alert_aggregation_groups_window_ends ON alert_aggregation_groups(window_ends_at);

CREATE TRIGGER trg_update_alert_aggregation_groups_timestamp
BEFORE UPDATE ON alert_aggregation_groups
FOR EACH ROW EXECUTE FUNCTION update_timestamp_column();

CREATE TABLE IF NOT EXISTS alert_aggregation_events (
    id BIGSERIAL PRIMARY KEY,
    aggregation_group_id BIGINT NOT NULL,
    alert_event_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alert_aggregation_events_group ON alert_aggregation_events(aggregation_group_id);
CREATE INDEX IF NOT EXISTS idx_alert_aggregation_events_event ON alert_aggregation_events(alert_event_id);

INSERT INTO alert_aggregation_rules (id, name, dimension_type, window_seconds, enabled) VALUES
('agg-api-path-5min', '按API路径聚合(5分钟)', 'api_path', 300, true),
('agg-tenant-5min', '按租户聚合(5分钟)', 'tenant_id', 300, true)
ON CONFLICT DO NOTHING;

INSERT INTO alert_suppression_rules (id, name, source_severity, source_status, target_severity, match_dimension_fields, enabled) VALUES
('suppress-critical-suppresses-warning-info', 'Critical告警抑制Warning和Info', 'critical', 'firing', 'info', 'dimension_value', true)
ON CONFLICT DO NOTHING;

-- Migration: Create tenant_configs table
-- Purpose: Store tenant-specific configuration key-value pairs
-- Author: Lurus Team
-- Date: 2026-01-25

-- Create tenant_configs table
-- 创建租户配置表
CREATE TABLE IF NOT EXISTS tenant_configs (
    id SERIAL PRIMARY KEY,                                -- Auto-increment primary key
    tenant_id VARCHAR(36) NOT NULL,                       -- Reference to tenants.id
    config_key VARCHAR(128) NOT NULL,                     -- Configuration key (e.g., "quota.new_user_quota")
    config_value TEXT,                                    -- Configuration value (stored as text, cast when using)
    config_type VARCHAR(32) DEFAULT 'string',             -- Value type: string/int/bool/json/float

    -- Metadata
    -- 元数据
    description VARCHAR(255),                             -- Configuration description
    is_system BOOLEAN DEFAULT FALSE,                      -- Whether this is a system config (read-only for tenants)
    is_encrypted BOOLEAN DEFAULT FALSE,                   -- Whether the value is encrypted

    -- Timestamps
    -- 时间戳
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- Constraints
    -- 约束
    CONSTRAINT uq_tenant_config_key UNIQUE (tenant_id, config_key),
    CONSTRAINT fk_tenant_config FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- Create indexes for performance
-- 创建索引以提升性能
CREATE INDEX idx_tenant_configs_tenant ON tenant_configs(tenant_id);
CREATE INDEX idx_tenant_configs_key ON tenant_configs(tenant_id, config_key);
CREATE INDEX idx_tenant_configs_type ON tenant_configs(config_type);
CREATE INDEX idx_tenant_configs_system ON tenant_configs(is_system);

-- Insert default configurations for default tenant
-- 为默认租户插入默认配置
INSERT INTO tenant_configs (tenant_id, config_key, config_value, config_type, description, is_system) VALUES
    -- User quota settings
    -- 用户额度设置
    ('default', 'quota.new_user_quota', '10000', 'int', 'Default quota for new users (in tokens)', FALSE),
    ('default', 'quota.max_user_quota', '1000000', 'int', 'Maximum quota per user (in tokens)', FALSE),
    ('default', 'quota.quota_reset_enabled', 'false', 'bool', 'Enable monthly quota reset', FALSE),

    -- Billing settings
    -- 计费设置
    ('default', 'billing.currency', 'CNY', 'string', 'Default currency for billing', FALSE),
    ('default', 'billing.tax_rate', '0.13', 'float', 'Tax rate (0.13 = 13%)', FALSE),
    ('default', 'billing.min_topup_amount', '1', 'int', 'Minimum top-up amount', FALSE),

    -- Feature toggles
    -- 功能开关
    ('default', 'features.enable_meilisearch', 'true', 'bool', 'Enable Meilisearch integration', TRUE),
    ('default', 'features.enable_subscriptions', 'true', 'bool', 'Enable subscription system', TRUE),
    ('default', 'features.enable_redemptions', 'true', 'bool', 'Enable redemption codes', TRUE),
    ('default', 'features.enable_oauth', 'true', 'bool', 'Enable OAuth login', TRUE),

    -- Security settings
    -- 安全设置
    ('default', 'security.max_login_attempts', '5', 'int', 'Maximum login attempts before lockout', TRUE),
    ('default', 'security.session_timeout', '86400', 'int', 'Session timeout in seconds (24 hours)', TRUE),
    ('default', 'security.token_expiry', '2592000', 'int', 'Access token expiry in seconds (30 days)', TRUE),

    -- Rate limiting
    -- 速率限制
    ('default', 'rate_limit.requests_per_minute', '60', 'int', 'Maximum API requests per minute', FALSE),
    ('default', 'rate_limit.requests_per_day', '10000', 'int', 'Maximum API requests per day', FALSE),

    -- Notification settings
    -- 通知设置
    ('default', 'notification.email_enabled', 'true', 'bool', 'Enable email notifications', FALSE),
    ('default', 'notification.low_quota_threshold', '1000', 'int', 'Quota threshold for low quota warning', FALSE)
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP;

-- Comments for documentation
-- 字段说明
COMMENT ON TABLE tenant_configs IS 'Tenant-specific configuration key-value store';
COMMENT ON COLUMN tenant_configs.id IS 'Auto-increment primary key';
COMMENT ON COLUMN tenant_configs.tenant_id IS 'Reference to tenants table';
COMMENT ON COLUMN tenant_configs.config_key IS 'Configuration key (dot-separated namespace, e.g., quota.new_user_quota)';
COMMENT ON COLUMN tenant_configs.config_value IS 'Configuration value (stored as text)';
COMMENT ON COLUMN tenant_configs.config_type IS 'Value type for type casting: string/int/bool/json/float';
COMMENT ON COLUMN tenant_configs.description IS 'Human-readable description of this config';
COMMENT ON COLUMN tenant_configs.is_system IS 'System configs are read-only for tenant admins';
COMMENT ON COLUMN tenant_configs.is_encrypted IS 'Whether the value is encrypted (for sensitive data)';

-- Migration: Create tenants table
-- Purpose: Multi-tenant SaaS core table, mapping Zitadel Organizations to lurus tenants
-- Author: Lurus Team
-- Date: 2026-01-25

-- Create tenants table
-- 创建租户表
CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(36) PRIMARY KEY,                           -- UUID (matches Zitadel Organization ID or custom ID)
    zitadel_org_id VARCHAR(128) UNIQUE NOT NULL,          -- Zitadel Organization ID (from urn:zitadel:iam:org:id)
    slug VARCHAR(64) UNIQUE NOT NULL,                     -- Tenant identifier for URLs (e.g., lurus, customer-a)
    name VARCHAR(128) NOT NULL,                           -- Tenant display name (e.g., Lurus Platform)
    status INT DEFAULT 1,                                 -- 1=enabled, 2=disabled, 3=suspended

    -- Business configuration
    -- 业务配置
    plan_type VARCHAR(32) DEFAULT 'free',                 -- Subscription plan: free/pro/enterprise
    max_users INT DEFAULT 100,                            -- Maximum users allowed for this tenant
    max_quota BIGINT DEFAULT 1000000,                     -- Maximum total quota (in tokens)

    -- Metadata
    -- 元数据
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Create indexes for performance
-- 创建索引以提升性能
CREATE INDEX idx_tenants_zitadel_org ON tenants(zitadel_org_id);
CREATE INDEX idx_tenants_slug ON tenants(slug);
CREATE INDEX idx_tenants_status ON tenants(status);
CREATE INDEX idx_tenants_plan ON tenants(plan_type);

-- Insert default tenant (Lurus Platform)
-- 插入默认租户（Lurus Platform）
-- Note: zitadel_org_id needs to be updated after Zitadel configuration
-- 注意：zitadel_org_id 需要在 Zitadel 配置完成后更新
INSERT INTO tenants (id, zitadel_org_id, slug, name, status, plan_type, max_users, max_quota)
VALUES (
    'default',                                            -- Default tenant ID
    'ZITADEL_DEFAULT_ORG_ID_PLACEHOLDER',                 -- Will be replaced with actual Zitadel Org ID
    'lurus',                                              -- Default slug
    'Lurus Platform',                                     -- Default tenant name
    1,                                                    -- Enabled
    'enterprise',                                         -- Enterprise plan for default tenant
    10000,                                                -- Max 10000 users
    1000000000                                            -- Max 1 billion tokens
) ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP;

-- Comments for documentation
-- 字段说明
COMMENT ON TABLE tenants IS 'Multi-tenant SaaS tenants table, mapping Zitadel Organizations';
COMMENT ON COLUMN tenants.id IS 'Tenant UUID, primary key';
COMMENT ON COLUMN tenants.zitadel_org_id IS 'Zitadel Organization ID (from JWT claim urn:zitadel:iam:org:id)';
COMMENT ON COLUMN tenants.slug IS 'URL-friendly tenant identifier (e.g., /api/v2/:tenant_slug/...)';
COMMENT ON COLUMN tenants.name IS 'Tenant display name';
COMMENT ON COLUMN tenants.status IS '1=enabled, 2=disabled, 3=suspended';
COMMENT ON COLUMN tenants.plan_type IS 'Subscription plan: free/pro/enterprise';
COMMENT ON COLUMN tenants.max_users IS 'Maximum number of users allowed';
COMMENT ON COLUMN tenants.max_quota IS 'Maximum total quota in tokens';

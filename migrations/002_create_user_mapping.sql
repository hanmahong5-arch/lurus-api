-- Migration: Create user_identity_mapping table
-- Purpose: Map Zitadel users to lurus users for multi-tenant identity management
-- Author: Lurus Team
-- Date: 2026-01-25

-- Create user_identity_mapping table
-- 创建用户身份映射表
CREATE TABLE IF NOT EXISTS user_identity_mapping (
    id SERIAL PRIMARY KEY,                                -- Auto-increment primary key
    lurus_user_id INT NOT NULL,                           -- Reference to lurus users.id
    zitadel_user_id VARCHAR(128) NOT NULL,                -- Zitadel user ID (from JWT claim "sub")
    tenant_id VARCHAR(36) NOT NULL,                       -- Reference to tenants.id

    -- User metadata synced from Zitadel
    -- 从 Zitadel 同步的用户元数据
    email VARCHAR(255),                                   -- User email (synced from Zitadel)
    display_name VARCHAR(128),                            -- User display name (synced from Zitadel)
    preferred_username VARCHAR(128),                      -- Preferred username (synced from Zitadel)

    -- Sync metadata
    -- 同步元数据
    last_sync_at TIMESTAMP,                               -- Last time user data was synced from Zitadel
    is_active BOOLEAN DEFAULT TRUE,                       -- Whether this mapping is active

    -- Timestamps
    -- 时间戳
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- Constraints
    -- 约束
    CONSTRAINT uq_zitadel_user_tenant UNIQUE (zitadel_user_id, tenant_id),
    CONSTRAINT fk_lurus_user FOREIGN KEY (lurus_user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- Create indexes for performance
-- 创建索引以提升性能
CREATE INDEX idx_mapping_zitadel_user ON user_identity_mapping(zitadel_user_id);
CREATE INDEX idx_mapping_lurus_user_tenant ON user_identity_mapping(lurus_user_id, tenant_id);
CREATE INDEX idx_mapping_tenant ON user_identity_mapping(tenant_id);
CREATE INDEX idx_mapping_email ON user_identity_mapping(email);
CREATE INDEX idx_mapping_active ON user_identity_mapping(is_active);

-- Comments for documentation
-- 字段说明
COMMENT ON TABLE user_identity_mapping IS 'Maps Zitadel users to lurus users in multi-tenant context';
COMMENT ON COLUMN user_identity_mapping.id IS 'Auto-increment primary key';
COMMENT ON COLUMN user_identity_mapping.lurus_user_id IS 'Reference to lurus users table';
COMMENT ON COLUMN user_identity_mapping.zitadel_user_id IS 'Zitadel user ID from JWT claim "sub"';
COMMENT ON COLUMN user_identity_mapping.tenant_id IS 'Reference to tenants table';
COMMENT ON COLUMN user_identity_mapping.email IS 'User email synced from Zitadel';
COMMENT ON COLUMN user_identity_mapping.display_name IS 'User display name synced from Zitadel';
COMMENT ON COLUMN user_identity_mapping.preferred_username IS 'Preferred username synced from Zitadel';
COMMENT ON COLUMN user_identity_mapping.last_sync_at IS 'Last sync timestamp from Zitadel';
COMMENT ON COLUMN user_identity_mapping.is_active IS 'Whether this mapping is currently active';

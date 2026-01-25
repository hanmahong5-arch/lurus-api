-- Migration: Add tenant_id to existing tables
-- Purpose: Enable multi-tenant data isolation for all core tables
-- Author: Lurus Team
-- Date: 2026-01-25
-- WARNING: This migration modifies production data. Backup database before running!
-- 警告：此迁移会修改生产数据。运行前请备份数据库！

-- ============================================================================
-- Add tenant_id column to core tables
-- 为核心表添加 tenant_id 字段
-- ============================================================================

-- Users table
-- 用户表
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) DEFAULT 'default' COMMENT 'Reference to tenants table';

-- Tokens table
-- 令牌表
ALTER TABLE tokens
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) DEFAULT 'default' COMMENT 'Reference to tenants table';

-- Channels table
-- 渠道表
ALTER TABLE channels
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) DEFAULT 'default' COMMENT 'Reference to tenants table';

-- ============================================================================
-- Add tenant_id to billing tables
-- 为计费相关表添加 tenant_id
-- ============================================================================

-- TopUps table
-- 充值表
ALTER TABLE topups
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) DEFAULT 'default' COMMENT 'Reference to tenants table';

-- Subscriptions table
-- 订阅表
ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) DEFAULT 'default' COMMENT 'Reference to tenants table';

-- Redemptions table
-- 兑换码表
ALTER TABLE redemptions
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) DEFAULT 'default' COMMENT 'Reference to tenants table';

-- ============================================================================
-- Add tenant_id to logging and audit tables
-- 为日志和审计表添加 tenant_id
-- ============================================================================

-- Logs table
-- 日志表
ALTER TABLE logs
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) DEFAULT 'default' COMMENT 'Reference to tenants table';

-- ============================================================================
-- Add tenant_id to authentication tables (optional, if kept)
-- 为认证相关表添加 tenant_id（可选，如果保留这些表）
-- ============================================================================

-- Passkeys table (if exists)
-- Passkey 表（如果存在）
ALTER TABLE passkeys
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) DEFAULT 'default' COMMENT 'Reference to tenants table'
    WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'passkeys');

-- TwoFA table (if exists)
-- 两步验证表（如果存在）
ALTER TABLE twofa
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(36) DEFAULT 'default' COMMENT 'Reference to tenants table'
    WHERE EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'twofa');

-- ============================================================================
-- Create indexes for tenant_id on all tables
-- 为所有表的 tenant_id 创建索引
-- ============================================================================

-- Users indexes
CREATE INDEX IF NOT EXISTS idx_users_tenant ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_users_tenant_username ON users(tenant_id, username);
CREATE INDEX IF NOT EXISTS idx_users_tenant_email ON users(tenant_id, email);

-- Tokens indexes
CREATE INDEX IF NOT EXISTS idx_tokens_tenant ON tokens(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tokens_tenant_user ON tokens(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_tokens_tenant_key ON tokens(tenant_id, key);

-- Channels indexes
CREATE INDEX IF NOT EXISTS idx_channels_tenant ON channels(tenant_id);
CREATE INDEX IF NOT EXISTS idx_channels_tenant_group ON channels(tenant_id, `group`);
CREATE INDEX IF NOT EXISTS idx_channels_tenant_type ON channels(tenant_id, type);

-- TopUps indexes
CREATE INDEX IF NOT EXISTS idx_topups_tenant ON topups(tenant_id);
CREATE INDEX IF NOT EXISTS idx_topups_tenant_user ON topups(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_topups_tenant_status ON topups(tenant_id, status);

-- Subscriptions indexes
CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant ON subscriptions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant_user ON subscriptions(tenant_id, user_id);

-- Redemptions indexes
CREATE INDEX IF NOT EXISTS idx_redemptions_tenant ON redemptions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_redemptions_tenant_code ON redemptions(tenant_id, code);

-- Logs indexes
CREATE INDEX IF NOT EXISTS idx_logs_tenant ON logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_logs_tenant_created ON logs(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_logs_tenant_user ON logs(tenant_id, user_id);

-- ============================================================================
-- Add foreign key constraints (optional, can be added later for safety)
-- 添加外键约束（可选，为安全起见可稍后添加）
-- ============================================================================

-- Note: Foreign keys are commented out for initial migration flexibility
-- Uncomment after verifying data integrity
-- 注意：外键约束被注释，以便初始迁移更灵活
-- 验证数据完整性后再取消注释

-- ALTER TABLE users ADD CONSTRAINT fk_users_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT;
-- ALTER TABLE tokens ADD CONSTRAINT fk_tokens_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
-- ALTER TABLE channels ADD CONSTRAINT fk_channels_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
-- ALTER TABLE topups ADD CONSTRAINT fk_topups_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT;
-- ALTER TABLE subscriptions ADD CONSTRAINT fk_subscriptions_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT;
-- ALTER TABLE redemptions ADD CONSTRAINT fk_redemptions_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
-- ALTER TABLE logs ADD CONSTRAINT fk_logs_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

-- ============================================================================
-- Update unique constraints to include tenant_id
-- 更新唯一约束以包含 tenant_id
-- ============================================================================

-- Users table: username should be unique per tenant
-- 用户表：username 在租户内唯一
ALTER TABLE users DROP INDEX IF EXISTS username;
ALTER TABLE users ADD CONSTRAINT uq_users_tenant_username UNIQUE (tenant_id, username);

-- Tokens table: key should be unique per tenant
-- 令牌表：key 在租户内唯一
ALTER TABLE tokens DROP INDEX IF EXISTS `key`;
ALTER TABLE tokens ADD CONSTRAINT uq_tokens_tenant_key UNIQUE (tenant_id, `key`);

-- Redemptions table: code should be unique per tenant
-- 兑换码表：code 在租户内唯一
ALTER TABLE redemptions DROP INDEX IF EXISTS code;
ALTER TABLE redemptions ADD CONSTRAINT uq_redemptions_tenant_code UNIQUE (tenant_id, code);

-- ============================================================================
-- Migrate existing data to default tenant
-- 将现有数据迁移到默认租户
-- ============================================================================

-- Update all existing rows to use default tenant
-- 将所有现有行更新为使用默认租户
UPDATE users SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
UPDATE tokens SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
UPDATE channels SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
UPDATE topups SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
UPDATE subscriptions SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
UPDATE redemptions SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';
UPDATE logs SET tenant_id = 'default' WHERE tenant_id IS NULL OR tenant_id = '';

-- ============================================================================
-- Set tenant_id to NOT NULL after migration
-- 迁移完成后设置 tenant_id 为 NOT NULL
-- ============================================================================

-- Note: Run these AFTER verifying all data has been migrated
-- 注意：在验证所有数据已迁移后再运行这些语句

-- ALTER TABLE users MODIFY tenant_id VARCHAR(36) NOT NULL;
-- ALTER TABLE tokens MODIFY tenant_id VARCHAR(36) NOT NULL;
-- ALTER TABLE channels MODIFY tenant_id VARCHAR(36) NOT NULL;
-- ALTER TABLE topups MODIFY tenant_id VARCHAR(36) NOT NULL;
-- ALTER TABLE subscriptions MODIFY tenant_id VARCHAR(36) NOT NULL;
-- ALTER TABLE redemptions MODIFY tenant_id VARCHAR(36) NOT NULL;
-- ALTER TABLE logs MODIFY tenant_id VARCHAR(36) NOT NULL;

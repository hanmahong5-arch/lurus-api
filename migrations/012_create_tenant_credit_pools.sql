-- Migration: Tenant credit pools + provisioning-key audit columns on tokens.
--
-- Implements ADR 2026-05-18 (tenant-credit-pool, Accepted) Phase 1 schema additions.
-- GORM auto-migrate also creates these tables; this SQL serves as the auditable
-- migration record and matches the convention from 009 / 010 / 011.
--
-- Phase 1 is purely additive:
--   - No backfill — tenants with no row are treated as "unlimited" by the
--     enforcement layer (max_balance = -1 sentinel).
--   - Existing token rows get default 0 for the two new audit columns,
--     meaning "legacy key, not issued via Provisioning API".
--
-- PostgreSQL 11+ ADD COLUMN with a constant DEFAULT is metadata-only (no rewrite).
--
-- See _bmad-output/planning-artifacts/adr-2026-05-18-tenant-credit-pool.md
-- §9 for the five accepted decisions (monthly default reset, 90d key TTL,
-- max_quota deprecation, wallet-debit-only topup, NATS alert channel).

-- ---------------------------------------------------------------------------
-- 1. tenant_credit_pools — per-tenant pre-paid balance + ceiling + reset schedule
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS tenant_credit_pools (
    id                  BIGSERIAL    PRIMARY KEY,
    tenant_id           VARCHAR(36)  NOT NULL UNIQUE,
    parent_tenant_id    VARCHAR(36),
    created_by_user_id  INTEGER      NOT NULL,
    current_balance     BIGINT       NOT NULL DEFAULT 0,
    max_balance         BIGINT       NOT NULL DEFAULT -1,
    reset_period        VARCHAR(16)  NOT NULL DEFAULT 'monthly',
    last_reset_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    next_reset_at       TIMESTAMPTZ,
    alert_threshold_pct INTEGER      NOT NULL DEFAULT 80,
    alert_fired_at      TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_credit_pools_parent
    ON tenant_credit_pools (parent_tenant_id);

-- Partial index: only scheduled-reset pools need the reset-cron scan;
-- "none" pools never appear here.
CREATE INDEX IF NOT EXISTS idx_tenant_credit_pools_next_reset
    ON tenant_credit_pools (next_reset_at)
    WHERE reset_period <> 'none';

-- ---------------------------------------------------------------------------
-- 2. tenant_credit_pool_draws — append-only audit ledger
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS tenant_credit_pool_draws (
    id            BIGSERIAL    PRIMARY KEY,
    pool_id       BIGINT       NOT NULL,
    tenant_id     VARCHAR(36)  NOT NULL,
    token_id      INTEGER,
    log_id        BIGINT,
    direction     SMALLINT     NOT NULL,
    amount        BIGINT       NOT NULL,
    reason        VARCHAR(32)  NOT NULL,
    actor_user_id INTEGER,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tenant_credit_pool_draws_pool_time
    ON tenant_credit_pool_draws (pool_id, created_at);

CREATE INDEX IF NOT EXISTS idx_tenant_credit_pool_draws_tenant_time
    ON tenant_credit_pool_draws (tenant_id, created_at);

-- Partial index: only rows tied to relay logs (excludes topup/reset draws).
CREATE INDEX IF NOT EXISTS idx_tenant_credit_pool_draws_log
    ON tenant_credit_pool_draws (log_id)
    WHERE log_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- 3. tokens — additive audit columns for Provisioning API issued keys
-- ---------------------------------------------------------------------------

ALTER TABLE tokens ADD COLUMN IF NOT EXISTS creator_user_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tokens ADD COLUMN IF NOT EXISTS last_used_at    BIGINT  NOT NULL DEFAULT 0;

-- Partial index: only Provisioning-API-issued keys (creator_user_id > 0) need
-- the "list keys by Reseller user" lookup path. Legacy keys with default 0
-- never hit this index.
CREATE INDEX IF NOT EXISTS idx_tokens_creator_user_id
    ON tokens (creator_user_id)
    WHERE creator_user_id > 0;

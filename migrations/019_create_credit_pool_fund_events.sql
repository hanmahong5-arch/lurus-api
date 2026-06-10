-- 019: credit-pool fund events — SEAM S1 BillingOutbox supply idempotency.
--
-- GORM AutoMigrate drives the schema from internal/domain/entity.CreditPoolFundEvent;
-- this file is the PostgreSQL manual-migration counterpart for environments
-- that apply SQL out-of-band. Idempotent — safe to re-run.
--
-- One row per platform BillingOutbox supply event. UNIQUE(event_id) is the
-- idempotency guard: a replayed POST /internal/v1/provisioning/tenants/:slug/
-- credit-pool/fund returns the original result without double-crediting the
-- pool. Kept separate from tenant_credit_pool_draws (relay audit ledger) so
-- the relay hot path never pays for cross-service event semantics.
-- See internal/adapter/repo/tenant_credit_pool.go FundPoolIdempotent.

CREATE TABLE IF NOT EXISTS credit_pool_fund_events (
    id          BIGSERIAL    PRIMARY KEY,
    event_id    VARCHAR(128) NOT NULL,
    tenant_id   VARCHAR(36)  NOT NULL,
    pool_id     BIGINT       NOT NULL,
    amount      BIGINT       NOT NULL,
    new_balance BIGINT       NOT NULL,
    source      VARCHAR(64)  NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_credit_pool_fund_events_event_id UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS idx_credit_pool_fund_events_tenant
    ON credit_pool_fund_events (tenant_id);

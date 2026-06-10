-- 020: privacy erasure requests — PIPL §47 account-erasure intent / progress / evidence.
--
-- GORM AutoMigrate drives the schema from internal/domain/entity.PrivacyErasureRequest;
-- this file is the PostgreSQL manual-migration counterpart for environments
-- that apply SQL out-of-band. Idempotent — safe to re-run.
--
-- One row per platform erasure event. UNIQUE(event_id) is the idempotency
-- guard: a replayed POST /internal/v1/privacy/erase returns the original row
-- without re-running the cascade. Rows are retained permanently as compliance
-- evidence (the account_id is a pseudonymous integer, not personal data).
-- step is the crash-resume cursor of the background executor
-- (internal/lifecycle/privacy_erasure.go).

CREATE TABLE IF NOT EXISTS privacy_erasure_requests (
    id            BIGSERIAL    PRIMARY KEY,
    event_id      VARCHAR(128) NOT NULL,
    account_id    BIGINT       NOT NULL,
    request_id    VARCHAR(64)  NOT NULL DEFAULT '',
    reason        VARCHAR(255) NOT NULL DEFAULT '',
    user_id       INTEGER      NOT NULL DEFAULT 0,
    tenant_id     VARCHAR(36)  NOT NULL DEFAULT '',
    status        VARCHAR(16)  NOT NULL,
    step          VARCHAR(32)  NOT NULL DEFAULT '',
    logs_scrubbed BIGINT       NOT NULL DEFAULT 0,
    last_error    VARCHAR(512) NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ,
    CONSTRAINT uq_privacy_erasure_requests_event_id UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS idx_privacy_erasure_requests_account
    ON privacy_erasure_requests (account_id);
CREATE INDEX IF NOT EXISTS idx_privacy_erasure_requests_status
    ON privacy_erasure_requests (status);
CREATE INDEX IF NOT EXISTS idx_privacy_erasure_requests_user
    ON privacy_erasure_requests (user_id);

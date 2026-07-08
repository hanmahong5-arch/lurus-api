-- 023_add_rate_limit_columns.sql
-- Idempotent PG-only addition of business rate-limit columns to tokens and
-- tenants: rpm_limit (requests per minute) and tpm_limit (LLM tokens per
-- minute), both INT NOT NULL DEFAULT 0 where 0 = unlimited.
--
-- WHY: middleware.BusinessRateLimit adds per-token / per-tenant RPM sliding
-- windows on the relay chain (rate-limit.go only covers the ClientIP
-- dimension). The limits are data, not config — a Reseller sets them per
-- token/tenant — so they live on the rows the relay already resolves.
-- DEFAULT 0 keeps every existing row unthrottled: behavior is unchanged
-- until an operator sets a positive limit. tpm_limit is provisioned now so
-- the schema doesn't need another migration when TPM enforcement lands (it
-- is NOT enforced yet — see the TPM TODO in business_rate_limit.go).
--
-- AutoMigrate note: tenants also gains these columns via GORM AutoMigrate
-- (repo.Tenant aliases entity.Tenant, which carries the new fields). tokens
-- does NOT — the AutoMigrate model is the repo-local Token struct
-- (internal/adapter/repo/token.go), which intentionally omits them — so this
-- migration is the authoritative DDL for tokens.rpm_limit / tokens.tpm_limit.
--
-- EXECUTION CONTRACT (internal/pkg/migration/runner.go): this body runs in ONE
-- transaction; the Runner appends the schema_migrations record afterwards and
-- does NOT use ON CONFLICT on that record, so the body MUST be fully idempotent.
--
-- IDEMPOTENCY / SAFETY:
--   * ADD COLUMN IF NOT EXISTS → re-runs are no-ops.
--   * Runs after AutoMigrate on the boot lease winner: whichever side creates
--     a column first, the other becomes a no-op (identical type + default).
--   * DR restores that skip AutoMigrate still converge on the full schema.

ALTER TABLE tokens ADD COLUMN IF NOT EXISTS rpm_limit INT NOT NULL DEFAULT 0;
ALTER TABLE tokens ADD COLUMN IF NOT EXISTS tpm_limit INT NOT NULL DEFAULT 0;

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS rpm_limit INT NOT NULL DEFAULT 0;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS tpm_limit INT NOT NULL DEFAULT 0;

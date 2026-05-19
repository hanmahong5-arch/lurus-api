-- Migration 014: composite index for Provisioning LIST queries.
--
-- Tier 1.1 (2026-05-19) adds
--     GET /internal/v1/provisioning/tenants/:slug/keys
-- which filters tokens by (creator_user_id, tenant_id) and orders by id DESC
-- for pagination.
--
-- Without a covering index, that scan is O(N) over the whole tokens table
-- per request. As Resellers fan out across many tenants p99 degrades fast
-- under modest growth. The composite (creator_user_id, tenant_id, id DESC)
-- lets PostgreSQL serve the query directly from the index in sorted order
-- — no sort step, no full scan.
--
-- `IF NOT EXISTS` keeps the migration idempotent across re-runs and
-- environments that may already have an equivalent index from a prior
-- ad-hoc add. `CREATE INDEX` (non-concurrent) is safe here because the
-- tokens table is still small (< 100k rows in stage / prod combined);
-- swap for `CREATE INDEX CONCURRENTLY` if this ships to a larger table.

CREATE INDEX IF NOT EXISTS idx_tokens_creator_tenant_id_desc
    ON tokens (creator_user_id, tenant_id, id DESC);

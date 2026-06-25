-- 022_converge_default_tenant_id.sql
-- Idempotent PG-only convergence of a NON-canonical "default" tenant id onto the
-- canonical id 'default', cascading the new id across every table that carries a
-- tenant reference.
--
-- WHY (SEAM S1 cash-path root fix): the bootstrap/default tenant's canonical id
-- is 'default' — that is what GORM struct tags (default:'default'), every Go
-- write path, and a fresh PG (migration 021 §4 seed) all use. STAGE, however,
-- was hand-seeded by the ops script deploy/k8s/r6-stage/seed-default-tenant.sql
-- with id='lurus-default' (slug still 'lurus'). The result is an orphan split:
-- the credit pool + configs live under tenant_id='lurus-default' while users and
-- tokens default to tenant_id='default'. At relay time the pool gate and the
-- post-consume debit both look the pool up by the token's tenant_id ('default'),
-- miss it (GetTenantCreditPool('default') → ErrPoolNotFound), and — per the
-- documented "no pool row = unlimited, bypass" back-compat semantics (ADR
-- 2026-05-18 §5) — SILENTLY skip billing. No double-charge, no error, just a
-- pool that is never drawn: a cash-path leak. See
-- doc/seam-s1-stage-worklog-2026-06-21.md.
--
-- This migration eliminates the drift at the data layer (the stray seed itself
-- is fixed in the same change so fresh deploys never reintroduce it). After it
-- runs, the tenant row, its pool, and all per-tenant rows share one id.
--
-- EXECUTION CONTRACT (internal/pkg/migration/runner.go): this body runs in ONE
-- transaction; the Runner appends the schema_migrations record afterwards and
-- does NOT use ON CONFLICT on that record, so the body MUST be fully idempotent.
--
-- IDEMPOTENCY / SAFETY:
--   * Fresh PG (021 already seeded id='default')        → v_old is NULL → no-op.
--   * Re-run after convergence (tenant id now 'default') → v_old is NULL → no-op.
--   * Both an id='default' AND a slug='lurus' non-'default' tenant exist
--     (ambiguous; should not happen) → WARNING + skip, never guess child owners.
--   * Tables absent on this deployment (topups/subscriptions never existed on PG;
--     playground_presets exists only on the newapi base) are skipped by a
--     to_regclass + information_schema.columns guard, so the same body is correct
--     across fresh-PG, STAGE, and DR-restore schemas.

DO $$
DECLARE
    -- The bootstrap/default tenant is the one with slug='lurus'. Its canonical
    -- id is 'default'; v_old captures a non-canonical id to converge away from.
    v_old  text;
    v_pair text;
    v_tbl  text;
    v_col  text;
    -- Every (table:column) that holds a tenant id, per the live STAGE schema
    -- sweep (information_schema columns LIKE '%tenant_id%') unioned with the Go
    -- entity/migration inventory. Apply to EVERY pair, not just the obvious ones;
    -- a missed table re-creates the orphan this migration exists to remove.
    -- parent_tenant_id is included so reseller child-pool hierarchy repoints too.
    v_pairs text[] := ARRAY[
        'audit_events:tenant_id',
        'channels:tenant_id',
        'credit_pool_fund_events:tenant_id',
        'internal_api_key_tenants:tenant_id',
        'logs:tenant_id',
        'playground_presets:tenant_id',
        'privacy_erasure_requests:tenant_id',
        'redemptions:tenant_id',
        'tenant_configs:tenant_id',
        'tenant_credit_pool_draws:tenant_id',
        'tenant_credit_pools:tenant_id',
        'tenant_credit_pools:parent_tenant_id',
        'tokens:tenant_id',
        'user_identity_mapping:tenant_id',
        'users:tenant_id'
    ];
BEGIN
    SELECT id INTO v_old
        FROM tenants
        WHERE slug = 'lurus' AND id <> 'default'
        LIMIT 1;

    IF v_old IS NULL THEN
        -- Already canonical (fresh PG / prior run) — nothing to converge.
        RETURN;
    END IF;

    IF EXISTS (SELECT 1 FROM tenants WHERE id = 'default') THEN
        -- A canonical default tenant already coexists with the non-canonical one.
        -- Child rows could legitimately belong to either; auto-merging would risk
        -- mis-parenting. Leave for manual reconciliation instead of guessing.
        RAISE WARNING 'migration 022: tenant id=default already exists alongside slug=lurus id=%; skipping auto-converge (manual reconcile required)', v_old;
        RETURN;
    END IF;

    -- Repoint every child row from the old id to 'default'. to_regclass skips a
    -- table absent on this deployment; the column check skips a table that lacks
    -- the column (defensive across schema variants). format(%I/%L) guards against
    -- identifier/literal injection even though all values here are constants.
    FOREACH v_pair IN ARRAY v_pairs LOOP
        v_tbl := split_part(v_pair, ':', 1);
        v_col := split_part(v_pair, ':', 2);
        IF to_regclass('public.' || v_tbl) IS NOT NULL
           AND EXISTS (
               SELECT 1 FROM information_schema.columns
               WHERE table_schema = 'public'
                 AND table_name   = v_tbl
                 AND column_name  = v_col
           ) THEN
            EXECUTE format('UPDATE public.%I SET %I = %L WHERE %I = %L',
                           v_tbl, v_col, 'default', v_col, v_old);
        END IF;
    END LOOP;

    -- Rename the tenant PK last (children already repointed). No FK is enforced
    -- on these varchar(36) tenant columns, so order is for clarity, not safety.
    UPDATE tenants SET id = 'default', updated_at = NOW() WHERE id = v_old;

    RAISE NOTICE 'migration 022: converged tenant id % -> default across % tenant-id columns', v_old, array_length(v_pairs, 1);
END $$;

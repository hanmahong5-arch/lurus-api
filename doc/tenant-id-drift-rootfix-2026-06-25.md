# Tenant-id drift root fix (SEAM S1 cash-path) — 2026-06-25

## Symptom
On STAGE the credit pool was never drawn for default-tenant users: a `/v1/chat/completions`
call returned 200 but the pool balance and billing logs did not move unless the user/token
`tenant_id` was hand-patched. A silent cash-path leak (no error, no double-charge — just no charge).

## Root cause (data drift, not a code bug)
The canonical bootstrap tenant id is **`default`**: every GORM struct tag
(`default:'default'`), every Go write path (`provisioning.go` uses the resolved `tenant.Id`,
`user_service.GetTenantIdFromContext` and `setup.go` default to `"default"`), and the fresh-PG
seed in `migrations/021_pg_baseline_gaps.sql` §4 all use `default`.

The ops convenience script `deploy/k8s/r6-stage/seed-default-tenant.sql` was written with
`id='lurus-default'` (slug still `lurus`). STAGE was hand-seeded from it, so the tenant row,
its credit pool, configs and draws lived under `lurus-default` while users/tokens defaulted to
`default`. At relay time both the gate (`middleware/pool_balance_check.go`) and the post-consume
debit (`app/quota.go:debitTenantPool`) look the pool up by the token's `tenant_id` (`default`),
miss it, and — per the documented "no pool row = unlimited, bypass" back-compat semantics
(ADR 2026-05-18 §5) — skip billing. The semantics are correct; the data was inconsistent.

A live-schema sweep found **15 tenant-id columns across 14 deployed tables** (including
`playground_presets`, an OSS-newapi-base table absent from newhub's entities/migrations, and
`tenant_credit_pools.parent_tenant_id`). Missing any one in a fix would re-create the orphan.

## Fix
1. **`deploy/k8s/r6-stage/seed-default-tenant.sql`** — id `lurus-default` → `default` so fresh
   deploys never reintroduce the drift (the `lurus-default-org` value left as-is is the
   `zitadel_org_id` placeholder, unrelated to the tenant id).
2. **`migrations/022_converge_default_tenant_id.sql`** — idempotent, PG-only convergence:
   if a tenant with `slug='lurus'` exists under a non-`default` id AND no `default` tenant row
   exists, repoint every one of the 15 tenant-id columns to `default` (to_regclass +
   information_schema guard skips tables absent on a given deployment) and rename the tenant PK.
   No-op on fresh PG / on re-run; on the ambiguous case (both ids present) it WARNs and skips
   rather than guess child ownership.
3. **`internal/pkg/migration/converge_default_tenant_pg_test.go`** — PG-gated integration tests:
   negative control (drift present), convergence + orphan-free + runner/SQL idempotency,
   fresh-PG no-op, ambiguous-skip safety.
4. Migration ledger reserves newhub **022 = converge_default_tenant_id**.

## Verification (real evidence, no mock)
- **Disposable PG dry-run** (throwaway DB on R6 CNPG): `lurus-default → default`, all targets
  exact, `orphans=0`, idempotent re-run = no-op.
- **STAGE apply** (`psql -f` against live `newhub`): `converged tenant id lurus-default ->
  default across 15 tenant-id columns`; after = single tenant id=`default`, every table under
  `default`, `orphans=0`, full scan of all 15 columns finds **zero** `lurus-default` rows.
- **STAGE relay billing proof** (no manual tenant patching; token is `default` solely via 022):
  pool `default` **99904 → 99901** (−3); real DeepSeek 200 `"SEAM S1 ROOT-FIX OK"`; new billing
  log id=5 `tenant_id=default` (quota 3, 16+9 tok); pool draws 2 → 3.

## Notes / follow-ups
- 022 is data-convergence only; no API/proto/NATS contract change.
- On owner deploy of the image carrying 022, the embedded runner re-runs 022 → idempotent
  no-op (STAGE already converged) → records it in `schema_migrations`.
- The simple `INSERT 'default' ON CONFLICT (id)` in origin/main's 021 would collide on the
  `slug` unique if it ever executed against a pre-existing `lurus-default` DB without a
  `schema_migrations` record; not a real DR risk (DR restores `schema_migrations` too), and 022
  removes `lurus-default` so subsequent 021 runs are clean no-ops.

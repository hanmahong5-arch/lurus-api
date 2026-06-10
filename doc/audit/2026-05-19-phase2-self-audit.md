# Phase 2 Self-Audit — Credit Pool + Provisioning API (2026-05-19)

**Scope**: 8-commit Phase 2 swarm (schema + α/β/γ/δ/ε lanes + sprint-status + regression fix). **Trigger**: reliability self-test before STAGE drill. **Auditors**: 3 parallel Explore agents (backend / frontend / edge+integration). **Outcome**: 3 P0 bugs fixed this session; P1/P2 gaps queued for Phase 3 STAGE validation. **Companion ADR**: `_bmad-output/planning-artifacts/adr-2026-05-18-tenant-credit-pool.md`.

> Per §4.1 ⑥: `go build` clean + hand-written unit tests passing is "markers present", NOT "measurement meaningful". Three independent audits found 3 real cash-path / tenant-isolation bugs the swarm missed (all fixed) + a coverage-gap list (deferred to Phase 3 where real HTTP traffic exercises them).

## P0 bugs fixed (3 commits)

### B1 — `video-router` missing `PoolBalanceCheck` (commit `d4a540a4`)

**Symptom**: a Reseller with exhausted credit pool could still spend through `/v1/video/generations`, `/v1/videos*`, `/v1/videos/:id/remix`, all `/kling/v1/*`, `POST /jimeng`.
**Root cause**: ADR §3.1 enforcement was written against chat-relay groups (`/v1`, `/mj`, `/suno`, `/v1/audio`, `/v1beta`); the informal "5 relay groups" phrasing missed video routes which live in a separate `gin.Group` set up by `SetVideoRouter`.
**Fix**: added `middleware.PoolBalanceCheck()` to all 3 video router groups in `internal/adapter/handler/router/video-router.go` (after `TokenAuth`, before `Distribute`); updated `internal/adapter/middleware/pool_balance_check.go` header to enumerate all **8** gated groups.
**Phase 3 validation**: drive a tenant pool to zero on R6 STAGE, `curl POST /v1/video/generations` → must 402.

### B2 — `Provisioning Create` had no tenant ownership check (commit `80e674ad` + follow-up)

**Symptom**: any holder of a `ScopeProvisioning` `InternalApiKey` could mint tokens for any tenant slug (incl. competitor's) — `creator_user_id` was attribution only, never a permission boundary. (Revoke was already safe via `token.TenantId != tenant.Id` guard; only Create was open.)
**Root cause**: ADR §4.2 specifies auth scope but not who-can-create-where; no `tenant_id` column on `internal_api_keys`.
**Fix — two-tier authorization** via `repo.InternalKeyAllowedForTenant(apiKey *InternalApiKey, tenantID string) bool`:
1. Platform-wide admin keys (`ScopeAll = "*"`) bypass the whitelist.
2. Narrow-scope keys (e.g. `ScopeProvisioning`-only) require explicit `(api_key_id, tenant_id)` row in `internal_api_key_tenants` (new `migrations/013_create_internal_api_key_tenants.sql`); missing row → 403 (fail-closed).
Handler `CreateProvisionedKey` returns `403 TENANT_NOT_AUTHORIZED` + `common.SysLog` audit line when `ScopeAll` absent AND no whitelist row. Integration test `TestInternalKeyAllowedForTenant` covers nil apiKey / zero id / empty tenant / ScopeAll bypass / narrow empty-whitelist deny / narrow authorised allow / narrow wrong-tenant deny (skipped when `TEST_POSTGRES_DSN` unset). Follow-up refactor made platform-admin keys work without manual INSERT.
**Deployment**: STAGE/PROD platform admin keys (`*` in `scopes` JSON) work out of the box; only Reseller narrow keys need a whitelist row:
```sql
INSERT INTO internal_api_key_tenants (api_key_id, tenant_id)
  VALUES (<reseller_key_id>, '<reseller_tenant_id>') ON CONFLICT DO NOTHING;
```
**Phase 3 validation**: unauthorised key → 403; authorised → 201; confirm audit log on 403 path.

**Migration 014 decision (Option A vs B)** — to permanently shape B2:
- **A — add nullable `tenant_id` column to `internal_api_keys`**: NULL = platform-wide, non-NULL = tenant-scoped; handler becomes `apiKey.TenantId == "" || apiKey.TenantId == tenant.Id`; drop the whitelist table.
- **B — keep whitelist table forever**: one key can target multiple tenants (useful for platform-admin key managing many Resellers); +1 table +1 join.
- Recommendation: **A for single-owner Reseller keys (common case) + B retained for the one platform-admin key spanning all tenants** (table optional, key-level `tenant_id` fast path). Decide before migration 014.

### B3 — NATS `publish → mark` ordering, fail-open (commit `ceaa3948`)

**Symptom**: `publishPoolThreshold` Step 3 published to NATS before Step 4 wrote `alert_fired_at`. If publish succeeded + schema write failed, event was on the wire but dedup didn't know — after the 1h Redis SETNX TTL expired, next trigger could re-publish → alert-storm. Lane δ acknowledged but left unhandled.
**Fix**: reorder Steps 3/4 — **mark before publish, fail-closed**. Trade-off (in source comment): a publish failure now leaves "marked but not delivered" for the dedup window, which ops can replay manually — preferred over alert-storm. Test rewrite: `Test 5 DoesNotMarkOnPublishFailure` → `Test 5 MarksEvenWhenPublishFails` (semantics reversed) + new `Test 5b DoesNotPublishWhenMarkFails`. All 8 pool-threshold tests pass.

## P1/P2 risk register (deferred to Phase 3)

| # | Sev | Concern | File/surface | Phase 3 mitigation |
|---|-----|---------|--------------|--------------------|
| C1 | P1 | 5 admin + 2 provisioning handlers have NO unit tests | `handler/tenant_credit_pool.go`, `handler/provisioning.go` | STAGE smoke real HTTP path |
| C2 | P1 | `app/quota.go` Phase 2.5 (post-consume pool debit + draw insert) no integration test | `app/quota.go:531-547` | STAGE end-to-end relay |
| C3 | P1 | Wallet-revert two-phase commit on topup failure untested | `handler/tenant_credit_pool.go:156-170` | STAGE topup drill: fail platform `WalletDebit`, confirm pool not credited |
| C4 | P1 | Frontend drawer lifecycle (X/overlay/Esc/post-success refresh) no tests | `web/src/pages/v2/Tenants/CreditPoolDrawer.jsx` | Phase 3 Playwright e2e |
| C5 | P1 | 401/500/network error paths on drawer no graceful-handling tests | same | same |
| C6 | P2 | "Pool absent = unlimited" = silent bypass (Reseller never Creates pool → uncapped) | `middleware/pool_balance_check.go:42-46` | Operator runbook: onboarding MUST include explicit pool Create + UI ceiling prompt |
| C7 | P2 | Concurrent topup+debit racing same `current_balance` ceiling uncovered | `repo/tenant_credit_pool.go` | atomic-debit-race test covers half; STAGE synthetic burst the rest |
| C8 | P2 | `next_reset_at` monthly/weekly arithmetic at month-end boundary (Jan31→Feb28?) uncovered | `repo/tenant_credit_pool.go:267-287` | Manual end-of-month verification, one cycle |
| C9 | P2 | Migration 012 backfill on 50M-row `tokens` table unknown (`ADD COLUMN DEFAULT 0` metadata-only PG11+, but index build not) | `migrations/012_create_tenant_credit_pools.sql` | DBA eval before PROD; STAGE on representative dataset |
| C10 | P2 | `internal_api_key_tenants` seeding required only for narrow-scope keys; no editing UI yet | `migrations/013_*` + B2 | Operator runbook + eventual Phase 3 admin UI |

C6 flagged twice: a Reseller who never Creates a pool keeps unlimited spend (intentional back-compat — existing tenants must not break), but must be loud in onboarding.

## NOT claimed (§4.1 ⑥)

- "Phase 2 fully verified" — only B1/B2/B3 evidenced; C1–C10 remain markers.
- "STAGE drill complete" — not run.
- "Switch integration unblocked" — depends on G1 (Switch Provisioning API timeline).
- "PROD push authorised" — gated on G2/G3.
- "Migration 013 safe on populated `internal_api_keys`" — CREATE TABLE only, no backfill; unproven on STAGE.

## Pre-STAGE-drill sequence

1. Apply migration 012 + 013 on STAGE (R6 / `test-newhub.lurus.cn`).
2. Drive Reseller pool to zero, curl every B1 gated endpoint → all must 402.
3. Platform admin key (`*` in scopes): Provisioning Create against any tenant → 201, no `internal_api_key_tenants` row (ScopeAll bypass).
4. Narrow-scope key (`["provisioning"]` only, no whitelist row) → 403 `TENANT_NOT_AUTHORIZED` + audit log line.
5. Watch `pool.threshold_crossed` NATS events during slow burn — observe Step 3 mark *before* wire event in `alert_fired_at`.

## Followups

- Migration 014 (Phase 3): decide Option A/B per B2.
- Lane γ frontend Playwright suite (C4/C5).
- Operator runbook: pool onboarding (C6), whitelist seeding (C10), end-of-month reset (C8).
- Migration 012 backfill audit on representative `tokens` snapshot (C9) — DBA pairing.

# Phase 2 Self-Audit — Credit Pool + Provisioning API

**Date**: 2026-05-19
**Scope**: 8-commit Phase 2 swarm (schema + α/β/γ/δ/ε lanes + sprint-status + regression fix)
**Trigger**: "每一个按钮、每一次跳转、每一个边缘情况都可靠吗？" — reliability self-test before STAGE drill
**Auditors**: three parallel Explore agents (backend coverage / frontend UI / edge + integration risk)
**Outcome**: 3 P0 bugs fixed in this session; P1/P2 gaps queued for Phase 3 STAGE validation
**Companion ADR**: `_bmad-output/planning-artifacts/adr-2026-05-18-tenant-credit-pool.md`

---

## 1. Why this audit

Phase 2 swarm landed `go build ./...` + `go vet ./...` + the unit tests
that were written all green. That passes the §4.1 ⑥ "markers present"
bar but not the "measurement is meaningful" bar — the question asked
was whether every button, jump, and edge case is reliable, and the
answer requires looking outside the cone of the lanes that wrote the
code. Three independent Explore audits confirmed three real cash-path
or tenant-isolation bugs the swarm missed, plus a list of coverage
gaps that are not bugs in themselves but are unproven.

The three bugs are all fixed in this session. The coverage gaps are
deferred to Phase 3 STAGE drill where they will be exercised by real
HTTP traffic rather than synthetic tests.

---

## 2. P0 bugs fixed (this session, three commits)

### B1 — `video-router` was missing `PoolBalanceCheck`

**Symptom**: a Reseller whose tenant credit pool was exhausted could
still spend through:

- `POST /v1/video/generations`, `GET /v1/video/generations/:task_id`
- `POST /v1/videos`, `GET /v1/videos/:task_id`,
  `POST /v1/videos/:video_id/remix`, `GET /v1/videos/:task_id/content`
- all `/kling/v1/*` text-to-video / image-to-video routes
- `POST /jimeng` (Jimeng official API entry)

**Root cause**: the ADR §3.1 enforcement design was written against
the chat-relay groups (`/v1`, `/mj`, `/suno`, `/v1/audio`, `/v1beta`)
and the term "5 relay groups" was reused informally in subsequent
docs (story-q3-phase2, sprint-status, process). Video routes live in
a separate `gin.Group` set up by `SetVideoRouter`, which the swarm
did not touch — so the pool gate added by Lane α never reached them.

**Fix**: `internal/adapter/handler/router/video-router.go` — added
`middleware.PoolBalanceCheck()` to all three video router groups in
the same position used elsewhere (after `TokenAuth`, before
`Distribute`). Updated `internal/adapter/middleware/pool_balance_check.go`
header comment to enumerate all eight gated groups.

**Commit**: `d4a540a4` — `feat(relay): gate video routes with PoolBalanceCheck (B1)`

**Phase 3 validation**: on R6 STAGE, drive a tenant pool to zero,
then `curl POST /v1/video/generations` — must return 402.

### B2 — `Provisioning Create` had no tenant ownership check

**Symptom**: any holder of a `ScopeProvisioning` `InternalApiKey` could
mint tokens for any tenant slug — including a competitor's tenant —
because `creator_user_id` was used only for attribution, never as a
permission boundary. (The Revoke path already had a `token.TenantId !=
tenant.Id` guard, so it was safe; only Create was open.)

**Root cause**: ADR §4.2 specifies the auth scope (`provisioning`) but
not the *who-can-create-where* mapping. The handler assumed scope was
sufficient. There is no `tenant_id` column on `internal_api_keys`
today, so even if the handler tried to enforce ownership there was
nothing to enforce against.

**Fix (interim, fail-closed)**: until `InternalApiKey` gains a
first-class `tenant_id` column (planned migration 014, Phase 3), the
handler consults a small whitelist table `internal_api_key_tenants`
seeded by the platform admin. Empty whitelist = deny-all. Specifically:

- new `repo.InternalKeyAllowedForTenant(keyID, tenantID) bool`
- new `CREATE TABLE internal_api_key_tenants (api_key_id, tenant_id)`
  in `migrations/013_create_internal_api_key_tenants.sql`
- handler `CreateProvisionedKey` returns `403 TENANT_NOT_AUTHORIZED`
  + `common.SysLog` audit line on a missing row
- new integration test `TestInternalKeyAllowedForTenant` covers:
  empty whitelist, authorised pair, wrong tenant, wrong key, zero
  inputs (all skipped when `TEST_POSTGRES_DSN` is unset)

**Commit**: `80e674ad` — `feat(provisioning): enforce tenant ownership on key creation (B2)`

**🔴 Deployment fail-closed warning**: once migration 013 ships, ALL
Provisioning Create requests return 403 until the operator INSERTs
the first row. **This is the intended behaviour, not a bug.** Before
the first Reseller can use the API, Anita (or a platform admin) must:

```sql
INSERT INTO internal_api_key_tenants (api_key_id, tenant_id)
  VALUES (<key_id>, '<tenant_id>');
```

…for every (key, tenant) pair that is legitimate.

**Phase 3 validation**: on R6 STAGE, exercise (a) unauthorised key →
must return 403, (b) authorised key → must return 201, (c) confirm
the audit log line is emitted on the 403 path.

### Option A vs Option B for the long-term shape of B2

Two ways to make this permanent in migration 014:

- **Option A — add `tenant_id` column to `internal_api_keys`**:
  smallest change to the storage layer; one nullable column. NULL
  means "platform-wide key, can target any tenant"; non-NULL means
  "tenant-scoped key, can only target this tenant". The handler's
  `InternalKeyAllowedForTenant` becomes `apiKey.TenantId == "" ||
  apiKey.TenantId == tenant.Id`. The whitelist table is dropped.
- **Option B — keep the whitelist table forever**: lets one key
  legitimately target multiple tenants (useful for a Lurus-side
  platform admin key managing many Resellers). Costs one extra
  table and one join.

Recommendation: **Option A for keys with a single owner Reseller**
(the common case), **Option B retained for the one platform-admin
key that must legitimately span all tenants**. Effectively both —
the table becomes optional, and key-level `tenant_id` is the fast
path. Decision to be made before migration 014 lands.

### B3 — NATS `publish → mark` ordering, fail-open

**Symptom**: in the pool-threshold publisher, Step 3 published to
NATS before Step 4 wrote `alert_fired_at` to schema. If publish
succeeded and the schema write failed, the event was on the wire
but dedup did not know — and once the 1h Redis SETNX TTL expired,
the next trigger could re-publish, opening an alert-storm window.
Lane δ's comment acknowledged this but left it unhandled.

**Fix**: reorder Steps 3/4 in `publishPoolThreshold` — mark before
publish, fail-closed. Trade-off acknowledged in the source comment:
a publish failure now produces a "marked but not delivered" state
for the dedup window, which ops can replay manually. We prefer this
to the alert-storm scenario.

Test rewrite:
- `Test 5 DoesNotMarkOnPublishFailure` → `Test 5 MarksEvenWhenPublishFails`
  (semantics reversed; new contract verified)
- `Test 5b DoesNotPublishWhenMarkFails` (new)

All 8 pool-threshold tests pass after the rewrite.

**Commit**: `ceaa3948` — `fix(nats): fail-closed on pool threshold mark-fired (B3)`

---

## 3. P1/P2 risk register (deferred to Phase 3)

§4.1 ⑥ "marker vs measurement" applies — `go build` clean and
hand-written unit tests passing does NOT imply the surfaces below
are covered. Phase 3 STAGE drill + Playwright + operator runbooks
will provide the missing measurement.

| # | Severity | Concern | File / surface | Phase 3 mitigation |
|---|----------|---------|----------------|-------------------|
| C1 | P1 | 5 admin handlers + 2 provisioning handlers have **no unit tests** | `handler/tenant_credit_pool.go`, `handler/provisioning.go` | STAGE smoke runs the real HTTP path |
| C2 | P1 | `app/quota.go` Phase 2.5 path (post-consume → pool debit + draw insert) has no integration test | `app/quota.go:531-547` | STAGE end-to-end relay |
| C3 | P1 | Wallet-revert two-phase commit path on topup failure is untested | `handler/tenant_credit_pool.go:156-170` | STAGE topup drill: artificially fail platform `WalletDebit`, confirm pool not credited |
| C4 | P1 | Frontend drawer lifecycle (X close / overlay click / Esc / post-success refresh) has no tests | `web/src/pages/v2/Tenants/CreditPoolDrawer.jsx` | Phase 3 Playwright e2e |
| C5 | P1 | 401 / 500 / network error paths on the drawer have no graceful-handling tests | same | same |
| C6 | P2 | "Pool absent = unlimited" is a *silent* bypass — a Reseller who never explicitly Creates a pool consumes uncapped | `middleware/pool_balance_check.go:42-46` | Operator runbook: Reseller onboarding MUST include explicit pool Create; UI prompt to set ceiling on first sub-tenant |
| C7 | P2 | Concurrent topup + debit racing toward the same `current_balance` ceiling is uncovered | `repo/tenant_credit_pool.go` | Existing atomic-debit-race test covers half; STAGE drill with synthetic burst will cover the other half |
| C8 | P2 | `next_reset_at` monthly/weekly arithmetic at month-end boundaries (e.g., Jan 31 → Feb 28?) is uncovered | `repo/tenant_credit_pool.go:267-287` | Manual end-of-month verification, one cycle |
| C9 | P2 | Migration 012 backfill behaviour on a 50M-row `tokens` table is unknown — `ADD COLUMN ... DEFAULT 0` is metadata-only on PG 11+ but the index build is not | `migrations/012_create_tenant_credit_pools.sql` | DBA evaluation before PROD; Phase 3 STAGE applies on a representative dataset |
| C10 | P2 | `internal_api_key_tenants` whitelist seeding is a manual operator step — no UI yet | `migrations/013_*` + B2 fix | Operator runbook entry; eventual Phase 3 admin UI |

C6 is worth flagging twice: a Reseller who never Creates a pool keeps
unlimited spend, defeating the whole feature. This is intentional
back-compat (existing tenants must not break) but it must be loud in
the onboarding flow.

---

## 4. What is NOT claimed (per §4.1 ⑥)

- "Phase 2 fully verified" — only B1/B2/B3 are evidenced; C1–C10 remain
  markers, not measurements.
- "STAGE drill complete" — not run yet.
- "Switch integration unblocked" — depends on G1 (Switch team commits
  Provisioning API timeline).
- "PROD push authorised" — gated on G2/G3 (Phase 3 evidence + pilot
  Reseller commitment).
- "Migration 013 safe on a populated `internal_api_keys` table" —
  CREATE TABLE only, no backfill; safe in theory but unproven on STAGE.

---

## 5. Sequence to run before STAGE drill

1. Apply migration 012 + 013 on STAGE (R6 / `test-newhub.lurus.cn`).
2. Seed `internal_api_key_tenants` with at least one (key, tenant)
   pair, otherwise every Provisioning Create returns 403 — that's
   the fail-closed contract from §2 B2.
3. Drive a Reseller pool to zero, then curl every gated endpoint
   from §2 B1 — must all return 402, none should slip through.
4. Run an unauthorised Provisioning Create — must return 403 with
   `error_code: "TENANT_NOT_AUTHORIZED"` and produce the audit log
   line.
5. Watch `pool.threshold_crossed` events on NATS during a slow burn
   — observe Step 3 mark *before* the wire event in `alert_fired_at`.

---

## 6. Followups

- Migration 014 (Phase 3) — decide Option A / B per §2 B2, write
  schema change, drop or keep `internal_api_key_tenants`.
- Lane γ frontend Playwright suite — covers C4 / C5.
- Operator runbook entries — pool onboarding (C6), whitelist seeding
  (C10), end-of-month reset verification (C8).
- Migration 012 backfill audit on a representative `tokens` snapshot
  (C9) — DBA pairing.

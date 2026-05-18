# ADR: Tenant Credit Pool — Per-Tenant Spend Caps for Reseller Mode

**Status**: Accepted (2026-05-18, Anita signoff)
**Date**: 2026-05-18
**Authors**: Architect (lurus-newhub)
**Affected service**: `2b-svc-newhub` (hub.lurus.cn)
**Signed off by**: Anita — all five §9 questions resolved 2026-05-18 (see §9 for the decision table)

---

## 1. Context & Problem Statement

Newhub currently enforces spend at the token level only: each `Token` row has `remain_quota` (a hard integer cap in lurus-units) and an `unlimited_quota` boolean. When a token is exhausted the relay returns 429. This is sufficient for Personal mode but breaks down for Reseller mode, which is the primary revenue vehicle for B2B accounts.

**The gap in concrete terms**:

A Reseller running 20 customer tenants today has no way to say "tenant acme-corp may consume at most 500 CNY this month." The Reseller creates a pool of tokens manually, distributes them, and can only track spend by summing `logs` after the fact — there is no pre-debit gate. Two failure modes follow:

- **Overspend**: A customer's key chews through relay quota uncapped; the cost lands on the Reseller's lurus-platform wallet without warning.
- **Underspend confusion**: The Reseller sets token-level `remain_quota` on each key, but this requires the Reseller to manually account for per-model cost variance, is not reset on a calendar schedule, and is invisible to the EndUser.

Analogous products have solved this with hierarchical budget enforcement: OpenRouter's Management Key + `limit`/`limit_reset` on provisioned keys; LiteLLM's four-level Org→Team→User→Key budget hierarchy; Portkey's Org→Workspace→API Key with 80%/100% threshold alerts. None of newhub's existing tables support this pattern.

**User stories this unlocks**:

- "As a Reseller, I want to cap tenant acme-corp at 500 CNY/month so that their runaway usage cannot drain my platform wallet beyond my contracted ceiling."
- "As a Reseller, I want to top up a tenant's pool mid-month without touching my platform wallet credentials, so that I can self-serve customer upgrades."
- "As a Reseller, I want to programmatically provision a named API key for a customer with a per-key USD ceiling, so I can automate onboarding without a browser UI." (mirrors OpenRouter's `POST /api/v1/keys`)
- "As an EndUser, I want to receive a 402 with a clear message when my tenant's monthly budget is exhausted, so I know to contact my Reseller."

---

## 2. Decision

Add a `tenant_credit_pools` table that holds a per-tenant pre-paid balance, an optional ceiling, and a reset schedule. During relay, the enforcement layer checks the tenant pool *before* per-token quota: if the pool is exhausted the request is rejected with HTTP 402 regardless of remaining token quota. Debit happens per-request inside a serializable database transaction to prevent overspend under burst load. Resellers manage pools via new session-authenticated admin endpoints. A new `ScopeProvisioning` internal bearer scope enables programmatic key creation that mirrors the OpenRouter Management Key pattern. All existing per-token quota logic is preserved unchanged (additive schema only in Phase 1; no backfill).

---

## 3. Schema Design

### 3.1 New table: `tenant_credit_pools`

Additive — GORM auto-migrate will create it on startup. No SQL migration required for Phase 1 (existing tenants get no row, which the enforcement layer interprets as "unlimited" — backward-compatible).

A SQL migration file (`012_create_tenant_credit_pools.sql`) should be created as the auditable record before the feature ships, following the convention established in migrations 009–011.

```
tenant_credit_pools
-------------------
id                  BIGSERIAL PRIMARY KEY
tenant_id           VARCHAR(36) NOT NULL UNIQUE  -- FK to tenants.id
parent_tenant_id    VARCHAR(36) NULL             -- NULL = top-level Reseller pool; non-NULL = sub-tenant delegated from parent
created_by_user_id  INTEGER NOT NULL             -- users.id of Reseller who created this pool
current_balance     BIGINT NOT NULL DEFAULT 0    -- lurus-units remaining; 0 = exhausted
max_balance         BIGINT NOT NULL DEFAULT -1   -- ceiling; -1 = unlimited (default for existing tenants)
reset_period        VARCHAR(16) NOT NULL DEFAULT 'none'
                                                 -- enum: 'none' | 'daily' | 'weekly' | 'monthly'
last_reset_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
next_reset_at       TIMESTAMPTZ NULL             -- precomputed; NULL when reset_period='none'
alert_threshold_pct INTEGER NOT NULL DEFAULT 80  -- fire alert when balance < (max_balance * threshold / 100)
alert_fired_at      TIMESTAMPTZ NULL             -- last time alert was emitted (deduplicate)
created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()

Indexes:
  UNIQUE (tenant_id)
  INDEX (parent_tenant_id)              -- query child pools for a Reseller
  INDEX (next_reset_at) WHERE reset_period != 'none'  -- partial index for reset job
```

**Backward compatibility**: rows are opt-in. Any tenant without a row is treated as `max_balance = -1` (unlimited). The `Tenant.max_quota` field on the existing `tenants` table is left intact but deprecated in meaning — it is not removed.

### 3.2 New table: `tenant_credit_pool_draws`

Audit ledger for every debit and credit. Write-heavy but append-only; no updates. Partitioning by month is a Phase 3 concern.

```
tenant_credit_pool_draws
------------------------
id              BIGSERIAL PRIMARY KEY
pool_id         BIGINT NOT NULL       -- FK to tenant_credit_pools.id
tenant_id       VARCHAR(36) NOT NULL  -- denormalized for query simplicity; indexed
token_id        INTEGER NULL          -- which token triggered this draw (NULL for topup/reset)
log_id          BIGINT NULL           -- FK to logs.id (NULL for topup/reset)
direction       SMALLINT NOT NULL     -- 1 = debit, -1 = credit (topup or reset)
amount          BIGINT NOT NULL       -- lurus-units; always positive
reason          VARCHAR(32) NOT NULL  -- 'relay_debit' | 'topup' | 'reset' | 'adjustment'
actor_user_id   INTEGER NULL          -- Reseller user who performed topup/adjustment; NULL for relay debits
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()

Indexes:
  INDEX (pool_id, created_at)   -- pool usage history paginated by time
  INDEX (tenant_id, created_at) -- admin dashboard query
  INDEX (log_id) WHERE log_id IS NOT NULL
```

### 3.3 Additive columns on `tokens`

Two columns are added to `Token` to support the provisioning API and future analytics (coordinated with Lane L3):

```
creator_user_id  INTEGER NULL   -- users.id of the Reseller user who issued this key via provisioning API
last_used_at     BIGINT NULL    -- Unix timestamp, updated on relay hit (replaces stale AccessedTime for provisioned keys)
```

Both are nullable and default NULL — no backfill required. GORM auto-migrate adds them. Auditable record goes in the same `012` SQL file as the pool table.

### 3.4 Debit strategy: per-request vs. per-billing-tick

**Decision: per-request debit inside a serializable transaction.**

Rationale:
- OpenRouter debits keys synchronously during request fulfillment; the `limit` on a provisioned key is a hard ceiling, not a soft advisory. Async settlement creates an overspend window proportional to tick frequency.
- The `BillingOutbox` pattern already exists in newhub for platform gRPC calls (see `billing_outbox` table). Per-request pool debit is local to the newhub Postgres instance — no network hop — so the latency cost is one row insert + one UPDATE under serializable isolation, estimated < 2 ms on the existing RDS instance.
- LiteLLM's approach (token budget enforced per-request with Redis atomic decrement) achieves the same goal. We use Postgres transactions rather than Redis to keep the pool balance authoritative in one place and avoid a dual-write between Redis and Postgres.

**Tradeoff acknowledged**: write amplification on high-concurrency tenants. Mitigation: the `tenant_credit_pool_draws` insert is append-only; the `tenant_credit_pools` UPDATE uses `FOR UPDATE SKIP LOCKED`-safe patterns; batch debit (accumulate 5-second window then flush) can be toggled per-tenant in Phase 3 without changing the schema.

---

## 4. API Contract Sketch

### 4.1 Reseller-facing (session auth, `v2/admin` prefix, existing Zitadel JWT + RootAuth middleware)

**Create or update pool for a sub-tenant**
```
POST /api/v2/admin/tenants/:id/credit-pool
Auth: Zitadel JWT (Reseller session, must own tenant :id)
Request:  { max_balance: int64, reset_period: "none"|"daily"|"weekly"|"monthly", alert_threshold_pct: int }
Response: { pool_id: int64, tenant_id: str, max_balance: int64, current_balance: int64, reset_period: str, next_reset_at: str|null }
Errors:   404 (tenant not found or not owned by caller), 422 (invalid reset_period), 409 (pool already exists — use PATCH to update)
```

**Top up a tenant pool**
```
POST /api/v2/admin/tenants/:id/credit-pool/topup
Auth: same
Request:  { amount: int64, note: string }
Response: { pool_id: int64, previous_balance: int64, new_balance: int64, draw_id: int64 }
Errors:   404, 422 (amount <= 0), 409 (would exceed max_balance ceiling)
```

**Query recent draws**
```
GET /api/v2/admin/tenants/:id/credit-pool/usage?page=1&limit=50&from=ISO8601&to=ISO8601
Auth: same
Response: { total: int64, draws: [{ id, direction, amount, reason, token_id, log_id, created_at }] }
Errors:   404, 400 (invalid date range)
```

### 4.2 Provisioning API (internal bearer key, new `ScopeProvisioning` scope)

Mirrors OpenRouter `POST /api/v1/keys` shape. Path namespace `/internal/v1/provisioning/` keeps it isolated from existing internal scopes.

**Create a provisioned key for a tenant**
```
POST /internal/v1/provisioning/tenants/:slug/keys
Auth: Bearer <management-key> with scope "provisioning:write"
Request:
  {
    name: string,
    limit: int64|null,           -- per-key quota ceiling in lurus-units; null = unlimited
    limit_reset: "none"|"daily"|"weekly"|"monthly",
    expires_at: ISO8601|null,
    model_allowlist: [string],   -- empty = all models allowed (mirrors OpenRouter model_whitelist)
    allow_ips: [string]          -- empty = no IP restriction
  }
Response:
  {
    id: int,
    key: string,                 -- full key, shown once
    name: string,
    creator_user_id: int,
    limit: int64|null,
    limit_reset: string,
    expires_at: string|null,
    created_at: string
  }
Errors: 404 (slug not found), 422 (invalid model in allowlist), 409 (name collision within tenant)
```

**Revoke a provisioned key**
```
DELETE /internal/v1/provisioning/tenants/:slug/keys/:id
Auth: same
Response: 204 No Content
Errors:   404
```

---

## 5. Quota Enforcement Semantics

**Precedence order** (evaluated in this exact sequence on every relay request):

1. **Token status**: if `token.Status != 1` (disabled/expired/deleted) → 401.
2. **Token expiry**: if `token.ExpiredTime > 0 && now > ExpiredTime` → 401.
3. **IP allowlist**: if `token.AllowIps` set and client IP not in list → 403.
4. **Tenant pool** (new): if tenant has a `tenant_credit_pools` row AND `max_balance != -1` AND `current_balance <= 0` → **402**.
5. **Per-token quota**: if `!token.UnlimitedQuota && token.RemainQuota <= 0` → **429**.

Rationale for 402 vs. 429 vs. 403 at step 4: HTTP 402 "Payment Required" is the industry signal for a spend ceiling hit (OpenAI, Anthropic, and OpenRouter all use 402 for quota-exhaustion scenarios tied to billing). 429 is reserved for rate limits (requests-per-minute). 403 is an authorization failure. Using distinct codes lets clients branch without parsing response bodies.

**Edge cases**:

- Pool at 0, token has remaining quota → **block** (HTTP 402). The pool is the outer envelope; a token cannot spend from a depleted pool.
- Token at 0, pool has balance → **block** (HTTP 429). The pool does not auto-refill per-token quota. Refill must be explicit (Reseller topup → pool balance; EndUser must get a new token).
- Pool `max_balance = -1` (unlimited row exists) → skip pool check entirely; fall through to per-token check.
- No pool row for tenant → same as unlimited; fall through to per-token check.
- `UnlimitedQuota = true` on token, pool has balance → pool still debited per-request (usage accounting), but token check is skipped.

---

## 6. Migration Strategy (Additive)

**Phase 1 — Schema only, no behavior change** (this ADR):
- Add `tenant_credit_pools`, `tenant_credit_pool_draws` via GORM auto-migrate + `012_create_tenant_credit_pools.sql`.
- Add `creator_user_id`, `last_used_at` columns to `tokens`.
- Enforcement layer added but only activates when a pool row exists with `max_balance != -1`.
- All existing tenants have no pool row → unlimited by default. Zero behavior change.
- No data backfill.

**Phase 2 — Reseller opt-in** (next sprint):
- Ship the admin UI for pool creation/topup.
- Ship the provisioning API endpoints.
- Resellers can opt tenants into capped pools voluntarily.
- Alert threshold notifications wired to platform notification service.

**Phase 3 — Deferred**:
- Forced-pool mode for tenants on downgraded plan (plan_type downgrade triggers pool ceiling from `tenants.max_quota`).
- Batch debit mode for high-throughput tenants (toggle per-pool).
- `tenant_credit_pool_draws` table partitioning by month.
- Portkey-style 80%/100% webhook callbacks to Reseller-configured URLs.

---

## 7. Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
|---|------|-----------|--------|-----------|
| 1 | **Race condition on concurrent pool debits** — two relay workers both read `current_balance = 10`, both approve, both debit, balance goes negative | High (burst traffic) | High (overspend) | Use `UPDATE tenant_credit_pools SET current_balance = current_balance - $1 WHERE id = $2 AND current_balance >= $1` — atomic conditional update returns 0 rows if balance insufficient; retry logic treats 0 rows as "pool exhausted." Wrap in `READ COMMITTED` (PG default is sufficient; serializable not needed because the conditional UPDATE is itself atomic). |
| 2 | **Pool drift vs. platform wallet** — relay debits pool but platform `WalletDebit` gRPC call fails; pool decremented but platform wallet not charged | Medium | Medium (revenue leakage) | Existing `BillingOutbox` pattern handles this: platform debit is written to outbox within the same transaction as pool debit; outbox worker retries until confirmed. Pool debit is local and immediate; platform settlement is eventually consistent but guaranteed. |
| 3 | **Audit gap** — `tenant_credit_pool_draws` not written for a relay debit (crash between debit and draw insert) | Low | Medium (compliance) | Both `tenant_credit_pools` UPDATE and `tenant_credit_pool_draws` INSERT must be in the same DB transaction. If the transaction rolls back, neither row is written. If the server crashes after commit, the draw row exists. This is as strong as the existing `billing_outbox` guarantee. |
| 4 | **Tenant deletion cascade** — a tenant is soft-deleted while active tokens still relay | Low | High (orphaned debits against a ghost pool) | On soft-delete of a tenant: (a) set all tenant's token statuses to disabled (existing cascade behavior covers this), (b) set `tenant_credit_pools.max_balance = 0` to block new relay, (c) leave pool row intact for audit. Hard cascade `ON DELETE` is explicitly not used (matches existing pattern in migration 004). |
| 5 | **Provisioning key compromise** — a leaked management key can create unlimited tokens for a tenant | Medium | High (abuse, billing exposure) | (a) Provisioning keys are scoped to `ScopeProvisioning` only (cannot read other tenants or platform credentials). (b) Each provisioned key carries `creator_user_id` for audit. (c) Revocation is immediate via `DELETE /internal/v1/provisioning/tenants/:slug/keys/:id`. (d) Future: automatic rotation after N days (see Open Questions §9). Blast radius is limited to the single tenant's pool ceiling — the Reseller's platform wallet is not directly accessible via a provisioning key. |

---

## 8. Test Plan

### Unit tests (before any relay integration)

1. **Pool decrement happy path**: given a pool with `current_balance = 100`, debit 10 → `current_balance = 90`, draw row inserted with `direction = 1, amount = 10, reason = 'relay_debit'`.
2. **Atomic debit race**: spawn 20 goroutines each attempting to debit 10 from a pool with `current_balance = 50`; assert exactly 5 succeed and the rest return "exhausted"; assert `current_balance = 0` (never negative).
3. **Exhausted pool rejection**: pool `current_balance = 0, max_balance = 100`; call enforce → expect enforcement returns pool-exhausted; assert no draw row written.
4. **Unlimited pool bypass**: pool row with `max_balance = -1` or no pool row; assert enforcement returns "allow" without touching `tenant_credit_pool_draws`.
5. **Reset period boundary**: pool with `reset_period = 'monthly'`, `next_reset_at = now() - 1s`; trigger reset job; assert `current_balance` restored to `max_balance`, `last_reset_at` updated, new `direction = -1` draw row with `reason = 'reset'`.

### Integration tests (relay path + DB)

1. **End-to-end provisioning + relay + pool debit**: create pool via `POST /api/v2/admin/tenants/:id/credit-pool`, create key via `POST /internal/v1/provisioning/tenants/:slug/keys` with `limit = 100`, relay one request that costs 10 units, assert pool `current_balance` decremented, `tenant_credit_pool_draws` has one row, relay log has matching `log_id`.
2. **Pool exhaustion blocks relay**: set pool `current_balance = 0`, attempt relay → assert HTTP 402, assert no draw row inserted, assert token's `remain_quota` unchanged (pool gate fires before token gate).
3. **Topup via admin API then relay succeeds**: topup 50 units, relay request costing 30, assert `current_balance = 20`.

### Chaos test

1. **Burst load against a thin pool**: pool `current_balance = 100`, fire 200 concurrent relay requests each costing 5 units (total demand: 1000 units). Assert: (a) final `current_balance >= 0` (never negative), (b) total draws summed `<= 100`, (c) rejected requests all return 402, (d) no deadlock or lock-wait timeout logged.

---

## 9. Resolved Decisions (2026-05-18, Anita signoff)

All five open questions have binding resolutions. Phase 2 implementation MUST conform to the table below. Deviation requires a new ADR amendment + fresh signoff.

| # | Topic | Resolution | Implementation Note |
|---|-------|-----------|--------------------|
| Q1 | `reset_period` default | **`'monthly'`** — auto-reset on calendar month boundary | UI defaults to `monthly`; `none` remains a selectable Reseller option. Cron/reset job triggers off the partial index `(next_reset_at) WHERE reset_period != 'none'`. |
| Q2 | Management Key rotation | **90-day TTL with T-14d / T-7d / T-1d alerts** | `internal_api_keys.expires_at` already exists — wire enforcement on `ScopeProvisioning` keys only (other internal scopes unchanged). Alerts published via NATS (see Q5). |
| Q3 | Legacy `tenants.max_quota` field | **Keep for back-compat, mark deprecated in code, plan Q4 migration** | Add `// Deprecated: superseded by tenant_credit_pools.max_balance; removed in Q4 migration` doc-comment on the Go struct field. Behavior of MaxQuota is unchanged in Phase 1/2. |
| Q4 | Pool topup source | **Wallet-debit only — no admin-grant endpoint** | `POST /credit-pool/topup` handler MUST call `DebitWalletGRPC` inside the same DB transaction as the pool increment (BillingOutbox pattern revert on platform debit failure). **No `/credit-pool/grant` route exists or will be added** — this forecloses the admin-grant abuse vector permanently. Operational ad-hoc adjustments use direct DB writes with audit trail, not an HTTP endpoint. |
| Q5 | Alert delivery channel | **NATS `LLM_EVENTS` stream** | New event type `pool.threshold_crossed` with payload `{tenant_id, pool_id, threshold_pct, current_balance, max_balance, fired_at}`. Notification service subscribes and routes to Reseller per existing preferences (consistent with how `IDENTITY_EVENTS` / `LUCRUM_EVENTS` are consumed today). |

**Strictest decision**: Q4. Implementation MUST NOT add any "convenience" path that bypasses `WalletDebit`. If `WalletDebit` is unavailable, topup fails and the outbox retries — pool balance is never incremented without a corresponding wallet debit.

**Followup**: a brief design note (`doc/contracts/switch-provisioning-api.md`) will extract §4.2 of this ADR into a Switch-team-facing contract during Phase 2 prep; the ADR remains the canonical source.

---

## 10. Out of Scope

1. **Workspace-level policy inheritance** (LiteLLM Org→Team model): newhub's two-level Reseller→EndUser hierarchy is sufficient for the current customer base. A full four-level hierarchy is a separate ADR.
2. **Semantic caching and cost deduplication**: reducing pool burn rate via prompt caching or response reuse is a performance optimization, not a billing primitive.
3. **Prompt registry and model allowlist enforcement at the router level**: the `model_allowlist` field on provisioned keys is stored but evaluated only at token validation time; complex policy inheritance (e.g., workspace-level model bans) is out of scope.
4. **Cross-tenant pool sharing** (a single pool spanning multiple tenants): each pool row has a 1:1 relationship with one `tenant_id`. Sub-tenant delegation via `parent_tenant_id` is schema-ready but not implemented in Phase 1 or 2.
5. **Real-time WebSocket push for pool balance events**: the alert mechanism is async (NATS or HTTP notify); streaming balance updates to a Reseller dashboard via WebSocket is a frontend feature and a separate story.

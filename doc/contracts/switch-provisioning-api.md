# Switch Provisioning API Contract

**Version**: v0.1.0 (draft)
**Service**: `2b-svc-newhub` (hub.lurus.cn) — STAGE: `test-newhub.lurus.cn` on R6
**Status**: Draft for Switch team integration review
**Last updated**: 2026-05-18 (Lane ε / Wave 1 Phase 2)
**Source ADR**: `_bmad-output/planning-artifacts/adr-2026-05-18-tenant-credit-pool.md` §4.2 (canonical)

---

## 1. Purpose

This document extracts the Provisioning API contract from the canonical
ADR (`adr-2026-05-18-tenant-credit-pool.md` §4.2) into a Switch-team-facing
spec. It exists so the Switch team (`2c-gui-switch`) can integrate against
a stable surface without having to read the entire ADR.

**Canonicality**: when this doc and the ADR disagree, the ADR is correct.
File a PR against this doc to re-sync — do not implement against this doc
alone if the divergence affects behaviour.

**Scope**: Reseller-driven programmatic key creation and revocation only.
Pool management (create / topup / usage query) lives on the
session-authenticated `v2/admin` surface (ADR §4.1) and is **not** Switch
team's integration target.

---

## 2. Authentication

> **🔴 Header convention — read this first**
>
> Provisioning API requires header `X-API-Key: <management-key>`. It is
> NOT `Authorization: Bearer <token>`. This matches the newhub
> `internal_api_auth.go` middleware that all `/internal/*` routes share.
> Sending `Authorization: Bearer …` will return HTTP 401 with
> `{"success": false, "message": "API key required"}` — the middleware
> reads the `X-API-Key` header only.

### 2.1 Issuing a management key

A management key is an `internal_api_keys` row with scope
`provisioning:write`. Issuance is an operator action on newhub:

```bash
# On newhub (operator session)
# See _bmad-output/planning-artifacts/adr-2026-05-18-tenant-credit-pool.md §9 Q2
#   for the 90-day TTL policy + T-14d/T-7d/T-1d rotation alert schedule.
```

Switch team receives the key out-of-band (1Password / encrypted channel).

### 2.2 Key rotation

Per ADR §9 Q2: management keys carry a 90-day TTL. Rotation is operator-driven
via NATS-published alerts at T-14d / T-7d / T-1d before `expires_at`. A
rotated key has a fresh value; the previous key is revoked atomically.

Switch team should treat any 401 response on a previously-working key as
"key has been rotated — fetch new key from secret store".

### 2.3 Scope check

The `provisioning:write` scope grants:
- Create / revoke tokens within any tenant.
- Read tenant slug → tenant ID mapping (implicit in `:slug` path arg).

It does NOT grant:
- Read/write across other internal scopes (`balance`, `quota`, `user`).
- Access to platform credentials, wallet, or Reseller session.

Blast radius is bounded to one tenant per call (the `:slug` parameter).

---

## 3. Endpoints

### 3.1 Create a provisioned key

```
POST /internal/v1/provisioning/tenants/:slug/keys
```

**Headers**:

```
X-API-Key: lurus_ik_<management-key>
Content-Type: application/json
```

**Path params**:
- `:slug` — tenant slug (URL-safe identifier; not the numeric tenant ID).

**Request body**:

```json
{
  "name": "acme-prod-key-2026-q3",
  "limit": 50000000,
  "limit_reset": "monthly",
  "expires_at": "2026-12-31T23:59:59Z",
  "model_allowlist": ["gpt-4o", "claude-sonnet-4-5"],
  "allow_ips": ["203.0.113.0/24"]
}
```

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `name` | string | yes | Human label for audit + UI display. Unique within tenant (see §4 idempotency). |
| `limit` | int64 \| null | no | Per-key quota ceiling in lurus-units. `null` = unlimited. |
| `limit_reset` | string | no | One of `"none"`, `"daily"`, `"weekly"`, `"monthly"`. Default: `"none"`. |
| `expires_at` | string \| null | no | ISO 8601 UTC. `null` = no expiry. |
| `model_allowlist` | string[] | no | Empty array = all models allowed (mirrors OpenRouter `model_whitelist`). |
| `allow_ips` | string[] | no | CIDR or single IPs. Empty = no IP restriction. |

**Success response** (HTTP 200):

```json
{
  "id": 12345,
  "key": "sk-lurus-abcdef0123456789...",
  "name": "acme-prod-key-2026-q3",
  "creator_user_id": 42,
  "limit": 50000000,
  "limit_reset": "monthly",
  "expires_at": "2026-12-31T23:59:59Z",
  "created_at": "2026-05-18T11:23:45Z"
}
```

> **Key visibility**: the `key` field is the only time the full token
> value is returned. Switch team MUST persist it on the same response
> turn — newhub does not store the raw key (only the hashed form). Lost
> keys cannot be recovered; the only remedy is revoke + recreate.

**Error responses**:

| Status | Body shape | Cause |
|--------|------------|-------|
| 401 | `{"success": false, "message": "API key required"}` | Missing `X-API-Key` header |
| 401 | `{"success": false, "message": "Invalid or expired API key"}` | Wrong / rotated / expired management key |
| 403 | `{"success": false, "message": "Insufficient permissions. Required scope: provisioning:write"}` | Key valid but lacks scope |
| 404 | `{"success": false, "message": "Tenant not found: <slug>"}` | Slug does not exist |
| 409 | `{"success": false, "message": "Token name collision within tenant"}` | Name already in use (see §4 idempotency) |
| 422 | `{"success": false, "message": "Invalid model in allowlist: <model>"}` | Model not registered in newhub |
| 422 | `{"success": false, "message": "Invalid limit_reset: <value>"}` | Not in {none, daily, weekly, monthly} |

**curl example**:

```bash
curl -X POST \
  https://hub.lurus.cn/internal/v1/provisioning/tenants/acme-corp/keys \
  -H "X-API-Key: lurus_ik_REPLACE_ME" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "acme-prod-key-2026-q3",
    "limit": 50000000,
    "limit_reset": "monthly",
    "expires_at": "2026-12-31T23:59:59Z",
    "model_allowlist": ["gpt-4o", "claude-sonnet-4-5"],
    "allow_ips": []
  }'
```

### 3.2 Revoke a provisioned key

```
DELETE /internal/v1/provisioning/tenants/:slug/keys/:key_id
```

**Headers**:

```
X-API-Key: lurus_ik_<management-key>
```

**Path params**:
- `:slug` — tenant slug.
- `:key_id` — numeric `id` returned by the create call (NOT the raw key string).

**Success response**: HTTP 204 No Content (empty body).

**Error responses**:

| Status | Cause |
|--------|-------|
| 401 | Missing / invalid `X-API-Key` |
| 403 | Scope `provisioning:write` not granted |
| 404 | Slug not found, or `key_id` does not belong to that tenant |

**curl example**:

```bash
curl -X DELETE \
  https://hub.lurus.cn/internal/v1/provisioning/tenants/acme-corp/keys/12345 \
  -H "X-API-Key: lurus_ik_REPLACE_ME" \
  -i
```

Revocation is immediate: the key's `status` flips to disabled in the
`tokens` table, in-flight relays still complete (no mid-stream kill), but
new requests using that key return 401 within one cache-sync cycle
(default 60s; see `SYNC_FREQUENCY` env).

---

## 4. Idempotency

The Provisioning API is **best-effort idempotent by `name` within a tenant**:

- Create with a `name` that already exists → 409 (do not auto-return the
  existing key — the raw key value is not retrievable).
- Switch team's integration should:
  1. Generate a deterministic `name` per (tenant, key-purpose) tuple
     (e.g., `acme-corp:prod-relay`).
  2. On retry after network failure, attempt create; on 409, assume the
     previous attempt succeeded — query the admin surface or persist
     the response from the successful turn.
- Revoke with a non-existent `key_id` → 404. Treat as success (the key
  is gone either way).

> **Known gap (Q4 improvement)**: the current schema does not store a
> client-provided idempotency key. A retry that races with the original
> within the same TCP timeout window CAN produce two tokens with
> different `id` values but the same `name` — the second wins the
> uniqueness check by losing (409). This is acceptable for the manual
> + 1-key-per-tenant onboarding flow targeted in v0.1.0, but should be
> hardened with a proper `Idempotency-Key` header before scaling to
> high-volume programmatic provisioning. Track as Q4 follow-up.

---

## 5. Version Lock

This contract is `v0.1.0`. The version follows semver-for-APIs:

- **Patch** (`v0.1.x`): documentation clarifications, additional error
  codes, new optional response fields. Backwards-compatible — no Switch
  coordination required.
- **Minor** (`v0.2.0`): new optional request fields, new endpoints within
  the `/internal/v1/provisioning/` namespace. Backwards-compatible —
  notify Switch team but no coordinated rollout needed.
- **Major** (`v1.0.0`+): breaking changes — request schema, status codes,
  path layout, auth header convention. **Requires Switch team
  coordination and a coordinated cutover.**

Breaking changes from v0.1.0 → v0.2.0 require:
1. Bump version header in this file + the canonical ADR.
2. File an issue in `2c-gui-switch` describing the migration.
3. Hold both contracts active for ≥ 7 days during cutover.

---

## 6. Switch Integration Handoff

**Consumer repo**: `2c-gui-switch` (Wails desktop app — Go backend +
TypeScript frontend).

**Switch's expected integration surface**:
- Switch end-users in Reseller mode provision keys for their EndUser
  tenants via this API.
- The management key is held by the Reseller desktop instance (encrypted
  at rest via Wails keychain integration).
- Each desktop session calls `POST /internal/v1/provisioning/tenants/:slug/keys`
  on EndUser onboarding, presents the returned `key` once for copy-paste,
  then discards it from memory.

**Where to file integration issues**:
- Contract bugs (this doc out of sync with newhub) → issues against
  `hanmahong5-arch/lurus-newhub` with label `contract:provisioning`.
- Switch-side integration bugs → issues against `2c-gui-switch`.
- Cross-cutting (e.g., key rotation UX) → root governance repo issues
  with both labels.

**STAGE integration target**:
- Hostname: `test-newhub.lurus.cn` (R6).
- Management key: provision via operator session on R6; rotated every
  90 days per ADR §9 Q2.

**Health check**: `GET https://hub.lurus.cn/api/status` returns
`{"success": true}` — Switch team can use this for connectivity check
before attempting `POST /internal/v1/provisioning/...`.

---

## 7. Out of Scope (Phase 2)

Items intentionally NOT in the v0.1.0 surface — track for v0.2.0+:

1. **Bulk key creation** — a single call creating N keys for N sub-tenants.
   Today: one call per key.
2. **Key listing** — `GET /internal/v1/provisioning/tenants/:slug/keys`
   to enumerate existing keys without raw-key disclosure (Switch UI uses
   the v2/admin session-authenticated surface for now).
3. **Programmatic management-key rotation** — currently operator action.
   See ADR §9 Q2 for the 90-day rotation policy.
4. **Idempotency-Key header** — see §4 known gap.
5. **Webhook callbacks** — push notifications on key usage / threshold
   events route through NATS `LLM_EVENTS` (ADR §9 Q5), not direct
   provisioning-API webhooks.

---

## 8. References

- **Canonical source**: `_bmad-output/planning-artifacts/adr-2026-05-18-tenant-credit-pool.md` §4.2
- **Middleware**: `internal/adapter/middleware/internal_api_auth.go`
- **Companion**: `_bmad-output/planning-artifacts/adr-2026-05-18-budget-alerts.md` (alert event taxonomy)
- **Runbook**: `doc/runbook/pool-threshold-alert.md` (operator triage on `CreditPoolBalanceLow`)
- **Story**: `_bmad-output/planning-artifacts/story-q3-phase2-credit-pool.md` (BMAD dev-story)
- **Consumer**: `2c-gui-switch` (Wails desktop, Reseller mode integration)

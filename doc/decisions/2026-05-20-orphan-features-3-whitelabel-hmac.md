# ADR: Orphan-Feature Backfill #3 — Whitelabel HMAC Key Derivation

**Status**: Accepted (backfill — feature is already shipped, R6 STAGE wired)
**Date**: 2026-05-20
**Source commit**: `9c06cde1` (build: wire LURUS_WHITELABEL_MASTER_SECRET to r6-stage)
**Code location**:
- `internal/adapter/handler/v2_admin_whitelabel.go`
- `internal/adapter/handler/v2_admin_whitelabel_test.go` (8 test cases)

## Context

Resellers deploying Lurus-Switch as a whitelabel GUI installer ship a `whitelabel.json` sidecar with their installer that encodes branding, default endpoints, and the tenant's API key. Without integrity protection, anyone in the distribution chain (CDN, archive editor, MITM) can tamper with that JSON — swap the API key, redirect endpoints, etc.

The shipped solution: each tenant has a deterministic HMAC key derived from `sha256(LURUS_WHITELABEL_MASTER_SECRET + tenant_slug)`. The reseller signs their `whitelabel.json` with this HMAC; Switch verifies on install. Tamper = signature mismatch = abort.

The platform-admin endpoint to retrieve a tenant's HMAC key was shipped without an ADR — only commit-level documentation existed.

## Decision (retroactive)

**Document the whitelabel HMAC scheme; promote to a Platform Capability registered in `lurus.yaml`; gate behind RootJWT auth (already implemented).**

### What it does

- **Endpoint**: `GET /api/v2/admin/whitelabel/hmac-key?tenant_slug=<slug>` (platform-admin only via `RootJWTAuth` middleware).
- **Returns**: `{"success": true, "data": {"hmac_key": "64-hex-char-string"}}`.
- **Derivation**: deterministic — `sha256(masterSecret + tenantSlug) → 64-char hex`. Calling twice for the same tenant returns the same key. No DB row needed.
- **Fail-closed**: if `LURUS_WHITELABEL_MASTER_SECRET` env var is unset or empty, the endpoint returns 500 + clear error. No silent fallback.
- **Tenant existence check**: derives only after confirming tenant exists. No keys for typo'd slugs.

### Reasoning to keep

1. **Production-deployed**: R6 STAGE has the env wired (commit `9c06cde1`). PROD already uses it for Switch installer flows.
2. **Cheap to maintain**: pure stateless derivation. No DB rows, no cron, no cache invalidation. The entire feature is ~50 LOC of handler + tests.
3. **Fills a real security gap**: without it, any reseller using a whitelabel installer is one tampered archive away from a credential-leak incident.
4. **Tests are comprehensive**: 8 test cases cover success, missing slug, invalid format, nonexistent tenant, env unset, determinism, tenant isolation.

### Reasoning not to expand

- Current scheme uses a single global `LURUS_WHITELABEL_MASTER_SECRET`. Per-reseller master secrets would let resellers rotate independently. **Not needed today** — we have one reseller test slug; revisit at scale.
- HMAC scheme is HMAC-SHA256 hex. Some integrations might want JWT/JWS for forward-compat. **Not needed today** — JSON-with-signature-field works fine.

## Consequences

**Positive:**
- Integrity-protected whitelabel installer flows are a real feature that resellers will use.
- No state to manage, no secret to rotate per tenant.

**Negative:**
- Single global master secret = single rotation event affecting all whitelabel installers if compromised. Mitigation: keep `LURUS_WHITELABEL_MASTER_SECRET` in K8s Secret with restricted access; document rotation runbook (see Action items).
- No introspection: a reseller can't verify their HMAC key without calling the platform-admin endpoint. Acceptable — they shouldn't need to.

**Risks:**
- If `LURUS_WHITELABEL_MASTER_SECRET` leaks, every issued HMAC is compromised → every whitelabel installer ever signed is verifiable by attackers. Mitigation: secret rotation triggers reseller re-signing campaign. Document in runbook.
- A misconfigured deploy (env not set) silently disables the feature — endpoint 500s, but resellers might not notice until they try to issue a new installer. Mitigation: startup-time check (already implemented via fail-closed env check at handler entry).

## Action items

- [ ] Add `whitelabel-hmac` to `lurus.yaml` capabilities section as `tier: production`.
- [ ] Write a runbook: `doc/runbooks/whitelabel-hmac-rotation.md` covering secret rotation + reseller notification flow.
- [ ] On any new whitelabel reseller onboarding: document the slug → HMAC key fetch in the reseller-side install flow.
- [ ] If a second master secret per reseller becomes a real ask: file a follow-up ADR for "Per-Reseller Whitelabel HMAC Scheme".

## What this ADR does NOT do

- Does not change the derivation algorithm.
- Does not migrate to per-reseller master secrets.
- Does not auto-rotate the master secret.

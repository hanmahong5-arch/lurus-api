# ADR: Orphan-Feature Backfill #3 — Whitelabel HMAC Key Derivation

**Status**: Accepted (backfill — shipped, R6 STAGE wired) · **Date**: 2026-05-20
**Source commit**: `9c06cde1` (build: wire LURUS_WHITELABEL_MASTER_SECRET to r6-stage)
**Code**: `internal/adapter/handler/v2_admin_whitelabel.go` + `_test.go` (8 cases)

## Context

Resellers shipping Lurus-Switch as a whitelabel installer include a `whitelabel.json` (branding, default endpoints, tenant API key). Without integrity protection, anyone in the distribution chain can tamper (swap key, redirect endpoint). Shipped solution: each tenant has a deterministic HMAC key `sha256(LURUS_WHITELABEL_MASTER_SECRET + tenant_slug)`; reseller signs `whitelabel.json`, Switch verifies on install; tamper = mismatch = abort. The admin endpoint shipped without an ADR.

## Decision (retroactive)

**Document the scheme; register as a Platform Capability in `lurus.yaml`; gate behind RootJWT (already done).**

What it does:
- Endpoint `GET /api/v2/admin/whitelabel/hmac-key?tenant_slug=<slug>` (platform-admin only, `RootJWTAuth`) → `{"success":true,"data":{"hmac_key":"<64-hex>"}}`.
- Derivation `sha256(masterSecret + tenantSlug) → 64-char hex` — deterministic, no DB row, idempotent.
- **Fail-closed**: if `LURUS_WHITELABEL_MASTER_SECRET` unset/empty → 500 + clear error (no silent fallback). Tenant-existence check before derivation (no keys for typo'd slugs).

**Keep because**: production-deployed (R6 STAGE env wired `9c06cde1`; PROD uses it for Switch); ~50 LOC stateless (no DB/cron/cache); fills a real security gap; 8 tests (success/missing-slug/invalid-format/nonexistent-tenant/env-unset/determinism/isolation). **Don't expand**: single global master secret (per-reseller secrets not needed at one-test-slug scale); HMAC-SHA256 hex (JWT/JWS not needed).

**Risks**: master-secret leak compromises every issued HMAC → all signed installers verifiable by attackers (mitigation: K8s Secret restricted access + rotation runbook + reseller re-sign campaign). Misconfigured deploy (env unset) silently disables → 500 only noticed on next installer issue (mitigation: fail-closed env check at handler entry).

## Action items

- [ ] Add `whitelabel-hmac` to `lurus.yaml` capabilities as `tier: production`.
- [ ] Write `doc/runbooks/whitelabel-hmac-rotation.md` (secret rotation + reseller notification).
- [ ] Document slug→HMAC-key fetch in reseller install flow on each new onboarding.
- [ ] If per-reseller master secret becomes a real ask: file "Per-Reseller Whitelabel HMAC Scheme" follow-up ADR.

Does NOT change the derivation algorithm, migrate to per-reseller secrets, or auto-rotate the master secret.

# ADR: Layer-0 Bridge Endpoint & Tenant Resolution

**Date**: 2026-05-26
**Status**: Accepted
**Deciders**: Wave-UAT Sε squad

---

## Context

The v2 frontend console initialises by calling `GET /api/v2/{tenant_slug}/auth/session-info`
(ZitaBootstrap) to resolve the current user's session. The response, however, did not
include `tenant_slug` — the frontend silently fell back to the hard-coded string `"lurus"`.

Two consequences:
1. Tenants with a non-`lurus` slug received broken API calls (wrong URL prefix).
2. The Playwright e2e harness (Sγ) could not drive sessions from headless Chromium because it
   cannot execute the full Zitadel PKCE flow (requires interactive login or a service account
   with PKCE bypass).

---

## Decision

**Option B chosen** — implement both of the following:

1. **Modify ZitaBootstrap to return `tenant_slug`** — resolve it from `user.TenantId` via
   `repo.GetTenantByID`; fall back to `"default"` if the tenant is not found.

2. **Add `/api/v2/bridge/exchange` endpoint** — accepts `?token=<E2E_BRIDGE_TOKEN>&user_id=<id>`,
   mints a session for the given user, and returns a session cookie plus `{"tenant_slug": "..."}`.
   Route registration is **conditional on `E2E_BRIDGE_TOKEN` env var being set**; the route
   does not exist in production where the env var is absent.

Implementation reference: commit `806f65ac`.

---

## Alternatives Considered

| Option | Description | Decision |
|--------|-------------|----------|
| A | Add `?tenant_slug=` query param to all v2 URLs | Rejected — too many call sites; breaks existing bookmarks and client SDKs. |
| B (chosen) | Return `tenant_slug` from bootstrap + add bridge endpoint | Accepted — minimal blast radius, self-contained. |
| C | Full e2e through real Zitadel (service account + PKCE) | Rejected — brittle, slow CI (~30s/test), requires per-tenant Zitadel service account provisioning. |

---

## Consequences

**Positive**
- Frontend no longer hard-codes `"lurus"` as the tenant slug; correct for all tenants.
- Playwright e2e suite can drive real STAGE sessions with a single `POST /bridge/exchange`,
  enabling hermetic CI without a Zitadel service account.

**Negative / Mitigations**
- Production is one env-var misconfiguration away from exposing a session-minting endpoint.
  **Mitigation**: route registration is gated at the router level (not just the handler) —
  if `E2E_BRIDGE_TOKEN == ""`, the route is never registered, so the path returns 404
  unconditionally.
- Bridge token is a shared secret in CI env; rotate if compromised via
  `gh secret set E2E_BRIDGE_TOKEN` + pod restart.

---

## References

- Implementation commit: `806f65ac`
- Playwright e2e spec using bridge: `web/e2e/auth.spec.ts`
- STAGING_KUBECONFIG blocker: Wave-UAT sprint-status.yaml

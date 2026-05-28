# Security Finding — Cross-Tenant Isolation Bypass via Session Fallback

**Date**: 2026-05-28
**Severity**: High (cross-tenant data disclosure)
**Status**: FIXED + regression test (proven to catch the bug)
**Component**: `internal/adapter/middleware/zitadel_auth.go` → `handleSessionFallback`
**Found during**: H1 留尾 tenant-isolation audit (Horizon Plan)

---

## Summary

`ZitadelAuth`'s session-fallback path derived the request's tenant from the URL
`:tenant_slug` instead of the authenticated user's own record. A user with a valid
session cookie (no Bearer token) could assume **any** tenant's context — and read
that tenant's data — simply by changing the slug in the path.

## Vulnerability

When no `Authorization: Bearer` header is present, `ZitadelAuth` calls
`handleSessionFallback`, which authenticates the user from the session `id`
(trusted) but then set:

```go
tenantSlug := c.Param("tenant_slug")        // attacker-controlled
tenant, _ := repo.GetTenantBySlug(tenantSlug)
tenantID := tenant.Id                        // <-- tenant from the URL, NOT the user
tenantCtx := &TenantContext{TenantID: tenantID, UserID: user.Id, ...}
```

There was **no check** that `user` belongs to `tenantID`.

### Why the downstream guard did not save us

`GetCreditPoolForEndUser` (the one `:tenant_slug` route currently behind
`ZitadelAuth`) has an explicit guard:

```go
tenant, _ := repo.GetTenantBySlug(c.Param("tenant_slug"))
if tenant.Id != tenantCtx.TenantID { return 403 TENANT_MISMATCH }
```

This guard is correct **only when `tenantCtx.TenantID` comes from an independent
source** (the JWT org claim, in the Bearer path). In the session-fallback path
`tenantCtx.TenantID` was *also* derived from the slug, so the guard compared the
slug to itself and **always passed**.

## Reachability & impact

- **Trigger**: any session-authenticated user (member of some tenant) issues
  `GET /api/v2/<victim-slug>/credit-pool/me` with their session cookie and no Bearer.
- **Disclosed today**: the victim tenant's credit-pool projection
  (`current_balance`, `max_balance`, health) — financial data.
- **Latent blast radius**: every future `:tenant_slug` route placed behind
  `ZitadelAuth` (e.g. H1.1 SCIM admin, the H2 MCP gateway) would have inherited the
  same bypass. Fixing it at the auth layer protects all of them at once.

The Bearer-JWT path (`mapZitadelUserToLurus(claims)`) and the session `UserAuth`
path (`authHelper` → `GetUserCache(userId).TenantId`) were **not** affected — both
already scope tenant to the authenticated identity.

## Fix

Derive the tenant from the authenticated user's own record in both
session-fallback branches, mirroring `authHelper`:

```go
tenantID := user.TenantId
if tenantID == "" { tenantID = "default" }
```

The URL slug is no longer trusted for tenant resolution. The downstream
`TENANT_MISMATCH` guard now compares the slug against the user's real tenant and
correctly returns 403 on a cross-tenant attempt.

## Proof the regression test catches the bug (§4.1③ independence)

`TestZitadelAuth_SessionFallback_TenantFromUserNotSlug`
(`internal/adapter/middleware/zitadel_session_fallback_test.go`) sets up tenant A
(the user's own) + victim tenant B, establishes a session for the A-user, and hits
`/api/v2/<B-slug>/probe`. It asserts the resolved tenant is A, not B.

- Against the **fixed** code: PASS (both subtests).
- Against the **unpatched** code (temporarily reverted to verify): FAIL with
  `CROSS-TENANT LEAK: session user of tenant "integration-test-tenant" resolved to
  victim tenant "victim-tenant-id" via URL slug`.

So the test is not a tautology — it fails on the vulnerable code and passes on the
fix. Existing `ZitadelAuth*` tests remain green (no behavior regression).

## Audit scope (all tenant-context setters reviewed)

| Path | Tenant source | Verdict |
|---|---|---|
| `auth.go` `authHelper` (session UserAuth) | `GetUserCache(userId).TenantId` | safe (user's own) |
| `zitadel_auth.go` Bearer JWT path | `mapZitadelUserToLurus(claims)` | safe (verified JWT) |
| `zitadel_auth.go` session-fallback ×2 | was URL slug | **FIXED** → user's own |
| `playground.go` `PlaygroundFanOut` | token resolved by `userID`, slug unused for data | safe |
| `oauth.go` `ZitadelLoginRedirect` | pre-auth redirect, slug validated for format only | safe |
| `internal_api_ext.go` provisioning | caller-supplied `tenantId` on `/internal/*` (bearer key + scope) | out of scope — service-to-service trust boundary |

## Known defense-in-depth follow-ups (NOT fixed here — deliberately deferred)

The GORM tenant plugin (`repo/tenant_plugin.go`) is a *secondary* net; the primary
isolation is manual `WHERE tenant_id` in repos. Two latent weaknesses were noted but
**not** changed in this surgical fix, because they alter behavior on every DB op and
need cross-path validation (incl. system crons) that requires an integration
harness:

1. **Hard-coded `hasTenantIDColumn` allowlist** (`users/tokens/channels/topups/
   subscriptions/redemptions/passkeys/twofa`). Tenant-scoped tables with a
   `tenant_id` column but absent from this map (`tenant_credit_pools`,
   `tenant_configs`, `user_mappings`, `audit_events`) get **no** plugin auto-filter.
   No *active* leak found (those paths filter manually), but a future table added via
   `GetTenantDB` and forgotten here would leak silently. Recommend: derive from
   `Statement.Schema.FieldsByDBName["tenant_id"]`.
2. **Fail-open on empty tenant context** for query/update/delete (create already
   fail-closes). A context-less `UPDATE`/`DELETE` on an allowlisted table hits all
   tenants. Recommend: fail-closed for update/delete once system crons are confirmed
   to use `WithoutTenantIsolation`.

These are tracked for a dedicated, integration-tested PR — not bundled into this
auth-layer fix (keeps blast radius surgical).

---

_Authored 2026-05-28 during the Horizon Plan H1 留尾 tenant-isolation audit._

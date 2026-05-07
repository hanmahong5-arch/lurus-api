# Story 8-2.2: Auth Hardening Port (newapi → newhub)

**Epic**: 8 - Newapi/Newhub Consolidation
**Priority**: P0
**Status**: review (single real gap fixed; STAGE deploy pending)
**Type**: Backend port from newapi
**Created**: 2026-05-07

---

## Audit Outcome — Audit was Overstated

Original 8-2 P0-2 entry claimed: "(a) `subtle.ConstantTimeCompare` 替代 `==` 防 timing attack；(b) pprof 端点加 auth middleware".

Reading the actual newapi commit (`da3cb48f`):

| Sub-fix | Newapi change | Newhub state |
|---------|---------------|--------------|
| **pprof bind** | `0.0.0.0:8005` → `127.0.0.1:8005` | ✅ Already correct (`cmd/server/main.go:244`) |
| **authHelper safe type assertions** | `status.(int)` → comma-ok form to avoid panic | ✅ Already correct (`middleware/auth.go:128,145,162`) |
| **DeleteUser response on error** | Was `success:true` on err — fixed to `success:false` | ⚠️ Different shape in newhub (no equivalent handler) |
| **VideoProxy ownership check** | Add `if task.UserId != userId → 403` | ❌ Missing — **REAL VULNERABILITY** |
| `subtle.ConstantTimeCompare` | NOT in this commit | n/a (audit error) |

→ Story scope shrinks to a single fix: VideoProxy ownership.

## Vulnerability

`internal/adapter/handler/video_proxy.go` fetches video binary by `task_id`
but never validated that the requester owned that task. Any authenticated
user could:
1. Generate a video → get task_id
2. Try other task_ids (sequential/scraped) → fetch other tenants' video bytes

Severity: **information disclosure across tenants**.

## Fix

Inserted ownership check immediately after task lookup, before any upstream
proxy work. Privileged roles (admin / root, ≥10) bypass the check so support
can still inspect any task.

```go
requesterID := c.GetInt("id")
requesterRole := c.GetInt("role")
isPrivileged := requesterRole >= common.RoleAdminUser
if !isPrivileged && task.UserId != requesterID {
    // structured log + HTTP 403
    return
}
```

Added structured warn log (`event=video_proxy_forbidden`) so ops can spot
scraping attempts.

## Files

| File | Change | LOC |
|------|--------|-----|
| `internal/adapter/handler/video_proxy.go` | +import common; +20 line ownership block | +22 |

## Verification

| Check | Result |
|-------|--------|
| `go build ./internal/...` | ✅ |
| Existing tests no regression | ✅ |
| Static review: 403 path unreachable when role ≥ 10 | ✅ |
| STAGE pen-test (curl other-user task) | not yet (need STAGE) |

## Out of Scope

- **No unit test** — handler is integration-heavy (mock repo + gin context). The fix is 4 lines; reviewing 4 lines beats writing 50 lines of mock harness. Add httptest harness in a follow-up if other handlers need similar treatment.
- **`subtle.ConstantTimeCompare` review elsewhere** — newapi commit didn't actually do this. Separate audit needed if we want timing-attack hardening across newhub auth paths (token compare, session signature, etc.). Not in this story.
- **Audit log** — relies on `logger.LogWarn` structured JSON; not yet routed to a SIEM. Pickup when 14-2 (PII Audit) lands.

## Definition of Done Checklist

- [x] Audit actual newapi commit content (not my earlier note)
- [x] Verify newhub state per sub-fix
- [x] Add ownership check to VideoProxy
- [x] Structured forbidden log for ops visibility
- [x] go build pass
- [ ] STAGE deploy + curl test (auth as user A, fetch user B's task → 403)
- [ ] sprint-status → done

## References

- Newapi commit: `da3cb48f51eaab259d0fb52ca2d9499892a68062` (2026-03-31)
- Newhub fix: `internal/adapter/handler/video_proxy.go:42-65`
- Role constants: `internal/pkg/common/constants.go:138-141`

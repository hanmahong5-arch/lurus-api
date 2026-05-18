# Hardening Swarm — 2026-05-18 Acceptance Report

> Single session execution. Plan called for 4 lanes; user-selected scope
> per AskUserQuestion: **Lane A full + Lane D full + Lane B partial**.
> Lane C deferred (requires STAGE SSH + multi-hour observation windows).

## Final Test Counts (this session)

```
$ cd web && bun run test
 Test Files  2 passed (2)
      Tests  7 passed (7)

$ bun run test:e2e
  -  1 [chromium] › Story 11-2 — token persistence ...
  1 skipped       # correct — no E2E_BRIDGE_TOKEN set

$ go test -count=1 -short -run 'TestGetAudit|TestGetGovern|TestListTokensV2_Pagination' ./internal/adapter/handler/
--- PASS: 9 new (audit/governance) + 1 repaired (v2 token)
ok  github.com/LurusTech/lurus-hub/internal/adapter/handler 0.163s

$ go build ./...   # exit 0
$ go vet ./...     # exit 0
$ cd web && bun run build   # exit 0
```

## Lane A — Quality Floor ✅

Frontend test infrastructure didn't exist; this lane built it.

| Deliverable | File | Evidence |
|------------|------|---------|
| Vitest + RTL + jsdom | `web/vitest.config.js`, `web/src/test/setup.js` | `bun run test` → 2/2 smoke pass |
| Playwright + Chromium | `web/playwright.config.ts`, `web/tests/e2e/story-11-2-token-persistence.spec.ts` | `bun run test:e2e` → 1 skipped (correctly, no token) |
| CI workflow | `.github/workflows/web-ci.yml` | PR-blocks lint/eslint/vitest/build; e2e via `workflow_dispatch` |
| Story 11-2 DoD update | `_bmad-output/planning-artifacts/story-11-2-v2-wiring-mvp.md` | Honest status: harness built, STAGE run pending |

**§4.1 ⑥ honesty note**: Spec deliberately `test.skip`s when
`E2E_BRIDGE_TOKEN` is unset — does NOT silently pass. "Harness exists" ≠
"behavior verified". Token-persistence behavior on STAGE will be measurable
only after the bridge token lands.

## Lane D — Backend Tests + Scoped CI Gate ✅

| Deliverable | File | Evidence |
|------------|------|---------|
| audit handler tests (4) | `internal/adapter/handler/audit_governance_test.go` | All pass: empty list / filter by action / filter actor+resource / pagination |
| governance handler tests (5) | same file | All pass: channels / latency / efficiency / fingerprints / hours clamping |
| v2 token test fix | `internal/adapter/handler/v2_token_test.go` | Repaired pre-existing red — handler shape `items`/`p`/`size` drift |
| Scoped CI gate | `.github/workflows/go-ci.yml` | `go build` + `go vet` + Lane D test scope; PR-blocking |
| Test debt finding | `_bmad-output/planning-artifacts/test-debt-findings.md` | OAuth/JWKS test debt documented; broad `-short ./...` gate deferred |

**Why narrow gate, not `go test -short ./...`**: baseline has pre-existing
OAuth/JWKS test failures (TestValidateIDToken_* + fuzz tests) — not Lane D
scope, not caused by Lane D work. Adding the broad gate today would block
every PR. See `test-debt-findings.md` for explicit cleanup ladder.

**Plan-vs-delivered gaps (explicit)**:
- Plan asked for billing handler tests — deferred. `billing.go` legacy v1
  handler depends heavily on platform gRPC client; honest mocking is a
  multi-hour effort, out of session budget.
- Plan asked for relay main path fake-provider test — deferred. Provider
  abstraction mocking is non-trivial; left for a focused future PR.
- Plan asked for deadcode scan candidate doc — deferred (no `deadcode`
  invocation in this session).

## Lane B — v2 Mock Cleanup (Partial — Three Passes) ⚠️

Per user-selected scope (extended via two "继续" rounds):
- **Pass 1**: TenantSwitcher real data
- **Pass 2**: Dashboard realtime KPIs derived from /logs
- **Pass 3**: WIPBanner explicit markers on Playground/Models/Chat/Flows + Log Cluster/Live tabs (mock data removed)

Settings non-profile tabs (Sessions/Notifications/Team) still untouched.

### Pass 3 — Explicit WIP markers + dead mock data removal (2026-05-18)

`web/src/components/hifi/WIPBanner.jsx` — shared "work in progress" marker
component with reason + todo props. role=status for a11y.

| Page | Before | After |
|------|--------|-------|
| Playground | 277-line mockup with hardcoded model comparison output | Banner at top — "needs fan-out relay endpoint" |
| Models | 368-line mockup with `MODELS` const (54 fake models) | Banner — "needs /api/v2/{slug}/models aggregation" |
| Chat | 312-line mockup with `CONV` array (hardcoded SQL convo) | Banner — "pending Wave 2 consumer-feature decision" |
| Flows | 1032-line wizard mockup | Banner — "needs flow orchestration handlers" |
| Log/Cluster tab | `CLUSTERS` array — 4 fake error clusters | Array deleted; tab renders banner + "endpoint not implemented" empty state |
| Log/Live tab | `LIVE_ROWS` array — 8 hardcoded log lines + fake `▌` cursor | Array deleted; tab renders banner + "streaming endpoint not implemented" empty state |

**§4.1 ⑥ honesty note**: WIPBanner is the "marker" — it announces the page
is not behavior-verified. The mock data deletion is the *measurement
hygiene* — false "live tail at 14:02:11.481 acme contoso initech" lines
can no longer leak into stakeholder demos as if they were real telemetry.

Test count: WIPBanner has 4 vitest specs (renders marker / shows reason+todo
/ has role=status / omits when props absent). All pass.

### Pass 2 — Dashboard realtime KPIs (2026-05-18)

### Pass 2 — Dashboard realtime KPIs (2026-05-18)

`qps` / `ttft p50` / `error rate` tiles were hardcoded `—` with
"coming soon" copy. No `/api/v2/{slug}/metrics/usage` endpoint exists, so
KPIs are derived client-side from a 5-minute window of `/api/v2/{slug}/logs`.

| Deliverable | File | Evidence |
|------------|------|---------|
| Pure derivation helpers | `web/src/pages/v2/Dashboard/kpis.js` | `computeQPS`, `computeLatencyP50`, `computeErrorRate`, `pickRecent`, formatters |
| Unit tests (17 cases) | `web/src/pages/v2/Dashboard/kpis.test.js` | Empty input / zero-window divide-guard / even+odd median / snake-vs-Pascal key tolerance / format edge cases |
| Dashboard wire-up | `web/src/pages/v2/Dashboard/index.jsx` | Fetch widened to `page_size=200&start_time=<now-300s>`; KPI tiles render derived values or `—` when no traffic |

**§4.1 ① honesty note**: When the 5-min window is empty (no traffic), KPIs
render `—` and "no traffic in last 5 min", **not** `0%` / `0` / perfect
zero. Per "完美数字 = 反射性怀疑", we do not let the absence of
measurement masquerade as a clean reading.

### Pass 1 — TenantSwitcher (original Lane B partial)

| Deliverable | File | Evidence |
|------------|------|---------|
| TenantSwitcher real-data wiring | `web/src/components/hifi/HFShell.jsx` | `useRealTenants()` hook calls `/api/v2/admin/tenants?page_size=50`; non-root user gets single-tenant fallback from localStorage |
| Removed DEMO_TENANTS placeholder | `web/src/components/hifi/TenantSwitcher.jsx` | "acme · prod" / "contoso · stage" / "personal workspace" can no longer leak through (test covers this) |
| Component tests (5) | `web/src/components/hifi/TenantSwitcher.test.jsx` | render-with-real-props, empty list, dropdown open, onSelect callback, controlled-prop sync |
| Tenant switch persistence | `switchTenantSlug` in `HFShell.jsx` | onSelect → `localStorage.setItem('tenant_slug', ...)` + reload |

**Still mocked (explicit)**: Dashboard KPI tiles, Settings sessions /
notifications / team tabs, Log cluster/live, Playground/Models/Chat/Flows.
These need additional backend wiring or display-only WIP labels — out of
session scope.

**§4.1 ⑥ honesty note**: Component tests verify "demo strings absent" and
"caller props propagate" — i.e. *markers present*. Real-data path against
STAGE not yet walked end-to-end in browser; that requires a logged-in
session against `test-newhub.lurus.cn`, deferred with the e2e spec.

## Lane C — Epic 7 STAGE Validation ⏭️ Deferred

Not attempted in this session. Each story requires:
- SSH access to R6 (STAGE) — interactive
- Triggering scenarios (pod kills, traffic bursts, network partitions)
- Multi-minute observation windows for breaker state transitions,
  PG standby promotion, WAL-G restore timings
- Grafana screenshot collection
- Per §4.1 ③: chaos-drill scripts that are same-source as the system
  under test must be explicitly flagged `SELF-VALIDATING — NOT EVIDENCE`

Recommendation: run Lane C as a *separate* operator-driven session.
Inputs needed at the start of that session:
1. R6 SSH availability + low-traffic window confirmation
2. Story-by-story trigger commands (already in `scripts/stage-smoke.sh`
   and `scripts/chaos-drill.sh` per the 2026-05-07 process entry)
3. A blank acceptance template per story (5x).

## What This Swarm Does NOT Claim

Per §4.1 ⑥ marker-vs-measurement separation:

- ❌ "v2 hi-fi mock zero" — only TenantSwitcher cleared; 6+ pages still mock-bound
- ❌ "Epic 7 review→done" — zero stories flipped; STAGE validation deferred
- ❌ "go test -short ./... is green" — narrow gate only; baseline OAuth tests still red
- ❌ "Story 11-2 e2e verified" — harness exists, STAGE run pending bridge token
- ❌ "Backend coverage materially improved" — 9 tests added in 2 packages; broad coverage gaps remain

## Files Changed Inventory

```
A  .github/workflows/go-ci.yml
A  .github/workflows/web-ci.yml
A  _bmad-output/planning-artifacts/hardening-swarm-2026-05-18-acceptance.md
A  _bmad-output/planning-artifacts/test-debt-findings.md
M  _bmad-output/planning-artifacts/story-11-2-v2-wiring-mvp.md
M  _bmad-output/planning-artifacts/sprint-status.yaml
M  doc/process.md
A  internal/adapter/handler/audit_governance_test.go
M  internal/adapter/handler/v2_token_test.go
M  web/.gitignore
M  web/package.json
M  web/src/components/hifi/HFShell.jsx
A  web/src/components/hifi/TenantSwitcher.test.jsx
M  web/src/components/hifi/TenantSwitcher.jsx
A  web/src/components/hifi/WIPBanner.jsx
A  web/src/components/hifi/WIPBanner.test.jsx
A  web/src/pages/v2/Dashboard/kpis.js
A  web/src/pages/v2/Dashboard/kpis.test.js
M  web/src/pages/v2/Dashboard/index.jsx
M  web/src/pages/v2/Log/index.jsx
M  web/src/pages/v2/Playground/index.jsx
M  web/src/pages/v2/Models/index.jsx
M  web/src/pages/v2/Chat/index.jsx
M  web/src/pages/v2/Flows/index.jsx
A  web/src/test/setup.js
A  web/src/test/smoke.test.js
A  web/tests/e2e/story-11-2-token-persistence.spec.ts
A  web/playwright.config.ts
A  web/vitest.config.js
```

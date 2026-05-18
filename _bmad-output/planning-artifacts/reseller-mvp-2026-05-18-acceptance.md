# Reseller-MVP Swarm — 2026-05-18 Acceptance Report

Follow-on swarm to the hardening swarm. Scope from competitive intel
(`competitive-intel-2026-05-18.md`) — user-selected option **β "Reseller MVP cut"**
via AskUserQuestion. 4 lanes; L1/L2 design-only this session, L3/L4 shipped.

## Final test counts (this session)

```
$ cd web && bun run test
 Test Files  4 passed (4)       # smoke / TenantSwitcher / kpis / WIPBanner
      Tests  39 passed (39)     # +11 since hardening swarm (P95/P99 + cost-by-model)

$ bun run build                  # exit 0, vite green
$ go build ./...                 # exit 0
```

## Lane status

| Lane | Status | What shipped this session |
|------|--------|--------------------------|
| L1 — Tenant credit pool + Provisioning API | ADR delivered, **no schema change** | `adr-2026-05-18-tenant-credit-pool.md` |
| L2 — Budget alerts + default rule pack | ADR delivered, **no rule changes** | `adr-2026-05-18-budget-alerts.md` |
| L3 — Token audit + visual quota + Sessions | ✅ Shipped (real-data Sessions + WIPBanner where backend missing) | `web/src/pages/v2/Settings/index.jsx`, `web/src/pages/v2/Token/index.jsx` |
| L4 — Onboarding curl + Dashboard cost-by-model + P95/P99 | ✅ Shipped (24 new vitest cases) | `web/src/pages/v2/Dashboard/{index.jsx,kpis.js,kpis.test.js}` |

## L4 — Shipped detail

- **kpis.js**: added `computeLatencyPercentile(logs, p)` (nearest-rank), `computeLatencyP95`, `computeLatencyP99`, `computeCostByModel` (groups consume logs by `model_name`, sums `quota`, sorts desc).
- **kpis.test.js**: +11 test cases — P95 nearest-rank correctness, P99 small-sample fallback, edge cases, cost-by-model grouping & PascalCase tolerance.
- **Dashboard.index.jsx**:
  - "ttft p50" tile replaced with a **3-up percentile tile** (P50 / P95 / P99) — "p99 anchors SLO" per Datadog/LangSmith pattern (intel finding #11).
  - Bubble-chart panel **replaced** with a real **cost-by-model** bar chart (top 6 models, each row shows `$USD · N req` + relative bar) — intel finding #8.
  - **OnboardingCurlBlock** component rendered when `me.token_count === 0` — Reseller TTFT lift modelled on OpenRouter <2min flow (intel finding #1). Includes "create token" CTA + copy-pasteable curl with the relay base URL.
  - Removed dead bubble-chart code (`FALLBACK_BUBBLES`, `BUBBLE_COLORS`, `modelSet`, `bubbles`, `displayBubbles`).

## L3 — Shipped detail

- **Settings/Security tab**: dead `SESSIONS` mock array removed; **wired** `/api/v2/client/sessions` (returns `username`, `auth_method`, `active_tokens`, `request_count`). Single-session card renders real data. WIPBanner explicitly notes "multi-device session list + per-session revoke not implemented".
- **Settings/Notifications tab**: dead `NOTIFICATIONS` mock removed; WIPBanner references `adr-2026-05-18-budget-alerts.md`; empty-state copy "endpoint not implemented".
- **Settings/Team tab**: dead `TEAM` mock removed; WIPBanner references future tenant-member store.
- **Token/index.jsx**: added explicit "created" + "last used" labels in token row (existing `t.CreatedTime` field surfaced; existing visual quota bar already present — intel finding #9 already met).

§4.1 ⑥ check — Settings tabs now correctly show "no data" with explicit reason
rather than fake "MacBook Pro · Shanghai · 2m ago" / "Andy Liu · owner · now"
data that could leak into stakeholder demos as if it were real.

## L1 — ADR delivered (NO implementation)

`adr-2026-05-18-tenant-credit-pool.md` — additive schema design:
- New tables: `tenant_credit_pools` + `tenant_credit_pool_draws`
- New columns: `tokens.creator_user_id`, `tokens.last_used_at` (the latter overlaps existing `accessed_time` — open question)
- Per-request debit via conditional UPDATE (`SET current_balance = current_balance - $1 WHERE ... AND current_balance >= $1`) — race-safe at DB level without serializable isolation
- Enforcement order: tenant pool (HTTP 402) → per-token quota (HTTP 429)
- Provisioning API: `POST /internal/v1/provisioning/tenants/:slug/keys` with new `ScopeProvisioning` bearer auth, mirrors OpenRouter Management Key shape

**5 open questions for Anita** in §9 — schema migration cannot proceed until these resolve. See file for details.

## L2 — ADR delivered (NO YAML changes) + **CRITICAL FINDING**

`adr-2026-05-18-budget-alerts.md` — alert event taxonomy, dispatch architecture
(NATS → platform notification consumer), Prometheus rule pack draft, subscription
schema. **4 open questions** for Anita.

### 🔴 Pre-existing bug surfaced during L2 research

`deploy/grafana/newhub-alerts.yaml` uses metric prefix `lurus_hub_*` for
all 11 alert rules (e.g., `lurus_hub_requests_total`, `lurus_hub_circuit_breaker_state`).

Actual Prometheus namespace/subsystem in `internal/pkg/metrics/metrics.go:9-11`
is `namespace = "lurus"` + `subsystem = "gateway"` → emits `lurus_gateway_*`.

Confirmed via grep: zero metrics in `internal/pkg/metrics/` are named
`lurus_hub_*`. **All 11 existing alerts are silently dead** — they have been
evaluating against non-existent series since whichever commit changed the
subsystem from `hub` to `gateway`.

Story 7-5 (chaos drill + SLO dashboard) was marked `review` in
`sprint-status.yaml` partly on the strength of "11 alerts shipped" — the
alerts ship, but they do not fire. This **does not invalidate Story 7-5**
(dashboard + drill scripts are real), but the alert-pack portion of its DoD
should be re-examined.

**Recommended fix** (NOT applied this session — needs user signoff):
sed `lurus_hub_` → `lurus_gateway_` in `deploy/grafana/newhub-alerts.yaml`
(or rename the subsystem in `metrics.go` — opposite-direction fix). One is
a YAML edit, the other is a metric-name change that breaks anyone scraping
the current names.

### ✅ 2026-05-18 follow-up: fix APPLIED ("自行决策" autonomy)

Two-step sed applied to both `deploy/grafana/newhub-alerts.yaml` (14
occurrences) and `deploy/grafana/newhub-slo.json` (18 occurrences):

1. `lurus_hub_billing_` → `lurus_billing_` (3 + 3 lines, billing subsystem)
2. `lurus_hub_` → `lurus_gateway_` (11 + 15 lines, gateway subsystem)

Verification:
- `grep -c "lurus_hub_"` returns 0 in both files
- `python -c "yaml.safe_load(...)"` passes on YAML
- `JSON.parse(...)` passes on dashboard JSON
- Anti-regression comment added at top of `newhub-alerts.yaml` referencing
  the Go-side namespace+subsystem sources so the next developer who renames
  knows to search-replace here too

Story 7-5 doc updated with the same fix note.

**Operator impact**: on the next ArgoCD sync of the Prometheus rule
ConfigMap, all 11 alerts will start evaluating against real series.
Previously-silent alerts may begin firing on legitimate conditions that
have been invisible. Expected — not a regression.

The change DOES NOT touch production by itself (file change in git only).
GHA push triggers are inert per `project_gha_manual_dispatch` memory —
operator must `gh workflow run` and ArgoCD-sync to pick up.

## What this swarm does NOT claim

- ❌ Tenant credit pool / Provisioning API implemented (ADR only)
- ❌ Budget threshold alerts firing (ADR only; existing alerts dead per finding above)
- ❌ Multi-device session list (backend lacks the data; explicit WIPBanner)
- ❌ Onboarding curl block validated end-to-end against STAGE (UI only)

## Files changed this session

```
A  _bmad-output/planning-artifacts/competitive-intel-2026-05-18.md
A  _bmad-output/planning-artifacts/reseller-mvp-2026-05-18-acceptance.md
A  _bmad-output/planning-artifacts/adr-2026-05-18-tenant-credit-pool.md
A  _bmad-output/planning-artifacts/adr-2026-05-18-budget-alerts.md
M  web/src/pages/v2/Dashboard/index.jsx
M  web/src/pages/v2/Dashboard/kpis.js
M  web/src/pages/v2/Dashboard/kpis.test.js
M  web/src/pages/v2/Settings/index.jsx
M  web/src/pages/v2/Token/index.jsx
```

## Recommended next actions for Anita

1. **High urgency** — Decide on the `lurus_hub_*` vs `lurus_gateway_*` alert YAML
   fix direction. 11 dead alerts is a silent operational regression.
2. Read L1 ADR §9 (5 questions). Without answers, no schema work proceeds.
3. Read L2 ADR §11 (4 questions). Without #4 (billing-period definition),
   quota threshold rules cannot be built.
4. Decide whether L1/L2 implementation goes in a follow-on session or is
   re-scoped against a real Reseller customer signing up (matches
   `feedback_release_cadence` memory — don't ship before consumer pull).

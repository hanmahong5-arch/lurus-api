# Story Q3-Phase2: Tenant Credit Pool + Provisioning API — End-to-End

**Epic**: Q3 Reseller Cash Path (cross-cuts E10 Reseller MVP + E7 Reliability)
**Priority**: P0
**Status**: in-progress (Phase 1 schema MERGED; Phase 2 swarm in flight)
**Type**: Multi-lane cross-stack feature (schema + handler + UI + observability + docs)
**Created**: 2026-05-18
**Source ADR**: `adr-2026-05-18-tenant-credit-pool.md` (Accepted, Anita signoff 2026-05-18)

---

## Problem Statement

Newhub today enforces spend at the token level only. Resellers cannot:
- Cap a sub-tenant at "500 CNY / month" with a hard pre-debit gate.
- Top up a sub-tenant pool mid-cycle without touching their platform wallet credentials directly.
- Programmatically provision a named API key for an EndUser (no automation surface).
- Receive 80% / 100% threshold notifications to top up before the hard wall.

The Q3 cash-path requires all four. Phase 1 (commit `f6a492ad`) shipped
the additive schema (`migration 012` + entity + repo stub). Phase 2 wires
handlers, middleware gates, Reseller UI, observability, and docs.

User stories the feature unlocks (verbatim from ADR §1):

- "As a Reseller, I want to cap tenant acme-corp at 500 CNY/month so that their
  runaway usage cannot drain my platform wallet beyond my contracted ceiling."
- "As a Reseller, I want to top up a tenant's pool mid-month without touching
  my platform wallet credentials, so that I can self-serve customer upgrades."
- "As a Reseller, I want to programmatically provision a named API key for a
  customer with a per-key USD ceiling, so I can automate onboarding without
  a browser UI." (mirrors OpenRouter Management Key)
- "As an EndUser, I want to receive a 402 with a clear message when my
  tenant's monthly budget is exhausted, so I know to contact my Reseller."

---

## Wave 1 Lane Split

Lane assignments inherit Wave 1 Phase 2 swarm conventions. Each lane has
its own worktree branch; this story is the cross-lane DoD anchor.

| Lane | Owner scope | Branch suffix |
|------|-------------|---------------|
| α | `internal/adapter/handler/v2_credit_pool.go`, router wiring (api-v2 + internal), pool middleware gate on relay | handler |
| β | unit + integration tests (5 §8 unit + 3 §8 integration + 1 chaos) | tests |
| γ | `web/src/pages/v2/Tenants/CreditPoolDrawer.jsx` + vitest cases | ui |
| δ | Prometheus metrics (`lurus_gateway_credit_pool_*`) + NATS `llm.pool.threshold` publisher + alert rule + dashboard panel | observability |
| ε | this story doc + `doc/contracts/switch-provisioning-api.md` + `doc/runbook/pool-threshold-alert.md` + retro reports + ADR §4.2 header fix | docs |

Phase 1 schema is foundation for all lanes — already merged to `main`
(commit `f6a492ad`).

---

## Acceptance Criteria (DoD)

### Backend implementation (Lanes α + β + δ)

- [ ] All 5 ADR §8 unit tests pass (handler + repo + middleware):
  - [ ] Pool decrement happy path
  - [ ] Atomic debit race (20 goroutines, 0 negative balance)
  - [ ] Exhausted pool rejection (no draw row written on reject)
  - [ ] Unlimited pool bypass (`max_balance = -1` or no row → skip gate)
  - [ ] Reset period boundary (monthly cron resets balance, writes `reset` draw)
- [ ] All 3 ADR §8 integration tests pass:
  - [ ] E2E provisioning → relay → pool debit (1 request, draw + log linkage)
  - [ ] Pool exhaustion blocks relay (HTTP 402, no draw, token unchanged)
  - [ ] Topup via admin API → wallet-debit-first → pool credited (Q4 wallet-debit-only rule)
- [ ] 1 chaos test passes:
  - [ ] Burst load against thin pool (200 concurrent × 5 units, pool = 100, balance never negative, no deadlock)
- [ ] Provisioning API endpoints registered on internal-api-router (Lane α scope):
  - [ ] `POST /internal/v1/provisioning/tenants/:slug/keys` (X-API-Key auth, `provisioning:write` scope)
  - [ ] `DELETE /internal/v1/provisioning/tenants/:slug/keys/:key_id` (same auth)
- [ ] Admin endpoints registered on api-v2-router (Lane α scope):
  - [ ] `POST /api/v2/admin/tenants/:id/credit-pool` (Zitadel JWT + RootAuth)
  - [ ] `POST /api/v2/admin/tenants/:id/credit-pool/topup` (same)
  - [ ] `GET /api/v2/admin/tenants/:id/credit-pool/usage` (same)
- [ ] Pool gate active on 5 relay groups (Lane α scope):
  - [ ] OpenAI chat completions + completions
  - [ ] Claude messages
  - [ ] Gemini relay
  - [ ] Images / audio / embeddings / rerank (grouped — one middleware on relay)
  - [ ] Midjourney + Suno (separate handler groups)
- [ ] NATS `llm.pool.threshold` event published on threshold-cross (Lane δ scope)
  - [ ] Dedup key in Redis (`pool_threshold:{pool_id}:{billing_period_epoch}`)
  - [ ] Event payload matches ADR §9 Q5 schema
- [ ] Metrics emit (Lane α call-sites, Lane δ declarations):
  - [ ] `lurus_gateway_credit_pool_balance{tenant_id, pool_id}` gauge
  - [ ] `lurus_gateway_credit_pool_max_balance{tenant_id, pool_id}` gauge
  - [ ] `lurus_gateway_credit_pool_debits_total{tenant_id, reason}` counter
  - [ ] `lurus_gateway_credit_pool_rejections_total{tenant_id}` counter (HTTP 402)
- [ ] Prometheus alert wired (Lane δ scope):
  - [ ] `CreditPoolBalanceLow` rule fires when `balance / max_balance < 0.2 for 10m`
  - [ ] Runbook link in annotation: `doc/runbook/pool-threshold-alert.md`

### Frontend implementation (Lane γ)

- [ ] UI drawer renders (`web/src/pages/v2/Tenants/CreditPoolDrawer.jsx`)
  - [ ] Reseller sees: current_balance, max_balance, reset_period, next_reset_at, last 10 draws
  - [ ] Topup button → modal with amount + note fields
  - [ ] Wallet-debit confirmation copy (per Q4: "this will debit your platform wallet")
- [ ] 5 vitest cases pass:
  - [ ] Drawer renders empty-state when tenant has no pool
  - [ ] Drawer renders balance + ceiling when pool exists
  - [ ] Topup button disabled when input invalid (amount ≤ 0)
  - [ ] Topup success refreshes balance display
  - [ ] Draws table paginates by 10

### Documentation (Lane ε — THIS LANE)

- [x] `doc/contracts/switch-provisioning-api.md` — Switch-facing contract spec (v0.1.0)
- [x] `doc/runbook/pool-threshold-alert.md` — operator runbook
- [x] `_bmad-output/planning-artifacts/adr-2026-05-18-tenant-credit-pool.md` §4.2 header fix (X-API-Key, not Bearer)
- [x] This story doc (DoD anchor for cross-lane coordination)
- [x] 5 retro acceptance reports for Story 7-{1,2,2.1,4,5} (review → done gate)
- [x] `epic-7-acceptance-report.md` master roll-up

### Out of scope (deferred)

Per ADR §10 + §6 Phase 3:
- Workspace-level policy inheritance (LiteLLM Org→Team model) — separate ADR.
- Semantic cache + cost deduplication — perf optimization, not billing primitive.
- Cross-tenant pool sharing — schema-ready via `parent_tenant_id` but not wired.
- Real-time WebSocket push for balance updates — frontend feature, separate story.
- Budget threshold notification (`balance.low` per L2 ADR) — depends on this story's
  schema landing first; wiring follows in a subsequent sprint.
- `Idempotency-Key` header support on provisioning API — Q4 hardening
  (see `doc/contracts/switch-provisioning-api.md` §4 known gap).

---

## Verification Evidence

> This section is populated when Phase 2 swarm closes + Phase 3 STAGE
> drill is executed. As of 2026-05-18 it is **deliberately blank** —
> Lane ε ships only the doc skeleton; lanes α / β / γ / δ commits fill
> in test counts, route registrations, and metric scrape evidence; the
> Phase 3 STAGE session fills in the chaos drill measurement.

```text
[ pending Phase 2 close ]
- Unit test count:        ...
- Integration test count: ...
- Chaos test result:      ...
- Route registrations:    ... (grep evidence)
- Metric scrape:          ... (curl /metrics output)
- NATS event capture:     ... (nats sub LLM_EVENTS proof)
- Frontend vitest count:  ...
- Frontend build:         ...
```

```text
[ pending Phase 3 STAGE drill, 2026-06-16+ ]
- chaos-drill.sh scenario D (pool exhaustion under burst): PASS/FAIL
- Measured: final current_balance >= 0, draws summed <= max_balance
- Measured: 402 rate during exhaustion (no false negatives)
- Operator signoff:        ...
```

---

## §4.1 ⑥ Honesty Note (marker-vs-measurement)

This story has many checkboxes. **Marker presence ≠ measurement validity.**

- "Routes registered" is a marker — grep evidence confirms text exists.
- "Pool gate active on 5 relay groups" is a marker until a chaos drill
  with a thin pool + burst load demonstrates the gate actually fires.
- "Atomic debit race test passes" is a marker until run under
  `go test -race` on a multi-core box with enough goroutines to exercise
  the conditional UPDATE path repeatedly.

Per global CLAUDE.md §4.1 ⑥ and §4.1 ①: completion of this story
requires both markers **and** an independent measurement that the gate
behaves correctly under load. Until the STAGE drill is run, mark this
story `review`, not `done`.

---

## References

- ADR: `_bmad-output/planning-artifacts/adr-2026-05-18-tenant-credit-pool.md`
- Companion ADR (alerts): `_bmad-output/planning-artifacts/adr-2026-05-18-budget-alerts.md`
- Contract spec (Switch-facing): `doc/contracts/switch-provisioning-api.md`
- Runbook (operator): `doc/runbook/pool-threshold-alert.md`
- Phase 1 schema commit: `f6a492ad` (migration 012 + entity + repo stub)

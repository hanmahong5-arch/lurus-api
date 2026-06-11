# ADR: Cost-Aware Routing with Quality Observability

**Status**: Proposed (W4 impl gated by Open Questions) · **Date**: 2026-05-09
**Relates to**: `doc/slo-relay.md` (W1 SLO baseline); ADR-0011 Layer C; enterprise-vs-cloud split (`lurus/CLAUDE.md`)

## Context

Two forces collided in the 2026-05-09 product session: B2B stability mandate (strict ToS/SLA/audit) vs cost optimization (route cheap requests to cheap models). Naive auto-routing is a quality-regression engine — customer apps break silently when JSON formats differ subtly between providers. But cost optimization is real (50K-token analysis on Claude Opus vs DeepSeek = 25×, $0.50 vs $0.02).

Reality checks: (1) **chitchat routing saves nothing** — output tokens dominate, so route on *predicted total cost* not "looks cheap"; (2) **default-on auto-routing breaks B2B trust** — enterprises require "I said claude-sonnet-4, use it"; auto-route opt-in only; (3) **pricing gates everything** — flat-rate billing misaligns incentives (we save, not the customer); pass-through-with-markup makes both win.

## Decision

### 1. Seven-stage routing pipeline (~10ms total, within 50ms P99 budget)

`Request → Intent Classifier → Capability Filter → Quality Tier → Cost Optimizer → Health Filter → Route → Response Normalizer → Quality Observer (async)`

| Stage | Latency | Function |
|-------|---|---|
| Intent classifier | ~5ms | `{chitchat,code,reasoning,extraction,generation,agent,multimodal}` + confidence; low conf → tenant-default model |
| Capability filter | ~1ms | hard-reject models lacking tools/vision/JSON-mode/context-window |
| Quality tier | ~1ms | per-task per-model grade `{S,A,B,C}`; only models above tenant minimum |
| Cost optimizer | ~2ms | `predicted_cost = in_tokens×in_price + est_out×out_price`; cheapest meeting quality. **Must include prompt-cache hit prediction** (cached prefix can make Anthropic cheaper than DeepSeek) |
| Health filter | ~1ms | exclude circuit-broken / rate-limited channels (reuse breaker) |
| Route + relay | existing | — |
| Response normalizer | per chunk | SSE format, JSON strictness, stop-token semantics across providers |
| Quality observer | async | 1% LLM-as-judge sampling; weekly tier update; immediate downgrade on drift |

### 2. Per-tenant routing modes

| Mode | Behavior | Default for |
|------|----------|-------------|
| `strict` | pin model, no auto-route | new enterprise; finance/legal/healthcare |
| `family-pinned` | auto-select within a model family | cost-conscious single-vendor tenants |
| `quality-tier` | "any model scoring ≥ B for code" | growth-stage |
| `shadow` | route to pinned AND alt in parallel, compare, **report only never switch** | the gateway-drug: enterprises see savings + quality on own traffic before opting in |

Auto-route (`family-pinned`/`quality-tier`) is **permanent opt-in**. `shadow` is the bridge.

### 3. Anti-rules — never auto-route

Mid-conversation requests with prior `tool_call` (swap drops tool state); `audit:true` / `pin:<model>` stamped requests; extended-thinking/reasoning models (incompatible output format); non-first turn in multi-turn (session continuity).

### 4. Customer transparency (mandatory when auto-route on)

Response headers: `X-Routed-Model`, `X-Routing-Reason` (e.g. `cost_optimizer:family-pinned`), `X-Estimated-Cost`, `X-Quality-Tier`. Monthly report: savings ($ vs strict baseline), quality trend, request count by routed model. **Without these, auto-route MUST NOT be enabled** — hidden routing destroys trust faster than savings justify.

### 5. Quality SLA contract clause

Any `family-pinned`/`quality-tier` tenant requires a contractual quality SLA: score within X% of strict-pinned baseline (X per-tenant); drift below threshold → automatic refund + revert to `strict` + notification; customer golden eval set is the reference. Without SLA, auto-routing transfers risk with no compensation — enterprises won't sign.

## Consequences

(+) opt-in + transparency + SLA = defensible B2B; real savings on large-output tasks (shared with customer); customer-controlled granularity; quality observer detects silent provider degradation for all tenants.
(−) classifier +5ms/request (short-circuit when `strict`); capability matrix needs ongoing per-model maintenance; observer ~1% extra LLM spend; shadow doubles upstream calls (short eval windows only); cost predictor accuracy depends on `est_out` stability.
Risks + mitigations: observer breaks → all auto-route tenants auto-revert to `strict` after >24h down; adversarial classifier gaming → confidence threshold + default-deny on ambiguous; capability matrix drift → weekly reconciliation; parser breakage despite normalizer → provider-pair integration tests + canary in shadow before any non-shadow change.

## Implementation phases

| Week | Deliverable | Customer-visible |
|------|-------------|------------------|
| W4 | capability matrix + hard-constraint routing + cost predictor | "if you used model Y, this would have cost $X" |
| W5 | shadow mode + customer routing DSL (YAML) | "I write my own rules" + shadow comparison data |
| W6 | heuristic intent classifier + quality observer sampling | `X-Intent`, `X-Routing-Confidence` headers |
| W7 | LLM-as-judge scoring + monthly savings report | monthly email + quality trend |
| W8+ | full auto-routing for explicit opt-in tenants | real savings, SLA-backed |

**Hard rule**: full auto-routing never default, even at W8+. Opt-in permanent.

## Resolved decisions

### Q1 — Pricing model (resolved 2026-05-09)

**Subscription unlocks newhub capability tiers + wallet pass-through with fixed markup% for LLM consumption.** Platform owns financial primitives (wallet/subscriptions/VIP/entitlements); newhub does NOT duplicate them.
- **Subscription (platform)** unlocks per tier: routing mode access; quality SLA tier; audit retention (30/90/365d); support tier (community/business-hours/24×7); internal Lurus services = enterprise tier + `internal_tenant:true`.
- **Wallet (platform)** debits `cost_to_customer = upstream_cost_cny × (1 + markup%)`; markup% contractual (e.g. 15%), no hidden margin.

Why it complements platform: zero financial-primitive duplication (newhub declares capability via entitlement keys); solves the §1 routing incentive problem (margin from subscription + markup%, decoupled from per-request cost → auto-route savings drain customer wallet slower, our margin unchanged → no incentive to degrade); CFO-predictable; differentiates from upstream New API (tiered capability packages vs pure token-metering).

Platform must expose (via `GetEntitlementsGRPC`): `newhub.routing.modes` (allowed mode set), `newhub.quality.sla_tier` (`none|bronze|silver|gold`), `newhub.audit.retention_days` (int), `newhub.support.tier` (`community|business|enterprise`). Coordination with platform required before W4 (filed as platform-side request).

### Q5 — Enterprise vs cloud build profile (resolved 2026-05-09)

**Single codebase, build-tag gating (`//go:build enterprise`); two binaries; three deployment profiles. Hard fork deferred (re-eval Q4 2026).** `//go:build enterprise` registers only `LegalRisk == None` channels (grey-area adapter sources excluded from compilation); `//go:build !enterprise` registers all.

| Profile | Binary | Audience | Billing |
|---------|--------|----------|---------|
| `internal` | enterprise | Lurus internal (Kova/Lucrum/Switch/Creator) | subsidized cross-charge |
| `enterprise` | enterprise | external enterprise | subscription + wallet markup% |
| `cloud` | cloud | external individual/overseas | wallet markup% (higher, self-serve) |

Rationale: independence (enterprise binary auditably free of grey-channel code); complementarity (internal products use enterprise binary, one backbone); reversibility (build-tag reversible, hard fork not); single relay-core maintenance.
Re-eval triggers for hard fork: cloud >30% of newhub revenue, OR enterprise audit demands separate repo, OR cloud velocity blocks enterprise stability (or vice versa) >2 sprints.

## Remaining open questions

3. **Customer DSL syntax** — YAML vs JSON Schema vs visual builder (~5× complexity differ). Owner: design. By: W5 start (2026-05-30).
4. **Classifier hosting** — in-process tiny model (Phi-3/Qwen-0.5B) vs sidecar vs shared service (affects latency/deps/cost). Owner: infra. By: W6 start (2026-06-06).
5. **Quality benchmark bootstrap** — HumanEval/MMLU don't cover customer-specific tasks; how to get per-customer golden eval sets without making it a sales blocker. Owner: customer success + product. By: W7 start (2026-06-13).

## Related

W1 (in flight): SLO baseline (`doc/slo-relay.md`). W2 (planned): upstream conn pool + HTTP/2 reuse (routing-independent). W3 (planned): stability shield (per-channel breaker + per-tenant rate limit; health filter depends on this). W4 (this ADR): blocked on Q1 + Q5 (now resolved).

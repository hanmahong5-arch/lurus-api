# ADR: Cost-Aware Routing with Quality Observability

**Status**: Proposed (W4 implementation gated by Open Questions below)
**Date**: 2026-05-09
**Relates to**: `doc/slo-relay.md` (W1 SLO baseline); ADR-0011 Layer C (zita-sdk-go integration); enterprise vs. cloud product split (`lurus/CLAUDE.md`)

## Context

Two strategic forces collided in the 2026-05-09 product session:

1. **B2B stability mandate** — newhub targets enterprises with strict ToS / SLA / audit requirements.
2. **Cost optimization mandate** — auto-route cheap requests to cheap models, save money.

Naive auto-routing ("classify intent → pick cheap model") is a quality-regression engine. Customer applications break silently when JSON output formats differ subtly between providers, and they don't notice for days. For B2B this is a contract-killer.

But cost optimization is real — a 50K-token document analysis on Claude Opus vs DeepSeek differs by 25× ($0.50 vs $0.02). The challenge is doing it without breaking trust.

Three reality checks shape the design:

- **Chitchat routing saves nothing.** A 5-token "hi" on GPT-4 vs DeepSeek differs by $0.0001. Output tokens dominate cost. Routing decisions must be based on **predicted total cost**, not "looks cheap to handle".
- **Default-on auto-routing breaks B2B trust.** Enterprises require "I said claude-sonnet-4, use claude-sonnet-4". Auto-route is opt-in only.
- **Pricing model gates everything.** If newhub bills flat-rate, auto-routing saves us, not the customer — incentive misalignment guarantees quality-degradation accusations. If we pass-through with markup, both parties win.

## Decision

### 1. Seven-stage routing pipeline

```
Request → Intent Classifier → Capability Filter → Quality Tier →
         Cost Optimizer → Health Filter → Route →
         Response Normalizer → Quality Observer (async)
```

| Stage | Latency | Function |
|-------|---|---|
| Intent classifier | ~5ms | Outputs `{chitchat, code, reasoning, extraction, generation, agent, multimodal}` + confidence; low confidence falls back to tenant-default model |
| Capability filter | ~1ms | Hard reject models lacking required features: tools, vision, JSON mode, max context window |
| Quality tier | ~1ms | Per-task per-model quality grade `{S, A, B, C}`; only consider models above tenant-specified minimum |
| Cost optimizer | ~2ms | `predicted_cost = in_tokens × in_price + est_out × out_price`; choose cheapest model meeting quality threshold. **Must include prompt-cache hit prediction** (Anthropic-style cached prefix can make Anthropic cheaper than DeepSeek on raw price) |
| Health filter | ~1ms | Exclude circuit-broken / rate-limited channels (reuse existing breaker) |
| Route + relay | (existing) | Existing relay path |
| Response normalizer | per chunk | SSE format, JSON strictness, stop-token semantics — normalize across providers so customer parsers don't break |
| Quality observer | async | 1% sampling LLM-as-judge; weekly update to quality tier; immediate downgrade on drift detection |

Total added overhead: ~10ms. Fits within W1 SLO budget (50ms newhub overhead P99).

### 2. Per-tenant routing modes

| Mode | Behavior | Default for |
|------|----------|-------------|
| `strict` | Pin to specified model; no auto-routing | New enterprise tenants; finance / legal / healthcare verticals |
| `family-pinned` | Auto-select within a model family ("only Claude Haiku/Sonnet/Opus") | Cost-conscious tenants who trust one vendor |
| `quality-tier` | "Code tasks: any model scoring ≥ B" | Growth-stage tenants |
| `shadow` | Route to pinned model AND alt-routed model in parallel; compare; **report only, never switch** | **Gateway drug** — enterprises see real savings + quality data on their own traffic before opting in |

Auto-route (`family-pinned` or `quality-tier`) is **permanent opt-in**. `shadow` mode is the bridge — without seeing concrete savings + quality data on their own traffic, no enterprise flips the switch.

### 3. Anti-rules — never auto-route

- Mid-conversation requests with prior `tool_call` (model swap drops tool state)
- Customer-stamped `audit: true` or `pin: <model>` requests (regulatory audit)
- Extended-thinking / reasoning models (output format incompatible across providers)
- Non-first turn in a multi-turn conversation (session continuity expectation)

### 4. Customer transparency (mandatory when auto-route enabled)

Per-request response headers:
- `X-Routed-Model: claude-3-5-haiku-20241022`
- `X-Routing-Reason: cost_optimizer:family-pinned`
- `X-Estimated-Cost: 0.0042`
- `X-Quality-Tier: A`

Monthly customer report:
- Auto-routing savings ($X saved vs strict-pinned baseline)
- Quality score trend (this month vs baseline)
- Request count by routed model

**Without these mechanisms, auto-route MUST NOT be enabled.** Hidden routing destroys customer trust faster than any savings can justify.

### 5. Quality SLA contract clause

Any tenant in `family-pinned` or `quality-tier` mode requires a contractual quality SLA:
- Quality score must remain within X% of strict-pinned baseline (X negotiated per tenant)
- Drift below threshold triggers automatic refund + revert to `strict` + customer notification
- Customer-supplied golden eval set is the measurement reference

Without SLA, auto-routing transfers risk to the customer with no compensation. Enterprises won't sign.

## Consequences

**Positive:**
- Enterprise-grade trust: opt-in + transparency + SLA → defensible B2B architecture.
- Real cost optimization where safe (large-output tasks); savings shared with customer (modulo pricing model).
- Customer-controlled granularity (strict / family / tier / shadow).
- Quality observer detects silent provider degradation — benefits all tenants, not just auto-routed ones.

**Negative:**
- Classifier inference adds ~5ms to every request. Mitigation: short-circuit when `mode == strict` (skip classifier entirely).
- Capability matrix needs ongoing maintenance (each new provider/model = manual feature mapping).
- Quality observer cost: 1% sampling × LLM-as-judge call ≈ ~1% additional LLM spend.
- Shadow mode doubles upstream calls for opted-in tenants — only enable in short evaluation windows (1-2 weeks per tenant).
- Cost predictor accuracy depends on `est_out` distribution stability; novel task types may regress.

**Risks:**
- Silent quality drop if observer breaks. Mitigation: observer self-monitoring; if down > 24h, all auto-route tenants auto-revert to `strict`.
- Adversarial prompts gaming the classifier. Mitigation: confidence threshold + default-deny on ambiguous classification.
- Capability matrix drift (provider adds capability, matrix stale). Mitigation: weekly reconciliation script.
- Customer parser breakage despite normalizer. Mitigation: provider-pair integration tests + canary in shadow mode before any routing change ships to non-shadow tenants.

## Implementation phases

| Week | Deliverable | Customer-visible outcome |
|------|-------------|--------------------------|
| W4 | Capability matrix + hard-constraint routing + cost predictor | Dashboard: "if you used model Y, this request would have cost $X" |
| W5 | Shadow mode + customer routing DSL (YAML) | "I write my own rules" + "I see shadow comparison data" |
| W6 | Heuristic intent classifier + quality observer sampling | `X-Intent` and `X-Routing-Confidence` headers |
| W7 | LLM-as-judge quality scoring + monthly savings report | Monthly email + quality trend chart |
| W8+ | Full auto-routing for explicit opt-in tenants | Real savings, SLA-backed |

**Hard rule**: Full auto-routing is never default, even at W8+. Opt-in is permanent.

## Resolved decisions

### Q1 — Pricing model (resolved 2026-05-09)

**Decision: subscription unlocks newhub capability tiers + wallet pass-through with fixed markup% for LLM consumption.**

Platform owns the financial primitives (wallet, subscriptions, VIP, entitlements). Newhub does NOT duplicate them. Instead:

- **Subscription (platform-managed)** unlocks newhub features per tier:
  - Routing mode access (`strict` / `family-pinned` / `quality-tier` / `shadow`)
  - Quality SLA tier (refund clause stringency)
  - Audit log retention (30 / 90 / 365 days)
  - Support tier (community / business-hours / 24×7)
  - Internal Lurus services = enterprise-tier subscription + `internal_tenant: true` tag
- **Wallet (platform-managed)** debits per request: `cost_to_customer = upstream_cost_cny × (1 + markup%)`. Markup% is contractual (e.g. 15%); no hidden margin.

**Why this complements platform without duplication:**

1. Zero financial-primitive duplication — platform stays sovereign on money, newhub declares capability requirements via entitlement keys.
2. Solves the routing incentive problem in §1 (Cost Optimizer): our margin comes from subscription + markup%, decoupled from per-request LLM cost. When auto-routing saves cost, customer's wallet drains slower → customer wins; our margin unchanged → no incentive to silently degrade quality.
3. CFO-predictable: subscription = fixed monthly (budget anchor); wallet = metered with monthly burn alerts (already in `RecordQuotaConsumed`).
4. Differentiation from upstream New API: New API is pure token-metering; we sell tiered capability packages. Same 10K tokens costs differently per tier (because tier includes SLA + audit + support), enabling SaaS pricing not commodity API pricing.

**Cross-team dependency on platform:** platform must expose the following entitlement keys, queried via `GetEntitlementsGRPC`:
- `newhub.routing.modes` → set of allowed routing modes
- `newhub.quality.sla_tier` → `none | bronze | silver | gold`
- `newhub.audit.retention_days` → int
- `newhub.support.tier` → `community | business | enterprise`

Coordination required with platform session before W4. Filed as platform-side request.

### Q5 — Enterprise vs. cloud build profile (resolved 2026-05-09)

**Decision: single codebase, build-tag gating (`//go:build enterprise`); two binaries; three deployment profiles. Hard fork deferred until product-market fit divergence is clear (target re-evaluation: Q4 2026).**

```go
//go:build enterprise
// only register channels with LegalRisk == None; grey-area
// adapter source files excluded from compilation entirely.

//go:build !enterprise
// register all channels including grey-area
```

| Profile | Binary | Audience | Billing |
|---------|--------|----------|---------|
| `internal` | enterprise | Lurus internal products (Kova / Lucrum / Switch / Creator) | Subsidized cross-charge |
| `enterprise` | enterprise | External enterprise customers | Subscription + wallet markup% |
| `cloud` | cloud | External individual developers / overseas | Wallet markup% (higher %, self-serve onboarding) |

**Rationale:**
- **Independence preserved**: each edition has coherent value prop, distinct SLA, different sales motion. Enterprise binary doesn't contain grey-channel adapter code at all — customer can audit the binary (not just the source repo) for ToS cleanliness.
- **Complementarity preserved**: internal Lurus products use the enterprise binary with subsidized internal-tenant tags, so the same hardened relay backbone serves the entire Lurus portfolio. One backbone, many fronts.
- **Reversibility**: build-tag separation is reversible; hard fork is not. We don't yet know whether cloud has real demand vs. brainstorm-only — keep optionality.
- **Single relay-core maintenance**: bug fixes, observability, routing logic written once, applied to both binaries. Hard fork would double maintenance and let quality diverge.

**Re-evaluation triggers** for hard fork:
- Cloud edition reaches >30% of newhub revenue, OR
- Enterprise customer audit explicitly demands separate repo (not just separate binary), OR
- Cloud feature velocity blocks enterprise stability (or vice versa) for >2 sprints

## Remaining open questions

3. **Customer DSL syntax** — YAML rule engine vs. JSON Schema vs. visual builder. Implementation complexity differs ~5×.
   - **Owner**: design. **Required by**: W5 start (2026-05-30).
4. **Classifier hosting** — in-process tiny model (Phi-3 / Qwen-0.5B), sidecar container, or shared classification service? Affects per-request latency, dependency footprint, cost.
   - **Owner**: infra. **Required by**: W6 start (2026-06-06).
5. **Quality benchmark bootstrap** — standard benchmarks (HumanEval, MMLU) cover code/general but not customer-specific tasks. How do we get golden eval sets per customer without making it a sales blocker?
   - **Owner**: customer success + product. **Required by**: W7 start (2026-06-13).

## Notes on related decisions

- **W1 (in flight)**: SLO baseline metrics define the latency budget this routing pipeline must fit within. See `doc/slo-relay.md`.
- **W2 (planned)**: Upstream connection pool + HTTP/2 reuse. Independent of routing — happens regardless of routing mode.
- **W3 (planned)**: Stability shield (per-channel circuit breaker enhancement, per-tenant rate limit). Routing health filter depends on this.
- **W4 (this ADR)**: Cannot start until Open Q1 (pricing) and Q5 (build profile) are resolved.

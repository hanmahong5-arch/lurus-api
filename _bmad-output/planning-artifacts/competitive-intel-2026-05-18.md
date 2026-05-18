# Competitive Intel + Synthesis — 2026-05-18

3 parallel sonnet agents (each ~4 min, web-research). Raw reports archived; **synthesis** below has the cross-correlated high-confidence findings ranked by ROI for newhub's Reseller mode.

## Synthesis — findings that appear in 2+ agent reports (high confidence)

### Onboarding / TTFT
1. **Pre-issued test token + working `curl` baked on registration** — A (OpenRouter <2min, Vercel $5 auto-credit) + C ("golden path newhub should match"). Today newhub: register → manually navigate to Tokens tab → click create. Drop-off point for Reseller demos.
2. **Browser "try it" / Workbench affordance** — C (Anthropic Workbench, OpenRouter Request Builder). v2 Playground page is mocked; no browser-only test path.

### Multi-tenant / Reseller (this is newhub's strategic position)
3. **Tenant-level credit pool with delegated hard cap** — A (LiteLLM Org→Team→User→Key 4-level budget) + C (Stripe Connect + OpenRouter `limit`/`limit_reset`). Newhub has per-token quota but no per-tenant pool. Without this Resellers can't cap a customer's spend without exposing their master billing.
4. **Provisioning API for programmatic sub-tenant key creation** — A (Portkey Org→Workspace→Key) + C (OpenRouter Management Key `POST /api/v1/keys` with `limit`/`limit_reset`/`expires_at`/`creator_user_id`). Resellers with >5 customers need automation, not UI clicks.
5. **Workspace-level policy inheritance** — A (Portkey, OpenRouter workspaces). Model allowlists / rate caps set at workspace propagate to all keys under it. Newhub has tenant isolation but no policy-inheritance layer.
6. **Self-service token UI for EndUser mode** — A (OpenRouter, LiteLLM user-facing self-serve). Newhub Token UI is admin-facing; EndUser tier can't rotate without admin.

### Cost & Quota UX
7. **80% / threshold budget alerts (webhook + email)** — A (Portkey, LiteLLM, CostHawk, Bifrost all do this) + C (Anthropic Console $10/$50/$100 thresholds) + B (Datadog cost monitors). Hard-block-at-100% with zero advance warning is industry behind.
8. **"Cost by model" pre-aggregated panel** — A (all 6 gateways) + B (model is the universal cost-attribution dimension). Newhub log has the raw data; v2 Dashboard doesn't surface it.
9. **Per-token quota as visual progress bar** — A (Portkey virtual keys / OpenRouter guardrail card showing $X of $Y, %). Newhub Token list shows raw integer remain_quota.
10. **"Created by / Last used at" audit columns on Token list** — C (OpenRouter `creator_user_id`, Stripe key prefix logs, OpenAI "Last used" date). Newhub log already captures this; just needs surfacing.

### Observability
11. **P95/P99 latency tile (not just P50)** — B (LangSmith default, Datadog). Newhub Dashboard just got P50 TTFT; P99 is what actually anchors SLOs.
12. **Cache hit rate as first-class tile with $-savings** — B (Helicone leader; shows "73% hit rate saving $1,247/month"). Newhub has `request_fingerprint` — half a feature.
13. **Prompt text preview in trace drill-down** — B (all 6 observability tools store + display). Newhub logs token counts but not prompt text — this is a **structural** gap (storage decision + retention policy).
14. **Default alert rule pack** — B (error rate >X%, P99 spike, hourly cost spike, daily 150% baseline, zero-response rate). Story 7-5 has the dashboard but ships no alert defaults.

### Settings / Account
15. **Active session list with per-session revoke** — C ("must invalidate stolen session without password reset"; GitHub/Anthropic/Linear all have this). Newhub Sessions tab currently mock — security gap.
16. **Notifications matrix** (event × channel checkbox grid) — C. Minimum events for newhub: quota 80%/100%, token created by another admin, billing balance low, failed-relay spike.
17. **Customer-facing usage CSV export** — A (Helicone, OpenRouter; Portkey enterprise via data lake). Resellers need this to invoice their own customers.

### Anti-recommendations (cross-confirmed; do NOT port)
- **Inline AI-judge / LLM-as-guardrail** — doubles latency + adds $0.002/req; GitHub issues show production users disable it (A). Use regex + blocklist instead.
- **Prompt registry inside gateway** — duplicates what LangChain/Instructor do better; makes relay stateful (A).
- **Semantic caching with shared embedding index** — both Helicone and Portkey leaked prompt patterns across tenants before retrofitting per-tenant namespace; only attempt with isolation designed from day 1 (A). Exact-match cache first.
- **LLM-as-judge eval pipeline** (Langfuse / LangSmith / Arize) — 4-6 weeks minimum; not a hardening-swarm-scope feature (B).

### Where newhub already matches/exceeds — keep marketing this
- gRPC billing integration to a platform wallet — no competitor does this; they all run their own ledger
- 30+ provider relay — comparable to LiteLLM, ahead of Helicone
- Internal API with scope-based bearer auth — more granular than Cloudflare/Vercel
- Redemption codes — unique to newhub's Reseller mode

## ROI-ranked actionable list

### Tier 1 — Quick wins (each 0.5-2 days, mostly UI-only, data already exists)
- **T1.A** Cost-by-model Dashboard panel — finding #8
- **T1.B** Per-token quota visual progress bar in Token list — finding #9
- **T1.C** "Created by / Last used at" columns on Token list — finding #10
- **T1.D** P95/P99 latency tile (extends our just-built kpis.js) — finding #11
- **T1.E** Cache hit / dedup-rate tile (aggregate over `request_fingerprint`) — finding #12
- **T1.F** Active session list + revoke (Settings Sessions) — finding #15
- **T1.G** Onboarding `curl` snippet + auto-token on first login — finding #1 (frontend half)

### Tier 2 — Medium lifts (each 3-7 days, schema or new endpoints)
- **T2.A** Tenant-level credit pool — finding #3 (schema migration + delegation API)
- **T2.B** Provisioning API for sub-tenant keys — finding #4
- **T2.C** 80% / threshold budget webhook alerts — finding #7
- **T2.D** EndUser self-service token UI — finding #6
- **T2.E** Default alert rule pack (Prometheus + Alertmanager) — finding #14
- **T2.F** CSV usage export endpoint — finding #17
- **T2.G** Notifications matrix UI — finding #16

### Tier 3 — Structural (each 1-3 weeks, defer unless committed)
- **T3.A** Prompt text storage + trace preview — finding #13 (storage cost + retention decision)
- **T3.B** Exact-match cache layer (Redis-backed) — anti-rec rules out semantic for now
- **T3.C** Workspace-level policy inheritance — finding #5
- **T3.D** Browser Playground / Workbench wired — finding #2

## Three swarm options

### Option α — "Polish & honesty" (2-3 days, all Tier 1)
4 parallel lanes:
- L1 Dashboard observability tiles: T1.A + T1.D + T1.E (extend kpis.js + new tiles)
- L2 Token list UX: T1.B + T1.C
- L3 Settings Sessions: T1.F (list + revoke)
- L4 Onboarding curl snippet: T1.G (Dashboard "first run" block)
**Risk**: shallow — doesn't move the Reseller story forward.

### Option β — "Reseller MVP cut" (5-8 days, mixed Tier 1+2)
4 parallel lanes:
- L1 Reseller pool + provisioning API: T2.A + T2.B (architect-led; biggest schema change)
- L2 Budget alerts: T2.C + T2.E (notification dispatch + default Prometheus rules)
- L3 Token list audit + sessions + quota bar: T1.B + T1.C + T1.F
- L4 Onboarding + Dashboard polish: T1.G + T1.A + T1.D
**Risk**: L1 is hard to time-box; schema migration needs Anita signoff.

### Option γ — "Observability deepening" (3-5 days, Tier 1.D/E + Tier 2.C/E + Tier 3.A scoped)
4 parallel lanes:
- L1 KPI tiles full: T1.D + T1.E + T1.A
- L2 Alert defaults: T2.C + T2.E
- L3 Prompt preview (read-only, fingerprint not full text yet): T3.A scoped to last-N-char hash
- L4 CSV export: T2.F
**Risk**: T3.A even scoped is contentious — storage cost not modeled.

## Source reports (verbatim)

Raw reports from the 3 sonnet agents are preserved in this commit's session
log. Citations to product pages are in the agents' returns; primary sources:

- Portkey: portkey.ai/pricing, portkey.ai/docs/guides/use-cases/multi-tenant-ai-feature
- Helicone: helicone.ai/pricing, docs.helicone.ai/guides/cookbooks/cost-tracking
- LiteLLM: docs.litellm.ai/docs/proxy/user_management_heirarchy
- OpenRouter: openrouter.ai/docs/quickstart, openrouter.ai/docs/api/api-reference/api-keys/create-keys
- Vercel AI Gateway: vercel.com/ai-gateway, vercel.com/docs/ai-gateway/pricing
- Cloudflare: developers.cloudflare.com/ai-gateway
- Langfuse: langfuse.com/docs/observability/features/token-and-cost-tracking
- LangSmith: docs.langchain.com/langsmith/observability
- Datadog: docs.datadoghq.com/llm_observability
- Stripe Connect: docs.stripe.com/keys/restricted-api-keys
- OpenAI Projects: platform.openai.com/docs/api-reference/project-api-keys

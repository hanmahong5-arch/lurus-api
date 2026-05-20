# ADR: D1 — newapi/newhub Fork Final Decision (Option B)

**Status**: Proposed (pending Anita confirmation)
**Date**: 2026-05-20
**Supersedes**: `doc/decisions/2026-05-05-newapi-newhub-fork-audit.md` (which left D1 open with 3 candidate options)
**Relates to**: Epic 8 (Newapi/Newhub Consolidation), sprint-status.yaml `open_decisions.D1_fork_strategy`

## Context

The 2026-05-05 fork audit revealed newhub is **not** "newapi + a multi-tenant layer on top" but a **parallel fork** that has independently grown the same LLM-gateway base into a superset (multi-tenant + Provisioning API + CreditPool + Phase 2 hardening). Three options were left open:

| Option | Description |
|---|---|
| A | Keep newapi as the LLM gateway; newhub stays the tenant layer wrapping it |
| **B** | **Retire newapi; newhub absorbs all LLM-gateway traffic** |
| C | Continue both in parallel — newapi for Lurus self-serve, newhub for resellers |

The 2026-05-05 audit recommended A. Subsequent facts have shifted the calculus:

1. **2026-05-08**: newapi was cut over to vanilla upstream `calciumion/new-api:v1.0.0-rc.4`. All 27 local commits (Gemini TTS, NATS events, admin api-key endpoint, cost-spike protections) were **dropped**. newapi is effectively now an upstream-tracking deployment, not an actively developed fork.
2. **2026-05-08 onward**: Any LLM-gateway bug that lands on newapi (e.g., the 2026-05-19 Gemini 2.5-flash thought/answer stream split) has to be **independently patched in newhub** if both forks are kept. Two-fork tax confirmed.
3. **newhub already ships everything newapi ships**: vendor adapters (30+), relay routes (150+), token/channel/user CRUD, V2 multi-tenant API, V1 compat — all present in newhub. The "newhub-on-top-of-newapi" framing in `lurus/CLAUDE.md` is **incorrect** and must be revised (see §Action Items).
4. **Provisioning API (Q3 Phase 2)** does not call newapi at all — it talks to newhub directly. The `POST /api/admin/users/:id/api-key` newapi endpoint expected by old documentation **does not exist** in the vanilla v1.0.0-rc.4 image (removed upstream). Anything that depended on it must already be migrating to newhub.

## Decision

**Adopt Option B: retire newapi as a deployed service; newhub absorbs all LLM-gateway traffic on `newapi.lurus.cn`.**

### Migration sequence (executed only after Anita confirms this ADR)

1. **R6 STAGE rehearsal** (T+0):
   - Spin up newhub on R6 STAGE with a `legacy` tenant pre-seeded
   - `pg_dump` newapi PROD DB (`tokens`, `users`, `channels`, `redemptions`, `logs`)
   - Restore into newhub STAGE DB with `tenant_id = legacy` injected on every row
   - Smoke: run identical prompts against newapi-PROD and newhub-STAGE; diff responses byte-for-byte where possible (modulo timing fields)
2. **Pre-cutover dual-run** (T+7 ~ T+14):
   - Point a copy of production traffic at newhub via shadow routing (a 5% mirror is enough)
   - Compare response status codes, latency, billing accuracy vs. newapi
   - Acceptance: ≥ 99.5% functional match, p95 latency within +10%, no billing discrepancies > 1¢ per tenant
3. **DNS cutover** (T+14):
   - Change `newapi.lurus.cn` Cloudflare record from newapi K8s Service to newhub K8s Service
   - Keep newapi pod running but **un-served** for 14 days as immediate rollback target
4. **Observation window** (T+14 ~ T+28):
   - Daily diff of error rates, P95, billing reconciliation between newhub and last-known-good newapi behavior
   - Customer support ticket review for any newapi-specific behavior break
5. **Hard retire** (T+28):
   - `kubectl delete` newapi deployment + service
   - Archive newapi git repo (read-only on GHCR; image tag pinned)
   - Update `lurus.yaml`: drop `2b-svc-newapi` entry, mark as historical
   - Update `lurus/CLAUDE.md`: rewrite the "newhub is on top of newapi" line

### Rollback (executable at any phase)

- Phases 1–3: discard newhub STAGE/PROD state, change DNS back (TTL ≤ 5min). Total rollback ≤ 15 minutes.
- Phase 4: re-point DNS to newapi service. Re-enable newapi deployment if scaled to zero. Same 15-minute window.
- Phase 5 (after hard retire): full restore from newapi image tag `ghcr.io/hanmahong5-arch/lurus-newapi:main-fc49e72` + DB restored from latest WAL-G snapshot. ETA ~30 minutes. Cutoff: once newhub has accumulated >7 days of newapi-only data (new tokens, new logs), rollback becomes data-lossy and should not be attempted.

## Why not A

- Indefinite two-fork tax: every upstream patch newapi needs (vanilla upgrade) must be **mirrored** to newhub. Engineering cost: ~0.5-1 day per upstream release.
- newapi is no longer actively developed — keeping it deployed means we're shipping an **unmaintained** binary as production cash-path infrastructure. Whatever security-critical bug appears in upstream is on us to backport into both forks.
- No new feature differentiates the two. Anything newapi does, newhub does. The "newhub builds on newapi" mental model gives architecture an extra layer that does not exist in the code.

## Why not C (parallel forever)

- Long-term maintenance burden compounds: two deployments, two monitoring stacks, two billing pipelines, two on-call surfaces, two ingress endpoints, two cert renewals.
- Customer confusion: same API surface delivered by two different binaries with subtle behavior differences (cookie names, error envelopes).
- Splits feature velocity: any cross-cutting feature (e.g., Provisioning API) has to ship twice or only on one side.

## Consequences

**Positive:**
- Single LLM-gateway codebase → upstream patches apply once, regression surface halves.
- Eliminates "which fork does this bug belong in" confusion.
- Frees mental model: `newapi.lurus.cn` is just newhub's primary domain.
- Unblocks Epic 8-3 / 8-4 / 8-5 (provider parity audit / cutover plan / yaml revise).

**Negative:**
- One-time data-migration risk (mitigated by R6 STAGE rehearsal + dual-run).
- Customer cookie name change (`session` vs. `lurus-session`) — users may need to re-login once during cutover. Acceptable one-time impact.
- Any external integration hardcoded to newapi-specific quirks needs verification. Audit must precede T+14.

**Risks:**
- newhub has bugs newapi doesn't (or vice versa) — surface uncovered only by dual-run. Mitigation: 5% mirror for 7-14 days.
- DB schema drift between newapi (vanilla upstream v1.0.0-rc.4) and newhub (forked) larger than expected. Mitigation: rehearse on R6 with PROD dump copy first; if schema diff is non-trivial, this ADR pauses for additional engineering before re-attempting.

## Action items (post-confirmation)

| Item | Owner | Sprint |
|---|---|---|
| Update `lurus/CLAUDE.md` "newhub on top of newapi" line | Anita | After ADR merge |
| Schedule R6 STAGE rehearsal slot | Anita | Wave A close-out |
| Pre-cutover audit: external clients hardcoded to newapi | Anita + customer success | T+0 → T+7 |
| `_bmad-output` sprint-status.yaml: `D1_fork_strategy` → `accepted (Option B)` | Opus | At ADR merge time |
| Move Epic 8-3 / 8-4 / 8-5 from `blocked` to `backlog` | Opus | At ADR merge time |

## Open questions deferred until rehearsal data is in

1. **Customer notification timing** — pre-cutover email vs. silent (acceptable given the one cookie re-login)?
2. **Migrating ledger row IDs** — keep newapi's auto-increment sequences or remap? Affects log/audit referential integrity.
3. **NATS LLM_EVENTS** — re-enable on newhub (newhub-side NATS publisher exists) or leave as documented drift since R6 NATS bridge is already silent on newapi?

These do not block the decision. They will be resolved during Phase 1 rehearsal.

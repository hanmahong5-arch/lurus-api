# Story 15-0 — Smart Routing Baseline (v0, already shipped)

**Status**: review (retroactive — describes what's already in main)
**Epic**: 15 (智能路由 / Smart Routing) — Phase 3 (差异化期, 2026-Q4 backlog)
**Date**: 2026-05-20
**Relates to**: ADR `2026-05-09-cost-aware-routing.md` (full vision, W4+ implementation pipeline)
**Code location**:
- `internal/app/hub/hub.go` (orchestrator singleton)
- `internal/app/hub/smart_routing.go` (`AdjustWeights`)
- `internal/app/hub/channel_scorer.go` (per-channel scoring logic)
- `internal/app/hub/smart_routing_test.go`, `channel_scorer_test.go`

## Why this story exists

The Cost-Aware Routing ADR (2026-05-09) describes a 7-stage pipeline (intent classifier → capability filter → quality tier → cost optimizer → health filter → route → response normalizer → quality observer) targeted for Weeks 4-8+. That full pipeline is not yet built.

However, the **observation/scoring foundation** for it **has been built and shipped**. It records per-channel success rate and latency EMA, computes a weighted score, and exposes `AdjustWeights()` that callers can invoke to modify channel selection weights based on observed performance.

This story documents the v0 baseline so that:
1. The roadmap is honest about what's already in production.
2. Future Epic 15 work has a clean starting point ("extend v0" rather than "design from scratch").
3. Reviewers can spot the gap: scoring exists, but the **actual call site that invokes `AdjustWeights()` during channel selection** is not yet wired. The v0 is half-active — it observes but does not yet influence routing.

## What v0 does today

### ChannelScorer (`channel_scorer.go`)

Per-channel state:
- Success count, failure count
- Latency EMA (exponential moving average, `alpha = 0.3`)

Score computation (weighted):
- `0.3 × latency_score + 0.5 × success_rate + 0.2 × cost_factor`
  - `latency_score`: 0–1s → 1.0; up to 5s → 0.4; ≥5s → <0.4
  - `success_rate`: total_success / (total_success + total_failure); 1.0 if no data
  - `cost_factor`: currently a placeholder constant (1.0 for all channels) — **the "cost" dimension is unwired**
- Channels with no observations default to score 0.5 (neutral)

Auto-decay: every minute, observations >10min old have their counts halved (keeps the scorer responsive to recent shifts).

### AdjustWeights (`smart_routing.go`)

Given a list of channels with original `weight` field, returns a new list with each weight multiplied by a factor derived from the score:
- score 1.0 → ×1.5 boost
- score 0.5 → ×1.0 (neutral)
- score 0.0 → ×0.5 reduction
- unknown / no-data → no change (×1.0)

Floor: weights never drop below a configurable minimum to prevent total channel starvation.

### Hub orchestrator (`hub.go`)

- Singleton `Hub` holding `Scorer` + `Aggregator` (usage aggregator for telemetry)
- `RecordRelayOutcome(channelID, success, latencyMs, ...)` called from the relay handler after each request
- Background loops:
  - 1min decay ticker (scorer)
  - Aggregator flush (per its config)

## What v0 does NOT do today

- **Cost factor is unwired**: the 0.2 weight in the score formula is a placeholder; all channels get the same cost factor. Per-channel pricing data (upstream model cost) is not yet plumbed into the scorer.
- **`AdjustWeights()` is not called during channel selection**: the function exists and is tested, but the relay's channel-pick path does not yet invoke it. Scorer state is recorded but ignored.
- **No intent classifier**: the ADR's stage 1 (classify request into chitchat / code / reasoning / extraction / generation / agent / multimodal) does not exist.
- **No capability filter**: ADR stage 2 (filter models lacking tools / vision / JSON mode / context window). Not built.
- **No quality tier**: ADR stage 3 (per-task per-model quality grade S/A/B/C). Not built.
- **No cost optimizer**: ADR stage 4 (predicted total cost computation incl. prompt-cache hit prediction). Not built.
- **No response normalizer**: ADR stage 7 (SSE format, JSON strictness, stop-token semantics). Not built.
- **No quality observer**: ADR stage 8 (1% sampling LLM-as-judge). Not built.

## Acceptance criteria for v0 status = "done"

This story is in `review` because v0 is shipped but not yet integrated into routing decisions. To mark `done`:

1. **Decide the call site**: where does relay pick a channel today, and what would invoking `AdjustWeights()` there look like? Document the call site in `hub.go` package doc.
2. **Wire AdjustWeights into relay channel selection**, gated behind a per-tenant feature flag (`smart_routing.v0_weights_enabled`) defaulting to OFF.
3. **Add an observation-only mode** so Epic 15 evaluation has a "what would weights have been if applied" trace before any tenant flips the flag.
4. **Add a smoke test**: tenant with flag OFF → original weights honored; tenant with flag ON → weights multiplied per scorer state.
5. **Surface scorer state in `/metrics`**: per-channel score gauge, per-channel latency EMA gauge, per-channel success rate gauge. (Aggregator already exposes some — verify and document.)

## Sequencing relative to Epic 15

Once v0 = `done` per the criteria above, Epic 15 stories layer on:
- 15-1: Capability filter (Stage 2)
- 15-2: Cost factor wiring (Stage 4 in cost-only mode; closes the placeholder in v0's formula)
- 15-3: Intent classifier (Stage 1) — heaviest item, ~5ms inference latency budget
- 15-4: Quality tier matrix + LLM-as-judge observer (Stages 3 + 8)
- 15-5: Response normalizer (Stage 7)

v0 is the foundation. Epic 15 stories extend it; they do not rebuild it.

## Risks if v0 stays half-wired indefinitely

- **Stale code**: scorer accumulates state forever for no purpose. Memory growth (per-channel maps) is bounded by channel count, so not a production hazard, but it is dead-weight code that future contributors will trip over.
- **Misleading metrics**: if scorer state is exposed in `/metrics` but `AdjustWeights` isn't called, a dashboard could mislead observers into thinking routing is dynamic when it's not.
- **Test debt accumulation**: scorer tests are passing today; if the surrounding code drifts and scorer becomes the only thing testing certain assumptions, future refactors might break it silently.

Mitigation: complete the acceptance criteria above (wire it OR remove it). "Half-shipped feature" is the worst state.

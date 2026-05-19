# Story 7-4 Acceptance Report — Quota Real Backpressure

**Date**: 2026-05-18
**Status**: `review → STAGE drill PENDING operator session`
**Story doc**: `story-7-4-quota-real-backpressure.md`

## Implementation Evidence (markers present)

- Rate-limit middleware in `internal/adapter/middleware/` covering
  CostSpikeLimit, ModelRequestRateLimit, CriticalRateLimit, BootstrapRateLimit.
- Token bucket / sliding window implementations exist; thresholds
  configured via env vars.
- Metrics exposed for rate-limit rejections so alerts can fire when
  the system is actively pushing back.

## Measurement Validity (NOT YET EVIDENCE)

**SELF-VALIDATING — NOT EVIDENCE**: middleware is plumbed, thresholds
are set. We have not yet observed the system serve degraded traffic
and reject excess load with the configured policy under real conditions.

A rate limit you never measured isn't a backpressure system; it's a
parameter table.

## STAGE Drill Plan (Phase 3, 2026-06-16+)

- Script: `scripts/chaos-drill.sh` — cost-spike scenario.
- Generate 10× normal request rate from a single token for 5 minutes.
- Expected: CostSpikeLimit kicks in, 429s start flowing, success rate
  drops to bounded steady-state (not zero, not 100%).
- Pass criteria: P95 success latency remains stable for non-spiking
  users (no global degradation); spike token sees structured 429.

## Status

- markers present: ✅
- measurement meaningful: ⏳ PENDING Phase 3 chaos drill
- PROD-ready: ⏳ PENDING G2 + drill evidence that limits hold

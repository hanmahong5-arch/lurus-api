# Story 7-1 Acceptance Report — Provider Circuit Breaker

**Date**: 2026-05-18
**Status**: `review → STAGE drill PENDING operator session`
**Story doc**: `story-7-1-provider-circuit-breaker.md`

## Implementation Evidence (markers present)

- Breaker state machine + threshold logic implemented in
  `internal/app/relay/` (search: `CircuitBreakerState`, `circuit_breaker`).
- Prometheus metrics exposed in `internal/pkg/metrics/metrics.go`:
  `lurus_gateway_circuit_breaker_state{channel_id}`,
  `lurus_gateway_circuit_breaker_trips_total{channel_id}`,
  `lurus_gateway_circuit_breaker_rejections_total{channel_id}`.
- Helpers `RecordCircuitBreakerState / Trip / Rejection` invoked from
  channel-selection path.
- Grafana panels for breaker state already present in `newhub-slo.json`.

## Measurement Validity (NOT YET EVIDENCE)

**SELF-VALIDATING — NOT EVIDENCE**: code committed, metrics wired, panels
visible. We have not yet observed a circuit actually opening and closing
under load with the exact thresholds we configured.

Per global CLAUDE.md §4.1 ⑥ — "markers present" (text + commits) is not
the same as "measurement is meaningful" (independent observation of
breaker dynamics under real failure).

## STAGE Drill Plan (Phase 3, 2026-06-16+)

- Script: `scripts/chaos-drill.sh` 5xx-burst scenario.
- Inject: 3 consecutive failures from one upstream channel.
- Expected: `lurus_gateway_circuit_breaker_state{channel_id=...} = 1`
  (open) for the cooldown window, then half-open probe attempt.
- Pass criteria: open after threshold trips, rejects N additional
  attempts (visible in `circuit_breaker_rejections_total` rate), then
  recovers on success probe.

## Status

- markers present: ✅
- measurement meaningful: ⏳ PENDING Phase 3 STAGE drill
- PROD-ready: ⏳ PENDING G2 authorization + chaos-drill evidence

# Story 7-5 Acceptance Report — Chaos Drill + SLO Dashboard

**Date**: 2026-05-18
**Status**: `review → STAGE drill PENDING operator session`
**Story doc**: `story-7-5-chaos-drill-slo-dashboard.md`

## Implementation Evidence (markers present)

- SLO dashboard: `deploy/grafana/newhub-slo.json` — 15+ panels covering
  request rate, latency P50/P95/P99, relay overhead, billing health,
  channel health, circuit-breaker state, and (as of 2026-05-18 Lane δ)
  tenant credit-pool panels.
- Alert pack: `deploy/grafana/newhub-alerts.yaml` — 11 alerts after the
  metric-prefix repair in commit `0c35935e` (was `lurus_hub_*` → now
  `lurus_gateway_*` / `lurus_billing_*`); 2 new pool alerts added in
  Lane δ commit `395d9065`.
- Chaos drill scripts in `scripts/` for 5xx burst, slow-loris, cost
  spike scenarios.

## Measurement Validity (NOT YET EVIDENCE)

**SELF-VALIDATING — NOT EVIDENCE**: dashboards render in Grafana, alert
expressions evaluate. The metric-prefix repair this session means alerts
are no longer silently dead — but **we have not yet observed a legitimate
production-like condition cause an alert to fire end-to-end** (Prometheus
rule → Alertmanager → notification channel).

The drill scripts exist but **have not been run on STAGE**.

## STAGE Drill Plan (Phase 3, 2026-06-16+)

- Three scenarios to execute on R6:
  1. `scripts/chaos-drill.sh 5xx-burst` — confirm CircuitBreakerOpenChannel
     alert fires + dashboard reflects state.
  2. `scripts/chaos-drill.sh cost-spike` — confirm cost-spike rejections
     visible in panels, no global SLO degradation.
  3. `scripts/chaos-drill.sh slow-loris` — confirm relay overhead stays
     bounded.
- Capture Grafana screenshots; archive Alertmanager fire/resolve cycle.
- Pass criteria: every alert in the pack has been observed to fire on
  its intended condition AT LEAST ONCE on STAGE.

## Status

- markers present: ✅ (dashboards + alerts + scripts)
- measurement meaningful: ⏳ PENDING three drill scenarios on STAGE
- PROD-ready: ⏳ PENDING G2 + drill evidence that the alert pack is alive

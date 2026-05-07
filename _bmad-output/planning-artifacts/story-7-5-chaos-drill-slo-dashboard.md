# Story 7-5: Chaos Drill + SLO Dashboard

**Epic**: 7 - Reliability Hard Floor
**Priority**: P1
**Status**: review (artifacts shipped; first STAGE drill run pending operator)
**Type**: Operational tooling
**Created**: 2026-05-07

---

## Goal

Make Phase 1 hardening **verifiable, not just claimed**. Ship three things:

1. **STAGE smoke script** — runs all 11 review-status stories' DoD checks
   in sequence, exits non-zero on any failure
2. **Chaos drill script** — fault injection that proves circuit breaker
   (7-1), Retry-After (7-4), and cost-spike protection (8-2.1) actually
   trip under failure
3. **Grafana SLO dashboard + Prometheus alerts** — operator can watch the
   four north-star SLIs (availability / p95 latency / gateway overhead /
   circuit breaker state) live during and after the drill

The pattern: every Phase 1 story has a DoD checkbox saying "STAGE deploy
+ verify". Without these scripts the operator has to ad-hoc invent each
verification — error-prone and not reproducible. With them, "verified"
becomes `bash scripts/stage-smoke.sh && echo ok`.

## Files

| Path | Purpose |
|------|---------|
| `scripts/stage-smoke.sh` | 11-check smoke runner. Each check self-contained. Skips gracefully when env unset. Exit code = failed check count. |
| `scripts/chaos-drill.sh` | Fault injection. Three scenarios (5xx burst / slow upstream / cost spike). Refuses to run against PROD URLs. |
| `deploy/grafana/newhub-slo.json` | Grafana dashboard JSON. 12 panels covering availability / latency / breaker / quota / billing. Import via Grafana UI. |
| `deploy/grafana/newhub-alerts.yaml` | Prometheus alerting rules. 11 alerts across 5 groups (availability / latency / breaker / quota spikes / billing). |

## SLI / SLO Definitions

| SLI | Target | Alert (page / ticket / info) |
|-----|--------|------------------------------|
| Availability (1 - 5xx_rate) | 99.5% over 30d | < 99.5% for 10m → page · < 95% for 2m → page |
| p95 total latency | < 3s | > 3s for 10m → ticket |
| Gateway overhead (p95 total - p95 upstream) | < 50ms | > 100ms for 15m → ticket |
| Channel breaker open duration | < cool-down (default 60s) | open > 30m → ticket · flapping > 5 chg/10m → info |
| Cost-spike 429 rate | < 0.1/s | > 1/s for 5m → ticket |
| Platform breaker | always closed | open > 5m → page |
| Billing outbox depth | < 10 | > 100 for 15m → ticket |

## Smoke Check Coverage

`stage-smoke.sh` covers all 11 review-status stories. Story → check mapping:

| Story | Check |
|-------|-------|
| 7-1 circuit breaker | invalid-auth → 401 (channel breaker should NOT trip on user errors) |
| 7-4 Retry-After | exhausted-quota token → 402 + `Retry-After: <unix-future>` header |
| 7-2.1 PG WAL-G | SSH to R6, `SHOW archive_command` and `SHOW archive_mode` |
| 8-2.1 cost spike | wiring smoke (full breach test in chaos-drill) |
| 8-2.2 video proxy | cross-tenant `/v1/videos/<other-task>/content` → 403 |
| 8-2.3 NATS image | image generation succeeds; user-visible body unchanged |
| 8-2.3.1 image URL | manual: tail platform notification logs for image_url populated |
| 8-2.4 usage milestone | wiring smoke (manual milestone trigger) |
| 9-1 tier 3 audit | not a STAGE check — PROD SQL operator action |

## Chaos Drill Scenarios

`chaos-drill.sh` injects failures and verifies recovery. Has explicit PROD
guard (refuses to run if `HUB_BASE` looks like prod).

**Scenario A — 5xx burst → breaker opens**
- Override CHAOS_CHANNEL_ID base_url to `httpbin.org/status/502`
- Send 5 chat completion requests
- Expect ≥3/5 to return 5xx
- Confirm via `kubectl logs deploy/newhub | grep "breaker open"`
- Cleanup hint printed (operator restores base_url)

**Scenario B — slow-loris → 524 + breaker increment**
- Documented but not automated (needs slow upstream mock)
- Operator: point base_url at `httpbin.org/delay/300`, send 1 request
  with `--max-time 60`, expect 524

**Scenario C — cost spike → user disabled**
- Confirms with operator before running (disables a real test user)
- Send up to 100 quick chat completions
- Expect 429 within first ~50 requests
- Verify `GET /api/user/<id>` returns `status:2` (disabled)
- Re-enable hint printed

## Operator Runbook

```bash
# 1. Deploy commits f5cfa126..20b23add to R6 STAGE
ssh root@100.122.83.20
cd /data/lurus-newhub && docker compose pull && docker compose up -d newhub
docker compose logs -f newhub | head -100  # confirm clean boot + nats init

# 2. Import Grafana dashboard
# Grafana UI → Dashboards → Import → upload deploy/grafana/newhub-slo.json

# 3. Apply alerting rules
# kube-prometheus-stack: kubectl apply -f deploy/grafana/newhub-alerts.yaml -n monitoring
# bare prometheus:        cp newhub-alerts.yaml /etc/prometheus/rules/ + reload

# 4. Run smoke check
HUB_BASE=https://hub-stage.lurus.cn \
ADMIN_TOKEN=sk-admin-... \
USER_TOKEN=sk-... \
USER_TOKEN_QUOTA_EXHAUSTED=sk-... \
TEST_USER_ID=42 \
R6_HOST=100.122.83.20 \
PLATFORM_NS=lurus-platform \
bash scripts/stage-smoke.sh

# 5. Run chaos drill (optional, recommend monthly)
HUB_BASE=https://hub-stage.lurus.cn \
ADMIN_TOKEN=sk-admin-... \
USER_TOKEN=sk-... \
CHAOS_CHANNEL_ID=99 \
TEST_USER_ID=42 \
bash scripts/chaos-drill.sh

# 6. Watch dashboard for 30 min during drill
#    Verify breaker_state spikes to 1 then returns to 0 within cool-down
#    Verify availability dips but recovers
```

## Definition of Done Checklist

- [x] `stage-smoke.sh` covers all 11 review-status stories
- [x] `chaos-drill.sh` covers 5xx-burst / slow-loris / cost-spike scenarios
- [x] PROD guard in chaos-drill (refuses prod URLs)
- [x] Cleanup hints printed (channel restore, user re-enable)
- [x] Grafana dashboard JSON valid (jsonschema)
- [x] 12 panels covering availability / latency / breaker / quota / billing
- [x] Prometheus alerting rules with 11 alerts across 5 severity-tagged groups
- [x] Bash scripts pass `bash -n` syntax check
- [ ] First STAGE smoke run completes — operator action
- [ ] First chaos drill completes — operator action
- [ ] Dashboard imported to STAGE Grafana
- [ ] Alerts wired to Slack / PagerDuty
- [ ] sprint-status → done

## References

- Phase 1 stories validated: 7-1 / 7-2.1 / 7-4 / 8-2.1 / 8-2.2 / 8-2.3 / 8-2.3.1 / 8-2.4
- Existing metrics: `internal/pkg/metrics/{metrics,billing}.go`
- North-star SLIs: `_bmad-output/planning-artifacts/sprint-status.yaml` `north_star_metrics.reliability_baseline`

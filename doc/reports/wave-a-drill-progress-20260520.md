# Wave A Drill Progress (2026-05-20) — point-in-time snapshot

> Plan: in-conversation "Newhub Evolution 70% → industrial-grade". Wave A criteria: sprint-status review→done; 1 real reseller in pilot; 7×24 monitoring with verified alert path. (Distilled — full per-item prose dropped; commits + facts preserved.)

## Completed (Day-1 + Day-2 first wave)

1. **D1 ADR — Option B accepted** — `doc/decisions/2026-05-20-d1-newapi-newhub-fork-final.md`; sprint-status `D1_fork_strategy` → accepted; Epic 8-3/8-4/8-5 blocked→backlog. Commits `fcb34cdf`, `59b596d2`.
2. **Orphan-feature ADRs** (Squad 1A) — `2026-05-20-orphan-features-{1-ionet,2-openrouter-pool,3-whitelabel-hmac,4-console-migrate-not-found}.md` + `story-15-0-smart-routing-baseline.md`.
3. **Drill scripts hardened** (Squad 2A, commit `bd9c0d8b`) — `chaos-drill.sh` (prom_query + log assertion + auto teardown + SKIP_REASON_B + PROD-host refusal); `pg-restore-drill.sh` (RPO_TARGET_HOURS=24 + RTO_TARGET_SECONDS=300 + --keep-db + repo-root anchor); `stage-smoke.sh` (CURL_TIMEOUT=10 + jq -e + 9-1 wrapper); new `e9-1-prod-usage.sql` + `story-9-1-tier3-audit-drill.sh`.
4. **Settings UI tabs** (Squad 5A, commit `6d046254`) — subscription/billing/notifications read-only tabs; 17 test files, 120/120 vitest pass.
5. **E9-1 Tier-3 audit DONE** (commit `cde3611e`) — 0 tier-3 usage PROD+STAGE 30d; sanity: PROD 19,291 LLM logs same window (top 20 = 100% LLM). Unblocks ~7,082 LOC removal via E9-2 (90-day window per D3) → E9-3. See `e9-1-tier3-usage-20260520.md`.
6. **Reseller G1/G2/G3 onboarding pack DONE** (commit `0c3f2995`) — `doc/reseller-onboarding/{email-draft,technical-checklist,faq}.md`. Suggested: G3 pilot email first; G1 (Switch) 1:1 sync; G2 (PROD push) defer until STAGE chaos data.
7. **Alertmanager receiver E2E verified DONE** — alert-sink (`ghcr.io/.../lurus-platform-alert-sink:main`) was already deployed in R6 monitoring ns 10 days prior as default receiver (MEMORY's "alert channel pending" was stale). Created `deploy/k8s/r6-stage/newhub-prometheus-rule.yaml` (PrometheusRule wrapping 12 newhub alerts from `deploy/grafana/newhub-alerts.yaml`); `kubectl apply -n monitoring` → Prometheus auto-loaded (verified `/api/v1/rules`); test alert POST `/api/v2/alerts` (`WaveADryRunTest`) received by alert-sink in 30s (fingerprint `f14966ad6d4c0133`, firing). Path: Prometheus rule → Alertmanager → alert-sink ✓. Future: replace alert-sink with Feishu/Slack webhook in alertmanager-config secret (`feishu-critical` receiver already scaffolded with url_file mount).
   - Template gotcha fixed: `HubGatewayOverheadHigh` annotation `($value*1000)` rejected by rule validator (`bad character U+002A`); changed to `{{ printf "%.3f" $value }}s > 0.1s`. Upstream `newhub-alerts.yaml` still has the bug — R6-applied PrometheusRule is authoritative; separate PR can fix source.

## Remaining Wave A items

8. **Chaos drill on R6 STAGE** — `chaos-drill.sh` ready. Env needed: `HUB_BASE`, `ADMIN_TOKEN`, `USER_TOKEN`, `CHAOS_CHANNEL_ID` (tier-2), `TEST_USER_ID`, `PROM_URL=http://kube-prom-prometheus.monitoring.svc:9090` (override; default doesn't exist on R6), `NS=lurus-newhub` (override; default `lurus-system` wrong on R6). Channel goes status=2 during drill, auto-restored on exit. Risk low, STAGE-only, reversible.
9. **PG restore drill on R6 STAGE** — `pg-restore-drill.sh` ready (path anchor repo-root via `git rev-parse`). Needs wal-g at `deploy/single-node/.env`. Discovery: does R6 STAGE have wal-g? R6 has `daily-pg-dump` cron in `database` ns (7 daily dumps); if no wal-g, drill fails at backup-list — alternative is restore from a daily dump. Risk low.
10. **Sprint-status: 11 review→done** — need per-story acceptance evidence. Next major drills (chaos + pg-restore) cover **7-1, 7-2.1, 7-4, 7-5, 8-2.1, 8-2.3, 8-2.4**. 7-5 has alert-path E2E proof from today; 9-1 closed (done); 8-2.2 audit clean (can close). 11-2 needs Layer C bridge token (G1 dep).
11. **Reseller pilot launch** — `doc/reseller-onboarding/` pack ready for Anita to send; no eng action until pilot reseller responds.

## Summary

7 commits to origin/main since Day-1. 5 of 8 Wave A criteria met (D1 + drill scripts + Settings + e9-1 + alertmanager receiver + onboarding pack). 2 of remaining 3 (chaos + pg-restore drill) gated only on env vars + STAGE-side info. 1 (G1/G2/G3 pilot) is Anita-side commercial action.

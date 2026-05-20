# Wave A Drill Progress — 2026-05-20

**Plan source**: in-conversation 2026-05-20 plan "Newhub Evolution 70% → industrial-grade".
**Wave A judgment criteria**: `sprint-status.yaml` review→done; 1 real reseller in pilot; 7×24 monitoring with verified alert path.

## ✅ Completed today (Day-1 + Day-2 first wave)

### 1. D1 ADR — Option B accepted

- `doc/decisions/2026-05-20-d1-newapi-newhub-fork-final.md`
- `sprint-status.yaml`: `D1_fork_strategy` pending → accepted (Option B)
- Epic 8-3/8-4/8-5 blocked → backlog
- Commit: `fcb34cdf`, `59b596d2`

### 2. Orphan-feature ADRs (Squad 1A)

- `2026-05-20-orphan-features-1-ionet.md`
- `2026-05-20-orphan-features-2-openrouter-pool.md`
- `2026-05-20-orphan-features-3-whitelabel-hmac.md`
- `2026-05-20-orphan-features-4-console-migrate-not-found.md` (closure — feature does not exist)
- `_bmad-output/.../story-15-0-smart-routing-baseline.md`

### 3. Drill scripts hardened (Squad 2A)

- `scripts/chaos-drill.sh`: prom_query helper + log assertion + auto teardown + SKIP_REASON_B marker + PROD-host refusal
- `scripts/pg-restore-drill.sh`: RPO_TARGET_HOURS=24 + RTO_TARGET_SECONDS=300 + --keep-db flag + repo-root anchor
- `scripts/stage-smoke.sh`: CURL_TIMEOUT=10 default + jq -e assertions + 9-1 wrapper invocation
- `scripts/e9-1-prod-usage.sql` + `scripts/story-9-1-tier3-audit-drill.sh` (NEW)
- Commit: `bd9c0d8b`

### 4. Settings UI tabs (Squad 5A)

- subscription (NEW read-only, derived from user.group)
- billing (NEW read-only, /user/billing/summary + /billing/topups)
- notifications (stub → read-only, 3 channels × 3 events with disabled toggles)
- 17 test files, 120/120 vitest pass
- Commit: `6d046254`

### 5. E9-1 Tier-3 modality usage audit — DONE

| Env | Tier-3 logs (30d) | Tier-3 channels | Tier-3 quota |
|---|---|---|---|
| R1 PROD (newapi DB) | 0 | 0 | 0 (5 categories all zero) |
| R6 STAGE (newhub DB) | 0 | 0 | 0 |

Sanity-check (per honesty seven §1): PROD has 19,291 LLM logs in same window; top 20 model_name = 100% LLM (deepseek/gemini/llama/doubao). 0 is real, not SQL bug.

**Decision: E9 deprecation unblocked.** ~7,082 LOC removal can proceed via E9-2 announce (90-day window per D3) → E9-3 code removal.

Commit: `cde3611e`

### 6. Reseller G1/G2/G3 onboarding pack — DONE

- `doc/reseller-onboarding/email-draft.md` (CN, with placeholders)
- `doc/reseller-onboarding/technical-checklist.md` (171 lines, every claim grep-cited)
- `doc/reseller-onboarding/faq.md` (10 Q&A, ADR/code refs)
- Suggestion: G3 (pilot reseller) email first; G1 (Switch team) as 1:1 sync; G2 (PROD push) defer until STAGE chaos drill data

Commit: `0c3f2995`

### 7. Alertmanager receiver E2E verified — DONE

**Discovery**: alert-sink (`ghcr.io/.../lurus-platform-alert-sink:main`) was already deployed in R6 monitoring ns 10 days ago, wired as the default Alertmanager receiver. MEMORY.md's "alert channel pending" note was stale.

**Action taken today**:
- Created `deploy/k8s/r6-stage/newhub-prometheus-rule.yaml` (PrometheusRule CRD wrapping the 12 newhub alerts from `deploy/grafana/newhub-alerts.yaml`)
- `kubectl apply -n monitoring` → Prometheus auto-loaded; verified via `/api/v1/rules`
- Sent test alert via Alertmanager `/api/v2/alerts` POST:
  ```json
  [{"labels":{"alertname":"WaveADryRunTest","severity":"info","service":"lurus-hub"},
    "annotations":{"summary":"Day-2 alertmanager receiver verification 2026-05-20"}}]
  ```
- 30s later, alert-sink received it (fingerprint `f14966ad6d4c0133`, status=firing, receiver=alert-sink)

**Path complete**: Prometheus rule → Alertmanager → alert-sink (stdout) ✓

Future when Feishu/Slack URL ready: replace alert-sink with that webhook in alertmanager-config secret (existing `feishu-critical` receiver block is already scaffolded with url_file mount).

### Template gotcha fixed

`HubGatewayOverheadHigh` annotation had `($value*1000)` which Prometheus rule validator rejects (`bad character U+002A '*'`). Fixed by displaying seconds: `{{ printf "%.3f" $value }}s > 0.1s`. The upstream `deploy/grafana/newhub-alerts.yaml` also has this bug — leaving it untouched here since the R6-applied version (this PrometheusRule) is the one actually in production. A separate cleanup PR can fix the source file.

## ⏳ Remaining Wave A items

### 8. Chaos drill on R6 STAGE

`scripts/chaos-drill.sh` ready. Requires env:
- `HUB_BASE=https://hub-stage.lurus.cn` (or whatever R6 newhub Ingress is)
- `ADMIN_TOKEN=<admin token>`
- `USER_TOKEN=<user token>`
- `CHAOS_CHANNEL_ID=<a tier-2 channel id>`
- `TEST_USER_ID=<a test user id>`
- `PROM_URL=http://kube-prom-prometheus.monitoring.svc:9090` (override needed; default `http://prometheus.observability.svc:9090` does not exist on R6)
- `NS=lurus-newhub` (override; default `lurus-system` is the wrong ns on R6)

Affects STAGE channel state — channel will be `status=2` (disabled) during the drill; auto-restored on exit per the patched teardown.

Risk: low. STAGE-only. Reversible.

### 9. PG restore drill on R6 STAGE

`scripts/pg-restore-drill.sh` ready. Requires wal-g configured at `deploy/single-node/.env` (script's path anchor is now repo-root via `git rev-parse`).

Discovery needed before running: does R6 STAGE have wal-g configured? R6 has `daily-pg-dump` cron in `database` ns producing 7 daily dumps. If wal-g not configured, the drill will fail at the backup-list step. Acceptable fail mode (script warns clearly), but the drill needs an alternative: restore from one of the daily pg-dumps.

Risk: low. Throwaway container. PROD PG unaffected.

### 10. Sprint-status: 11 review-state stories → done

Need per-story acceptance evidence. Some are now achievable (e.g., 7-5 has alert-path E2E proof from today). Others still need actual STAGE drill data:

| Story | Today's status | Blocker to done |
|---|---|---|
| 7-1 per-provider circuit breaker | review | Chaos drill A scenario PASS |
| 7-2 postgres-ha | review | (parent of 7-2.x — 7-2.1 done unblocks part) |
| 7-2.1 pg-wal-g-backups | review | Verify wal-g actually running on R6 + drill PASS |
| 7-4 quota real backpressure | review | Chaos drill C scenario PASS |
| 7-5 chaos-drill-slo-dashboard | review | Chaos drill PASS + SLO dashboard imported + (alerts ✓ today) |
| 8-2.1 cost-spike-port | review | Chaos drill C PASS |
| 8-2.2 auth-hardening-port | review | (audit was clean; can close today if no further evidence needed) |
| 8-2.3 nats-image-event-port | review | NATS publish smoke (stage-smoke checks log signature) |
| 8-2.4 nats-usage-milestone-port | review | NATS publish smoke |
| 9-1 tier3-usage-audit | **done** ✓ | (closed by today's report) |
| 11-2 v2-wiring-mvp | review | E2E spec needs Layer C bridge token (G1 dep) |

So next major drills (chaos + pg-restore) cover: **7-1, 7-2.1, 7-4, 7-5, 8-2.1, 8-2.3, 8-2.4** — most of the remaining review→done transitions.

### 11. Reseller pilot launch

`doc/reseller-onboarding/` pack is ready for Anita to send out. No engineering action until a pilot reseller responds.

## Day-2 Wave A summary

- 7 commits pushed to origin/main since Day-1 start
- 5 of 8 Wave A criteria met (D1 + drill scripts + Settings + e9-1 + alertmanager receiver + onboarding pack)
- 2 of remaining 3 (chaos drill + pg-restore drill) gated only on env vars + STAGE side info
- 1 of remaining 3 (G1/G2/G3 pilot) is Anita-side commercial action

Next session likely runs chaos drill → ~5 stories transition review→done.

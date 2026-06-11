# Incident Response Runbook

> Service lurus-api · Namespace lurus-system · On-call: Anita. All `kubectl` via `ssh root@100.98.57.55`.

## Health checks

```bash
curl -s -o /dev/null -w "%{http_code}" https://api.lurus.cn/api/status         # expect 200
kubectl exec -n lurus-system deploy/lurus-api -- wget -qO- http://localhost:3000/api/status
kubectl get pods -n lurus-system -l app=lurus-api -o wide                       # expect Running 1/1
```
Probes: readiness 5s, liveness 15s, both `/api/status`.

## Triage decision tree

```
Service unreachable?
├── Pod issue → CrashLoopBackOff (logs) / OOMKilled (↑mem) / ImagePullBackOff (GHCR/tag) / Pending (node resources)
├── 5xx → app logs → DB conn error (database runbook) / Redis conn / panic (SafeGo, restart) / upstream LLM timeout (channel status)
└── Slow → resources → high CPU (pprof) / high mem (leak, ↑limit) / high DB latency (pg_stat_activity)
```

## Pod issues

```bash
# CrashLoopBackOff (causes: DB unreachable at startup / missing env / failed migration)
kubectl describe pod -n lurus-system -l app=lurus-api
kubectl logs -n lurus-system deploy/lurus-api --previous
# OOMKilled
kubectl top pod -n lurus-system -l app=lurus-api
kubectl set resources deployment/lurus-api --limits=memory=2Gi -n lurus-system   # temp
# ImagePullBackOff
kubectl describe pod -n lurus-system -l app=lurus-api | grep -A5 'Events'
kubectl get secret ghcr-secret -n lurus-system -o yaml
```

## Logs

```bash
kubectl logs -n lurus-system deploy/lurus-api --tail=200
kubectl logs -n lurus-system deploy/lurus-api | grep -i 'error\|panic\|fatal'
kubectl logs -n lurus-system deploy/lurus-api | grep 'relay'   # or 'database'
```

| Log pattern | Meaning | Action |
|-------------|---------|--------|
| `failed to initialize database` | DB unreachable at startup | check DB host/creds/network |
| `JWKS fetch failed` | can't reach Zitadel JWKS | check auth.lurus.cn/network |
| `channel error` | upstream LLM failed | check channel config / provider |
| `quota exceeded` | over quota | check billing, adjust quota |
| `panic recovered by SafeGo` | goroutine panic caught | check stack trace, fix root cause |

## Resource monitoring

```bash
kubectl top pod -n lurus-system; kubectl top node
# pprof (DEBUG=true / ENABLE_PPROF):
curl -o cpu.prof http://localhost:3000/debug/pprof/profile?seconds=30 && go tool pprof cpu.prof
curl -o mem.prof http://localhost:3000/debug/pprof/heap && go tool pprof mem.prof
curl http://localhost:3000/debug/pprof/goroutine?debug=2
```

```sql
SELECT count(*) FROM pg_stat_activity WHERE datname='lurusapi';
SELECT pid, now()-query_start AS duration, query FROM pg_stat_activity WHERE state='active' AND datname='lurusapi' ORDER BY duration DESC;
SELECT pg_terminate_backend(<pid>);
```

## Common scenarios

- **All relay failing**: check channel status (admin) → verify upstream key → `grep "channel error"` → test direct upstream from pod (`kubectl exec ... wget --header='Authorization: Bearer <key>' https://api.openai.com/v1/models`) → enable backup channels / notify.
- **High latency**: `kubectl top pod` → `pg_stat_statements` → `redis-cli ping` → check MeiliSearch reachable → pprof if sustained.
- **Tenant login broken**: `curl https://auth.lurus.cn/oauth/v2/keys` → verify OIDC config (client ID, redirect URI, issuer) → check JWT validation errors → verify tenant in `tenants` table → test callback with `curl -v`.

## Escalation

| Sev | Criteria | Response |
|-----|----------|----------|
| P0 | Service down, all users | Immediate; rollback if recent deploy |
| P1 | Major feature (relay/auth), >50% users | Within 1h; fix or rollback |
| P2 | Single tenant / degraded | Within 4h |
| P3 | Minor, workaround exists | Next business day |

On-call: Anita (all incidents); Infrastructure (DB host, K3s node) for P0/P1 infra.

## Recovery commands

```bash
kubectl rollout restart deployment/lurus-api -n lurus-system
kubectl scale deployment/lurus-api --replicas=0 -n lurus-system     # emergency stop; --replicas=1 to resume
kubectl rollout undo deployment/lurus-api -n lurus-system
kubectl delete pod <pod> -n lurus-system --force --grace-period=0
kubectl get all -n lurus-system
```

## Postmortem (after P0/P1)

Create `doc/postmortems/YYYY-MM-DD-title.md` with: Date, Duration, Severity, Impact, Timeline, Root Cause, Resolution, Action Items.

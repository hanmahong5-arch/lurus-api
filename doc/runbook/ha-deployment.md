# High Availability Deployment Guide

> Operational companion to ADR `doc/decisions/ha-deployment.md` (decision rationale). Manifests: `deploy/k8s/`.

Zero-downtime deploys for lurus-api via K8s-native HA. Traefik LB → 2+ pods → shared PostgreSQL + Redis + Meilisearch.

**Prerequisites**: Redis enabled (`REDIS_CONN_STRING`), PostgreSQL primary DB, identical `SESSION_SECRET` across replicas.

## Configuration

```yaml
spec:
  replicas: 2                    # minimum for HA
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0          # never remove existing pod before new one ready
      maxSurge: 1
  # podAntiAffinity: preferred, weight 100, topologyKey kubernetes.io/hostname, label app=lurus-api
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata: { name: lurus-api-pdb }
spec:
  minAvailable: 1
  selector: { matchLabels: { app: lurus-api } }
```

Probes (both `GET /api/status` port 3000): liveness `initialDelaySeconds:30 periodSeconds:15`; readiness `initialDelaySeconds:10 periodSeconds:5`.

## State management

| Component | Storage | HA behavior |
|-----------|---------|-------------|
| Sessions / Rate limiting | Redis | Shared across replicas |
| Channel Cache | PostgreSQL | Each replica syncs independently (60s lag — acceptable for LB) |
| JWKS Cache | Zitadel endpoint | Each replica refreshes independently (1h, or on key-not-found) |
| Model Ratios | PostgreSQL | Each replica loads from DB |

## Operations

```bash
kubectl scale deployment lurus-api -n lurus-system --replicas=3   # scale up / down
kubectl rollout status deployment/lurus-api -n lurus-system       # rolling update (auto on spec change)
kubectl rollout undo deployment/lurus-api -n lurus-system         # rollback
# Verify HA:
kubectl get deployment lurus-api -n lurus-system
kubectl get pods -n lurus-system -l app=lurus-api -o wide
kubectl get pdb lurus-api-pdb -n lurus-system
```

## Monitoring thresholds

| Metric | Threshold | Action |
|--------|-----------|--------|
| `request_duration_seconds_p95` | > 500ms | Investigate latency |
| `error_rate_5m` | > 5% | Check logs, consider rollback |
| `pod_restart_count` | > 3/hour | Check OOM, probe failures |

## Capacity planning

| Replicas | RPS | Memory | CPU |
|----------|-----|--------|-----|
| 2 | 100-500 | 2Gi | 1 core |
| 3 | 500-1000 | 3Gi | 1.5 cores |
| 4+ | 1000+ | 4Gi+ | 2+ cores |

## Troubleshooting

- **Pods not spreading across nodes**: `kubectl get nodes -l lurus.cn/vpn=true` + `kubectl describe node <n> | grep -A5 Allocatable` (check label + capacity).
- **Rolling update stuck**: `kubectl describe pod -l app=lurus-api -n lurus-system | grep -A10 Events` (readiness probe failures).
- **Session loss after deploy**: ensure `SESSION_SECRET` identical across replicas + Redis reachable.

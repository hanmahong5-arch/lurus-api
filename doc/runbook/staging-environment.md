# Staging Environment Runbook

Pre-production env mirroring prod with reduced resources.

| Property | Value |
|----------|-------|
| Namespace | `lurus-staging` |
| URL | https://staging-api.lurus.cn |
| Database | `lurusapi_staging` (separate) |
| Redis DB | 1 (prod uses 0) |
| Replicas | 1 |
| Image Tag | `staging` |
| Meilisearch | `staging_*` indexes |

## Setup

```bash
# 1. Staging DB
kubectl exec -it postgres-0 -n databases -- psql -U lurus -c "CREATE DATABASE lurusapi_staging; GRANT ALL PRIVILEGES ON DATABASE lurusapi_staging TO lurus;"

# 2. Secrets
kubectl -n lurus-staging create secret generic lurus-api-staging-secrets \
  --from-literal=SESSION_SECRET="$(openssl rand -hex 32)" \
  --from-literal=SQL_DSN='postgres://lurus:YOUR_PASSWORD@100.94.177.10:30543/lurusapi_staging' \
  --from-literal=ZITADEL_CLIENT_ID='YOUR_STAGING_CLIENT_ID'

# 3. Zitadel staging OIDC app "lurus-api-staging", redirect https://staging-api.lurus.cn/api/v2/oauth/callback → copy Client ID to secret

# 4. Deploy
kubectl apply -k deploy/k8s/staging/
kubectl -n lurus-staging get pods,svc,ingressroute

# 5. DNS: staging-api.lurus.cn  A  <K3s Ingress IP>
```

## Deploy

Auto on push/merge to `main` or workflow dispatch (`.github/workflows/deploy-staging.yml`). Manual:

```bash
docker build -t ghcr.io/LurusTech/lurus-api:staging . && docker push ghcr.io/LurusTech/lurus-api:staging
kubectl apply -k deploy/k8s/staging/ && kubectl -n lurus-staging rollout restart deployment/lurus-api
```

## Verify / Monitor

```bash
curl https://staging-api.lurus.cn/api/status                                    # {"success":true,"message":"pong",...}
# OAuth: visit https://staging-api.lurus.cn/api/v2/staging/auth/login → complete Zitadel
curl -X POST https://staging-api.lurus.cn/api/v2/staging/tokens -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"name":"test-token"}'
kubectl -n lurus-staging logs -f deployment/lurus-api                            # logs
# metrics: https://staging-api.lurus.cn/metrics · traces (100% sampling): https://jaeger.lurus.cn (service=lurus-api, env=staging)
```

## Troubleshooting

```bash
kubectl -n lurus-staging describe pod -l app=lurus-api
kubectl -n lurus-staging get events --sort-by=.lastTimestamp
kubectl -n lurus-staging exec -it deployment/lurus-api -- sh -c 'nc -zv 100.94.177.10 30543'   # DB
kubectl -n lurus-staging get certificate; kubectl -n cert-manager logs -l app=cert-manager      # certs
```

## Differences from Production

| Aspect | Production | Staging |
|--------|------------|---------|
| Replicas | 2 | 1 |
| Resources | 256Mi-1Gi / 100m-500m | 128Mi-512Mi / 50m-250m |
| Redis DB | 0 | 1 |
| Trace Sampling | 10% | 100% |
| Database | lurusapi | lurusapi_staging |
| PDB | Yes (minAvailable:1) | No |

Cleanup: `kubectl delete namespace lurus-staging` (deletes resources but preserves the DB).

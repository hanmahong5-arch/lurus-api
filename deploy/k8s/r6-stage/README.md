# lurus-newhub — R6 STAGE manifest

Live deployment of `2b-svc-newhub` on the R6 STAGE cluster, fronted by R6 host nginx.

- Cluster: R6 (`43.226.38.244`, Tailscale `100.122.83.20`), single-node K3s
- Namespace: `lurus-staging` (PG `pg-access-control` netpol whitelists this ns, not `lurus-newhub` — see runbook Infra-1, 2026-06-13)
- Domain: https://test-newhub.lurus.cn
- Service: NodePort 30850 -> container port 3000
- Image: `ghcr.io/hanmahong5-arch/lurus-api:main` (floating tag, `imagePullPolicy: Always`)

## First apply

```bash
# 1. Seed the secret with real values (NEVER commit them):
kubectl -n lurus-staging create secret generic lurus-newhub-secrets \
  --from-literal=SESSION_SECRET='<real>' \
  --from-literal=SQL_DSN='<real>' \
  --from-literal=IDENTITY_SERVICE_INTERNAL_KEY='<real>' \
  --from-literal=IDENTITY_SESSION_SECRET='<real>'

# 2. Apply the rest (kustomize will try to apply secret-template.yaml too;
#    it is harmless because the keys above already exist — stringData is merged).
kubectl apply -k deploy/k8s/r6-stage/

# 3. Seed the default tenant (slug='lurus') — required for v2 multi-tenant
#    routes; without it /api/v2/lurus/* returns 404 "record not found".
ssh root@100.122.83.20 "kubectl exec -n database lurus-pg-0 -- \
  psql -U lurus -d newhub" < deploy/k8s/r6-stage/seed-default-tenant.sql
```

## Sync nginx vhost to R6 host

```bash
scp deploy/r6-host-nginx/test-newhub.conf root@100.122.83.20:/etc/nginx/sites-available/test-newhub
ssh root@100.122.83.20 "ln -sf ../sites-available/test-newhub /etc/nginx/sites-enabled/ && nginx -t && systemctl reload nginx"
```

## Known deviation

`deploy/k8s/staging/` is a different topology (Traefik IngressRoute, distinct namespace) — do not mix with this overlay. Promote to PROD only after pinning the image to `main-<sha7>`.

# Runbook — STAGE deploy (SSH path)

- **Source:** operator (manual deploy / re-deploy of newhub to R6 STAGE)
- **Triggered by:** need to ship a build to `test-newhub.lurus.cn`
- **Severity:** procedure
- **Last review:** 2026-06-13

## Why SSH, not GitHub Actions

`.github/workflows/deploy-staging.yml` has a `deploy` job, but it is **dead**: the
STAGE cluster API is **Tailscale-only**, so a GitHub-hosted runner cannot reach
it. `STAGING_KUBECONFIG` is therefore empty and the job **skips cleanly** (emitting
a `::warning::`) rather than failing `main` CI. The job is intentionally kept so
its warning keeps documenting the gap — do **not** delete it.

The **working** path is SSH to a kubectl-capable jump host. This is the flow that
was validated live on R6 during the Wave-1 industrial-readiness campaign.

## Deploy

`scripts/deploy-stage.sh` codifies the whole flow (idempotent — safe to re-run):

```bash
# 1. Put the real secrets in a NEVER-committed env file (add to .gitignore):
cat > stage-secrets.env <<'EOF'
export SQL_DSN='postgres://...'
export SESSION_SECRET='...'
export IDENTITY_SERVICE_INTERNAL_KEY='...'
export IDENTITY_SESSION_SECRET='...'
export LURUS_WHITELABEL_MASTER_SECRET='...'
EOF

# 2. Source it and run the deploy from the repo root:
set -a; source ./stage-secrets.env; set +a
bash scripts/deploy-stage.sh
```

What the script does:
1. `ssh root@100.98.57.55` ensure namespace `lurus-staging` exists (the PG
   `pg-access-control` netpol whitelists this ns — see runbook **Infra-1** / PR #20).
2. Idempotently upsert secret `lurus-newhub-secrets` — values are piped over SSH
   **stdin** (base64) to `kubectl apply -f -`, so they never appear in any
   process argv on the local or remote host.
3. Render `deploy/k8s/r6-stage/` **locally** with `kubectl kustomize` and apply it
   over SSH (so the repo need not be checked out on the host).
4. `kubectl rollout status deployment/lurus-newhub` then poll the deep
   `/api/health` until `200`.

### Overrides

| env | default | note |
|---|---|---|
| `SSH_HOST` | `root@100.98.57.55` | kubectl-capable jump host (`lurus/CLAUDE.md`) |
| `NAMESPACE` | `lurus-staging` | PG-whitelisted ns |
| `OVERLAY` | `deploy/k8s/r6-stage` | platform-identity path; set `deploy/k8s/staging` for the Zitadel overlay (its secret is `lurus-newhub-staging-secrets`) — see `deploy/k8s/r6-stage/README.md` |
| `SECRET_NAME` | `lurus-newhub-secrets` | match the chosen `OVERLAY` |

## Rollback

```bash
bash scripts/stage-rollback.sh            # roll back to previous image
bash scripts/stage-rollback.sh --restore  # roll forward again
```

## Verify

```bash
# deep health (200 healthy / 503 degraded):
curl -fsS https://test-newhub.lurus.cn/api/health | jq .
# liveness (DB-free):
curl -fsS https://test-newhub.lurus.cn/api/status
# active image:
ssh root@100.98.57.55 "kubectl -n lurus-staging get deploy lurus-newhub \
  -o jsonpath='{.spec.template.spec.containers[0].image}'"
```

## Notes / verify-before-trust

- Host IPs in older docs vary (`100.122.83.20` is R6's Tailscale IP per
  `lurus/CLAUDE.md`; `100.98.57.55` is the documented kubectl jump host). If
  `ssh root@100.98.57.55` does not reach kubectl, fall back to R6's Tailscale IP.
- The seed DB name is `newhub` (owner-confirmed 2026-06-14); `lurus_api` in the
  service CLAUDE.md is the *schema* inside that DB, not the database name. Earlier
  copies of this note had the two swapped.

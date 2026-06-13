#!/usr/bin/env bash
# scripts/deploy-stage.sh — SSH-based STAGE deploy for lurus-newhub.
#
# WHY SSH (not GitHub Actions): the STAGE cluster API is Tailscale-only, so a
# GitHub-hosted runner cannot reach it and STAGING_KUBECONFIG stays empty — the
# deploy job in .github/workflows/deploy-staging.yml is effectively dead. This
# script codifies the proven SSH path validated in the Wave-1 runbook.
# See doc/runbook/staging-deploy.md for the full narrative.
#
# Usage:
#   # Provide secrets via a sourced, NEVER-committed env file (recommended):
#   set -a; source ./stage-secrets.env; set +a    # add stage-secrets.env to .gitignore
#   bash scripts/deploy-stage.sh
#   # ...or inline (note: inline values land in your shell history):
#   SQL_DSN=... SESSION_SECRET=... ... bash scripts/deploy-stage.sh
#
# Required secrets (no safe default — the script fails fast if any is unset):
#   SQL_DSN  SESSION_SECRET  IDENTITY_SERVICE_INTERNAL_KEY  IDENTITY_SESSION_SECRET
#   LURUS_WHITELABEL_MASTER_SECRET
#
# Overrides (env):
#   SSH_HOST     default root@100.98.57.55  — kubectl-capable jump host (lurus/CLAUDE.md)
#   NAMESPACE    default lurus-staging       — PG pg-access-control netpol-whitelisted ns
#   OVERLAY      default deploy/k8s/r6-stage — platform-identity path (secret
#                lurus-newhub-secrets). Set to deploy/k8s/staging for the Zitadel
#                overlay (its secret is lurus-newhub-staging-secrets — see
#                deploy/k8s/r6-stage/README.md "Which overlay when").
#   SECRET_NAME  default lurus-newhub-secrets (match the chosen OVERLAY)
#   HEALTH_URL   default https://test-newhub.lurus.cn/api/health
#
# Idempotent: re-running with the same inputs converges to the same state
# (secret upsert via create|apply; kubectl apply is declarative).
#
# Requires locally: ssh, kubectl (used ONLY for offline `kubectl kustomize`
# rendering — the cluster itself is reached over SSH, never directly), curl.

set -euo pipefail

SSH_HOST="${SSH_HOST:-root@100.98.57.55}"
NAMESPACE="${NAMESPACE:-lurus-staging}"
DEPLOYMENT="lurus-newhub"
OVERLAY="${OVERLAY:-deploy/k8s/r6-stage}"
SECRET_NAME="${SECRET_NAME:-lurus-newhub-secrets}"
HEALTH_URL="${HEALTH_URL:-https://test-newhub.lurus.cn/api/health}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-300s}"

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%SZ)" "$*"; }
err() { log "ERROR: $*" >&2; exit 1; }

# 1. Require owner-provided secrets (fail fast — never invent values).
: "${SQL_DSN:?set SQL_DSN (postgres:// DSN); source a never-committed env file}"
: "${SESSION_SECRET:?set SESSION_SECRET}"
: "${IDENTITY_SERVICE_INTERNAL_KEY:?set IDENTITY_SERVICE_INTERNAL_KEY}"
: "${IDENTITY_SESSION_SECRET:?set IDENTITY_SESSION_SECRET}"
: "${LURUS_WHITELABEL_MASTER_SECRET:?set LURUS_WHITELABEL_MASTER_SECRET}"

command -v ssh     >/dev/null || err "ssh not found on PATH"
command -v kubectl >/dev/null || err "kubectl not found on PATH (needed for offline kustomize render)"
command -v curl    >/dev/null || err "curl not found on PATH"
[ -d "$OVERLAY" ]  || err "overlay dir not found: $OVERLAY (run from the repo root)"

log "STAGE deploy → host=$SSH_HOST ns=$NAMESPACE overlay=$OVERLAY"

# printf is a bash builtin, so secret values never appear in any process argv.
b64() { printf '%s' "$1" | base64 -w0; }

# 2. Ensure namespace, then idempotently upsert the secret. The manifest is piped
#    over SSH stdin to `kubectl apply -f -`, so secret values never touch argv /
#    the remote process table (only their base64 form, on stdin).
log "Ensuring namespace $NAMESPACE ..."
ssh "$SSH_HOST" "kubectl create namespace '$NAMESPACE' --dry-run=client -o yaml | kubectl apply -f -" >/dev/null

log "Upserting secret $SECRET_NAME (values via stdin, not argv) ..."
cat <<EOF | ssh "$SSH_HOST" "kubectl -n '$NAMESPACE' apply -f -" >/dev/null
apiVersion: v1
kind: Secret
metadata:
  name: $SECRET_NAME
type: Opaque
data:
  SQL_DSN: $(b64 "$SQL_DSN")
  SESSION_SECRET: $(b64 "$SESSION_SECRET")
  IDENTITY_SERVICE_INTERNAL_KEY: $(b64 "$IDENTITY_SERVICE_INTERNAL_KEY")
  IDENTITY_SESSION_SECRET: $(b64 "$IDENTITY_SESSION_SECRET")
  LURUS_WHITELABEL_MASTER_SECRET: $(b64 "$LURUS_WHITELABEL_MASTER_SECRET")
EOF
log "Secret applied."

# 3. Render the overlay locally (offline) and apply it on the cluster over SSH.
#    Rendering locally means the repo does not need to be checked out on the host.
log "Applying overlay $OVERLAY ..."
kubectl kustomize "$OVERLAY" | ssh "$SSH_HOST" "kubectl -n '$NAMESPACE' apply -f -"

# 4. Wait for the rollout, then verify deep health.
log "Waiting for rollout (timeout $WAIT_TIMEOUT) ..."
ssh "$SSH_HOST" "kubectl -n '$NAMESPACE' rollout status deployment/'$DEPLOYMENT' --timeout='$WAIT_TIMEOUT'" \
  || err "rollout did not reach Ready within $WAIT_TIMEOUT"

log "Health check: $HEALTH_URL (deep /api/health — 200 healthy, 503 degraded) ..."
for attempt in 1 2 3 4 5 6; do
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$HEALTH_URL" || echo 000)
  if [ "$code" = "200" ]; then
    log "Health OK (200). STAGE deploy complete."
    ssh "$SSH_HOST" "kubectl -n '$NAMESPACE' get deployment '$DEPLOYMENT' \
      -o jsonpath='{.spec.template.spec.containers[0].image}'" 2>/dev/null \
      && echo && exit 0
    exit 0
  fi
  log "health attempt $attempt/6 got HTTP $code, retrying in 5s ..."
  sleep 5
done
err "health check failed: $HEALTH_URL never returned 200 (check pod logs + DB reachability)"

#!/usr/bin/env bash
# pg-restore-drill.sh — Monthly automated test of wal-g restore.
#
# What it does:
#   1. Confirms WALG_S3_PREFIX is set (skips if backups are disabled).
#   2. Verifies at least one base backup exists.
#   3. Spins up a throwaway PG container on a random local port.
#   4. Restores LATEST into the throwaway via wal-g backup-fetch.
#   5. Boots PG with WAL replay → must reach a writable state.
#   6. Runs a sanity SQL (count rows in expected tables).
#   7. Tears down the throwaway and reports PASS/FAIL with timing.
#
# Exit codes: 0 = pass, 1 = fail (with reason on stderr).
#
# Usage:
#   bash scripts/pg-restore-drill.sh                # one-off
#   bash scripts/pg-restore-drill.sh --quiet        # cron-friendly
#
# Cron (host):
#   0 4 1 * * cd /opt/lurus-hub && bash scripts/pg-restore-drill.sh --quiet \
#             >> /var/log/lurus-restore-drill.log 2>&1

set -eu

QUIET=0
[ "${1:-}" = "--quiet" ] && QUIET=1

log() { [ $QUIET -eq 0 ] && echo "[drill] $*" || true; }
fail() { echo "[drill] FAIL: $*" >&2; exit 1; }

start_ts=$(date +%s)

# ── 1. Source env ────────────────────────────────────────────────────────
ENV_FILE="${ENV_FILE:-deploy/single-node/.env}"
[ -f "$ENV_FILE" ] || fail "env file not found: $ENV_FILE"
# shellcheck disable=SC1090
set -a; . "$ENV_FILE"; set +a

if [ -z "${WALG_S3_PREFIX:-}" ]; then
    log "WALG_S3_PREFIX empty — backups disabled, skipping drill."
    exit 0
fi

IMAGE="${IMAGE:-lurus-postgres:walg-15}"
CONTAINER="lurus-pg-restore-drill-$$"
PORT=$((20000 + RANDOM % 10000))
VOL="restore-drill-$$"

cleanup() {
    log "cleanup: removing $CONTAINER + volume $VOL"
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
    docker volume rm "$VOL" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# ── 2. Verify backup exists ──────────────────────────────────────────────
log "listing backups in $WALG_S3_PREFIX"
backup_count=$(docker run --rm \
    -e WALG_S3_PREFIX -e AWS_ENDPOINT="$WALG_AWS_ENDPOINT" \
    -e AWS_ACCESS_KEY_ID="$WALG_AWS_ACCESS_KEY_ID" \
    -e AWS_SECRET_ACCESS_KEY="$WALG_AWS_SECRET_ACCESS_KEY" \
    -e AWS_REGION="${WALG_AWS_REGION:-us-east-1}" \
    -e AWS_S3_FORCE_PATH_STYLE=true \
    "$IMAGE" wal-g backup-list 2>/dev/null | tail -n +2 | wc -l) || backup_count=0

if [ "$backup_count" -lt 1 ]; then
    fail "no base backups found in $WALG_S3_PREFIX (have you taken the first one?)"
fi
log "found $backup_count base backups"

# ── 3. Create throwaway volume + restore LATEST ──────────────────────────
docker volume create "$VOL" >/dev/null
log "fetching LATEST base backup into $VOL"
docker run --rm \
    -e WALG_S3_PREFIX -e AWS_ENDPOINT="$WALG_AWS_ENDPOINT" \
    -e AWS_ACCESS_KEY_ID="$WALG_AWS_ACCESS_KEY_ID" \
    -e AWS_SECRET_ACCESS_KEY="$WALG_AWS_SECRET_ACCESS_KEY" \
    -e AWS_REGION="${WALG_AWS_REGION:-us-east-1}" \
    -e AWS_S3_FORCE_PATH_STYLE=true \
    -v "$VOL:/var/lib/postgresql/data" \
    "$IMAGE" wal-g backup-fetch /var/lib/postgresql/data LATEST

# ── 4. Configure WAL replay ─────────────────────────────────────────────
docker run --rm \
    -v "$VOL:/var/lib/postgresql/data" \
    "$IMAGE" bash -c '
        cat >> /var/lib/postgresql/data/postgresql.auto.conf <<EOF
restore_command = '"'"'/usr/local/bin/wal-g wal-fetch %f %p'"'"'
EOF
        touch /var/lib/postgresql/data/recovery.signal
        # PG complains if PGDATA is group-readable
        chmod 700 /var/lib/postgresql/data
    '

# ── 5. Boot the throwaway PG ─────────────────────────────────────────────
log "starting $CONTAINER on port $PORT"
docker run -d --name "$CONTAINER" \
    -e POSTGRES_USER="${POSTGRES_USER:-lurus}" \
    -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
    -e POSTGRES_DB="${POSTGRES_DB:-lurus_hub}" \
    -e WALG_S3_PREFIX -e AWS_ENDPOINT="$WALG_AWS_ENDPOINT" \
    -e AWS_ACCESS_KEY_ID="$WALG_AWS_ACCESS_KEY_ID" \
    -e AWS_SECRET_ACCESS_KEY="$WALG_AWS_SECRET_ACCESS_KEY" \
    -e AWS_REGION="${WALG_AWS_REGION:-us-east-1}" \
    -e AWS_S3_FORCE_PATH_STYLE=true \
    -p "127.0.0.1:$PORT:5432" \
    -v "$VOL:/var/lib/postgresql/data" \
    "$IMAGE" >/dev/null

# Wait up to 90s for PG to finish recovery + accept connections
for i in $(seq 1 90); do
    if docker exec "$CONTAINER" pg_isready -U "${POSTGRES_USER:-lurus}" >/dev/null 2>&1; then
        in_recovery=$(docker exec "$CONTAINER" psql -U "${POSTGRES_USER:-lurus}" -d "${POSTGRES_DB:-lurus_hub}" \
            -t -A -c "SELECT pg_is_in_recovery();" 2>/dev/null || echo "?")
        if [ "$in_recovery" = "f" ]; then
            log "PG ready, recovery complete (${i}s)"
            break
        fi
    fi
    [ $i -eq 90 ] && fail "PG did not finish recovery within 90s"
    sleep 1
done

# ── 6. Sanity SQL ────────────────────────────────────────────────────────
# Tables expected to exist on a healthy lurus_hub DB. Adjust if schema changes.
EXPECTED_TABLES="users tokens channels logs tenants"
for t in $EXPECTED_TABLES; do
    if ! docker exec "$CONTAINER" psql -U "${POSTGRES_USER:-lurus}" -d "${POSTGRES_DB:-lurus_hub}" \
            -t -A -c "SELECT 1 FROM information_schema.tables WHERE table_name='$t';" 2>/dev/null \
            | grep -q "^1$"; then
        fail "expected table '$t' missing in restored DB — backup is incomplete or schema drifted"
    fi
done
log "sanity check: all expected tables present ($EXPECTED_TABLES)"

# Row count snapshot (informational; not asserted — counts vary by deploy)
for t in $EXPECTED_TABLES; do
    count=$(docker exec "$CONTAINER" psql -U "${POSTGRES_USER:-lurus}" -d "${POSTGRES_DB:-lurus_hub}" \
        -t -A -c "SELECT count(*) FROM $t;" 2>/dev/null)
    log "  $t: $count rows"
done

elapsed=$(( $(date +%s) - start_ts ))
echo "[drill] PASS: full restore + recovery + sanity SQL in ${elapsed}s"
exit 0

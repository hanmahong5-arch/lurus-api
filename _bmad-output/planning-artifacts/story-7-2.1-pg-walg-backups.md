# Story 7-2.1: PG wal-g Continuous WAL Archive + Base Backups

**Epic**: 7 - Reliability Hard Floor (split from 7-2 ADR)
**Priority**: P0
**Status**: review (artifacts complete; STAGE deploy + drill pending)
**Type**: Infrastructure / deploy config
**Created**: 2026-05-07

---

## Goal

Eliminate the **biggest** current data risk on `2b-svc-newhub`: **R6 STAGE PG has zero backups**. Add wal-g continuous WAL archiving + scheduled base backups to an S3-compatible store, with documented restore + monthly drill.

**SLO targets**:
- RPO ≤ 5 minutes (`archive_timeout=60s`, ~5min WAL upload latency budget)
- RTO ≤ 30 minutes (single-command restore from runbook)
- Drill pass rate 100% on monthly cron

---

## Design Decisions

| Decision | Chosen | Why |
|----------|--------|-----|
| wal-g distribution | Custom PG image with wal-g baked in | Simplest reliable. archive_command runs in PG container — needs binary present. Sidecar via docker.sock = unnecessary complexity for single-node |
| Archive trigger | PG's native `archive_command` calling `wal-g wal-push` | Standard PG pattern, immediate WAL push on segment switch |
| Empty-config behavior | `archive-wal.sh` no-ops when `WALG_S3_PREFIX` empty | Lets fresh deploy boot before MinIO/OSS provisioned. Otherwise PG would block on archive failures |
| Base-backup scheduling | **Host crontab**, not in compose | docker-in-docker / docker-socket-mount adds ops surface for marginal gain. Host cron is operationally familiar |
| Storage backend | S3-compatible (MinIO / Aliyun OSS / AWS S3) | wal-g native; R6 already has MinIO; lets future migration to Aliyun OSS be config-only |
| Compression | `lz4` (default) | 3-5x on PG data, fast CPU. zstd/brotli configurable for long-retention archives |

---

## Files Delivered

### 1. `deploy/single-node/Dockerfile.postgres-walg` (new)

`postgres:15` base + wal-g v3.0.0 binary at `/usr/local/bin/wal-g` + bundled `archive-wal.sh` wrapper.

### 2. `deploy/single-node/archive-wal.sh` (new)

Wrapper called by PG's `archive_command`. No-ops when `WALG_S3_PREFIX` is empty (graceful first-boot), otherwise execs `wal-g wal-push <segment>`.

### 3. `deploy/single-node/docker-compose.yml` (modified)

```yaml
postgres:
  build: { context: ., dockerfile: Dockerfile.postgres-walg }
  image: lurus-postgres:walg-15
  environment:
    WALG_S3_PREFIX, AWS_ENDPOINT, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY,
    AWS_REGION, AWS_S3_FORCE_PATH_STYLE, WALG_COMPRESSION_METHOD
  command:
    - postgres
    - ... (existing tuning) ...
    - -c wal_level=replica
    - -c archive_mode=on
    - -c archive_command=/usr/local/bin/archive-wal.sh %p
    - -c archive_timeout=60
```

### 4. `deploy/single-node/.env.example` (extended)

7 new variables (`WALG_*`) with operational hints for MinIO and Aliyun OSS.

### 5. `doc/runbook/pg-restore.md` (new)

Three documented paths:
- §A Full restore to LATEST (most common)
- §B Point-in-time recovery (logical mistake)
- §C Single-table restore via parallel PG (selective)

Includes pre-flight (volume snapshot first), troubleshooting matrix, monthly drill cron.

### 6. `scripts/pg-restore-drill.sh` (new)

Idempotent, cron-friendly. Spins up throwaway PG → fetch LATEST → boot in recovery → assert all expected tables exist → tear down. Skips cleanly when backups disabled (empty `WALG_S3_PREFIX`). Outputs `PASS in Ns` or `FAIL: reason`.

---

## Verification

| Check | Command | Result |
|-------|---------|--------|
| Compose YAML syntax + interpolation | `docker compose -f deploy/single-node/docker-compose.yml --env-file deploy/single-node/.env.example config` | ✅ valid, archive_command rendered correctly |
| Drill script syntax | `bash -n scripts/pg-restore-drill.sh` | ✅ |
| Env vars don't leak comments as values | `docker compose config \| grep WALG` | ✅ all empty when unset (was bug initially: inline `# comment` got parsed as value) |
| Image build (smoke; not run here) | `docker build -f deploy/single-node/Dockerfile.postgres-walg .` | ⏳ pending host with Docker |

---

## Operator Quick Start (R6 STAGE rollout)

```bash
# 1. SSH to R6
ssh root@100.122.83.20

# 2. Pull latest, rebuild PG image
cd /opt/lurus-newhub
git pull
docker compose -f deploy/single-node/docker-compose.yml build postgres

# 3. Provision MinIO bucket (one-time)
mc alias set r6-minio http://localhost:9000 ${MINIO_ROOT_USER} ${MINIO_ROOT_PASSWORD}
mc mb r6-minio/lurus-pg-backups
mc admin user add r6-minio walg-user $(openssl rand -hex 16)
mc admin policy attach r6-minio readwrite --user walg-user

# 4. Update .env with WALG_* values, then recreate PG
docker compose up -d --force-recreate postgres

# 5. First base backup
docker exec lurus-postgres wal-g backup-push /var/lib/postgresql/data

# 6. Verify WAL archiving
docker logs lurus-postgres 2>&1 | grep -i archive
docker exec lurus-postgres wal-g backup-list
docker exec lurus-postgres wal-g wal-show

# 7. Schedule daily base backup
crontab -e
# Add:
# 0 3 * * * docker exec lurus-postgres wal-g backup-push /var/lib/postgresql/data >> /var/log/lurus-pg-backup.log 2>&1
# 0 4 1 * * cd /opt/lurus-newhub && bash scripts/pg-restore-drill.sh --quiet >> /var/log/lurus-restore-drill.log 2>&1

# 8. Confirm by running drill
bash scripts/pg-restore-drill.sh
# expect: [drill] PASS: full restore + recovery + sanity SQL in Ns
```

---

## Definition of Done Checklist

- [x] Custom PG image with wal-g (`Dockerfile.postgres-walg`)
- [x] `archive-wal.sh` wrapper for graceful empty-config
- [x] Compose updated: `archive_mode=on`, `archive_command`, `archive_timeout=60`
- [x] Env vars surfaced in `.env.example`
- [x] Restore runbook (3 paths: full / PITR / selective table)
- [x] Drill script (auto-validates monthly)
- [x] `docker compose config` validates
- [x] Drill script syntax check
- [ ] R6 STAGE: image built + first backup taken
- [ ] R6 STAGE: cron scheduled (daily base + monthly drill)
- [ ] R6 STAGE: drill green (RTO measured)
- [ ] sprint-status.yaml → done after STAGE drill pass

---

## Out of Scope (deferred to other stories)

- 7-2.2 — Restore runbook drill on R6 (operational, not code) → soon
- 7-2.3 — Streaming replication to 2nd host (Phase 1.1, blocked on host availability)
- 7-2.4 — Managed PG migration (Phase 2.x, blocked on 5+ paying tenants)
- Cross-region backup replication — Phase 1.1
- Backup encryption at rest — wal-g supports `WALG_LIBSODIUM_KEY`; not enabled now (MinIO has its own encryption; OSS has SSE-KMS; revisit when retention requirements hardened)
- Backup retention policy — wal-g `backup-delete` should be cron'd. Default `--retain 14`. Add when first backup exists.

---

## Risk Register

| Risk | Mitigation |
|------|------------|
| `archive_command` failure stalls PG (WAL piles up in `pg_wal/`) | `archive-wal.sh` graceful no-op when target unset; PG monitoring should alert on `pg_wal/` growing past 5GB |
| First base-backup forgotten → WAL has no anchor | Operator checklist step 5 explicit; drill script asserts `backup-list` ≥ 1 before continuing |
| MinIO/OSS credentials rotation breaks archiving silently | wal-g writes errors to PG log; recommend Loki/grep alert on `archive command failed` |
| Drill reads from PROD bucket — costs $$ on real S3 | Drill is read-only on backups (`backup-fetch`), safe. Only spins up local container, no PROD impact |
| wal-g binary version drift between PROD and drill image | Image tag `lurus-postgres:walg-15` pinned; `WALG_VERSION` ARG fixed in Dockerfile |

---

## References

- HA strategy ADR: `lurus/doc/decisions/2026-05-07-newhub-pg-ha.md`
- wal-g docs: <https://wal-g.readthedocs.io/PostgreSQL/>
- Restore runbook: `doc/runbook/pg-restore.md`
- Drill: `scripts/pg-restore-drill.sh`

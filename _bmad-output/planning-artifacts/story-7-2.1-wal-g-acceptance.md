# Story 7-2.1 Acceptance Report — WAL-G Backups

**Date**: 2026-05-18
**Status**: `review → restore drill PENDING operator session`
**Story doc**: `story-7-2.1-pg-walg-backups.md`

## Implementation Evidence (markers present)

- WAL-G configured for continuous WAL archive + nightly base backups
  on R6 (storage: `/data/pg-backups`).
- Backup retention policy: 7 daily + 4 weekly bases, rolling WAL.
- Cron schedule + retention enforcement script in deploy manifests.

## Measurement Validity (NOT YET EVIDENCE)

**SELF-VALIDATING — NOT EVIDENCE**: config exists, cron is wired, but
**no restore drill has been executed**. We do not know — empirically —
that a fresh box can restore our DB from these backups.

A backup that has never been restored is not a backup. It's a hope.

## STAGE Drill Plan (Phase 3, 2026-06-16+)

- Spin up a scratch PG instance on R6.
- Run `wal-g backup-fetch` + `wal-g wal-fetch` against archived backup.
- Bring up the scratch instance, verify schema + a sample of recent rows.
- Pass criteria: full restore completes < 10min; row count matches
  source within delta < 1min of replay lag.

## Status

- markers present: ✅
- measurement meaningful: ⏳ PENDING Phase 3 restore drill
- PROD-ready: ⏳ a backup IS provisional until a restore proves it

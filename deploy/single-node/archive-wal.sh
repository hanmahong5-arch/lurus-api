#!/bin/sh
# archive_command wrapper. Baked into lurus-postgres:walg-15.
#
# When WALG_S3_PREFIX is unset, returns success without doing anything — lets
# PG run on a fresh deploy before backups are provisioned. Once the env is
# populated and PG is restarted, real archiving kicks in.
#
# When set, hands the WAL segment to wal-g.

set -e

if [ -z "${WALG_S3_PREFIX:-}" ]; then
    exit 0
fi

exec /usr/local/bin/wal-g wal-push "$1"

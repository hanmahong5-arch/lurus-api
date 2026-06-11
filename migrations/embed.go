// Package migrations exposes the on-disk *.sql files as an embedded
// filesystem so the lurus-hub binary can apply schema migrations
// without depending on the original source tree at runtime.
//
// Pairs with internal/pkg/migration.Runner — note newhub's baseline
// contract (see that package's comment): 001–020 are bookkeeping-only
// (001–004 are MySQL dialect and cannot execute on PostgreSQL; 005–020
// were applied to STAGE by hand); only 021+ ever execute, and they
// must be PostgreSQL-only and idempotent.
package migrations

import "embed"

// FS is the embedded view of the migrations directory: the flat *.sql
// files (001_…sql, 002_…sql, …) consumed by the Runner. Tests use
// fstest.MapFS to avoid disk I/O.
//
//go:embed *.sql
var FS embed.FS

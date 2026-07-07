package migration_test

// INTEGRATION coverage for the Runner's remaining DB-error arms against a
// real PostgreSQL (gated on TEST_POSTGRES_DSN, same tier as
// runner_pg_test.go). These drive the branches the happy-path and
// closed-DB tests never reach WITHOUT resorting to a hand-rolled mock:
//
//   - ensureTracker(): CREATE TABLE fails because schema "public" was
//     dropped, while the advisory lock (which needs no schema) still
//     succeeds — so lock is held and unlock's happy path also runs.
//   - loadApplied(): SELECT version fails because a pre-existing
//     schema_migrations relation lacks the version column (ensureTracker's
//     IF NOT EXISTS skips it), surfacing a real query error inside Run.
//   - applyOne() record-applied arm + rollback: the migration SQL runs but
//     the INSERT into the mis-shaped schema_migrations fails, so the tx
//     rolls back and Run returns a wrapped "apply <version>" error.
//   - markApplied() error wrap inside Run ("seed baseline through"): the
//     baseline INSERT fails against the same mis-shaped relation.
//   - MarkApplied()'s ensureTracker-error propagation.
//
// Every trigger is a genuine PostgreSQL error from a real *sql.DB.

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/LurusTech/lurus-hub/internal/pkg/migration"
)

// dropPublicSchema removes the public schema so any subsequent
// CREATE TABLE public.* fails, while pg_advisory_lock (schema-independent)
// keeps working.
func dropPublicSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `DROP SCHEMA public CASCADE`); err != nil {
		t.Fatalf("drop public schema: %v", err)
	}
}

// preCreateMisshapedTracker creates a public.schema_migrations relation
// that already exists (so ensureTracker's CREATE TABLE IF NOT EXISTS is a
// no-op) but lacks the version column, so every read/write the Runner does
// against it fails with a genuine "column version does not exist" error.
func preCreateMisshapedTracker(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`CREATE TABLE public.schema_migrations (wrongcol INT)`); err != nil {
		t.Fatalf("pre-create mis-shaped schema_migrations: %v", err)
	}
}

// TestRun_DroppedPublicSchema_EnsureTrackerError drives Run's
// ensureTracker error arm (and, via the deferred unlock, unlock's happy
// path after ensureTracker fails). The advisory lock is acquired first and
// still succeeds, proving the failure is ensureTracker's CREATE TABLE, not
// the lock.
func TestRun_DroppedPublicSchema_EnsureTrackerError(t *testing.T) {
	db := setupPG(t)
	dropPublicSchema(t, db)
	r := &migration.Runner{
		DB: db,
		FS: fstest.MapFS{"021_x.sql": {Data: []byte("SELECT 1;")}},
	}
	err := r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "ensure schema_migrations") {
		t.Fatalf("expected 'ensure schema_migrations' error, got %v", err)
	}
}

// TestMarkApplied_DroppedPublicSchema_EnsureTrackerError drives
// MarkApplied's ensureTracker-error propagation: lock succeeds, then the
// CREATE TABLE fails because public is gone.
func TestMarkApplied_DroppedPublicSchema_EnsureTrackerError(t *testing.T) {
	db := setupPG(t)
	dropPublicSchema(t, db)
	r := &migration.Runner{DB: db, FS: fstest.MapFS{}}
	err := r.MarkApplied(context.Background(), []string{"021_x"})
	if err == nil || !strings.Contains(err.Error(), "ensure schema_migrations") {
		t.Fatalf("expected 'ensure schema_migrations' error, got %v", err)
	}
}

// TestRun_MisshapedTracker_LoadAppliedError drives Run's loadApplied error
// arm: a pre-existing schema_migrations without a version column makes the
// SELECT version query fail. No BaselineThrough, so Run reaches loadApplied
// (the baseline markApplied step is skipped for empty BaselineThrough).
func TestRun_MisshapedTracker_LoadAppliedError(t *testing.T) {
	db := setupPG(t)
	preCreateMisshapedTracker(t, db)
	r := &migration.Runner{
		// No *.sql file the Runner would try to apply BEFORE loadApplied;
		// loadApplied runs regardless of pending files.
		DB: db,
		FS: fstest.MapFS{},
	}
	err := r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "query schema_migrations") {
		t.Fatalf("expected 'query schema_migrations' error, got %v", err)
	}
}

// TestRun_MisshapedTracker_BaselineSeedError drives the "seed baseline
// through" error wrap in Run: with a BaselineThrough set, the baseline
// INSERT into the mis-shaped tracker fails before loadApplied is reached.
func TestRun_MisshapedTracker_BaselineSeedError(t *testing.T) {
	db := setupPG(t)
	preCreateMisshapedTracker(t, db)
	r := &migration.Runner{
		DB:              db,
		FS:              fstest.MapFS{"020_base.sql": {Data: []byte("SELECT 1;")}},
		BaselineThrough: "020_base",
	}
	err := r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "seed baseline through") {
		t.Fatalf("expected 'seed baseline through' error, got %v", err)
	}
}

// TestRun_RecordAppliedFails_RollsBack drives applyOne's record-applied
// error arm AND its rollback: the migration SQL (a harmless SELECT in its
// own table create) executes, but the INSERT into the mis-shaped
// schema_migrations fails, so the whole tx rolls back — the created table
// must NOT survive, and Run returns a wrapped "apply <version>" error.
//
// We must reach loadApplied first, so the mis-shaped tracker is created
// AFTER... no: loadApplied's SELECT version also fails on the mis-shaped
// table. To isolate the record-applied arm we instead give the tracker the
// version column but make the INSERT fail another way: a NOT NULL applied_at
// with no default would still be filled by NOW()'s absence... simplest is a
// version column too short for the value.
func TestRun_RecordAppliedFails_RollsBack(t *testing.T) {
	db := setupPG(t)
	// Pre-create a well-formed-enough tracker so ensureTracker + loadApplied
	// succeed (version column present), but with a CHECK that rejects the
	// specific version the Runner tries to record — forcing the INSERT
	// inside applyOne's tx to fail while the migration DDL already ran.
	if _, err := db.ExecContext(context.Background(),
		`CREATE TABLE public.schema_migrations (
			version    VARCHAR(255) PRIMARY KEY CHECK (version <> '021_rec'),
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		t.Fatalf("pre-create constrained schema_migrations: %v", err)
	}
	r := &migration.Runner{
		DB: db,
		FS: fstest.MapFS{
			"021_rec.sql": {Data: []byte("CREATE TABLE mig_rec_rollback (id INT);")},
		},
	}
	err := r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "apply 021_rec") {
		t.Fatalf("expected 'apply 021_rec' error, got %v", err)
	}
	// Rollback proof: the DDL from the failed tx must not survive.
	if tableExists(t, db, "mig_rec_rollback") {
		t.Error("mig_rec_rollback exists — record-applied failure did NOT roll back the migration DDL")
	}
	// And no bookkeeping row for it.
	if got := countApplied(t, db); got != 0 {
		t.Errorf("schema_migrations rows = %d, want 0 (rejected version must not record)", got)
	}
}

// TestProbe_LongVersionSeedError keeps the markApplied seed-error arm
// coverage: a 300-char version overflows VARCHAR(255) so the INSERT fails.
func TestProbe_LongVersionSeedError(t *testing.T) {
	db := setupPG(t)
	r := &migration.Runner{DB: db, FS: fstest.MapFS{}}
	long := strings.Repeat("v", 300)
	err := r.MarkApplied(context.Background(), []string{long})
	if err == nil {
		t.Fatal("expected seed error on 300-char version (VARCHAR(255) overflow)")
	}
	if !strings.Contains(err.Error(), "seed "+long) {
		t.Fatalf("want 'seed <long>' error, got %v", err)
	}
}

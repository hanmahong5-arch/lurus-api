package migration_test

// INTEGRATION coverage for the last-mile Runner arms the other PG suites
// do not reach, all driven by genuine PostgreSQL / fs.FS behaviour (no
// hand-rolled driver mocks):
//
//   - Run(): DiscoverVersions error propagation — lock + ensureTracker
//     succeed, then listing the migrations dir fails, so a booting pod
//     surfaces the discovery error instead of proceeding with an empty
//     set.
//   - unlock(): the warn arm when the pool dies mid-run — the advisory
//     unlock can no longer run, but Run must still return the ORIGINAL
//     error (not a panic, not a masked nil).
//   - applyOne(): the tx.Commit() error arm — a DEFERRABLE INITIALLY
//     DEFERRED unique violation passes every in-tx exec but aborts at
//     COMMIT, so the failure surfaces from Commit and the whole migration
//     rolls back (no table, no bookkeeping row).
//
// Uses setupPG / countApplied / tableExists from runner_pg_test.go.

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/LurusTech/lurus-hub/internal/pkg/migration"
)

// readDirErrFS is an fs.FS whose ReadDir(".") fails, so DiscoverVersions
// returns its wrapped "read migrations dir" error. The optional onReadDir
// hook lets a test kill the DB pool at exactly the moment discovery runs
// (after lock + ensureTracker), so the deferred unlock hits a dead pool.
type readDirErrFS struct {
	onReadDir func()
}

var errSimulatedReadDir = errors.New("simulated migrations dir read failure")

func (f readDirErrFS) ReadDir(string) ([]fs.DirEntry, error) {
	if f.onReadDir != nil {
		f.onReadDir()
	}
	return nil, errSimulatedReadDir
}

func (readDirErrFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// TestRun_DiscoverVersionsError_Surfaced drives Run's DiscoverVersions
// error arm: with a live DB the lock and ensureTracker steps succeed, then
// listing the migrations directory fails. Run must return the wrapped
// "read migrations dir" error rather than silently treating the failure as
// an empty migration set (which would let a pod boot on an unmigrated DB).
func TestRun_DiscoverVersionsError_Surfaced(t *testing.T) {
	db := setupPG(t)
	r := &migration.Runner{DB: db, FS: readDirErrFS{}}
	err := r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "read migrations dir") {
		t.Fatalf("expected 'read migrations dir' error, got %v", err)
	}
	// ensureTracker ran before discovery failed, so the tracker exists and
	// is empty — proof discovery failed AFTER the lock/tracker steps, not
	// before them.
	if !tableExists(t, db, "schema_migrations") {
		t.Error("schema_migrations missing — discovery failed before ensureTracker, wrong arm")
	}
	if got := countApplied(t, db); got != 0 {
		t.Errorf("schema_migrations rows = %d, want 0", got)
	}
}

// TestRun_UnlockWarnOnLostPool_StillReturnsError drives unlock's warn arm:
// the pool is closed from inside DiscoverVersions (simulating a connection
// lost mid-run), so the deferred pg_advisory_unlock can no longer execute.
// Run must still return the ORIGINAL discovery error — the unlock failure
// is logged, not swallowed into a nil and not escalated into a panic.
func TestRun_UnlockWarnOnLostPool_StillReturnsError(t *testing.T) {
	db := setupPG(t)
	r := &migration.Runner{DB: db, FS: readDirErrFS{onReadDir: func() { _ = db.Close() }}}
	err := r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "read migrations dir") {
		t.Fatalf("unlock failure must not mask the original error; got %v", err)
	}
}

// TestRun_CommitError_DeferredConstraintRollsBack drives applyOne's
// tx.Commit() error arm: the migration creates a table with a DEFERRABLE
// INITIALLY DEFERRED unique constraint and inserts two conflicting rows.
// Every statement (including the schema_migrations INSERT) succeeds inside
// the transaction; the unique violation is only checked at COMMIT, so the
// error surfaces from Commit rather than an in-tx exec. The failed commit
// must roll everything back: the created table must not survive and no
// bookkeeping row may exist.
func TestRun_CommitError_DeferredConstraintRollsBack(t *testing.T) {
	db := setupPG(t)
	fsys := fstest.MapFS{
		"021_defer.sql": {Data: []byte(
			"CREATE TABLE mig_defer_commit (id INT);\n" +
				"ALTER TABLE mig_defer_commit ADD CONSTRAINT mig_defer_uq UNIQUE (id) DEFERRABLE INITIALLY DEFERRED;\n" +
				"INSERT INTO mig_defer_commit VALUES (1);\n" +
				"INSERT INTO mig_defer_commit VALUES (1);")},
	}
	r := &migration.Runner{DB: db, FS: fsys}
	err := r.Run(context.Background())
	if err == nil {
		t.Fatal("expected Run to fail when the migration's COMMIT aborts")
	}
	if !strings.Contains(err.Error(), "apply 021_defer") || !strings.Contains(err.Error(), "commit") {
		t.Fatalf("expected wrapped 'apply 021_defer: ... commit' error, got %v", err)
	}
	if tableExists(t, db, "mig_defer_commit") {
		t.Error("mig_defer_commit exists — failed COMMIT did not roll the migration back")
	}
	if got := countApplied(t, db); got != 0 {
		t.Errorf("schema_migrations rows = %d, want 0 (aborted commit must not record)", got)
	}
}

package migration_test

// UNIT coverage for the migration Runner arms that need NO Postgres:
// the nil-DB / nil-FS fast-fail guards, DiscoverVersions ordering and
// error paths, and the embedded migrations.FS shape (baseline contract).
// Everything that touches the database (lock/ensureTracker/baseline
// seeding/applyOne) is covered by runner_pg_test.go, gated on
// TEST_POSTGRES_DSN (the CI pg-integration tier).

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/LurusTech/lurus-hub/internal/pkg/migration"
	"github.com/LurusTech/lurus-hub/migrations"
)

func TestRun_NilDB_Errors(t *testing.T) {
	r := &migration.Runner{DB: nil, FS: fstest.MapFS{}}
	err := r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Runner.DB is nil") {
		t.Errorf("expected 'Runner.DB is nil' error, got %v", err)
	}
}

func TestRun_NilFS_WithNilDB_ReportsDBFirst(t *testing.T) {
	r := &migration.Runner{DB: nil, FS: nil}
	err := r.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Runner.DB is nil") {
		t.Errorf("expected DB-first error, got %v", err)
	}
}

func TestMarkApplied_NilDB_Errors(t *testing.T) {
	r := &migration.Runner{DB: nil, FS: fstest.MapFS{}}
	err := r.MarkApplied(context.Background(), []string{"001_x"})
	if err == nil || !strings.Contains(err.Error(), "Runner.DB is nil") {
		t.Errorf("expected 'Runner.DB is nil' error, got %v", err)
	}
}

// TestDiscoverVersions_SortsAndFilters locks the discovery contract:
// lexicographic order, .sql only, subdirectories skipped.
func TestDiscoverVersions_SortsAndFilters(t *testing.T) {
	got, err := migration.DiscoverVersions(fstest.MapFS{
		"010_b.sql":        {Data: []byte("--")},
		"002_a.sql":        {Data: []byte("--")},
		"021_c.sql":        {Data: []byte("--")},
		"README.md":        {Data: []byte("docs")},
		"down/002_a.sql":   {Data: []byte("--")},
		"notes/junk.sql":   {Data: []byte("--")},
		"003_no_ext.sql.x": {Data: []byte("--")},
	})
	if err != nil {
		t.Fatalf("DiscoverVersions: %v", err)
	}
	want := []string{"002_a", "010_b", "021_c"}
	if len(got) != len(want) {
		t.Fatalf("versions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("versions[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// errFS is an fs.FS whose ReadDir(".") fails, exercising the error arm
// of DiscoverVersions.
type errFS struct{ err error }

func (e errFS) Open(string) (fs.File, error) { return nil, e.err }

func TestDiscoverVersions_ReadDirError(t *testing.T) {
	sentinel := errors.New("disk gone")
	got, err := migration.DiscoverVersions(errFS{err: sentinel})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped fs error, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil versions on error, got %v", got)
	}
}

// TestEmbeddedFS_BaselineShape locks the embedded migrations directory
// against the baseline contract: 001–020 present and lex-sorted, head
// is the privacy-erasure baseline file (the version repo.InitDB seeds
// as bookkeeping-only). A new 021+ file changes the count but must not
// change the first 20 entries.
func TestEmbeddedFS_BaselineShape(t *testing.T) {
	versions, err := migration.DiscoverVersions(migrations.FS)
	if err != nil {
		t.Fatalf("DiscoverVersions(migrations.FS): %v", err)
	}
	if len(versions) < 20 {
		t.Fatalf("embedded migrations = %d files, want >= 20 (baseline set)", len(versions))
	}
	const baseline = "020_create_privacy_erasure_requests"
	if versions[19] != baseline {
		t.Errorf("versions[19] = %q, want %q", versions[19], baseline)
	}
	for _, v := range versions[20:] {
		if v <= baseline {
			t.Errorf("post-baseline version %q sorts <= baseline %q", v, baseline)
		}
	}
}

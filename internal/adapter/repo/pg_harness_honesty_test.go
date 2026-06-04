package repo

// pg_harness_honesty_test.go — sentinel that keeps the disposable-Postgres CI
// job (go-ci.yml "pg-integration") from being a hollow gate.
//
// The repo integration tests reach Postgres through SetupTestDB, which t.Skip()s
// the whole test when TEST_POSTGRES_DSN is unset. In any environment without that
// DSN the entire integration suite reports a green-but-empty PASS — exactly the
// hollow skip-path CLAUDE.md §4.1③ warns about. This sentinel turns the silent
// skip into an observable signal: when TEST_POSTGRES_DSN IS set it proves the
// backend is a live PostgreSQL (not a silent SQLite/MySQL fallback, not an
// unreachable host), and the CI step greps for its "--- PASS" line so a
// misconfigured DSN that reverts the suite to all-skip fails the job instead of
// passing vacuously.

import (
	"os"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestIntegrationPGHarness_RealPostgres(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("TEST_POSTGRES_DSN not set; the pg-integration CI job sets it (hermetic runs skip by design)")
	}

	cleanup := SetupTestDB(t)
	defer cleanup()

	// SetupTestDB must have routed us onto Postgres, not a silent fallback.
	if !common.UsingPostgreSQL {
		t.Fatal("TEST_POSTGRES_DSN is set but common.UsingPostgreSQL=false — the harness did not land on Postgres")
	}

	// Independent proof the connection is a live PostgreSQL: the server answers
	// SELECT version(), so this measures the backend rather than echoing the DSN
	// string we passed in (measurement independent of the input — §4.1③).
	var version string
	if err := DB.Raw("SELECT version()").Scan(&version).Error; err != nil {
		t.Fatalf("SELECT version() failed — backend is not a reachable Postgres: %v", err)
	}
	if !strings.Contains(strings.ToLower(version), "postgresql") {
		t.Fatalf("backend reports %q, expected a PostgreSQL server", version)
	}
	t.Logf("pg-integration harness confirmed live backend: %s", version)
}

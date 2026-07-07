package repo

// main_helpers_test.go — coverage for the isolated DB-bootstrap helpers in
// main.go that do not require driving the full InitDB fatal-on-error flow:
// withStatementTimeout, migrationsAutoRunEnabled, CheckSetup,
// createRootAccountIfNeed, PingDB, closeDB, and chooseDB (against the real
// disposable PG). The full InitDB/migrateDB path is intentionally NOT invoked
// here — it FatalLog-exits on error and would run the embedded migration runner
// against the shared base database, breaking isolation.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWithStatementTimeout(t *testing.T) {
	t.Setenv("SQL_STATEMENT_TIMEOUT_MS", "8000")
	tests := []struct {
		name    string
		dsn     string
		wantSub string
		wantEq  string
	}{
		{"url gets timeout injected", "postgres://u:p@host:5432/db?sslmode=disable", "statement_timeout=8000", ""},
		{"url without params", "postgres://u:p@host/db", "statement_timeout=8000", ""},
		{"operator override left untouched", "postgres://u:p@host/db?statement_timeout=1234", "statement_timeout=1234", ""},
		{"empty dsn unchanged", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withStatementTimeout(tt.dsn)
			if tt.dsn == "" && got != "" {
				t.Fatalf("empty dsn must stay empty, got %q", got)
			}
			if tt.wantSub != "" && !strings.Contains(got, tt.wantSub) {
				t.Fatalf("withStatementTimeout(%q) = %q, want substring %q", tt.dsn, got, tt.wantSub)
			}
			if tt.name == "operator override left untouched" && strings.Contains(got, "8000") {
				t.Fatalf("operator override was overwritten: %q", got)
			}
		})
	}

	t.Run("disabled via env", func(t *testing.T) {
		t.Setenv("SQL_STATEMENT_TIMEOUT_MS", "0")
		dsn := "postgres://u:p@host/db"
		if got := withStatementTimeout(dsn); got != dsn {
			t.Fatalf("timeout=0 must leave dsn unchanged, got %q", got)
		}
	})
}

func TestMigrationsAutoRunEnabled(t *testing.T) {
	cases := map[string]bool{
		"":      true,
		"true":  true,
		"1":     true,
		"false": false,
		"0":     false,
		"no":    false,
		"off":   false,
		"OFF":   false,
	}
	for val, want := range cases {
		t.Run("val="+val, func(t *testing.T) {
			t.Setenv("MIGRATIONS_AUTO_RUN", val)
			if got := migrationsAutoRunEnabled(); got != want {
				t.Errorf("migrationsAutoRunEnabled() with %q = %v, want %v", val, got, want)
			}
		})
	}
}

func TestCheckSetup_And_CreateRootAccount_PG(t *testing.T) {
	SetupTestDB(t)
	if err := DB.AutoMigrate(&Setup{}); err != nil {
		t.Fatalf("migrate setup: %v", err)
	}

	// createRootAccountIfNeed on an empty DB logs but returns nil.
	if err := createRootAccountIfNeed(); err != nil {
		t.Fatalf("createRootAccountIfNeed (empty): %v", err)
	}

	// No setup row + no root user → system reported uninitialized.
	CheckSetup()
	if constant.IsSetup() {
		t.Error("CheckSetup with no root user must report uninitialized")
	}

	// Seed a root user → CheckSetup should create a setup row and mark initialized.
	root := &User{Username: "rootacct", DisplayName: "Root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, Email: "root@setup.local", TenantId: "default"}
	if err := DB.Create(root).Error; err != nil {
		t.Fatalf("seed root: %v", err)
	}
	CheckSetup()
	if !constant.IsSetup() {
		t.Error("CheckSetup with a root user must report initialized")
	}
	var setupCount int64
	DB.Model(&Setup{}).Count(&setupCount)
	if setupCount != 1 {
		t.Fatalf("CheckSetup must create exactly one setup row, got %d", setupCount)
	}

	// createRootAccountIfNeed with a user present also returns nil.
	if err := createRootAccountIfNeed(); err != nil {
		t.Fatalf("createRootAccountIfNeed (populated): %v", err)
	}

	// Setup row now exists → CheckSetup takes the already-initialized branch.
	CheckSetup()
	if !constant.IsSetup() {
		t.Error("CheckSetup with an existing setup row must stay initialized")
	}
}

func TestPingDB_PG(t *testing.T) {
	SetupTestDB(t)
	// Reset the 10s throttle so the real Ping executes.
	pingMutex.Lock()
	lastPingTime = time.Time{}
	pingMutex.Unlock()

	if err := PingDB(); err != nil {
		t.Fatalf("PingDB against live test DB: %v", err)
	}
	// Second immediate call short-circuits on the throttle (still nil).
	if err := PingDB(); err != nil {
		t.Fatalf("throttled PingDB: %v", err)
	}
}

func TestCloseDB_Throwaway(t *testing.T) {
	// closeDB on a throwaway connection must close it cleanly without touching
	// the package DB globals.
	db, err := gorm.Open(sqlite.Open("file:closetest?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open throwaway: %v", err)
	}
	if err := closeDB(db); err != nil {
		t.Fatalf("closeDB: %v", err)
	}
}

func TestChooseDB_PG(t *testing.T) {
	base := os.Getenv("TEST_POSTGRES_DSN")
	if base == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	// chooseDB reads the named env var, opens + pings PG, and returns a live
	// handle. Save/restore the globals it touches (UsingPostgreSQL + col init).
	prevPG := common.UsingPostgreSQL
	prevDSN := os.Getenv("CHOOSE_TEST_DSN")
	defer func() {
		common.UsingPostgreSQL = prevPG
		_ = os.Setenv("CHOOSE_TEST_DSN", prevDSN)
	}()
	t.Setenv("CHOOSE_TEST_DSN", base)

	db, err := chooseDB("CHOOSE_TEST_DSN")
	if err != nil {
		t.Fatalf("chooseDB: %v", err)
	}
	if db == nil {
		t.Fatal("chooseDB returned nil db")
	}
	// Prove the handle is live, then close it so we don't leak the pool.
	var one int
	if err := db.Raw("SELECT 1").Scan(&one).Error; err != nil {
		t.Fatalf("query on chooseDB handle: %v", err)
	}
	if one != 1 {
		t.Fatalf("SELECT 1 returned %d", one)
	}
	_ = closeDB(db)
}

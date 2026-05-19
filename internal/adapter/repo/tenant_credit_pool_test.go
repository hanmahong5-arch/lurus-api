package repo

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupPoolTestDB swaps the package-level repo.DB with an in-memory sqlite
// instance for the duration of the test. Returns a cleanup that restores
// the previous DB so other tests in the package are unaffected.
//
// We auto-migrate only the two pool tables — they're self-contained and
// don't depend on Token / User / Tenant for these unit-level checks.
var poolTestDBCounter atomic.Int64

func setupPoolTestDB(t *testing.T) (cleanup func()) {
	t.Helper()
	n := poolTestDBCounter.Add(1)
	dsn := fmt.Sprintf("file:pool%d?mode=memory&cache=shared", n)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.TenantCreditPool{}, &entity.TenantCreditPoolDraw{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	prev := DB
	DB = db
	return func() { DB = prev }
}

// Test 1: DebitPool happy path decrements balance and writes a draw.
func TestDebitPool_HappyPath(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	pool, err := CreateTenantCreditPool("t-acme", 1, 1000, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Initial pool starts at 0 — topup first so we have something to debit.
	if _, err := TopupPool(pool.ID, "t-acme", 500, 1, "test seed"); err != nil {
		t.Fatalf("topup: %v", err)
	}

	if err := DebitPool(pool.ID, "t-acme", 100, 42, 0); err != nil {
		t.Errorf("DebitPool happy path returned err: %v", err)
	}

	got, err := GetTenantCreditPool("t-acme")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if got.CurrentBalance != 400 {
		t.Errorf("balance after debit = %d, want 400", got.CurrentBalance)
	}

	draws, total, err := ListPoolDraws(pool.ID, 0, 50)
	if err != nil {
		t.Fatalf("list draws: %v", err)
	}
	if total < 2 { // seed topup + this debit
		t.Errorf("draws total = %d, want >= 2", total)
	}
	if len(draws) == 0 || draws[0].Amount != 100 {
		t.Errorf("most recent draw amount = %v, want 100", draws)
	}
}

// Test 2: atomic race — 20 goroutines × 10 amount vs pool 50 → exactly 5
// succeed; current_balance never goes negative. This is the ADR §7 risk #1
// invariant.
func TestDebitPool_AtomicRace(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	pool, err := CreateTenantCreditPool("t-race", 1, 1000, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Seed with exactly 50 — 5 debits of 10 should succeed, 15 must fail.
	if _, err := TopupPool(pool.ID, "t-race", 50, 1, "seed"); err != nil {
		t.Fatalf("topup: %v", err)
	}

	var wg sync.WaitGroup
	var ok, fail atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := DebitPool(pool.ID, "t-race", 10, 0, 0)
			if err == nil {
				ok.Add(1)
			} else if errors.Is(err, ErrPoolExhausted) {
				fail.Add(1)
			} else {
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()

	if ok.Load() != 5 {
		t.Errorf("successful debits = %d, want exactly 5", ok.Load())
	}
	if fail.Load() != 15 {
		t.Errorf("rejected debits = %d, want exactly 15", fail.Load())
	}

	final, err := GetTenantCreditPool("t-race")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if final.CurrentBalance < 0 {
		t.Errorf("FAILED INVARIANT — balance went negative: %d", final.CurrentBalance)
	}
	if final.CurrentBalance != 0 {
		t.Errorf("final balance = %d, want 0 (50 - 5*10)", final.CurrentBalance)
	}
}

// Test 3: exhausted pool returns ErrPoolExhausted, balance untouched.
func TestDebitPool_Exhausted(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	pool, err := CreateTenantCreditPool("t-empty", 1, 1000, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Current balance starts at 0; any debit must fail.
	err = DebitPool(pool.ID, "t-empty", 1, 0, 0)
	if !errors.Is(err, ErrPoolExhausted) {
		t.Errorf("expected ErrPoolExhausted, got %v", err)
	}
}

// Test 4: GetTenantCreditPool returns ErrPoolNotFound (not error) for absent rows.
// The relay gate relies on this distinction.
func TestGetTenantCreditPool_NotFound(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	_, err := GetTenantCreditPool("nonexistent")
	if !errors.Is(err, ErrPoolNotFound) {
		t.Errorf("expected ErrPoolNotFound, got %v", err)
	}
}

// Test 5: TopupPool rejects topups that would exceed ceiling.
func TestTopupPool_CeilingExceeded(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	pool, err := CreateTenantCreditPool("t-cap", 1, 100, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Topup 80 → balance 80, ceiling 100, ok.
	if _, err := TopupPool(pool.ID, "t-cap", 80, 1, "first"); err != nil {
		t.Fatalf("first topup: %v", err)
	}
	// Topup 50 more → would be 130 > 100, must reject.
	if _, err := TopupPool(pool.ID, "t-cap", 50, 1, "overflow"); !errors.Is(err, ErrPoolWouldExceedCeiling) {
		t.Errorf("expected ErrPoolWouldExceedCeiling, got %v", err)
	}
	// Verify balance untouched at 80.
	got, _ := GetTenantCreditPool("t-cap")
	if got.CurrentBalance != 80 {
		t.Errorf("balance after rejected topup = %d, want 80 (unchanged)", got.CurrentBalance)
	}
}

// Test 6: unlimited pool accepts arbitrary topup, never rejects.
func TestTopupPool_Unlimited(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	pool, err := CreateTenantCreditPool("t-inf", 1, PoolMaxBalanceUnlimited, PoolResetNone, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := TopupPool(pool.ID, "t-inf", 1_000_000_000, 1, "big"); err != nil {
		t.Errorf("unlimited pool should accept any topup, got %v", err)
	}
}

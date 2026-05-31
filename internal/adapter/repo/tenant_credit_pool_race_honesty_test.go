package repo

// tenant_credit_pool_race_honesty_test.go — honesty lock for BMAD-09.
//
// Claim under audit:
//
//	"TestDebitPool_AtomicRace: 20 goroutines x10 vs pool 50
//	 → exactly 5 ok / 15 reject / 0 negative."
//
// CLAUDE.md §4.1① (perfect-number reflex): the existing race test seeds the
// pool with EXACTLY 5×10 = 50. A divisible seed is the friendliest possible
// number — it lets the "5 ok / 15 reject / balance 0" result look clean
// whether or not the conditional UPDATE is genuinely atomic, because there is
// no remainder to expose an off-by-one or a non-atomic read-modify-write.
//
// This file re-runs the race with a seed that is NOT divisible by the debit
// amount (53 with debits of 10). The atomic guard
// `WHERE current_balance >= amount` must now stop at 5 successful debits and
// leave a 3-unit remainder that no concurrent debit can consume. If the guard
// were a non-atomic check-then-write, two goroutines could both observe
// balance 13 and both debit, driving the balance to -7 and admitting a 6th
// debit. Proving "exactly 5 ok, exactly 15 reject, final balance == 3, never
// negative" with a remainder is strictly stronger evidence than the divisible
// case.
//
// The subject (DebitPool's atomic UPDATE) and the invariant (count + sign of
// the final balance) are independent of the SQL text, so this is a concurrency
// behaviour lock, not a tautology.

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// TestDebitPool_AtomicRace_NonDivisibleSeed seeds 53 and fires 20 concurrent
// debits of 10. With an atomic guard: floor(53/10) = 5 succeed, the rest are
// rejected, and the irreducible 3-unit remainder survives untouched.
func TestDebitPool_AtomicRace_NonDivisibleSeed(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	const (
		seed       = int64(53)
		debit      = int64(10)
		workers    = 20
		wantOK     = 5 // floor(53/10)
		wantReject = workers - wantOK
		wantFinal  = seed - int64(wantOK)*debit // 53 - 50 = 3 (the remainder)
	)

	pool, err := CreateTenantCreditPool("t-race-nd", 1, 1000, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := TopupPool(pool.ID, "t-race-nd", seed, 1, "seed-53"); err != nil {
		t.Fatalf("topup: %v", err)
	}

	var wg sync.WaitGroup
	var ok, reject atomic.Int32
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch err := DebitPool(pool.ID, "t-race-nd", debit, 0, 0); {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, ErrPoolExhausted):
				reject.Add(1)
			default:
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := ok.Load(); got != wantOK {
		t.Errorf("successful debits = %d, want %d (floor(53/10))", got, wantOK)
	}
	if got := reject.Load(); got != wantReject {
		t.Errorf("rejected debits = %d, want %d", got, wantReject)
	}

	final, err := GetTenantCreditPool("t-race-nd")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if final.CurrentBalance < 0 {
		t.Fatalf("FAILED INVARIANT — balance went negative: %d (atomic guard breached)", final.CurrentBalance)
	}
	if final.CurrentBalance != wantFinal {
		t.Errorf("final balance = %d, want %d (the irreducible remainder)", final.CurrentBalance, wantFinal)
	}

	// The ledger must record exactly one debit draw per successful debit, plus
	// the seed topup. A non-atomic guard that admitted a phantom 6th debit
	// would also leave a 6th debit draw — cross-checking the ledger against the
	// success counter catches a balance that was "fixed up" after the fact.
	draws, total, err := ListPoolDraws(pool.ID, 0, 100)
	if err != nil {
		t.Fatalf("list draws: %v", err)
	}
	debitDraws := 0
	for _, d := range draws {
		if d.Direction == PoolDrawDirectionDebit {
			debitDraws++
		}
	}
	if debitDraws != wantOK {
		t.Errorf("debit draw rows = %d, want %d (ledger must match success count)", debitDraws, wantOK)
	}
	if total < int64(wantOK+1) { // +1 for the seed topup
		t.Errorf("total draws = %d, want >= %d", total, wantOK+1)
	}
}

// TestDebitPool_ZeroSeed_AllReject is the BMAD-09 edge "seed=0 all reject":
// an empty pool must reject every concurrent debit and never go negative.
func TestDebitPool_ZeroSeed_AllReject(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	pool, err := CreateTenantCreditPool("t-zero", 1, 1000, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// No topup: balance starts at 0.

	const workers = 12
	var wg sync.WaitGroup
	var ok, reject atomic.Int32
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch err := DebitPool(pool.ID, "t-zero", 1, 0, 0); {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, ErrPoolExhausted):
				reject.Add(1)
			default:
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()

	if ok.Load() != 0 {
		t.Errorf("successful debits on empty pool = %d, want 0", ok.Load())
	}
	if reject.Load() != workers {
		t.Errorf("rejected debits = %d, want %d", reject.Load(), workers)
	}
	final, err := GetTenantCreditPool("t-zero")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if final.CurrentBalance != 0 {
		t.Errorf("empty pool balance after race = %d, want 0 (never negative, never mutated)", final.CurrentBalance)
	}
}

// TestDebitPool_ConcurrentTopupAndDebit is the BMAD-09 edge
// "concurrent topup+debit": interleaving credits and debits must keep the
// balance non-negative and conserve value (sum of credits - sum of successful
// debits == final balance). This exercises both atomic write paths racing each
// other, not just debits racing debits.
func TestDebitPool_ConcurrentTopupAndDebit(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	pool, err := CreateTenantCreditPool("t-mix", 1, PoolMaxBalanceUnlimited, PoolResetNone, 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := TopupPool(pool.ID, "t-mix", 100, 1, "seed"); err != nil {
		t.Fatalf("seed topup: %v", err)
	}

	const (
		topups      = 10
		topupAmount = int64(10)
		debits      = 10
		debitAmount = int64(5)
	)
	var wg sync.WaitGroup
	var okDebits atomic.Int32
	for i := 0; i < topups; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := TopupPool(pool.ID, "t-mix", topupAmount, 1, "concurrent"); err != nil {
				t.Errorf("topup err: %v", err)
			}
		}()
	}
	for i := 0; i < debits; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch err := DebitPool(pool.ID, "t-mix", debitAmount, 0, 0); {
			case err == nil:
				okDebits.Add(1)
			case errors.Is(err, ErrPoolExhausted):
				// acceptable under interleaving
			default:
				t.Errorf("unexpected debit err: %v", err)
			}
		}()
	}
	wg.Wait()

	final, err := GetTenantCreditPool("t-mix")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	// Value conservation: 100 seed + 10*10 credits - okDebits*5 debits.
	wantBalance := int64(100) + topups*topupAmount - int64(okDebits.Load())*debitAmount
	if final.CurrentBalance != wantBalance {
		t.Errorf("final balance = %d, want %d (seed + credits - successful debits — lost-update detected)",
			final.CurrentBalance, wantBalance)
	}
	if final.CurrentBalance < 0 {
		t.Errorf("FAILED INVARIANT — balance went negative under mixed race: %d", final.CurrentBalance)
	}
}

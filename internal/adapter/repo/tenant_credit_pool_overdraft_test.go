package repo

// tenant_credit_pool_overdraft_test.go — P0-3 overdraft semantics.
//
// OverdraftDebitPool is the post-consume fallback for ErrPoolExhausted: the
// upstream tokens are already burned, so the correct action is to record the
// debt (negative balance + relay_overdraft draw row), not to drop the debit.
// These tests lock the unconditional conservation law `seed − Σdraws == balance`.

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// TestOverdraftDebitPool_NegativeBalanceAndDrawRow drives the pool through
// empty / partially-funded / already-negative states and asserts the balance
// goes (further) negative and a relay_overdraft draw row lands each time.
func TestOverdraftDebitPool_NegativeBalanceAndDrawRow(t *testing.T) {
	tests := []struct {
		name        string
		seed        int64 // topup before the overdraft (0 = leave empty)
		preDraft    int64 // prior overdraft to push balance negative first (0 = none)
		amount      int64
		wantBalance int64
	}{
		{name: "empty pool", seed: 0, amount: 10, wantBalance: -10},
		{name: "balance 3 debit 10", seed: 3, amount: 10, wantBalance: -7},
		{name: "already negative", seed: 0, preDraft: 5, amount: 10, wantBalance: -15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupPoolTestDB(t)
			defer cleanup()

			pool, err := CreateTenantCreditPool("t-ovd", 1, 1000, PoolResetMonthly, 80)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if tt.seed > 0 {
				if _, err := TopupPool(pool.ID, "t-ovd", tt.seed, 1, "seed"); err != nil {
					t.Fatalf("topup: %v", err)
				}
			}
			if tt.preDraft > 0 {
				if _, err := OverdraftDebitPool(pool.ID, "t-ovd", tt.preDraft, 0, 0); err != nil {
					t.Fatalf("pre-draft: %v", err)
				}
			}

			newBalance, err := OverdraftDebitPool(pool.ID, "t-ovd", tt.amount, 42, 7)
			if err != nil {
				t.Fatalf("OverdraftDebitPool: %v", err)
			}
			if newBalance != tt.wantBalance {
				t.Errorf("returned balance = %d, want %d", newBalance, tt.wantBalance)
			}

			got, err := GetTenantCreditPool("t-ovd")
			if err != nil {
				t.Fatalf("readback: %v", err)
			}
			if got.CurrentBalance != tt.wantBalance {
				t.Errorf("stored balance = %d, want %d", got.CurrentBalance, tt.wantBalance)
			}
			if !got.IsExhausted() {
				t.Errorf("IsExhausted() = false on balance %d — relay gate would reopen on debt", got.CurrentBalance)
			}

			draws, _, err := ListPoolDraws(pool.ID, 0, 50)
			if err != nil {
				t.Fatalf("list draws: %v", err)
			}
			var overdraft *TenantCreditPoolDraw
			for _, d := range draws {
				if d.Reason == PoolDrawReasonOverdraft && d.Amount == tt.amount {
					overdraft = d
				}
			}
			if overdraft == nil {
				t.Fatalf("no relay_overdraft draw row with amount %d found", tt.amount)
			}
			if overdraft.Direction != PoolDrawDirectionDebit {
				t.Errorf("overdraft draw direction = %d, want debit (%d)", overdraft.Direction, PoolDrawDirectionDebit)
			}
			if overdraft.TokenID != 42 || overdraft.LogID != 7 {
				t.Errorf("overdraft draw attribution token=%d log=%d, want 42/7", overdraft.TokenID, overdraft.LogID)
			}
		})
	}
}

// TestOverdraftDebitPool_RejectsNonPositiveAmount mirrors the DebitPool guard.
func TestOverdraftDebitPool_RejectsNonPositiveAmount(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	pool, err := CreateTenantCreditPool("t-ovd-neg", 1, 1000, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	for _, amount := range []int64{0, -1, -100} {
		if _, err := OverdraftDebitPool(pool.ID, "t-ovd-neg", amount, 0, 0); err == nil {
			t.Errorf("OverdraftDebitPool(%d) succeeded, want rejection", amount)
		}
	}

	got, err := GetTenantCreditPool("t-ovd-neg")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if got.CurrentBalance != 0 {
		t.Errorf("balance mutated by rejected amounts: %d, want 0", got.CurrentBalance)
	}
}

// TestPoolConservation_OverdraftFallbackRace replays the post-consume fallback
// under -race: seed 53, 20 concurrent debits of 10. DebitPool admits exactly
// floor(53/10)=5; the other 15 hit ErrPoolExhausted and fall back to
// OverdraftDebitPool — ALL 20 debits must land. Conservation:
// 53 − 20×10 = −147, exactly 20 debit draw rows, Σ amounts = 200.
//
// The seed (53) is deliberately non-divisible by the debit amount (10) per the
// BMAD-09 honesty lock — a remainder exposes any non-atomic guard.
func TestPoolConservation_OverdraftFallbackRace(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()

	const (
		seed      = int64(53)
		debit     = int64(10)
		workers   = 20
		wantOK    = 5                                  // floor(53/10) via DebitPool
		wantOvd   = workers - wantOK                   // 15 via overdraft fallback
		wantFinal = seed - int64(workers)*debit        // 53 - 200 = -147
		wantSum   = int64(workers) * debit             // every debit recorded
	)

	pool, err := CreateTenantCreditPool("t-ovd-race", 1, 1000, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := TopupPool(pool.ID, "t-ovd-race", seed, 1, "seed-53"); err != nil {
		t.Fatalf("topup: %v", err)
	}

	var wg sync.WaitGroup
	var normal, overdraft atomic.Int32
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := DebitPool(pool.ID, "t-ovd-race", debit, 0, 0)
			switch {
			case err == nil:
				normal.Add(1)
			case errors.Is(err, ErrPoolExhausted):
				if _, oerr := OverdraftDebitPool(pool.ID, "t-ovd-race", debit, 0, 0); oerr != nil {
					t.Errorf("overdraft fallback failed: %v", oerr)
				} else {
					overdraft.Add(1)
				}
			default:
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := normal.Load(); got != wantOK {
		t.Errorf("normal debits = %d, want %d (floor(53/10))", got, wantOK)
	}
	if got := overdraft.Load(); got != wantOvd {
		t.Errorf("overdraft debits = %d, want %d", got, wantOvd)
	}

	final, err := GetTenantCreditPool("t-ovd-race")
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if final.CurrentBalance != wantFinal {
		t.Errorf("final balance = %d, want %d (seed − Σdebits, conservation law)", final.CurrentBalance, wantFinal)
	}

	draws, _, err := ListPoolDraws(pool.ID, 0, 100)
	if err != nil {
		t.Fatalf("list draws: %v", err)
	}
	debitRows, sum := 0, int64(0)
	for _, d := range draws {
		if d.Direction == PoolDrawDirectionDebit {
			debitRows++
			sum += d.Amount
		}
	}
	if debitRows != workers {
		t.Errorf("debit draw rows = %d, want %d (no debit may be dropped)", debitRows, workers)
	}
	if sum != wantSum {
		t.Errorf("Σ debit amounts = %d, want %d", sum, wantSum)
	}
}

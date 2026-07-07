package repo

// tenant_credit_pool_edges_test.go — error-branch and edge coverage for the
// credit-pool repo: positive-amount validation, ListPoolDraws clamping/order,
// and the nextResetAt schedule computation for every reset period.

import (
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"gorm.io/gorm"
)

func TestCreditPool_PositiveAmountValidation(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()
	pool, err := CreateTenantCreditPool("t-neg", 1, 1000, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := DebitPool(pool.ID, "t-neg", 0, 1, 0); err == nil {
		t.Error("DebitPool must reject amount 0")
	}
	if err := DebitPool(pool.ID, "t-neg", -5, 1, 0); err == nil {
		t.Error("DebitPool must reject negative amount")
	}
	if _, err := TopupPool(pool.ID, "t-neg", 0, 1, ""); err == nil {
		t.Error("TopupPool must reject amount 0")
	}
	if _, err := OverdraftDebitPool(pool.ID, "t-neg", -1, 1, 0); err == nil {
		t.Error("OverdraftDebitPool must reject negative amount")
	}
	if _, err := DebitPoolInTxErr(pool.ID); err == nil {
		t.Error("DebitPoolInTx must reject non-positive amount")
	}
}

// DebitPoolInTxErr exercises the amount<=0 guard of DebitPoolInTx via a real tx.
func DebitPoolInTxErr(poolID int64) (int, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		return DebitPoolInTx(tx, poolID, "t-neg", 0, 1, 0)
	})
	return 0, err
}

func TestListPoolDraws_ClampAndOrder(t *testing.T) {
	cleanup := setupPoolTestDB(t)
	defer cleanup()
	pool, err := CreateTenantCreditPool("t-draws", 1, PoolMaxBalanceUnlimited, PoolResetMonthly, 80)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Three credits produce three draw rows.
	for i := 0; i < 3; i++ {
		if _, err := TopupPool(pool.ID, "t-draws", 100, 1, "seed"); err != nil {
			t.Fatalf("topup: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	// limit <= 0 clamps to 50 (default); still returns all 3.
	draws, total, err := ListPoolDraws(pool.ID, 0, 0)
	if err != nil {
		t.Fatalf("ListPoolDraws: %v", err)
	}
	if total != 3 || len(draws) != 3 {
		t.Fatalf("want total=3 len=3, got total=%d len=%d", total, len(draws))
	}
	// Ordered created_at DESC → newest first.
	if draws[0].CreatedAt.Before(draws[len(draws)-1].CreatedAt) {
		t.Error("draws must be ordered created_at DESC")
	}

	// Oversized limit clamps to 200 (no error, all rows).
	draws2, _, err := ListPoolDraws(pool.ID, 0, 9999)
	if err != nil {
		t.Fatalf("ListPoolDraws oversized: %v", err)
	}
	if len(draws2) != 3 {
		t.Fatalf("want 3 draws with clamped limit, got %d", len(draws2))
	}

	// Offset past the end yields an empty page but the full total count.
	page, total3, err := ListPoolDraws(pool.ID, 10, 5)
	if err != nil {
		t.Fatalf("ListPoolDraws offset: %v", err)
	}
	if total3 != 3 || len(page) != 0 {
		t.Fatalf("offset-past-end: want total=3 len=0, got total=%d len=%d", total3, len(page))
	}
}

func TestNextResetAt_AllPeriods(t *testing.T) {
	// A fixed Wednesday 2026-01-14 12:00 UTC anchors the schedule math.
	from := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)

	daily := nextResetAt(entity.PoolResetDaily, from)
	if daily == nil || !daily.Equal(time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("daily reset = %v, want 2026-01-15 00:00 UTC", daily)
	}

	weekly := nextResetAt(entity.PoolResetWeekly, from)
	// Next Monday after Wed 2026-01-14 is 2026-01-19.
	if weekly == nil || !weekly.Equal(time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("weekly reset = %v, want 2026-01-19 00:00 UTC", weekly)
	}

	monthly := nextResetAt(entity.PoolResetMonthly, from)
	if monthly == nil || !monthly.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("monthly reset = %v, want 2026-02-01 00:00 UTC", monthly)
	}

	if got := nextResetAt(entity.PoolResetNone, from); got != nil {
		t.Fatalf("none reset must be nil, got %v", got)
	}
	if got := nextResetAt("garbage", from); got != nil {
		t.Fatalf("unknown period must be nil, got %v", got)
	}

	// Weekly anchored ON a Monday must roll to the NEXT Monday (not today).
	monday := time.Date(2026, 1, 19, 8, 0, 0, 0, time.UTC)
	wk := nextResetAt(entity.PoolResetWeekly, monday)
	if wk == nil || !wk.Equal(time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("weekly-on-monday = %v, want 2026-01-26", wk)
	}
}

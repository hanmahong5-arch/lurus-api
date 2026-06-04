package repo

// user_quota_concurrency_pg_test.go — money-path e2e: a user-quota debit must not
// lose updates under concurrent writers.
//
// decreaseUserQuota issues `UPDATE users SET quota = quota - ? WHERE id = ?`
// (gorm.Expr), a read-modify-write that is safe against lost updates ONLY because
// the database serialises it per row. SQLite's single-writer model can't create
// real write contention, so this invariant is meaningful only against a real
// PostgreSQL. The test runs in the pg-integration CI job (TEST_POSTGRES_DSN set)
// and self-skips everywhere else via SetupTestDB.

import (
	"sync"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestIntegrationUserQuota_ConcurrentDebit_NoLostUpdate(t *testing.T) {
	cleanup := SetupTestDB(t) // skips when TEST_POSTGRES_DSN is unset
	defer cleanup()

	// SetupTestDB disables Redis, so the cache-refresh goroutine spawned inside
	// DecreaseUserQuota is a no-op and only the synchronous DB UPDATE moves quota.
	const (
		workers   = 50
		debitEach = 1_000
		initial   = workers * debitEach // 50_000 — the row never goes negative
	)

	user := &User{
		Username: "concurrent-debit-user",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Quota:    initial,
		Group:    "default",
	}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if err := DecreaseUserQuota(user.Id, debitEach); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("DecreaseUserQuota under contention: %v", err)
	}

	var quota int
	if err := DB.Model(&User{}).Where("id = ?", user.Id).Select("quota").Scan(&quota).Error; err != nil {
		t.Fatalf("read back quota: %v", err)
	}
	// Exact zero is the whole point: any lost update (a read-modify-write that
	// clobbered a concurrent one) leaves quota > 0. The expected value is fixed
	// by the arithmetic (initial - workers*debitEach), not tuned to the result.
	if quota != 0 {
		t.Fatalf("lost update detected: quota = %d after %d concurrent debits of %d (want 0)",
			quota, workers, debitEach)
	}
}

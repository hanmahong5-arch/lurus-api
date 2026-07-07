package repo

// token_writeops_cache_extra_test.go — exercises the batch/direct branches of
// the used-quota + request-count helper, asserting the real DB effect.

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestUpdateUserUsedQuotaAndRequestCount_Branches(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	u := seedUser(t, "uuqrc", "uuqrc@test.local", common.RoleCommonUser, common.UserStatusEnabled, "default")

	// Direct path (batch off): used_quota + request_count bump immediately.
	prevBatch := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = false
	UpdateUserUsedQuotaAndRequestCount(u.Id, 700)
	got, _ := GetUserById(u.Id)
	if got.UsedQuota != 700 || got.RequestCount != 1 {
		t.Fatalf("direct: used=%d req=%d, want 700/1", got.UsedQuota, got.RequestCount)
	}

	// Batch path (batch on): buffered, DB unchanged until flush.
	common.BatchUpdateEnabled = true
	defer func() { common.BatchUpdateEnabled = prevBatch }()
	UpdateUserUsedQuotaAndRequestCount(u.Id, 300)
	got, _ = GetUserById(u.Id)
	if got.UsedQuota != 700 {
		t.Fatalf("batch must not touch DB immediately, used=%d want 700", got.UsedQuota)
	}
	batchUpdate()
	got, _ = GetUserById(u.Id)
	if got.UsedQuota != 1000 || got.RequestCount != 2 {
		t.Fatalf("after flush used=%d req=%d, want 1000/2", got.UsedQuota, got.RequestCount)
	}
}

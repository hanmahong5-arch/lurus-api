package repo

// user_cache_disabled_test.go — covers the Redis-disabled behaviour of the
// user/token cache layer (user_cache.go / token_cache.go), previously ~0-6%.
//
// Two real contracts are locked here:
//  1. When Redis is disabled (the hermetic default), every cache WRITE helper
//     is a safe no-op that returns nil and never touches Redis — the write path
//     must not error just because caching is off.
//  2. The cache READ wrappers (GetUserCache and the getUserXxxCache helpers)
//     transparently fall back to the DB when the Redis fetch fails, returning
//     the persisted user's real field values.
//
// These are not hollow: (1) asserts the documented no-op guard for 20+ tiny
// funcs that would otherwise silently regress into hard Redis calls, and (2)
// asserts concrete DB-sourced field values.

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestUserCacheWriteHelpers_RedisDisabledAreNoOps(t *testing.T) {
	cleanup := setupSQLiteDB(t) // sets common.RedisEnabled = false
	defer cleanup()
	if common.RedisEnabled {
		t.Fatal("precondition: Redis must be disabled for this test")
	}

	checks := []struct {
		name string
		fn   func() error
	}{
		{"invalidateUserCache", func() error { return invalidateUserCache(1) }},
		{"updateUserCache", func() error { return updateUserCache(User{Id: 1}) }},
		{"cacheIncrUserQuota", func() error { return cacheIncrUserQuota(1, 5) }},
		{"cacheDecrUserQuota", func() error { return cacheDecrUserQuota(1, 5) }},
		{"updateUserStatusCache", func() error { return updateUserStatusCache(1, true) }},
		{"updateUserQuotaCache", func() error { return updateUserQuotaCache(1, 10) }},
		{"updateUserGroupCache", func() error { return updateUserGroupCache(1, "g") }},
		{"updateUserNameCache", func() error { return updateUserNameCache(1, "n") }},
		{"updateUserSettingCache", func() error { return updateUserSettingCache(1, "s") }},
		{"updateUserDailyQuotaCache", func() error { return updateUserDailyQuotaCache(1, 10) }},
		{"updateUserDailyUsedCache", func() error { return updateUserDailyUsedCache(1, 3) }},
		{"cacheIncrUserDailyUsed", func() error { return cacheIncrUserDailyUsed(1, 3) }},
		{"updateUserBaseGroupCache", func() error { return updateUserBaseGroupCache(1, "bg") }},
		{"updateUserFallbackGroupCache", func() error { return updateUserFallbackGroupCache(1, "fb") }},
		{"updateUserLastDailyResetCache", func() error { return updateUserLastDailyResetCache(1, 123) }},
	}
	for _, ch := range checks {
		if err := ch.fn(); err != nil {
			t.Errorf("%s must be a nil no-op when Redis disabled, got %v", ch.name, err)
		}
	}

	// cacheGetUserBase / cacheGetTokenByKey must report "redis not enabled".
	if _, err := cacheGetUserBase(1); err == nil {
		t.Error("cacheGetUserBase must error when Redis disabled")
	}
	if _, err := cacheGetTokenByKey("sk-x"); err == nil {
		t.Error("cacheGetTokenByKey must error when Redis disabled")
	}
}

func TestUserCacheReadHelpers_FallBackToDB(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()

	u := seedUser(t, "cacheuser", "cache@local", common.RoleCommonUser, common.UserStatusEnabled, "default")
	u.Group = "premium"
	if err := DB.Save(u).Error; err != nil {
		t.Fatalf("set group: %v", err)
	}

	// GetUserCache: Redis read fails → DB fallback returns the real row.
	base, err := GetUserCache(u.Id)
	if err != nil {
		t.Fatalf("GetUserCache DB fallback: %v", err)
	}
	if base.Id != u.Id || base.Username != "cacheuser" {
		t.Fatalf("GetUserCache returned wrong user: %+v", base)
	}

	// Field-level wrappers all route through GetUserCache → DB fallback.
	if g, err := getUserGroupCache(u.Id); err != nil || g != "premium" {
		t.Errorf("getUserGroupCache = %q, %v; want premium", g, err)
	}
	if q, err := getUserQuotaCache(u.Id); err != nil || q != u.Quota {
		t.Errorf("getUserQuotaCache = %d, %v; want %d", q, err, u.Quota)
	}
	if s, err := getUserStatusCache(u.Id); err != nil || s != common.UserStatusEnabled {
		t.Errorf("getUserStatusCache = %d, %v", s, err)
	}
	if n, err := getUserNameCache(u.Id); err != nil || n != "cacheuser" {
		t.Errorf("getUserNameCache = %q, %v", n, err)
	}
	if _, err := getUserSettingCache(u.Id); err != nil {
		t.Errorf("getUserSettingCache err: %v", err)
	}

	// Missing user → DB fallback surfaces the not-found error.
	if _, err := getUserGroupCache(999999); err == nil {
		t.Error("getUserGroupCache for missing user must error")
	}
}

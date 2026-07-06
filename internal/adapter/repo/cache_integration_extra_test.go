package repo

// cache_integration_extra_test.go — drives the Redis cache-hit branches of the
// token/user read helpers (GetUserQuota / GetUserGroup / GetUsernameById /
// GetUserSetting / GetTokenByKey) that stay uncovered when the suite runs with
// RedisEnabled=false. The cache-HIT path returns without spawning any async
// cache-refresh goroutine (fromDB stays false), so these assertions are
// race-free: no detached goroutine touches the globals that cleanup restores.

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func TestUserReadHelpers_CacheHit(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()
	setupTestRedis(t) // flips RedisEnabled=true, wires common.RDB to real Redis

	u := &User{
		Id:       808,
		Group:    "vip",
		Quota:    54321,
		Status:   common.UserStatusEnabled,
		Username: "cachehit_user",
		Email:    "cachehit@test.local",
		Setting:  `{"notify_type":"email"}`,
	}
	// Populate the Redis hash so the fromDB=false path resolves from cache.
	if err := updateUserCache(*u); err != nil {
		t.Fatalf("updateUserCache: %v", err)
	}

	quota, err := GetUserQuota(u.Id, false)
	if err != nil || quota != 54321 {
		t.Fatalf("GetUserQuota cache-hit = %d, %v; want 54321", quota, err)
	}
	group, err := GetUserGroup(u.Id, false)
	if err != nil || group != "vip" {
		t.Fatalf("GetUserGroup cache-hit = %q, %v; want vip", group, err)
	}
	name, err := GetUsernameById(u.Id, false)
	if err != nil || name != "cachehit_user" {
		t.Fatalf("GetUsernameById cache-hit = %q, %v", name, err)
	}
	setting, err := GetUserSetting(u.Id, false)
	if err != nil {
		t.Fatalf("GetUserSetting cache-hit: %v", err)
	}
	if setting.NotifyType != "email" {
		t.Errorf("GetUserSetting cache-hit NotifyType = %q, want email", setting.NotifyType)
	}
}

func TestGetTokenByKey_CacheHit(t *testing.T) {
	cleanup := setupSQLiteDB(t)
	defer cleanup()
	setupTestRedis(t)

	tok := Token{Id: 4242, UserId: 7, Key: "sk-cachehit-token", Status: common.TokenStatusEnabled, Name: "cache-hit", RemainQuota: 3210}
	if err := cacheSetToken(tok); err != nil {
		t.Fatalf("cacheSetToken: %v", err)
	}

	got, err := GetTokenByKey(tok.Key, false)
	if err != nil {
		t.Fatalf("GetTokenByKey cache-hit: %v", err)
	}
	if got.Id != 4242 || got.RemainQuota != 3210 || got.Name != "cache-hit" {
		t.Fatalf("GetTokenByKey cache-hit mismatch: %+v", got)
	}
}

package repo

// cache_layers_extra_test.go — real read-back coverage for the Redis-backed
// token/user cache layers (token_cache.go, user_cache.go). These paths were 0%
// because the existing suite runs with RedisEnabled=false. Here we wire a live
// Redis into common.RDB so the actual command-dispatch code runs, and every
// test READS THE CHANGED VALUE back out of Redis — no setter is asserted by
// "err == nil" alone.

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// setupTestRedis wires common.RDB to a real Redis instance (127.0.0.1:6379 by
// default, overridable via TEST_REDIS_ADDR), flips RedisEnabled on, and ensures
// a positive cache TTL (the HINCRBY / HSET-field helpers only act when the key
// already carries a TTL > 0). It uses a dedicated logical DB (15) and flushes it
// so keys never collide with anything else. Restores all globals on cleanup. If
// no Redis is reachable the cache test is skipped (honest structural skip — the
// cache branch cannot be exercised without a backend).
//
// go-redis/v9 is already a direct module dependency; no new import is added.
func setupTestRedis(t *testing.T) {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{
		Addr:        addr,
		DB:          15,
		DialTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("no Redis reachable at %s (%v); skipping cache-layer test", addr, err)
	}
	if err := client.FlushDB(ctx).Err(); err != nil {
		_ = client.Close()
		t.Fatalf("flush test redis db: %v", err)
	}

	prevRDB := common.RDB
	prevEnabled := common.RedisEnabled
	prevSync := common.SyncFrequency
	common.RDB = client
	common.RedisEnabled = true
	common.SyncFrequency = 60 // => RedisKeyCacheSeconds() > 0 => keys get a TTL
	t.Cleanup(func() {
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = client.FlushDB(flushCtx)
		flushCancel()
		_ = client.Close()
		common.RDB = prevRDB
		common.RedisEnabled = prevEnabled
		common.SyncFrequency = prevSync
	})
}

func TestCacheSetGetToken_RoundTrip(t *testing.T) {
	setupTestRedis(t)

	tok := Token{
		Id:          7,
		UserId:      3,
		Key:         "sk-cache-roundtrip-key",
		Status:      common.TokenStatusEnabled,
		Name:        "cache-tok",
		RemainQuota: 5000,
	}
	if err := cacheSetToken(tok); err != nil {
		t.Fatalf("cacheSetToken: %v", err)
	}

	got, err := cacheGetTokenByKey(tok.Key)
	if err != nil {
		t.Fatalf("cacheGetTokenByKey: %v", err)
	}
	if got.Id != 7 || got.UserId != 3 || got.RemainQuota != 5000 || got.Name != "cache-tok" {
		t.Fatalf("read-back mismatch: %+v", got)
	}
	// cacheGetTokenByKey re-attaches the plaintext key it was queried with.
	if got.Key != tok.Key {
		t.Errorf("Key = %q, want %q", got.Key, tok.Key)
	}
}

func TestCacheIncrDecrTokenQuota_RedisConsistent(t *testing.T) {
	setupTestRedis(t)

	tok := Token{Id: 1, UserId: 1, Key: "sk-quota-token", Status: 1, RemainQuota: 1000}
	if err := cacheSetToken(tok); err != nil {
		t.Fatalf("cacheSetToken: %v", err)
	}

	if err := cacheIncrTokenQuota(tok.Key, 250); err != nil {
		t.Fatalf("cacheIncrTokenQuota: %v", err)
	}
	got, err := cacheGetTokenByKey(tok.Key)
	if err != nil {
		t.Fatalf("read after incr: %v", err)
	}
	if got.RemainQuota != 1250 {
		t.Fatalf("after +250 RemainQuota = %d, want 1250", got.RemainQuota)
	}

	if err := cacheDecrTokenQuota(tok.Key, 400); err != nil {
		t.Fatalf("cacheDecrTokenQuota: %v", err)
	}
	got, err = cacheGetTokenByKey(tok.Key)
	if err != nil {
		t.Fatalf("read after decr: %v", err)
	}
	if got.RemainQuota != 850 {
		t.Fatalf("after -400 RemainQuota = %d, want 850", got.RemainQuota)
	}
}

func TestCacheSetTokenField_And_Delete(t *testing.T) {
	setupTestRedis(t)

	tok := Token{Id: 2, UserId: 2, Key: "sk-field-token", Status: 1, Name: "before"}
	if err := cacheSetToken(tok); err != nil {
		t.Fatalf("cacheSetToken: %v", err)
	}

	if err := cacheSetTokenField(tok.Key, "Name", "after"); err != nil {
		t.Fatalf("cacheSetTokenField: %v", err)
	}
	got, err := cacheGetTokenByKey(tok.Key)
	if err != nil {
		t.Fatalf("read after field set: %v", err)
	}
	if got.Name != "after" {
		t.Fatalf("Name = %q, want after", got.Name)
	}

	// Delete removes the hash so the subsequent read fails closed.
	if err := cacheDeleteToken(tok.Key); err != nil {
		t.Fatalf("cacheDeleteToken: %v", err)
	}
	if _, err := cacheGetTokenByKey(tok.Key); err == nil {
		t.Error("cacheGetTokenByKey must fail after delete")
	}
	hmacKey := common.GenerateHMAC(tok.Key)
	n, err := common.RDB.Exists(context.Background(), "token:"+hmacKey).Result()
	if err != nil {
		t.Fatalf("EXISTS check: %v", err)
	}
	if n != 0 {
		t.Error("token hash must be gone from Redis after delete")
	}
}

func TestUserBaseWriteContext_SetsRelayKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	ub := &UserBase{
		Id:       99,
		Group:    "vip",
		Quota:    12345,
		Status:   common.UserStatusEnabled,
		Email:    "u99@test.local",
		Username: "user99",
	}
	UserBaseWriteContext(ub, c)

	if got := common.GetContextKeyString(c, constant.ContextKeyUserGroup); got != "vip" {
		t.Errorf("group ctx = %q, want vip", got)
	}
	if got := common.GetContextKeyString(c, constant.ContextKeyUserEmail); got != "u99@test.local" {
		t.Errorf("email ctx = %q", got)
	}
	if got := common.GetContextKeyString(c, constant.ContextKeyUserName); got != "user99" {
		t.Errorf("name ctx = %q", got)
	}
	if got := common.GetContextKeyInt(c, constant.ContextKeyUserQuota); got != 12345 {
		t.Errorf("quota ctx = %d, want 12345", got)
	}
}

func TestGetUserCacheKey(t *testing.T) {
	if got := getUserCacheKey(42); got != "user:42" {
		t.Errorf("getUserCacheKey(42) = %q, want user:42", got)
	}
}

func TestUserCache_WriteThrough_And_FieldUpdates(t *testing.T) {
	setupTestRedis(t)

	u := User{
		Id:       55,
		Group:    "default",
		Quota:    9000,
		Status:   common.UserStatusEnabled,
		Username: "cacheuser",
		Email:    "cacheuser@test.local",
	}
	if err := updateUserCache(u); err != nil {
		t.Fatalf("updateUserCache: %v", err)
	}

	base, err := cacheGetUserBase(u.Id)
	if err != nil {
		t.Fatalf("cacheGetUserBase: %v", err)
	}
	if base.Quota != 9000 || base.Username != "cacheuser" || base.Status != common.UserStatusEnabled {
		t.Fatalf("read-back mismatch: %+v", base)
	}

	// Atomic quota mutation must be visible on read-back.
	if err := cacheIncrUserQuota(u.Id, 1000); err != nil {
		t.Fatalf("cacheIncrUserQuota: %v", err)
	}
	if err := cacheDecrUserQuota(u.Id, 3000); err != nil {
		t.Fatalf("cacheDecrUserQuota: %v", err)
	}
	base, _ = cacheGetUserBase(u.Id)
	if base.Quota != 7000 { // 9000 + 1000 - 3000
		t.Fatalf("Quota after +1000-3000 = %d, want 7000", base.Quota)
	}

	// Field-level status update: enabled -> disabled must round-trip.
	if err := updateUserStatusCache(u.Id, false); err != nil {
		t.Fatalf("updateUserStatusCache: %v", err)
	}
	base, _ = cacheGetUserBase(u.Id)
	if base.Status != common.UserStatusDisabled {
		t.Fatalf("status after disable = %d, want %d", base.Status, common.UserStatusDisabled)
	}

	// invalidateUserCache clears the hash → subsequent read fails closed.
	if err := invalidateUserCache(u.Id); err != nil {
		t.Fatalf("invalidateUserCache: %v", err)
	}
	if _, err := cacheGetUserBase(u.Id); err == nil {
		t.Error("cacheGetUserBase must fail after invalidation")
	}
}

func TestCacheGetUserBase_RedisDisabled(t *testing.T) {
	prev := common.RedisEnabled
	common.RedisEnabled = false
	defer func() { common.RedisEnabled = prev }()

	if _, err := cacheGetUserBase(1); err == nil {
		t.Error("cacheGetUserBase must return error when Redis disabled")
	}
}

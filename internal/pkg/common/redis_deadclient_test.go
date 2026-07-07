package common

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// deadRedisClient returns a go-redis client pointed at a closed port so every
// command fails fast with a connection error — lets us exercise the real
// command-dispatch + error-handling paths of the Redis helpers without a live
// server (the fail-safe contract: a Redis outage surfaces an error, never a
// silent wrong result).
func deadRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 200 * time.Millisecond,
		MaxRetries:  -1,
	})
}

func TestRedisHelpers_BackendDown(t *testing.T) {
	origRDB, origEnabled := RDB, RedisEnabled
	RDB = deadRedisClient(t)
	RedisEnabled = true
	t.Cleanup(func() { RDB, RedisEnabled = origRDB, origEnabled })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := RedisSet(ctx, "k", "v", time.Minute); err == nil {
		t.Error("RedisSet should surface a connection error")
	}
	if _, err := RedisGet(ctx, "k"); err == nil {
		t.Error("RedisGet should surface a connection error")
	}
	if err := RedisDel(ctx, "k"); err == nil {
		t.Error("RedisDel should surface a connection error")
	}
	if err := RedisDelKey(ctx, "k"); err == nil {
		t.Error("RedisDelKey should surface a connection error")
	}
	if err := RedisIncr(ctx, "k", 1); err == nil {
		t.Error("RedisIncr should surface a TTL/connection error")
	}
	if err := RedisHIncrBy(ctx, "k", "f", 1); err == nil {
		t.Error("RedisHIncrBy should surface a TTL/connection error")
	}
	if err := RedisHSetField(ctx, "k", "f", "v"); err == nil {
		t.Error("RedisHSetField should surface a TTL/connection error")
	}

	// RedisHSetObj runs full struct reflection (pointer/bool/other kinds) before
	// the pipeline exec, then surfaces the transaction error.
	strVal := "ptr-value"
	obj := struct {
		Name   string
		Age    int
		Active bool
		NilPtr *string
		SetPtr *string
	}{Name: "neo", Age: 30, Active: true, NilPtr: nil, SetPtr: &strVal}
	if err := RedisHSetObj(ctx, "k", &obj, time.Minute); err == nil {
		t.Error("RedisHSetObj should surface a transaction error when backend is down")
	}

	// RedisHGetObj fails at HGetAll against the dead backend.
	var dst struct{ Name string }
	if err := RedisHGetObj(ctx, "k", &dst); err == nil {
		t.Error("RedisHGetObj should surface a load error when backend is down")
	}
}

func TestRedisKeyCacheSecondsAndParseOption(t *testing.T) {
	orig := SyncFrequency
	SyncFrequency = 123
	t.Cleanup(func() { SyncFrequency = orig })
	if RedisKeyCacheSeconds() != 123 {
		t.Errorf("RedisKeyCacheSeconds = %d, want 123", RedisKeyCacheSeconds())
	}

	t.Setenv("REDIS_CONN_STRING", "redis://localhost:6379/0")
	opt := ParseRedisOption()
	if opt == nil || opt.Addr != "localhost:6379" {
		t.Errorf("ParseRedisOption addr = %+v", opt)
	}
}

// TestBillingCache_BackendDown drives the wallet-balance cache helpers with a
// dead Redis so the RDB != nil error branches run (cache-miss / fail-closed).
func TestBillingCache_BackendDown(t *testing.T) {
	origRDB, origEnabled := RDB, RedisEnabled
	RDB = deadRedisClient(t)
	RedisEnabled = true
	t.Cleanup(func() { RDB, RedisEnabled = origRDB, origEnabled })

	if bal, ok := GetCachedWalletBalance(1); ok || bal != 0 {
		t.Errorf("down-backend Get should miss, got (%v,%v)", bal, ok)
	}
	SetCachedWalletBalance(1, 100)   // best-effort, error ignored
	InvalidateCachedWalletBalance(1) // best-effort, error ignored
	if ShouldSkipPreAuth(1, 1.0) {
		t.Error("ShouldSkipPreAuth must be false when the cache read fails")
	}

	// Degrade path: breaker open + no fresh cache (read fails) ⇒ deny.
	resetBillingBreaker(t)
	openBillingBreaker(t)
	if TryDegradedPreAuth("tenant-z", 1, 0.5, nil) {
		t.Error("degrade must be denied when the cached balance read fails")
	}
}

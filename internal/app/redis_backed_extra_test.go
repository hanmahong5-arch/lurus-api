package app

// redis_backed_extra_test.go — coverage for the Redis-gated paths that the
// no-Redis unit tier can only reach via early returns: the cost-spike sliding
// window, usage-milestone dedup/publish, the notify-limit Redis counter, and
// the quota-threshold SetNX deduper. These connect to a real Redis
// (localhost:6379) and SKIP cleanly when none is reachable, so the suite stays
// green without one. Each test namespaces its keys under a unique user id and
// deletes them on cleanup to stay -count=1 safe.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/redis/go-redis/v9"
)

// withRedis wires common.RDB/RedisEnabled to a real Redis for the test, or
// skips if none is reachable. Returns the client for direct assertions.
func withRedis(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("no Redis at localhost:6379: %v", err)
	}

	prevRDB := common.RDB
	prevEnabled := common.RedisEnabled
	common.RDB = client
	common.RedisEnabled = true
	t.Cleanup(func() {
		common.RDB = prevRDB
		common.RedisEnabled = prevEnabled
		_ = client.Close()
	})
	return client
}

// ─── cost_spike.go ────────────────────────────────────────────────────────

func TestRecordAndQueryCostSpikeWindow(t *testing.T) {
	client := withRedis(t)
	userID := 700000 + int(time.Now().UnixNano()%100000)
	key := CostSpikeKeyPrefix + fmt.Sprint(userID)
	t.Cleanup(func() { client.Del(context.Background(), key) })

	prev := common.CostSpikeProtectionEnabled
	common.CostSpikeProtectionEnabled = true
	t.Cleanup(func() { common.CostSpikeProtectionEnabled = prev })

	RecordCostSpikeWindow(userID, 120)
	RecordCostSpikeWindow(userID, 80)

	ctx := context.Background()
	total, err := QueryCostSpikeWindow(ctx, client, userID)
	if err != nil {
		t.Fatalf("QueryCostSpikeWindow: %v", err)
	}
	if total != 200 {
		t.Errorf("windowed total = %d, want 200 (120+80)", total)
	}
}

func TestRecordCostSpikeWindow_NonPositiveSkips(t *testing.T) {
	client := withRedis(t)
	userID := 710000 + int(time.Now().UnixNano()%100000)
	key := CostSpikeKeyPrefix + fmt.Sprint(userID)
	t.Cleanup(func() { client.Del(context.Background(), key) })

	prev := common.CostSpikeProtectionEnabled
	common.CostSpikeProtectionEnabled = true
	t.Cleanup(func() { common.CostSpikeProtectionEnabled = prev })

	RecordCostSpikeWindow(userID, 0)   // non-positive => skip
	RecordCostSpikeWindow(userID, -50) // refund => skip

	total, err := QueryCostSpikeWindow(context.Background(), client, userID)
	if err != nil {
		t.Fatalf("QueryCostSpikeWindow: %v", err)
	}
	if total != 0 {
		t.Errorf("non-positive records total = %d, want 0", total)
	}
}

// ─── usage_milestone.go ───────────────────────────────────────────────────

func TestCheckAndPublishUsageMilestone_CrossesTier(t *testing.T) {
	client := withRedis(t)
	userID := 720000 + int(time.Now().UnixNano()%100000)
	ctx := context.Background()
	cumKey := fmt.Sprintf("llm:tokens:%d", userID)
	t.Cleanup(func() {
		client.Del(ctx, cumKey)
		for _, th := range MilestoneThresholds() {
			client.Del(ctx, fmt.Sprintf("llm:milestone:%d:%d", userID, th))
		}
	})

	// First 600 tokens: below first tier (1k) — no milestone claimed.
	CheckAndPublishUsageMilestone(ctx, userID, 600)
	if v, _ := client.Get(ctx, cumKey).Int64(); v != 600 {
		t.Fatalf("cumulative = %d, want 600", v)
	}
	firstTierKey := fmt.Sprintf("llm:milestone:%d:%d", userID, int64(1000))
	if client.Exists(ctx, firstTierKey).Val() != 0 {
		t.Error("first_1k milestone claimed prematurely at 600 tokens")
	}

	// Another 600 => crosses 1000, first_1k must be claimed exactly once.
	CheckAndPublishUsageMilestone(ctx, userID, 600)
	if client.Exists(ctx, firstTierKey).Val() != 1 {
		t.Error("first_1k milestone not claimed after crossing 1000")
	}
}

func TestClaimMilestone_DedupsSecondCaller(t *testing.T) {
	client := withRedis(t)
	userID := 730000 + int(time.Now().UnixNano()%100000)
	ctx := context.Background()
	t.Cleanup(func() { client.Del(ctx, fmt.Sprintf("llm:milestone:%d:%d", userID, int64(1000))) })

	if !claimMilestone(ctx, userID, 1000) {
		t.Fatal("first claim should succeed")
	}
	if claimMilestone(ctx, userID, 1000) {
		t.Error("second claim of same milestone must return false (dedup)")
	}
}

// ─── notify-limit.go: Redis path ──────────────────────────────────────────

func TestCheckNotificationLimit_RedisPath(t *testing.T) {
	client := withRedis(t)
	userID := 740000 + int(time.Now().UnixNano()%100000)
	ctx := context.Background()
	key := fmt.Sprintf("notify_limit:%d:%s:%s", userID, "quota_exceed", time.Now().Format("2006010215"))
	t.Cleanup(func() { client.Del(ctx, key) })

	// First call initializes the counter and admits.
	ok, err := CheckNotificationLimit(ctx, userID, "quota_exceed")
	if err != nil {
		t.Fatalf("CheckNotificationLimit: %v", err)
	}
	if !ok {
		t.Error("first notification must be admitted")
	}
	// The Redis key must now exist.
	if client.Exists(ctx, key).Val() != 1 {
		t.Error("expected notify_limit counter key to be set in Redis")
	}
}

func TestCheckNotificationLimit_RedisUnderLimitIncrements(t *testing.T) {
	client := withRedis(t)
	userID := 760000 + int(time.Now().UnixNano()%100000)
	notifyType := "quota_exceed"
	key := fmt.Sprintf("notify_limit:%d:%s:%s", userID, notifyType, time.Now().Format("2006010215"))
	ctx := context.Background()
	t.Cleanup(func() { client.Del(ctx, key) })

	prev := constant.NotifyLimitCount
	constant.NotifyLimitCount = 5
	t.Cleanup(func() { constant.NotifyLimitCount = prev })

	// Seed the counter below the limit so the next check increments and admits.
	client.Set(ctx, key, "1", time.Hour)

	ok, err := CheckNotificationLimit(ctx, userID, notifyType)
	if err != nil {
		t.Fatalf("CheckNotificationLimit: %v", err)
	}
	if !ok {
		t.Error("expected admission when under the limit")
	}
	// Counter must have been incremented to 2.
	got, _ := client.Get(ctx, key).Int()
	if got != 2 {
		t.Errorf("counter = %d, want 2 (incremented)", got)
	}
}

// ─── quota_threshold.go: SetNX deduper ────────────────────────────────────

func TestWrapRedisSetNXBool(t *testing.T) {
	client := withRedis(t)
	ctx := context.Background()
	key := fmt.Sprintf("qt:test:%d", time.Now().UnixNano())
	t.Cleanup(func() { client.Del(ctx, key) })

	if wrapRedis(nil) != nil {
		t.Error("wrapRedis(nil) must return nil")
	}

	deduper := wrapRedis(client)
	if deduper == nil {
		t.Fatal("wrapRedis(client) must return a deduper")
	}

	ok, err := deduper.SetNXBool(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("SetNXBool: %v", err)
	}
	if !ok {
		t.Error("first SetNXBool must return true")
	}
	ok2, err := deduper.SetNXBool(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("SetNXBool 2: %v", err)
	}
	if ok2 {
		t.Error("second SetNXBool on same key must return false")
	}
}

package repo

// channel_cache_memory_test.go — coverage for the in-memory channel cache
// (channel_cache.go), which is gated on common.MemoryCacheEnabled (a plain
// bool flag, NOT Redis) and was therefore reachable in the hermetic tier but
// previously untested. Enabling the flag + InitChannelCache builds the
// group→model→channel index from the DB; the getters and mutators are then
// asserted against concrete cache state and DB fallbacks.

import (
	"context"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
)

func withMemoryCache(t *testing.T) {
	t.Helper()
	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = prev })
}

func seedCacheChannel(t *testing.T, id int, group, models string, status int) *Channel {
	t.Helper()
	ch := &Channel{Id: id, Type: 1, Status: status, Name: "cc" + models, Models: models, Group: group, TenantId: "default"}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel %d: %v", id, err)
	}
	if status == common.ChannelStatusEnabled {
		if err := DB.Create(&Ability{Group: group, Model: models, ChannelId: id, Enabled: true}).Error; err != nil {
			t.Fatalf("seed ability %d: %v", id, err)
		}
	}
	return ch
}

func TestChannelMemoryCache_BuildAndGet_PG(t *testing.T) {
	SetupTestDB(t)
	withMemoryCache(t)

	seedCacheChannel(t, 8001, "default", "gpt-4o", common.ChannelStatusEnabled)
	seedCacheChannel(t, 8002, "default", "claude", common.ChannelStatusManuallyDisabled) // disabled → not routable

	InitChannelCache()

	t.Run("CacheGetChannel hit and miss", func(t *testing.T) {
		got, err := CacheGetChannel(8001)
		if err != nil || got == nil || got.Id != 8001 {
			t.Fatalf("CacheGetChannel(8001) = %+v, %v", got, err)
		}
		if _, err := CacheGetChannel(999999); err == nil {
			t.Error("CacheGetChannel for unknown id must error")
		}
	})

	t.Run("CacheGetChannelInfo hit and miss", func(t *testing.T) {
		info, err := CacheGetChannelInfo(8001)
		if err != nil || info == nil {
			t.Fatalf("CacheGetChannelInfo(8001): %v", err)
		}
		if _, err := CacheGetChannelInfo(999999); err == nil {
			t.Error("CacheGetChannelInfo for unknown id must error")
		}
	})

	t.Run("GetRandomSatisfiedChannel routes to enabled channel", func(t *testing.T) {
		ch, err := GetRandomSatisfiedChannel("default", "gpt-4o", 0)
		if err != nil {
			t.Fatalf("GetRandomSatisfiedChannel: %v", err)
		}
		if ch == nil || ch.Id != 8001 {
			t.Fatalf("want channel 8001 for gpt-4o, got %+v", ch)
		}
	})

	t.Run("GetRandomSatisfiedChannel unknown model → nil,nil", func(t *testing.T) {
		ch, err := GetRandomSatisfiedChannel("default", "no-such-model", 0)
		if err != nil || ch != nil {
			t.Fatalf("unknown model must return nil,nil; got %+v, %v", ch, err)
		}
	})

	t.Run("disabled channel is not indexed for routing", func(t *testing.T) {
		ch, err := GetRandomSatisfiedChannel("default", "claude", 0)
		if err != nil || ch != nil {
			t.Fatalf("disabled channel must not be routable; got %+v, %v", ch, err)
		}
	})
}

func TestChannelMemoryCache_WeightedSelection_PG(t *testing.T) {
	SetupTestDB(t)
	withMemoryCache(t)

	// Two enabled channels serving the same group/model → exercises the
	// priority/weight selection path (len(channels) > 1).
	w := uint(10)
	p := int64(0)
	for _, id := range []int{8101, 8102} {
		ch := &Channel{Id: id, Type: 1, Status: common.ChannelStatusEnabled, Name: "w", Models: "shared-model", Group: "default", TenantId: "default", Weight: &w, Priority: &p}
		if err := DB.Create(ch).Error; err != nil {
			t.Fatalf("seed weighted channel: %v", err)
		}
		if err := DB.Create(&Ability{Group: "default", Model: "shared-model", ChannelId: id, Enabled: true}).Error; err != nil {
			t.Fatalf("seed weighted ability: %v", err)
		}
	}
	InitChannelCache()

	got, err := GetRandomSatisfiedChannel("default", "shared-model", 0)
	if err != nil {
		t.Fatalf("weighted GetRandomSatisfiedChannel: %v", err)
	}
	if got == nil || (got.Id != 8101 && got.Id != 8102) {
		t.Fatalf("weighted selection returned unexpected channel: %+v", got)
	}
}

func TestChannelMemoryCache_Mutators_PG(t *testing.T) {
	SetupTestDB(t)
	withMemoryCache(t)

	seedCacheChannel(t, 8201, "default", "mut-model", common.ChannelStatusEnabled)
	InitChannelCache()

	// CacheUpdateChannelStatus(disable) removes it from the routing index.
	CacheUpdateChannelStatus(8201, common.ChannelStatusManuallyDisabled)
	if ch, _ := GetRandomSatisfiedChannel("default", "mut-model", 0); ch != nil {
		t.Fatalf("disabled channel still routable after CacheUpdateChannelStatus: %+v", ch)
	}
	// The cached channel object reflects the new status.
	if cached, err := CacheGetChannel(8201); err != nil || cached.Status != common.ChannelStatusManuallyDisabled {
		t.Fatalf("CacheUpdateChannelStatus did not mutate cached status: %+v, %v", cached, err)
	}

	// CacheUpdateChannel replaces the cached object.
	replacement := &Channel{Id: 8201, Type: 1, Status: common.ChannelStatusEnabled, Name: "replaced", Models: "mut-model", Group: "default", TenantId: "default"}
	CacheUpdateChannel(replacement)
	if cached, err := CacheGetChannel(8201); err != nil || cached.Name != "replaced" {
		t.Fatalf("CacheUpdateChannel did not replace cached object: %+v, %v", cached, err)
	}
	// nil channel is a safe no-op.
	CacheUpdateChannel(nil)
}

func TestChannelMemoryCache_DisabledFlagFallsBackToDB_PG(t *testing.T) {
	SetupTestDB(t)
	// MemoryCacheEnabled stays false → getters must hit the DB directly.
	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	defer func() { common.MemoryCacheEnabled = prev }()

	seedCacheChannel(t, 8301, "default", "db-model", common.ChannelStatusEnabled)

	got, err := CacheGetChannel(8301)
	if err != nil || got == nil || got.Id != 8301 {
		t.Fatalf("DB-fallback CacheGetChannel: %+v, %v", got, err)
	}
	info, err := CacheGetChannelInfo(8301)
	if err != nil || info == nil {
		t.Fatalf("DB-fallback CacheGetChannelInfo: %v", err)
	}
	// Mutators are no-ops when memory cache is off (must not panic).
	CacheUpdateChannelStatus(8301, common.ChannelStatusManuallyDisabled)
	CacheUpdateChannel(got)
}

func TestSyncChannelCacheWithContext_Cancel(t *testing.T) {
	SetupTestDB(t)
	withMemoryCache(t)
	seedCacheChannel(t, 8401, "default", "sync-model", common.ChannelStatusEnabled)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		SyncChannelCacheWithContext(ctx, 1)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SyncChannelCacheWithContext did not exit on cancel")
	}
}

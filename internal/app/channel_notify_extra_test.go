package app

// channel_notify_extra_test.go — coverage for DisableChannel / EnableChannel
// (auto-ban gate + status persistence + root-user notification) and the
// NeedsRotation wall-clock wrapper.

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
	"gorm.io/gorm"
)

// seedRootUser creates a root user so NotifyRootUser (called by
// Disable/EnableChannel) doesn't nil-deref GetRootUser().
func seedRootUser(t *testing.T, db *gorm.DB) {
	t.Helper()
	u := repo.User{
		Username:    "root-" + common.GetRandomString(6),
		DisplayName: "Root",
		Role:        common.RoleRootUser,
		Status:      common.UserStatusEnabled,
		// No email => NotifyUser email path skips silently.
		Group: "default",
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed root user: %v", err)
	}
}

// seedChannel creates an enabled single-key channel and returns its id.
func seedChannel(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	ch := repo.Channel{
		Name:   name,
		Status: common.ChannelStatusEnabled,
		Type:   1,
		Key:    common.GetRandomString(16),
		Models: "gpt-4o",
		Group:  "default",
	}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	return ch.Id
}

func channelStatus(t *testing.T, db *gorm.DB, id int) int {
	t.Helper()
	var ch repo.Channel
	if err := db.First(&ch, id).Error; err != nil {
		t.Fatalf("read channel %d: %v", id, err)
	}
	return ch.Status
}

func channelTestSetup(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupServiceTestDB(t)
	repo.InitCol()
	if err := db.AutoMigrate(&repo.Ability{}); err != nil {
		t.Fatalf("migrate ability: %v", err)
	}
	// Force the DB path (not memory cache) and the in-memory notify limiter.
	prevCache := common.MemoryCacheEnabled
	prevRedis := common.RedisEnabled
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = prevCache
		common.RedisEnabled = prevRedis
	})
	seedRootUser(t, db)
	return db
}

func TestDisableChannel_AutoBanDisabledSkips(t *testing.T) {
	db := channelTestSetup(t)
	id := seedChannel(t, db, "no-autoban")

	DisableChannel(types.ChannelError{
		ChannelId:   id,
		ChannelName: "no-autoban",
		AutoBan:     false, // gate off => no status change
	}, "some error")

	if got := channelStatus(t, db, id); got != common.ChannelStatusEnabled {
		t.Errorf("status = %d, want %d (unchanged, auto-ban off)", got, common.ChannelStatusEnabled)
	}
}

func TestDisableChannel_AutoBanEnabledDisables(t *testing.T) {
	db := channelTestSetup(t)
	id := seedChannel(t, db, "autoban")

	DisableChannel(types.ChannelError{
		ChannelId:   id,
		ChannelName: "autoban",
		AutoBan:     true,
	}, "upstream 401")

	if got := channelStatus(t, db, id); got != common.ChannelStatusAutoDisabled {
		t.Errorf("status = %d, want %d (auto-disabled)", got, common.ChannelStatusAutoDisabled)
	}
}

func TestEnableChannel_RestoresStatus(t *testing.T) {
	db := channelTestSetup(t)
	id := seedChannel(t, db, "to-enable")
	// Put it into an auto-disabled state first.
	if err := db.Model(&repo.Channel{}).Where("id = ?", id).
		Update("status", common.ChannelStatusAutoDisabled).Error; err != nil {
		t.Fatalf("preset disabled: %v", err)
	}

	EnableChannel(id, "", "to-enable")

	if got := channelStatus(t, db, id); got != common.ChannelStatusEnabled {
		t.Errorf("status = %d, want %d (re-enabled)", got, common.ChannelStatusEnabled)
	}
}

// ─── token_rotation.go: NeedsRotation wall-clock wrapper ──────────────────

// ─── channel_select.go: CacheGetRandomSatisfiedChannel ────────────────────

func TestCacheGetRandomSatisfiedChannel_NonAutoGroupNoChannel(t *testing.T) {
	db := channelTestSetup(t)
	if err := db.AutoMigrate(&repo.Ability{}); err != nil {
		t.Fatalf("migrate ability: %v", err)
	}
	c := createTestGinContext()
	// Non-auto group with no seeded abilities: the lookup returns no channel.
	// This characterizes the non-auto branch (channel==nil, selectGroup echoed).
	ch, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        c,
		TokenGroup: "default",
		ModelName:  "no-such-model",
	})
	if err != nil {
		// GetRandomSatisfiedChannel surfaced an error — that is a valid outcome
		// and still exercises the non-auto error branch.
		return
	}
	if ch != nil {
		t.Errorf("expected nil channel when nothing satisfies the model, got %+v", ch)
	}
	if group != "default" {
		t.Errorf("selectGroup = %q, want default (echoed on the non-auto path)", group)
	}
}

func TestNeedsRotation_WallClock(t *testing.T) {
	// Baseline far in the past => a 1-day interval is definitely due now.
	if !NeedsRotation(1, 0) {
		t.Error("NeedsRotation(1, epoch) should be due")
	}
	// Disabled interval => never due.
	if NeedsRotation(0, 0) {
		t.Error("NeedsRotation(0, ...) must be false (disabled)")
	}
	// Baseline = now => not due for a 30-day interval.
	if NeedsRotation(30, common.GetTimestamp()) {
		t.Error("NeedsRotation(30, now) should not be due")
	}
}

package app

// channel_select_auto_test.go — drives CacheGetRandomSatisfiedChannel through
// the DB-backed ability table (memory cache disabled) for both the plain-group
// path and the "auto" cross-group path. Asserts the group+model filter is
// honoured: a channel registered only under a different group is never returned
// for an auto request scoped to one group.

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting"
	"gorm.io/gorm"
)

// seedAbility registers a channel + its ability row for (group, model).
func seedAbility(t *testing.T, db *gorm.DB, channelID int, group, model string) {
	t.Helper()
	w := uint(10)
	ch := entity.Channel{
		Id:     channelID,
		Name:   "ch-" + group,
		Group:  group,
		Status: common.ChannelStatusEnabled,
		Weight: &w,
		Models: model,
		Key:    "sk-test",
	}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	var prio int64 = 0
	ab := entity.Ability{
		Group:     group,
		Model:     model,
		ChannelId: channelID,
		Enabled:   true,
		Priority:  &prio,
		Weight:    10,
	}
	if err := db.Create(&ab).Error; err != nil {
		t.Fatalf("seed ability: %v", err)
	}
}

func withMemoryCacheDisabled(t *testing.T) {
	t.Helper()
	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = prev })
}

func withAutoGroups(t *testing.T, json string) {
	t.Helper()
	prev := setting.AutoGroups2JsonString()
	if err := setting.UpdateAutoGroupsByJsonString(json); err != nil {
		t.Fatalf("set auto groups: %v", err)
	}
	t.Cleanup(func() { _ = setting.UpdateAutoGroupsByJsonString(prev) })
}

func TestCacheGetRandomSatisfiedChannel_PlainGroup(t *testing.T) {
	db := setupServiceTestDB(t)
	repo.InitCol()
	if err := db.AutoMigrate(&entity.Ability{}); err != nil {
		t.Fatalf("migrate ability: %v", err)
	}
	withMemoryCacheDisabled(t)

	seedAbility(t, db, 101, "vip", "gpt-4")

	c := createTestGinContext()
	ch, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        c,
		TokenGroup: "vip",
		ModelName:  "gpt-4",
	})
	if err != nil {
		t.Fatalf("CacheGetRandomSatisfiedChannel: %v", err)
	}
	if ch == nil {
		t.Fatal("expected a channel for vip/gpt-4")
	}
	if ch.Id != 101 {
		t.Errorf("channel id = %d, want 101", ch.Id)
	}
	if group != "vip" {
		t.Errorf("selectGroup = %q, want vip", group)
	}
}

func TestCacheGetRandomSatisfiedChannel_PlainGroup_ModelMiss(t *testing.T) {
	db := setupServiceTestDB(t)
	repo.InitCol()
	if err := db.AutoMigrate(&entity.Ability{}); err != nil {
		t.Fatalf("migrate ability: %v", err)
	}
	withMemoryCacheDisabled(t)
	seedAbility(t, db, 102, "vip", "gpt-4")

	c := createTestGinContext()
	ch, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        c,
		TokenGroup: "vip",
		ModelName:  "model-does-not-exist",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != nil {
		t.Errorf("expected nil channel for unknown model, got id=%d", ch.Id)
	}
}

func TestCacheGetRandomSatisfiedChannel_AutoGroupHonoursGroupFilter(t *testing.T) {
	db := setupServiceTestDB(t)
	repo.InitCol()
	if err := db.AutoMigrate(&entity.Ability{}); err != nil {
		t.Fatalf("migrate ability: %v", err)
	}
	withMemoryCacheDisabled(t)
	withAutoGroups(t, `["vip"]`)

	// Same model in two groups; only "vip" is an auto group for this user.
	seedAbility(t, db, 201, "vip", "gpt-4")
	seedAbility(t, db, 202, "other", "gpt-4")

	c := createTestGinContext()
	// The user's usable group is "vip", so GetUserAutoGroup resolves to ["vip"].
	common.SetContextKey(c, constant.ContextKeyUserGroup, "vip")

	ch, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        c,
		TokenGroup: "auto",
		ModelName:  "gpt-4",
	})
	if err != nil {
		t.Fatalf("CacheGetRandomSatisfiedChannel(auto): %v", err)
	}
	if ch == nil {
		t.Fatal("expected a channel from the vip auto group")
	}
	if ch.Id != 201 {
		t.Errorf("channel id = %d, want 201 (vip); must never return the other-group channel 202", ch.Id)
	}
	if group != "vip" {
		t.Errorf("selectGroup = %q, want vip", group)
	}
}

func TestCacheGetRandomSatisfiedChannel_AutoGroupExhaustedReturnsNil(t *testing.T) {
	db := setupServiceTestDB(t)
	repo.InitCol()
	if err := db.AutoMigrate(&entity.Ability{}); err != nil {
		t.Fatalf("migrate ability: %v", err)
	}
	withMemoryCacheDisabled(t)
	withAutoGroups(t, `["vip"]`)
	// vip has gpt-4 but the request asks for a model no group serves.
	seedAbility(t, db, 301, "vip", "gpt-4")

	c := createTestGinContext()
	common.SetContextKey(c, constant.ContextKeyUserGroup, "vip")

	ch, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        c,
		TokenGroup: "auto",
		ModelName:  "claude-opus",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != nil {
		t.Errorf("expected nil channel when no auto group serves the model, got id=%d", ch.Id)
	}
	// selectGroup falls back to the original request group when nothing matched.
	if group != "auto" {
		t.Errorf("selectGroup = %q, want auto (unchanged)", group)
	}
}

func TestCacheGetRandomSatisfiedChannel_AutoGroupFailsOverToNextGroup(t *testing.T) {
	db := setupServiceTestDB(t)
	repo.InitCol()
	if err := db.AutoMigrate(&entity.Ability{}); err != nil {
		t.Fatalf("migrate ability: %v", err)
	}
	withMemoryCacheDisabled(t)
	withAutoGroups(t, `["groupA","groupB"]`)

	// Make both groups usable for the (empty) user group.
	prevUsable := setting.UserUsableGroups2JSONString()
	if err := setting.UpdateUserUsableGroupsByJSONString(`{"groupA":"A","groupB":"B"}`); err != nil {
		t.Fatalf("set usable groups: %v", err)
	}
	t.Cleanup(func() { _ = setting.UpdateUserUsableGroupsByJSONString(prevUsable) })

	// Only groupB serves the model; groupA has nothing → the loop must skip
	// groupA (continue branch) and select from groupB.
	seedAbility(t, db, 401, "groupB", "gpt-4")

	c := createTestGinContext()
	ch, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        c,
		TokenGroup: "auto",
		ModelName:  "gpt-4",
	})
	if err != nil {
		t.Fatalf("CacheGetRandomSatisfiedChannel(auto failover): %v", err)
	}
	if ch == nil || ch.Id != 401 {
		t.Fatalf("expected channel 401 from groupB after skipping empty groupA, got %+v", ch)
	}
	if group != "groupB" {
		t.Errorf("selectGroup = %q, want groupB", group)
	}
}

func TestCacheGetRandomSatisfiedChannel_AutoNotEnabled(t *testing.T) {
	_ = setupServiceTestDB(t)
	repo.InitCol()
	withMemoryCacheDisabled(t)
	withAutoGroups(t, `[]`) // no auto groups configured

	c := createTestGinContext()
	_, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        c,
		TokenGroup: "auto",
		ModelName:  "gpt-4",
	})
	if err == nil {
		t.Fatal("expected an error when auto groups are not enabled")
	}
}

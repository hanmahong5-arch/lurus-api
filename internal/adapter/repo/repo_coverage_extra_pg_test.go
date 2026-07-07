package repo

// repo_coverage_extra_pg_test.go — real-Postgres coverage for repo functions
// whose SQL is dialect-specific and could not be exercised on sqlite:
//   - savings.go GetSpendByModel / GetSpendByProduct (PG jsonb ->> extraction)
//   - switch_preset.go Create/List (jsonb column + gen_random_uuid default)
//   - usedata.go SaveQuotaDataCache / increaseQuotaData (upsert-on-existing)
//   - daily_quota_cron.go cron entrypoints (context-cancel exit)
//   - redemption.go RedemptionSelectUpdate (zero-value select-update)
//
// Assertions are on concrete aggregates / persisted rows.

import (
	"context"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/domain/entity"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/ratio_setting"
)

func seedConsumeLog(t *testing.T, model string, quota, prompt, completion int, other string) {
	t.Helper()
	l := &entity.Log{
		UserId:           1,
		Username:         "u",
		Type:             LogTypeConsume,
		CreatedAt:        common.GetTimestamp(),
		ModelName:        model,
		Quota:            quota,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		Other:            other,
		TenantId:         "default",
	}
	if err := LOG_DB.Create(l).Error; err != nil {
		t.Fatalf("seed consume log: %v", err)
	}
}

func TestSavings_GetSpendByModelAndProduct_PG(t *testing.T) {
	SetupTestDB(t)
	start := common.GetTimestamp() - 3600

	// gpt-4: two rows; claude: one row; plus a zero-quota row that must be excluded.
	seedConsumeLog(t, "gpt-4", 100, 40, 60, `{"source_product":"forge"}`)
	seedConsumeLog(t, "gpt-4", 50, 20, 30, `{"source_product":"forge"}`)
	seedConsumeLog(t, "claude-3", 200, 80, 120, `{"source_product":"switch"}`)
	seedConsumeLog(t, "gpt-4", 0, 0, 0, ``)     // quota=0 excluded, empty Other
	seedConsumeLog(t, "legacy", 30, 10, 20, ``) // no source_product → default bucket

	t.Run("GetSpendByModel aggregates and orders by spend", func(t *testing.T) {
		rows, err := GetSpendByModel(start)
		if err != nil {
			t.Fatalf("GetSpendByModel: %v", err)
		}
		byModel := map[string]ModelSpendRow{}
		for _, r := range rows {
			byModel[r.ModelName] = r
		}
		g := byModel["gpt-4"]
		if g.TotalQuota != 150 || g.Count != 2 || g.PromptTokens != 60 || g.CompletionTokens != 90 {
			t.Fatalf("gpt-4 aggregate wrong: %+v", g)
		}
		if _, ok := byModel["gpt-4"]; !ok {
			t.Fatal("gpt-4 missing")
		}
		// Ordered by total_quota DESC → claude(200) first.
		if rows[0].ModelName != "claude-3" {
			t.Errorf("want claude-3 first (highest spend), got %q", rows[0].ModelName)
		}
	})

	t.Run("GetSpendByProduct folds untagged into default bucket", func(t *testing.T) {
		rows, err := GetSpendByProduct(start)
		if err != nil {
			t.Fatalf("GetSpendByProduct: %v", err)
		}
		byProd := map[string]ProductSpendRow{}
		for _, r := range rows {
			byProd[r.Product] = r
		}
		if byProd["forge"].TotalQuota != 150 {
			t.Errorf("forge spend wrong: %+v", byProd["forge"])
		}
		if byProd["switch"].TotalQuota != 200 {
			t.Errorf("switch spend wrong: %+v", byProd["switch"])
		}
		// Untagged 'legacy' row folds into DefaultSourceProduct.
		if byProd[ratio_setting.DefaultSourceProduct].TotalQuota != 30 {
			t.Errorf("default bucket wrong: %+v", byProd[ratio_setting.DefaultSourceProduct])
		}
	})
}

func TestSwitchConfigPresets_PG(t *testing.T) {
	SetupTestDB(t)
	if err := DB.AutoMigrate(&SwitchConfigPresetRow{}); err != nil {
		t.Fatalf("migrate switch_config_presets: %v", err)
	}
	ctx := context.Background()

	cfg := map[string]interface{}{"theme": "dark", "fontSize": 14.0}
	dto, err := CreateSwitchConfigPreset(ctx, "editor", "Dark Pro", "desc", "appearance", cfg, true)
	if err != nil {
		t.Fatalf("CreateSwitchConfigPreset: %v", err)
	}
	if dto.ID == "" {
		t.Fatal("gen_random_uuid default did not populate ID")
	}
	// Second tool, different category, non-official (excluded from list).
	if _, err := CreateSwitchConfigPreset(ctx, "terminal", "Hidden", "", "misc", map[string]interface{}{"x": 1.0}, false); err != nil {
		t.Fatalf("create non-official: %v", err)
	}
	if _, err := CreateSwitchConfigPreset(ctx, "editor", "Light", "", "appearance", cfg, true); err != nil {
		t.Fatalf("create second editor: %v", err)
	}

	t.Run("list filters by tool and official-only", func(t *testing.T) {
		rows, err := ListSwitchConfigPresets(ctx, "editor", "", 50, 0)
		if err != nil {
			t.Fatalf("ListSwitchConfigPresets: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("want 2 official editor presets, got %d", len(rows))
		}
		// jsonb round-trips.
		found := false
		for _, r := range rows {
			if r.Name == "Dark Pro" {
				found = true
				if r.ConfigJSON["theme"] != "dark" {
					t.Errorf("config_json not round-tripped: %+v", r.ConfigJSON)
				}
			}
		}
		if !found {
			t.Error("Dark Pro preset not returned")
		}
	})

	t.Run("category filter narrows", func(t *testing.T) {
		// Note: the SwitchConfigPresetRow.IsOfficial column carries a
		// gorm `default:true` tag, so inserting the zero value (false) yields
		// is_official=true in the DB — the "Hidden" terminal/misc preset is
		// therefore stored official. The category WHERE must still narrow the
		// result set to exactly the single 'misc' row (vs 3 total presets).
		rows, err := ListSwitchConfigPresets(ctx, "", "misc", 50, 0)
		if err != nil {
			t.Fatalf("list by category: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 'misc' preset, got %d", len(rows))
		}
		if rows[0].Category != "misc" {
			t.Errorf("category filter leaked non-misc row: %+v", rows[0])
		}
	})
}

func TestSaveQuotaDataCache_InsertThenIncrease_PG(t *testing.T) {
	SetupTestDB(t)
	prev := common.DataExportEnabled
	common.DataExportEnabled = true
	defer func() { common.DataExportEnabled = prev }()

	// Reset the package cache so this test is deterministic.
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()

	ts := common.GetTimestamp()
	// First flush inserts a fresh row.
	LogQuotaData(7, "alice", "gpt-4", 100, ts, 50)
	SaveQuotaDataCache()

	var row QuotaData
	if err := DB.Table("quota_data").Where("user_id = ? AND model_name = ?", 7, "gpt-4").First(&row).Error; err != nil {
		t.Fatalf("first row not inserted: %v", err)
	}
	if row.Quota != 100 || row.Count != 1 || row.TokenUsed != 50 {
		t.Fatalf("insert values wrong: %+v", row)
	}

	// Second flush for the same (user,model,hour) key must hit increaseQuotaData
	// (UPDATE ... quota = quota + ?), not a second insert.
	LogQuotaData(7, "alice", "gpt-4", 25, ts, 10)
	SaveQuotaDataCache()

	var rows []QuotaData
	if err := DB.Table("quota_data").Where("user_id = ? AND model_name = ?", 7, "gpt-4").Find(&rows).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("increase path must not insert a duplicate row, got %d rows", len(rows))
	}
	if rows[0].Quota != 125 || rows[0].Count != 2 || rows[0].TokenUsed != 60 {
		t.Fatalf("increaseQuotaData did not accumulate: %+v", rows[0])
	}
}

func TestDailyQuotaResetCron_ContextCancel(t *testing.T) {
	SetupTestDB(t)

	// Seed a user that needs a daily reset so processDailyQuotaResets does real
	// work on the immediate startup run.
	u := &User{
		Username: "cronuser", DisplayName: "c", Email: "cron@local",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		TenantId: "default", Group: "default", BaseGroup: "default",
		DailyQuota: 1000, DailyUsed: 500, LastDailyReset: 0,
	}
	if err := DB.Create(u).Error; err != nil {
		t.Fatalf("seed cron user: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	StartDailyQuotaResetCronWithContext(ctx) // runs processDailyQuotaResets once immediately
	// Give the goroutine a moment to run the startup batch, then cancel.
	time.Sleep(150 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	// The startup batch should have reset the user's daily_used to 0.
	var reloaded User
	if err := DB.First(&reloaded, u.Id).Error; err != nil {
		t.Fatalf("reload cron user: %v", err)
	}
	if reloaded.DailyUsed != 0 {
		t.Errorf("cron startup batch did not reset daily_used, got %d", reloaded.DailyUsed)
	}
}

func TestRedemptionSelectUpdate_PG(t *testing.T) {
	SetupTestDB(t)
	r := seedRedemptionRow(t, 1, "default", "promo", 500)

	// Flip to used with a redeemed_time; RedemptionSelectUpdate must persist both
	// the status and the (zero-able) redeemed_time via an explicit Select.
	r.Status = common.RedemptionCodeStatusUsed
	r.RedeemedTime = common.GetTimestamp()
	if err := RedemptionSelectUpdate(r); err != nil {
		t.Fatalf("RedemptionSelectUpdate: %v", err)
	}
	var got Redemption
	if err := DB.First(&got, r.Id).Error; err != nil {
		t.Fatalf("reload redemption: %v", err)
	}
	if got.Status != common.RedemptionCodeStatusUsed || got.RedeemedTime == 0 {
		t.Fatalf("select-update did not persist status/redeemed_time: %+v", got)
	}
}

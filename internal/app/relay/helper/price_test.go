package helper

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/operation_setting"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/ratio_setting"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"

	"github.com/gin-gonic/gin"
)

func priceCtx() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c
}

// seedRatios installs deterministic ratio/price/group tables for the duration of a
// test, restoring the previous ones afterwards so other tests are unaffected.
func seedRatios(t *testing.T, modelRatio, modelPrice, groupRatio string, groupGroup map[string]map[string]float64) {
	t.Helper()
	prevRatio := ratio_setting.ModelRatio2JSONString()
	prevPrice := ratio_setting.ModelPrice2JSONString()
	prevGroup := ratio_setting.GroupRatio2JSONString()
	prevGroupGroup := ratio_setting.GroupGroupRatio

	if err := ratio_setting.UpdateModelRatioByJSONString(modelRatio); err != nil {
		t.Fatalf("seed model ratio: %v", err)
	}
	if err := ratio_setting.UpdateModelPriceByJSONString(modelPrice); err != nil {
		t.Fatalf("seed model price: %v", err)
	}
	if err := ratio_setting.UpdateGroupRatioByJSONString(groupRatio); err != nil {
		t.Fatalf("seed group ratio: %v", err)
	}
	ratio_setting.GroupGroupRatio = groupGroup

	t.Cleanup(func() {
		_ = ratio_setting.UpdateModelRatioByJSONString(prevRatio)
		_ = ratio_setting.UpdateModelPriceByJSONString(prevPrice)
		_ = ratio_setting.UpdateGroupRatioByJSONString(prevGroup)
		ratio_setting.GroupGroupRatio = prevGroupGroup
	})
}

func TestHandleGroupRatio(t *testing.T) {
	t.Run("normal group ratio", func(t *testing.T) {
		seedRatios(t, `{}`, `{}`, `{"default":1.5}`, map[string]map[string]float64{})
		info := &relaycommon.RelayInfo{UsingGroup: "default", UserGroup: "u"}
		gri := HandleGroupRatio(priceCtx(), info)
		if gri.GroupRatio != 1.5 {
			t.Errorf("GroupRatio = %v, want 1.5", gri.GroupRatio)
		}
		if gri.HasSpecialRatio {
			t.Error("HasSpecialRatio should be false")
		}
	})

	t.Run("user special group-group ratio wins", func(t *testing.T) {
		seedRatios(t, `{}`, `{}`, `{"default":1.5}`,
			map[string]map[string]float64{"vip": {"default": 0.5}})
		info := &relaycommon.RelayInfo{UsingGroup: "default", UserGroup: "vip"}
		gri := HandleGroupRatio(priceCtx(), info)
		if !gri.HasSpecialRatio {
			t.Fatal("HasSpecialRatio should be true")
		}
		if gri.GroupRatio != 0.5 || gri.GroupSpecialRatio != 0.5 {
			t.Errorf("special ratio = %v/%v, want 0.5", gri.GroupRatio, gri.GroupSpecialRatio)
		}
	})

	t.Run("auto_group overrides using group", func(t *testing.T) {
		seedRatios(t, `{}`, `{}`, `{"auto":2.0,"default":1.0}`, map[string]map[string]float64{})
		c := priceCtx()
		c.Set("auto_group", "auto")
		info := &relaycommon.RelayInfo{UsingGroup: "default", UserGroup: "u"}
		gri := HandleGroupRatio(c, info)
		if info.UsingGroup != "auto" {
			t.Errorf("UsingGroup = %q, want auto", info.UsingGroup)
		}
		if gri.GroupRatio != 2.0 {
			t.Errorf("GroupRatio = %v, want 2.0", gri.GroupRatio)
		}
	})
}

func TestModelPriceHelper_RatioBased(t *testing.T) {
	seedRatios(t, `{"ratio-model":2.0}`, `{}`, `{"default":1.5}`, map[string]map[string]float64{})
	info := &relaycommon.RelayInfo{OriginModelName: "ratio-model", UsingGroup: "default"}
	pd, err := ModelPriceHelper(priceCtx(), info, 1000, &types.TokenCountMeta{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pd.UsePrice {
		t.Error("UsePrice should be false for ratio-based model")
	}
	if pd.ModelRatio != 2.0 {
		t.Errorf("ModelRatio = %v, want 2.0", pd.ModelRatio)
	}
	if pd.GroupRatioInfo.GroupRatio != 1.5 {
		t.Errorf("GroupRatio = %v, want 1.5", pd.GroupRatioInfo.GroupRatio)
	}
	// preConsumedTokens = max(1000, 500) = 1000; ratio = 2.0*1.5 = 3.0 -> 3000
	if pd.QuotaToPreConsume != 3000 {
		t.Errorf("QuotaToPreConsume = %d, want 3000", pd.QuotaToPreConsume)
	}
	// relayInfo.PriceData is populated as a side effect
	if info.PriceData.ModelRatio != 2.0 {
		t.Errorf("info.PriceData not populated: %v", info.PriceData.ModelRatio)
	}
}

func TestModelPriceHelper_RatioMaxTokens(t *testing.T) {
	seedRatios(t, `{"ratio-model":1.0}`, `{}`, `{"default":1.0}`, map[string]map[string]float64{})
	info := &relaycommon.RelayInfo{OriginModelName: "ratio-model", UsingGroup: "default"}
	// promptTokens 100 -> max(100,500)=500; +MaxTokens 200 = 700; ratio 1.0 -> 700
	pd, err := ModelPriceHelper(priceCtx(), info, 100, &types.TokenCountMeta{MaxTokens: 200})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pd.QuotaToPreConsume != 700 {
		t.Errorf("QuotaToPreConsume = %d, want 700", pd.QuotaToPreConsume)
	}
}

func TestModelPriceHelper_PriceBased(t *testing.T) {
	seedRatios(t, `{}`, `{"price-model":0.5}`, `{"default":1.5}`, map[string]map[string]float64{})
	info := &relaycommon.RelayInfo{OriginModelName: "price-model", UsingGroup: "default"}
	pd, err := ModelPriceHelper(priceCtx(), info, 100, &types.TokenCountMeta{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !pd.UsePrice {
		t.Error("UsePrice should be true")
	}
	if pd.ModelPrice != 0.5 {
		t.Errorf("ModelPrice = %v, want 0.5", pd.ModelPrice)
	}
	// 0.5 * QuotaPerUnit(500000) * 1.5 = 375000
	if pd.QuotaToPreConsume != 375000 {
		t.Errorf("QuotaToPreConsume = %d, want 375000", pd.QuotaToPreConsume)
	}
}

func TestModelPriceHelper_PriceWithImageRatio(t *testing.T) {
	seedRatios(t, `{}`, `{"price-model":0.5}`, `{"default":1.0}`, map[string]map[string]float64{})
	info := &relaycommon.RelayInfo{OriginModelName: "price-model", UsingGroup: "default"}
	pd, err := ModelPriceHelper(priceCtx(), info, 100, &types.TokenCountMeta{ImagePriceRatio: 2.0})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// modelPrice 0.5 * imageRatio 2 = 1.0 -> 1.0*500000*1.0 = 500000
	if pd.ModelPrice != 1.0 {
		t.Errorf("ModelPrice = %v, want 1.0", pd.ModelPrice)
	}
	if pd.QuotaToPreConsume != 500000 {
		t.Errorf("QuotaToPreConsume = %d, want 500000", pd.QuotaToPreConsume)
	}
}

func TestModelPriceHelper_UnsetRatioRejected(t *testing.T) {
	seedRatios(t, `{}`, `{}`, `{"default":1.0}`, map[string]map[string]float64{})
	info := &relaycommon.RelayInfo{OriginModelName: "unknown-family-zzz-9999", UsingGroup: "default"}
	_, err := ModelPriceHelper(priceCtx(), info, 100, &types.TokenCountMeta{})
	if err == nil {
		t.Fatal("expected unset-ratio error")
	}
}

func TestModelPriceHelper_UnsetRatioAcceptedInSelfUse(t *testing.T) {
	seedRatios(t, `{}`, `{}`, `{"default":1.0}`, map[string]map[string]float64{})
	info := &relaycommon.RelayInfo{
		OriginModelName: "unknown-family-zzz-9999",
		UsingGroup:      "default",
		UserSetting:     dto.UserSetting{AcceptUnsetRatioModel: true},
	}
	if _, err := ModelPriceHelper(priceCtx(), info, 100, &types.TokenCountMeta{}); err != nil {
		t.Fatalf("self-use should accept unset ratio, got %v", err)
	}
}

func TestModelPriceHelper_FreeModel(t *testing.T) {
	// disable free-model pre-consume so a zero group ratio zeroes the quota
	qs := operation_setting.GetQuotaSetting()
	prev := qs.EnableFreeModelPreConsume
	qs.EnableFreeModelPreConsume = false
	t.Cleanup(func() { qs.EnableFreeModelPreConsume = prev })

	seedRatios(t, `{"ratio-model":2.0}`, `{}`, `{"free":0}`, map[string]map[string]float64{})
	info := &relaycommon.RelayInfo{OriginModelName: "ratio-model", UsingGroup: "free"}
	pd, err := ModelPriceHelper(priceCtx(), info, 100, &types.TokenCountMeta{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !pd.FreeModel {
		t.Error("FreeModel should be true when group ratio is 0")
	}
	if pd.QuotaToPreConsume != 0 {
		t.Errorf("QuotaToPreConsume = %d, want 0", pd.QuotaToPreConsume)
	}
}

func TestModelPriceHelper_FreeByZeroPrice(t *testing.T) {
	qs := operation_setting.GetQuotaSetting()
	prev := qs.EnableFreeModelPreConsume
	qs.EnableFreeModelPreConsume = false
	t.Cleanup(func() { qs.EnableFreeModelPreConsume = prev })

	seedRatios(t, `{}`, `{"price-model":0}`, `{"default":1.0}`, map[string]map[string]float64{})
	info := &relaycommon.RelayInfo{OriginModelName: "price-model", UsingGroup: "default"}
	pd, err := ModelPriceHelper(priceCtx(), info, 100, &types.TokenCountMeta{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !pd.FreeModel {
		t.Error("FreeModel should be true when price is 0")
	}
}

func TestModelPriceHelperPerCall(t *testing.T) {
	t.Run("configured per-call price", func(t *testing.T) {
		seedRatios(t, `{}`, `{"mj-model":0.2}`, `{"default":1.0}`, map[string]map[string]float64{})
		info := &relaycommon.RelayInfo{OriginModelName: "mj-model", UsingGroup: "default"}
		pd := ModelPriceHelperPerCall(priceCtx(), info)
		if pd.ModelPrice != 0.2 {
			t.Errorf("ModelPrice = %v, want 0.2", pd.ModelPrice)
		}
		// 0.2 * 500000 * 1.0 = 100000
		if pd.Quota != 100000 {
			t.Errorf("Quota = %d, want 100000", pd.Quota)
		}
	})

	t.Run("fallback to hardcoded default when unconfigured", func(t *testing.T) {
		seedRatios(t, `{}`, `{}`, `{"default":1.0}`, map[string]map[string]float64{})
		info := &relaycommon.RelayInfo{OriginModelName: "totally-unknown-percall-xyz", UsingGroup: "default"}
		pd := ModelPriceHelperPerCall(priceCtx(), info)
		if pd.ModelPrice != 0.1 {
			t.Errorf("ModelPrice = %v, want 0.1 default", pd.ModelPrice)
		}
		if pd.Quota != 50000 {
			t.Errorf("Quota = %d, want 50000", pd.Quota)
		}
	})
}

func TestContainPriceOrRatio(t *testing.T) {
	seedRatios(t, `{"has-ratio":1.0}`, `{"has-price":0.3}`, `{"default":1.0}`, map[string]map[string]float64{})
	if !ContainPriceOrRatio("has-price") {
		t.Error("has-price should be found")
	}
	if !ContainPriceOrRatio("has-ratio") {
		t.Error("has-ratio should be found")
	}
	if ContainPriceOrRatio("neither-configured-zzz-9999") {
		t.Error("unconfigured model should not be found")
	}
}

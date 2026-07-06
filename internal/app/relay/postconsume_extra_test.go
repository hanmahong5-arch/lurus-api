package relay

import (
	"net/http"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

// TestPostConsumeQuota_ResponsesToolsAndTokenDetails exercises the token-based
// pricing path with cache/image/cached-creation/audio token details, responses
// web-search + file-search tool billing, image-generation-call billing, claude
// web-search billing, per-key OtherRatios, and gpt-4o-gizmo log-model rewriting.
// The observable contract: a positive quota is computed and debited from the
// user's balance (playground path skips token settlement but still debits user).
func TestPostConsumeQuota_ResponsesToolsAndTokenDetails(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	const startQuota = 100_000_000
	u := &repo.User{Username: "pc-tools", Quota: startQuota}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	info := newRelayInfo(u.Id, 0, constant.APITypeOpenAI)
	info.OriginModelName = "gpt-4o-gizmo-custom"
	info.IsPlayground = true // skip token-quota settlement; user quota still debited
	info.UserQuota = startQuota
	info.PriceData = types.PriceData{
		ModelRatio:         2,
		CompletionRatio:    1,
		CacheRatio:         0.5,
		CacheCreationRatio: 1,
		ImageRatio:         1,
		OtherRatios:        map[string]float64{"surcharge": 2},
		GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1.5},
	}
	info.ResponsesUsageInfo = &relaycommon.ResponsesUsageInfo{
		BuiltInTools: map[string]*relaycommon.BuildInToolInfo{
			dto.BuildInToolWebSearchPreview: {CallCount: 2, SearchContextSize: "high"},
			dto.BuildInToolFileSearch:       {CallCount: 1},
		},
	}

	c, _ := newJSONContext(http.MethodPost, "/", nil)
	c.Set("token_name", "tkn")
	c.Set("image_generation_call", true)
	c.Set("image_generation_call_quality", "hd")
	c.Set("image_generation_call_size", "1024x1024")
	c.Set("claude_web_search_requests", 3)

	usage := &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}
	usage.PromptTokensDetails.CachedTokens = 100
	usage.PromptTokensDetails.ImageTokens = 50
	usage.PromptTokensDetails.CachedCreationTokens = 20
	usage.PromptTokensDetails.AudioTokens = 10

	postConsumeQuota(c, info, usage)

	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.Quota >= startQuota {
		t.Errorf("expected user quota to be debited, before=%d after=%d", startQuota, refreshed.Quota)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", refreshed.RequestCount)
	}
}

// TestPostConsumeQuota_UsePriceSearchPreviewGizmo covers the fixed-price billing
// branch (UsePrice=true), the search-preview web-search branch (model name
// suffix, no ResponsesUsageInfo), and the gpt-4-gizmo log-model rewrite.
func TestPostConsumeQuota_UsePriceSearchPreviewGizmo(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	const startQuota = 100_000_000
	u := &repo.User{Username: "pc-price", Quota: startQuota}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	info := newRelayInfo(u.Id, 0, constant.APITypeOpenAI)
	info.OriginModelName = "gpt-4-gizmo-search-preview" // gizmo prefix + search-preview suffix
	info.IsPlayground = true
	info.UserQuota = startQuota
	info.PriceData = types.PriceData{
		UsePrice:       true,
		ModelPrice:     0.02,
		GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
	}

	c, _ := newJSONContext(http.MethodPost, "/", nil)
	c.Set("token_name", "tkn")
	// No search context size set → defaults to "medium".

	usage := &dto.Usage{PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200}
	postConsumeQuota(c, info, usage)

	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.Quota >= startQuota {
		t.Errorf("expected fixed-price debit, before=%d after=%d", startQuota, refreshed.Quota)
	}
}

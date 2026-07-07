package app

// log_info_extra_test.go — coverage for the pure log "other-info" builders and
// the notify-limit cleanup lifecycle. The builders feed the consume-log Other
// column that downstream billing/analytics reads, so the field mapping is
// asserted key-by-key.

import (
	"context"
	"testing"
	"time"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

func TestGenerateMjOtherInfo_WithSpecialRatio(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{RequestURLPath: "/mj/submit/imagine?token=abc"}
	price := types.PerCallPriceData{
		ModelPrice: 0.1,
		GroupRatioInfo: types.GroupRatioInfo{
			GroupRatio:        1.5,
			GroupSpecialRatio: 0.8,
			HasSpecialRatio:   true,
		},
	}
	other := GenerateMjOtherInfo(relayInfo, price)

	if other["model_price"] != 0.1 {
		t.Errorf("model_price = %v, want 0.1", other["model_price"])
	}
	if other["group_ratio"] != 1.5 {
		t.Errorf("group_ratio = %v, want 1.5", other["group_ratio"])
	}
	if other["user_group_ratio"] != 0.8 {
		t.Errorf("user_group_ratio = %v, want 0.8 (special ratio)", other["user_group_ratio"])
	}
	// appendRequestPath must trim the query string.
	if other["request_path"] != "/mj/submit/imagine" {
		t.Errorf("request_path = %v, want /mj/submit/imagine (query trimmed)", other["request_path"])
	}
}

func TestGenerateMjOtherInfo_NoSpecialRatio(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	price := types.PerCallPriceData{
		ModelPrice:     0.2,
		GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1.0, HasSpecialRatio: false},
	}
	other := GenerateMjOtherInfo(relayInfo, price)
	if _, ok := other["user_group_ratio"]; ok {
		t.Error("user_group_ratio should be absent when HasSpecialRatio is false")
	}
}

func TestGenerateTextOtherInfo_ModelMappedAndReasoning(t *testing.T) {
	c := createTestGinContext()
	relayInfo := &relaycommon.RelayInfo{
		ReasoningEffort: "high",
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     true,
			UpstreamModelName: "gpt-4o-2024",
		},
	}
	other := GenerateTextOtherInfo(c, relayInfo, 2.0, 1.0, 3.0, 100, 0.5, 0.01, 0.9)

	if other["model_ratio"] != 2.0 || other["completion_ratio"] != 3.0 {
		t.Errorf("ratios = (%v,%v), want (2.0,3.0)", other["model_ratio"], other["completion_ratio"])
	}
	if other["reasoning_effort"] != "high" {
		t.Errorf("reasoning_effort = %v, want high", other["reasoning_effort"])
	}
	if other["is_model_mapped"] != true {
		t.Errorf("is_model_mapped = %v, want true", other["is_model_mapped"])
	}
	if other["upstream_model_name"] != "gpt-4o-2024" {
		t.Errorf("upstream_model_name = %v, want gpt-4o-2024", other["upstream_model_name"])
	}
}

func TestAcSearch_EmptyAndWhitespaceDict(t *testing.T) {
	// Empty dict => getOrBuildAC returns nil (acKey==""), so no match.
	if found, hits := AcSearch("anything", nil, false); found || len(hits) != 0 {
		t.Errorf("empty dict => (%v,%v), want (false,none)", found, hits)
	}
	// Whitespace-only entries normalize away => still no usable machine.
	if found, _ := AcSearch("anything", []string{"   ", ""}, false); found {
		t.Error("whitespace-only dict must not match")
	}
}

func TestGenerateClaudeOtherInfo_CacheCreationBuckets(t *testing.T) {
	c := createTestGinContext()
	relayInfo := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	// Non-zero 5m/1h cache-creation token counts must surface the corresponding
	// keys in the Other map (the cache-bucket branches).
	other := GenerateClaudeOtherInfo(c, relayInfo,
		3.0, 1.0, 5.0, // modelRatio, groupRatio, completionRatio
		100, 0.5, // cacheTokens, cacheRatio
		200, 2.0, // cacheCreationTokens, cacheCreationRatio
		50, 1.25, // 5m tokens/ratio
		30, 2.0, // 1h tokens/ratio
		0.01, 0.9)

	if other["cache_creation_tokens_5m"] != 50 {
		t.Errorf("cache_creation_tokens_5m = %v, want 50", other["cache_creation_tokens_5m"])
	}
	if other["cache_creation_tokens_1h"] != 30 {
		t.Errorf("cache_creation_tokens_1h = %v, want 30", other["cache_creation_tokens_1h"])
	}
	if other["cache_creation_ratio_5m"] != 1.25 {
		t.Errorf("cache_creation_ratio_5m = %v, want 1.25", other["cache_creation_ratio_5m"])
	}
}

// TestStartCleanupTaskWithContext_CancelStops proves the graceful-shutdown
// branch: a cancelled context makes the cleanup loop return promptly.
func TestStartCleanupTaskWithContext_CancelStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		startCleanupTaskWithContext(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
		// returned as expected
	case <-time.After(2 * time.Second):
		t.Fatal("startCleanupTaskWithContext did not return after context cancel")
	}
}

// TestStopNotifyLimitCleanup_NilCancelSafe proves StopNotifyLimitCleanup is a
// safe no-op path (does not panic) — the nil-cancel guard.
func TestStopNotifyLimitCleanup_NilCancelSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("StopNotifyLimitCleanup panicked: %v", r)
		}
	}()
	StopNotifyLimitCleanup()
}

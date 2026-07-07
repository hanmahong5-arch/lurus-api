package ratio_setting

import (
	"encoding/json"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/setting/operation_setting"
)

// ---------------------------------------------------------------------------
// model_ratio.go — price map
// ---------------------------------------------------------------------------

func TestModelPrice_RoundTripAndLookup(t *testing.T) {
	InitRatioSettings()

	// Seed a custom price map; wildcard gizmo entry must resolve via
	// FormatMatchingModelName prefix collapse.
	if err := UpdateModelPriceByJSONString(`{"dall-e-3":0.04,"gpt-4-gizmo-*":0.1}`); err != nil {
		t.Fatalf("UpdateModelPriceByJSONString: %v", err)
	}

	if got := GetModelPriceMap()["dall-e-3"]; got != 0.04 {
		t.Errorf("GetModelPriceMap dall-e-3 = %v, want 0.04", got)
	}

	// exact hit
	if p, ok := GetModelPrice("dall-e-3", false); !ok || p != 0.04 {
		t.Errorf("GetModelPrice(dall-e-3) = %v,%v want 0.04,true", p, ok)
	}
	// wildcard collapse: gpt-4-gizmo-anything -> gpt-4-gizmo-*
	if p, ok := GetModelPrice("gpt-4-gizmo-abc", false); !ok || p != 0.1 {
		t.Errorf("GetModelPrice(gpt-4-gizmo-abc) = %v,%v want 0.1,true", p, ok)
	}
	// miss -> -1,false (printErr true exercises the log branch)
	if p, ok := GetModelPrice("no-such-model", true); ok || p != -1 {
		t.Errorf("GetModelPrice(no-such-model) = %v,%v want -1,false", p, ok)
	}

	// JSON round-trip is valid JSON of the seeded map
	var back map[string]float64
	if err := json.Unmarshal([]byte(ModelPrice2JSONString()), &back); err != nil {
		t.Fatalf("ModelPrice2JSONString not valid JSON: %v", err)
	}
	if back["dall-e-3"] != 0.04 {
		t.Errorf("round-trip dall-e-3 = %v, want 0.04", back["dall-e-3"])
	}

	// invalid JSON -> error branch, map reset to empty
	if err := UpdateModelPriceByJSONString(`{not json`); err == nil {
		t.Error("UpdateModelPriceByJSONString(invalid) expected error, got nil")
	}

	InitRatioSettings()
}

// ---------------------------------------------------------------------------
// model_ratio.go — GetModelRatio resolution tiers
// ---------------------------------------------------------------------------

func TestGetModelRatio_ExplicitFamilyDefault(t *testing.T) {
	InitRatioSettings()

	// Save + restore operation_setting knobs mutated below.
	origMarkup := operation_setting.ModelFallbackMarkup
	origSelfUse := operation_setting.SelfUseModeEnabled
	t.Cleanup(func() {
		operation_setting.ModelFallbackMarkup = origMarkup
		operation_setting.SelfUseModeEnabled = origSelfUse
		InitRatioSettings()
	})

	// 1. explicit override wins
	if err := UpdateModelRatioByJSONString(`{"my-model":3.5}`); err != nil {
		t.Fatalf("UpdateModelRatioByJSONString: %v", err)
	}
	if r, ok, name := GetModelRatio("my-model"); !ok || r != 3.5 || name != "my-model" {
		t.Errorf("explicit GetModelRatio = %v,%v,%q want 3.5,true,my-model", r, ok, name)
	}

	// 2. family fallback with markup
	operation_setting.ModelFallbackMarkup = 2.0
	// gemini-99-flash-lite is not explicit but matches family flash-lite=0.05
	if r, ok, _ := GetModelRatio("gemini-99-flash-lite-x"); !ok || r != 0.05*2.0 {
		t.Errorf("family GetModelRatio = %v,%v want %v,true", r, ok, 0.05*2.0)
	}

	// 2b. markup <= 0 defaults to 1.0
	operation_setting.ModelFallbackMarkup = 0
	if r, ok, _ := GetModelRatio("gemini-99-flash-lite-x"); !ok || r != 0.05 {
		t.Errorf("family GetModelRatio (markup<=0) = %v,%v want 0.05,true", r, ok)
	}

	// 3. no family: self-use disabled -> ok=false, ratio 37.5
	operation_setting.SelfUseModeEnabled = false
	if r, ok, _ := GetModelRatio("totally-unknown-xyz"); ok || r != 37.5 {
		t.Errorf("unknown GetModelRatio (selfuse off) = %v,%v want 37.5,false", r, ok)
	}
	// 3b. no family: self-use enabled -> ok=true
	operation_setting.SelfUseModeEnabled = true
	if r, ok, _ := GetModelRatio("totally-unknown-xyz"); !ok || r != 37.5 {
		t.Errorf("unknown GetModelRatio (selfuse on) = %v,%v want 37.5,true", r, ok)
	}
}

func TestGetModelPricingSource_MarkupClamp(t *testing.T) {
	InitRatioSettings()
	orig := operation_setting.ModelFallbackMarkup
	t.Cleanup(func() {
		operation_setting.ModelFallbackMarkup = orig
		InitRatioSettings()
	})

	operation_setting.ModelFallbackMarkup = 0 // triggers markup<=0 -> 1.0 clamp
	ps := GetModelPricingSource("gemini-99-flash-lite-x")
	if ps.Source != "family_fallback" || ps.Markup != 1.0 || ps.Ratio != 0.05 {
		t.Errorf("PricingSource markup clamp = %+v, want family_fallback markup=1 ratio=0.05", ps)
	}
}

func TestGetModelRatioOrPrice(t *testing.T) {
	InitRatioSettings()
	origSelf := operation_setting.SelfUseModeEnabled
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = origSelf
		InitRatioSettings()
	})
	operation_setting.SelfUseModeEnabled = false

	// price path: dall-e-3 is in default price map
	if v, usePrice, exist := GetModelRatioOrPrice("dall-e-3"); !usePrice || !exist || v != 0.04 {
		t.Errorf("GetModelRatioOrPrice(dall-e-3) = %v,%v,%v want 0.04,true,true", v, usePrice, exist)
	}
	// ratio path: gpt-4o in ratio map, not price map
	if v, usePrice, exist := GetModelRatioOrPrice("gpt-4o"); usePrice || !exist || v != 1.25 {
		t.Errorf("GetModelRatioOrPrice(gpt-4o) = %v,%v,%v want 1.25,false,true", v, usePrice, exist)
	}
	// neither: unknown -> 37.5,false,false
	if v, usePrice, exist := GetModelRatioOrPrice("nope-unknown-zzz"); usePrice || exist || v != 37.5 {
		t.Errorf("GetModelRatioOrPrice(unknown) = %v,%v,%v want 37.5,false,false", v, usePrice, exist)
	}
}

// ---------------------------------------------------------------------------
// model_ratio.go — completion ratio
// ---------------------------------------------------------------------------

func TestCompletionRatio_RoundTripAndOverride(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)

	// slash-model override path is checked first
	if err := UpdateCompletionRatioByJSONString(`{"vendor/custom":9,"gpt-4o":2.2}`); err != nil {
		t.Fatalf("UpdateCompletionRatioByJSONString: %v", err)
	}
	if got := GetCompletionRatio("vendor/custom"); got != 9 {
		t.Errorf("GetCompletionRatio(vendor/custom) = %v, want 9", got)
	}
	// gpt-4o: hardcoded returns (4,false) so the map override (2.2) wins
	if got := GetCompletionRatio("gpt-4o"); got != 2.2 {
		t.Errorf("GetCompletionRatio(gpt-4o override) = %v, want 2.2", got)
	}

	if got := GetCompletionRatioMap()["vendor/custom"]; got != 9 {
		t.Errorf("GetCompletionRatioMap = %v, want 9", got)
	}
	var back map[string]float64
	if err := json.Unmarshal([]byte(CompletionRatio2JSONString()), &back); err != nil {
		t.Fatalf("CompletionRatio2JSONString invalid: %v", err)
	}
	if back["gpt-4o"] != 2.2 {
		t.Errorf("round-trip gpt-4o = %v, want 2.2", back["gpt-4o"])
	}

	if err := UpdateCompletionRatioByJSONString(`{bad`); err == nil {
		t.Error("UpdateCompletionRatioByJSONString(invalid) expected error")
	}
}

func TestGetCompletionRatio_HardcodedFallback(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)
	// empty map so hardcoded path drives the value
	if err := UpdateCompletionRatioByJSONString(`{}`); err != nil {
		t.Fatal(err)
	}
	// claude-3-* -> hardcoded 5,true
	if got := GetCompletionRatio("claude-3-opus-20240229"); got != 5 {
		t.Errorf("GetCompletionRatio(claude-3) = %v, want 5", got)
	}
	// unknown -> hardcoded default 1,false, and not in map -> returns 1
	if got := GetCompletionRatio("brand-new-unknown-model"); got != 1 {
		t.Errorf("GetCompletionRatio(unknown) = %v, want 1", got)
	}
}

func TestGetHardcodedCompletionModelRatio(t *testing.T) {
	tests := []struct {
		name       string
		wantRatio  float64
		wantContai bool
	}{
		{"gpt-4-all", 2, false},
		{"foo-gizmo-*", 2, false},
		{"gpt-4o-2024-05-13", 3, true},
		{"gpt-4o-mini-tts", 20, false},
		{"gpt-4o", 4, false},
		{"gpt-5", 8, true},
		{"gpt-4.5-preview", 2, true},
		{"gpt-4-turbo", 3, true},
		{"gpt-4-1106", 3, true},
		{"gpt-4", 2, false},
		{"o1-preview", 4, true},
		{"o3-mini", 4, true},
		{"chatgpt-4o-latest", 3, true},
		{"claude-3-haiku-20240307", 5, true},
		{"claude-sonnet-4-20250514", 5, true},
		{"claude-opus-4-20250514", 5, true},
		{"claude-haiku-4-5-20251001", 5, true},
		{"claude-instant-1", 3, true},
		{"claude-2.1", 3, true},
		// gpt-3.5-* is caught by the earlier `gpt-` prefix block -> 2,false
		// (the dedicated gpt-3.5 branch below it is unreachable dead code).
		{"gpt-3.5-turbo", 2, false},
		{"mistral-large", 3, true},
		{"gemini-1.5-pro", 4, true},
		{"gemini-2.0-flash", 4, true},
		{"gemini-2.5-pro", 8, false},
		{"gemini-2.5-flash-preview-05-20-nothinking", 4, false},
		{"gemini-2.5-flash-preview-05-20", 3.5 / 0.15, false},
		{"gemini-2.5-flash-lite-preview-06-17", 4, false},
		{"gemini-2.5-flash", 2.5 / 0.3, false},
		{"gemini-robotics-er-1.5-preview", 2.5 / 0.3, false},
		{"gemini-3-pro-image-preview", 60, false},
		{"gemini-3-pro-preview", 6, false},
		{"gemini-3-flash-preview", 4, false},
		{"gemini-3.1-flash-lite-preview", 4, false},
		{"gemini-3.1-flash-image-preview", 4, false},
		{"gemini-embedding-001", 4, false},
		{"command-r", 3, true},
		{"command-r-plus", 5, true},
		{"command-r-08-2024", 4, true},
		{"command-r-plus-08-2024", 4, true},
		{"command-light", 4, false},
		{"ERNIE-Speed-8K", 2, true},
		{"ERNIE-Lite-8K", 2, true},
		{"ERNIE-Character-8K", 2, true},
		{"ERNIE-Functions-8K", 2, true},
		{"llama2-70b-4096", 0.8 / 0.64, true},
		{"llama3-8b-8192", 2, true},
		{"llama3-70b-8192", 0.79 / 0.59, true},
		{"some-random-model", 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, c := getHardcodedCompletionModelRatio(tt.name)
			if r != tt.wantRatio || c != tt.wantContai {
				t.Errorf("getHardcodedCompletionModelRatio(%q) = %v,%v want %v,%v",
					tt.name, r, c, tt.wantRatio, tt.wantContai)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// model_ratio.go — image / audio ratios
// ---------------------------------------------------------------------------

func TestImageRatio(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)

	if err := UpdateImageRatioByJSONString(`{"gpt-image-1":2}`); err != nil {
		t.Fatalf("UpdateImageRatioByJSONString: %v", err)
	}
	if r, ok := GetImageRatio("gpt-image-1"); !ok || r != 2 {
		t.Errorf("GetImageRatio(gpt-image-1) = %v,%v want 2,true", r, ok)
	}
	if r, ok := GetImageRatio("no-image"); ok || r != 1 {
		t.Errorf("GetImageRatio(no-image) = %v,%v want 1,false", r, ok)
	}
	var back map[string]float64
	if err := json.Unmarshal([]byte(ImageRatio2JSONString()), &back); err != nil {
		t.Fatalf("ImageRatio2JSONString invalid: %v", err)
	}
	if err := UpdateImageRatioByJSONString(`{bad`); err == nil {
		t.Error("UpdateImageRatioByJSONString(invalid) expected error")
	}
}

func TestAudioRatios(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)

	if err := UpdateAudioRatioByJSONString(`{"gpt-4o-audio-preview":16}`); err != nil {
		t.Fatalf("UpdateAudioRatioByJSONString: %v", err)
	}
	if r := GetAudioRatio("gpt-4o-audio-preview"); r != 16 {
		t.Errorf("GetAudioRatio hit = %v, want 16", r)
	}
	if r := GetAudioRatio("no-audio"); r != 1 {
		t.Errorf("GetAudioRatio miss = %v, want 1", r)
	}
	if !ContainsAudioRatio("gpt-4o-audio-preview") {
		t.Error("ContainsAudioRatio(gpt-4o-audio-preview) = false, want true")
	}
	if ContainsAudioRatio("no-audio") {
		t.Error("ContainsAudioRatio(no-audio) = true, want false")
	}
	var back map[string]float64
	if err := json.Unmarshal([]byte(AudioRatio2JSONString()), &back); err != nil {
		t.Fatalf("AudioRatio2JSONString invalid: %v", err)
	}
	if err := UpdateAudioRatioByJSONString(`{bad`); err == nil {
		t.Error("UpdateAudioRatioByJSONString(invalid) expected error")
	}

	if err := UpdateAudioCompletionRatioByJSONString(`{"gpt-4o-mini-tts":1}`); err != nil {
		t.Fatalf("UpdateAudioCompletionRatioByJSONString: %v", err)
	}
	if r := GetAudioCompletionRatio("gpt-4o-mini-tts"); r != 1 {
		t.Errorf("GetAudioCompletionRatio hit = %v, want 1", r)
	}
	if r := GetAudioCompletionRatio("no-audio-c"); r != 1 {
		t.Errorf("GetAudioCompletionRatio miss = %v, want 1", r)
	}
	if !ContainsAudioCompletionRatio("gpt-4o-mini-tts") {
		t.Error("ContainsAudioCompletionRatio hit = false, want true")
	}
	if ContainsAudioCompletionRatio("no-audio-c") {
		t.Error("ContainsAudioCompletionRatio miss = true, want false")
	}
	if err := json.Unmarshal([]byte(AudioCompletionRatio2JSONString()), &back); err != nil {
		t.Fatalf("AudioCompletionRatio2JSONString invalid: %v", err)
	}
	if err := UpdateAudioCompletionRatioByJSONString(`{bad`); err == nil {
		t.Error("UpdateAudioCompletionRatioByJSONString(invalid) expected error")
	}
}

// ---------------------------------------------------------------------------
// model_ratio.go — model ratio map JSON + copies + defaults
// ---------------------------------------------------------------------------

func TestModelRatioMapAndCopies(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)

	var back map[string]float64
	if err := json.Unmarshal([]byte(ModelRatio2JSONString()), &back); err != nil {
		t.Fatalf("ModelRatio2JSONString invalid: %v", err)
	}
	if back["gpt-4o"] != 1.25 {
		t.Errorf("ModelRatio JSON gpt-4o = %v, want 1.25", back["gpt-4o"])
	}

	// copies must be independent snapshots
	mr := GetModelRatioCopy()
	mr["gpt-4o"] = 999
	if GetModelRatioCopy()["gpt-4o"] == 999 {
		t.Error("GetModelRatioCopy is not an independent copy")
	}
	mp := GetModelPriceCopy()
	mp["dall-e-3"] = 999
	if GetModelPriceCopy()["dall-e-3"] == 999 {
		t.Error("GetModelPriceCopy is not an independent copy")
	}
	cr := GetCompletionRatioCopy()
	cr["gpt-4-all"] = 999
	if GetCompletionRatioCopy()["gpt-4-all"] == 999 {
		t.Error("GetCompletionRatioCopy is not an independent copy")
	}

	if err := json.Unmarshal([]byte(DefaultModelRatio2JSONString()), &back); err != nil {
		t.Fatalf("DefaultModelRatio2JSONString invalid: %v", err)
	}
	if GetDefaultModelRatioMap()["gpt-4o"] != 1.25 {
		t.Error("GetDefaultModelRatioMap gpt-4o wrong")
	}
	if GetDefaultModelPriceMap()["dall-e-3"] != 0.04 {
		t.Error("GetDefaultModelPriceMap dall-e-3 wrong")
	}
	if GetDefaultImageRatioMap()["gpt-image-1"] != 2 {
		t.Error("GetDefaultImageRatioMap gpt-image-1 wrong")
	}
	if GetDefaultAudioRatioMap()["gpt-4o-audio-preview"] != 16 {
		t.Error("GetDefaultAudioRatioMap wrong")
	}
	if GetDefaultAudioCompletionRatioMap()["gpt-4o-mini-tts"] != 1 {
		t.Error("GetDefaultAudioCompletionRatioMap wrong")
	}
}

// ---------------------------------------------------------------------------
// model_ratio.go — FormatMatchingModelName / handleThinkingBudgetModel
// ---------------------------------------------------------------------------

func TestFormatMatchingModelName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		// thinking-budget collapse
		{"gemini-2.5-flash-lite-thinking-1024", "gemini-2.5-flash-lite-thinking-*"},
		{"gemini-2.5-flash-thinking-2048", "gemini-2.5-flash-thinking-*"},
		{"gemini-2.5-pro-thinking-4096", "gemini-2.5-pro-thinking-*"},
		// prefix matches but no -thinking- -> unchanged (false branch)
		{"gemini-2.5-flash-preview-05-20", "gemini-2.5-flash-preview-05-20"},
		{"gemini-2.5-pro-preview-03-25", "gemini-2.5-pro-preview-03-25"},
		// gizmo collapse
		{"gpt-4-gizmo-g-abc", "gpt-4-gizmo-*"},
		{"gpt-4o-gizmo-g-xyz", "gpt-4o-gizmo-*"},
		// passthrough
		{"gpt-4o", "gpt-4o"},
	}
	for _, tt := range tests {
		if got := FormatMatchingModelName(tt.in); got != tt.want {
			t.Errorf("FormatMatchingModelName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// group_ratio.go
// ---------------------------------------------------------------------------

func TestGroupRatio_RoundTripAndLookup(t *testing.T) {
	origJSON := GroupRatio2JSONString()
	t.Cleanup(func() { _ = UpdateGroupRatioByJSONString(origJSON) })

	if err := UpdateGroupRatioByJSONString(`{"default":1,"vip":0.9}`); err != nil {
		t.Fatalf("UpdateGroupRatioByJSONString: %v", err)
	}
	if r := GetGroupRatio("vip"); r != 0.9 {
		t.Errorf("GetGroupRatio(vip) = %v, want 0.9", r)
	}
	// unknown group -> default 1
	if r := GetGroupRatio("ghost"); r != 1 {
		t.Errorf("GetGroupRatio(ghost) = %v, want 1", r)
	}
	if !ContainsGroupRatio("vip") {
		t.Error("ContainsGroupRatio(vip) = false, want true")
	}
	if ContainsGroupRatio("ghost") {
		t.Error("ContainsGroupRatio(ghost) = true, want false")
	}
	cp := GetGroupRatioCopy()
	cp["vip"] = 999
	if GetGroupRatioCopy()["vip"] == 999 {
		t.Error("GetGroupRatioCopy is not independent")
	}
	var back map[string]float64
	if err := json.Unmarshal([]byte(GroupRatio2JSONString()), &back); err != nil {
		t.Fatalf("GroupRatio2JSONString invalid: %v", err)
	}
	if err := UpdateGroupRatioByJSONString(`{bad`); err == nil {
		t.Error("UpdateGroupRatioByJSONString(invalid) expected error")
	}
}

func TestGroupGroupRatio(t *testing.T) {
	origJSON := GroupGroupRatio2JSONString()
	t.Cleanup(func() { _ = UpdateGroupGroupRatioByJSONString(origJSON) })

	if err := UpdateGroupGroupRatioByJSONString(`{"vip":{"edit_this":0.8}}`); err != nil {
		t.Fatalf("UpdateGroupGroupRatioByJSONString: %v", err)
	}
	// override precedence: (vip, edit_this) resolves to the nested override
	if r, ok := GetGroupGroupRatio("vip", "edit_this"); !ok || r != 0.8 {
		t.Errorf("GetGroupGroupRatio(vip,edit_this) = %v,%v want 0.8,true", r, ok)
	}
	// unknown userGroup
	if r, ok := GetGroupGroupRatio("ghost", "edit_this"); ok || r != -1 {
		t.Errorf("GetGroupGroupRatio(ghost,..) = %v,%v want -1,false", r, ok)
	}
	// known userGroup, unknown usingGroup
	if r, ok := GetGroupGroupRatio("vip", "ghost"); ok || r != -1 {
		t.Errorf("GetGroupGroupRatio(vip,ghost) = %v,%v want -1,false", r, ok)
	}
	var back map[string]map[string]float64
	if err := json.Unmarshal([]byte(GroupGroupRatio2JSONString()), &back); err != nil {
		t.Fatalf("GroupGroupRatio2JSONString invalid: %v", err)
	}
	if err := UpdateGroupGroupRatioByJSONString(`{bad`); err == nil {
		t.Error("UpdateGroupGroupRatioByJSONString(invalid) expected error")
	}
}

func TestCheckGroupRatio(t *testing.T) {
	if err := CheckGroupRatio(`{"default":1,"vip":0.5}`); err != nil {
		t.Errorf("CheckGroupRatio(valid) = %v, want nil", err)
	}
	if err := CheckGroupRatio(`{"vip":-0.1}`); err == nil {
		t.Error("CheckGroupRatio(negative) expected error")
	}
	if err := CheckGroupRatio(`{bad`); err == nil {
		t.Error("CheckGroupRatio(invalid json) expected error")
	}
}

func TestGetGroupRatioSetting(t *testing.T) {
	s := GetGroupRatioSetting()
	if s == nil || s.GroupSpecialUsableGroup == nil {
		t.Fatal("GetGroupRatioSetting returned nil / nil special usable group")
	}
	// Force the lazy-init branch: nil out then re-fetch.
	s.GroupSpecialUsableGroup = nil
	s2 := GetGroupRatioSetting()
	if s2.GroupSpecialUsableGroup == nil {
		t.Error("GetGroupRatioSetting did not lazily re-init GroupSpecialUsableGroup")
	}
}

// ---------------------------------------------------------------------------
// cache_ratio.go
// ---------------------------------------------------------------------------

func TestCacheRatio(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)

	if err := UpdateCacheRatioByJSONString(`{"gpt-4o":0.5}`); err != nil {
		t.Fatalf("UpdateCacheRatioByJSONString: %v", err)
	}
	if r, ok := GetCacheRatio("gpt-4o"); !ok || r != 0.5 {
		t.Errorf("GetCacheRatio(gpt-4o) = %v,%v want 0.5,true", r, ok)
	}
	if r, ok := GetCacheRatio("no-cache"); ok || r != 1 {
		t.Errorf("GetCacheRatio(no-cache) = %v,%v want 1,false", r, ok)
	}
	if GetCacheRatioMap()["gpt-4o"] != 0.5 {
		t.Error("GetCacheRatioMap gpt-4o wrong")
	}
	cp := GetCacheRatioCopy()
	cp["gpt-4o"] = 999
	if GetCacheRatioCopy()["gpt-4o"] == 999 {
		t.Error("GetCacheRatioCopy is not independent")
	}
	var back map[string]float64
	if err := json.Unmarshal([]byte(CacheRatio2JSONString()), &back); err != nil {
		t.Fatalf("CacheRatio2JSONString invalid: %v", err)
	}
	if err := UpdateCacheRatioByJSONString(`{bad`); err == nil {
		t.Error("UpdateCacheRatioByJSONString(invalid) expected error")
	}
}

func TestGetCreateCacheRatio(t *testing.T) {
	if r, ok := GetCreateCacheRatio("claude-3-opus-20240229"); !ok || r != 1.25 {
		t.Errorf("GetCreateCacheRatio(hit) = %v,%v want 1.25,true", r, ok)
	}
	if r, ok := GetCreateCacheRatio("no-create-cache"); ok || r != 1.25 {
		t.Errorf("GetCreateCacheRatio(miss) = %v,%v want 1.25,false", r, ok)
	}
}

// ---------------------------------------------------------------------------
// expose_ratio.go / exposed_cache.go
// ---------------------------------------------------------------------------

func TestExposeRatioEnabled(t *testing.T) {
	orig := IsExposeRatioEnabled()
	t.Cleanup(func() { SetExposeRatioEnabled(orig) })

	SetExposeRatioEnabled(true)
	if !IsExposeRatioEnabled() {
		t.Error("IsExposeRatioEnabled = false after SetExposeRatioEnabled(true)")
	}
	SetExposeRatioEnabled(false)
	if IsExposeRatioEnabled() {
		t.Error("IsExposeRatioEnabled = true after SetExposeRatioEnabled(false)")
	}
}

func TestGetExposedData_BuildCacheInvalidate(t *testing.T) {
	InitRatioSettings()
	t.Cleanup(InitRatioSettings)

	InvalidateExposedDataCache()
	d := GetExposedData()
	if _, ok := d["model_ratio"]; !ok {
		t.Fatal("GetExposedData missing model_ratio key")
	}
	if _, ok := d["completion_ratio"]; !ok {
		t.Error("GetExposedData missing completion_ratio key")
	}
	if _, ok := d["cache_ratio"]; !ok {
		t.Error("GetExposedData missing cache_ratio key")
	}
	if _, ok := d["model_price"]; !ok {
		t.Error("GetExposedData missing model_price key")
	}

	// returned map is a clone: mutating it must not affect a subsequent read
	d["injected"] = 1
	if _, ok := GetExposedData()["injected"]; ok {
		t.Error("GetExposedData returned a shared (non-cloned) map")
	}

	// second call hits the cached branch (before TTL expiry)
	_ = GetExposedData()

	InvalidateExposedDataCache()
}

// ---------------------------------------------------------------------------
// model_equivalence.go
// ---------------------------------------------------------------------------

func TestGetModelEquivalence(t *testing.T) {
	eq, ok := GetModelEquivalence("gpt-4o")
	if !ok || eq.Family != "gpt" || eq.ConservativeSub != "gpt-4o-mini" {
		t.Errorf("GetModelEquivalence(gpt-4o) = %+v,%v unexpected", eq, ok)
	}
	// normalization: gizmo prefix collapses but is not in the curated map -> false
	if _, ok := GetModelEquivalence("totally-unknown-model"); ok {
		t.Error("GetModelEquivalence(unknown) = true, want false")
	}
}

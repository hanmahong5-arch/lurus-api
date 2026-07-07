package app

// token_counter_extra_test.go — coverage for the pure token-accounting helpers
// (text/audio/realtime). These feed directly into pre-consume quota, so an
// off-by-one here mis-prices requests. Assertions check exact counts where the
// arithmetic is deterministic and monotonic relationships otherwise.

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

// getImageToken model-classification branches that return without decoding an
// actual image (glm short-circuit, detail=low, and the GetMediaToken=false
// 3× floor for each model family).
func TestGetImageToken_ModelClassification(t *testing.T) {
	// glm-4 short-circuits to a fixed 1047 regardless of anything else.
	if got, err := getImageToken(&types.FileMeta{}, "glm-4v", false); err != nil || got != 1047 {
		t.Errorf("glm-4 => (%d,%v), want (1047,nil)", got, err)
	}

	// detail=low (non patch-based model) returns baseTokens for the 4o family.
	low := &types.FileMeta{Detail: "low"}
	if got, err := getImageToken(low, "gpt-4o", false); err != nil || got != 85 {
		t.Errorf("gpt-4o detail=low => (%d,%v), want (85,nil)", got, err)
	}

	// With GetMediaToken off (default), each family returns 3*baseTokens.
	cases := []struct {
		model string
		base  int
	}{
		{"gpt-4o", 85},
		{"gpt-4o-mini", 2833},
		{"o1-preview", 75},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			got, err := getImageToken(&types.FileMeta{Detail: "high"}, tc.model, false)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != 3*tc.base {
				t.Errorf("%s => %d, want %d (3*base, GetMediaToken off)", tc.model, got, 3*tc.base)
			}
		})
	}
}

func TestGetImageToken_NilFileMeta(t *testing.T) {
	if _, err := getImageToken(nil, "gpt-4o", false); err == nil {
		t.Fatal("expected error for nil file meta")
	}
}

// pngDataURL encodes a solid w×h PNG as a data URL.
func pngDataURL(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png %dx%d: %v", w, h, err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestGetImageToken_PatchBasedModel(t *testing.T) {
	prev := constant.GetMediaToken
	prevNS := constant.GetMediaTokenNotStream
	constant.GetMediaToken = true
	constant.GetMediaTokenNotStream = true
	defer func() { constant.GetMediaToken = prev; constant.GetMediaTokenNotStream = prevNS }()

	// gpt-4.1-mini is patch-based (32x32 patches, 1.62 multiplier). A 128x64
	// image => ceil(128/32)*ceil(64/32) = 4*2 = 8 patches, below the 1536 cap.
	fm := &types.FileMeta{OriginData: pngDataURL(t, 128, 64), Detail: "high"}
	got, err := getImageToken(fm, "gpt-4.1-mini", false)
	if err != nil {
		t.Fatalf("getImageToken patch: %v", err)
	}
	// round(8 * 1.62) = 13.
	if got != 13 {
		t.Errorf("patch tokens = %d, want 13 (8 patches * 1.62)", got)
	}
}

func TestGetImageToken_TileBasedLargeImage(t *testing.T) {
	prev := constant.GetMediaToken
	prevNS := constant.GetMediaTokenNotStream
	constant.GetMediaToken = true
	constant.GetMediaTokenNotStream = true
	defer func() { constant.GetMediaToken = prev; constant.GetMediaTokenNotStream = prevNS }()

	// A wide image (>2048 on the long side) exercises the fit-to-2048 scale
	// branch of the tile-based calculation for gpt-4o.
	fm := &types.FileMeta{OriginData: pngDataURL(t, 3000, 1000), Detail: "high"}
	got, err := getImageToken(fm, "gpt-4o", false)
	if err != nil {
		t.Fatalf("getImageToken tile: %v", err)
	}
	if got <= 85 {
		t.Errorf("tile tokens = %d, want > baseTokens for a large image", got)
	}
}

// TestGetImageToken_DecodesAndComputesTiles exercises the full tile-token
// computation: GetMediaToken on forces the real image decode + dimension-based
// tile math (rather than the 3× floor short-circuit).
func TestGetImageToken_DecodesAndComputesTiles(t *testing.T) {
	prevMedia := constant.GetMediaToken
	prevMediaNS := constant.GetMediaTokenNotStream
	constant.GetMediaToken = true
	constant.GetMediaTokenNotStream = true
	defer func() {
		constant.GetMediaToken = prevMedia
		constant.GetMediaTokenNotStream = prevMediaNS
	}()

	// Build a real PNG data-URL so DecodeBase64ImageData yields valid dimensions.
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	fm := &types.FileMeta{OriginData: dataURL, Detail: "high"}
	got, err := getImageToken(fm, "gpt-4o", false)
	if err != nil {
		t.Fatalf("getImageToken decode: %v", err)
	}
	// A 64x64 image needs at least the base tokens plus one tile.
	if got <= 85 {
		t.Errorf("computed image tokens = %d, want > baseTokens (real tile math)", got)
	}
}

func TestCountTextToken_EmptyIsZero(t *testing.T) {
	if got := CountTextToken("", "gpt-3.5-turbo"); got != 0 {
		t.Errorf("empty text = %d, want 0", got)
	}
}

func TestCountTextToken_NonEmptyPositive(t *testing.T) {
	got := CountTextToken("The quick brown fox jumps over the lazy dog", "gpt-3.5-turbo")
	if got <= 0 {
		t.Errorf("non-empty text token count = %d, want > 0", got)
	}
}

func TestCountTextToken_NonOpenAIModelEstimates(t *testing.T) {
	// Non-OpenAI models fall back to EstimateTokenByModel; still positive.
	got := CountTextToken("some text for a non openai model", "claude-3-5-sonnet")
	if got <= 0 {
		t.Errorf("non-openai estimate = %d, want > 0", got)
	}
}

func TestCountTokenInput_TypeSwitch(t *testing.T) {
	cases := []struct {
		name  string
		input any
	}{
		{"string", "hello world"},
		{"string slice", []string{"hello", "world"}},
		{"interface slice", []interface{}{"a", 1, true}},
		{"other type coerced", 12345},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CountTokenInput(tc.input, "gpt-3.5-turbo")
			if got <= 0 {
				t.Errorf("CountTokenInput(%s) = %d, want > 0", tc.name, got)
			}
		})
	}
}

func TestCountAudioTokenInput_EmptyIsZero(t *testing.T) {
	got, err := CountAudioTokenInput("", "wav")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 0 {
		t.Errorf("empty audio input = %d, want 0", got)
	}
}

func TestCountAudioTokenOutput_EmptyIsZero(t *testing.T) {
	got, err := CountAudioTokenOutput("", "wav")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 0 {
		t.Errorf("empty audio output = %d, want 0", got)
	}
}

func TestCountAudioTokenInput_InvalidBase64Errors(t *testing.T) {
	// Non-empty but undecodable audio must surface an error, not silently return 0.
	_, err := CountAudioTokenInput("!!!not-base64!!!", "wav")
	if err == nil {
		t.Fatal("expected error for invalid audio payload")
	}
}

func TestCountTokenRealtime_SessionUpdateCountsInstructions(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	ev := dto.RealtimeEvent{
		Type: dto.RealtimeEventTypeSessionUpdate,
		Session: &dto.RealtimeSession{
			Instructions: "you are a helpful assistant answering questions",
		},
	}
	text, audio, err := CountTokenRealtime(info, ev, "gpt-4o-realtime-preview")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if text <= 0 {
		t.Errorf("session-update text tokens = %d, want > 0", text)
	}
	if audio != 0 {
		t.Errorf("session-update audio tokens = %d, want 0", audio)
	}
}

func TestCountTokenRealtime_TranscriptDeltaCountsText(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	ev := dto.RealtimeEvent{
		Type:  dto.RealtimeEventResponseAudioTranscriptionDelta,
		Delta: "transcribed words here",
	}
	text, audio, err := CountTokenRealtime(info, ev, "gpt-4o-realtime-preview")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if text <= 0 || audio != 0 {
		t.Errorf("transcript delta = (text %d, audio %d), want (>0, 0)", text, audio)
	}
}

func TestCountTokenRealtime_ConversationItemMessage(t *testing.T) {
	info := &relaycommon.RelayInfo{}
	ev := dto.RealtimeEvent{
		Type: dto.RealtimeEventConversationItemCreated,
		Item: &dto.RealtimeItem{
			Type: "message",
			Content: []dto.RealtimeContent{
				{Type: "input_text", Text: "what is the weather today"},
				{Type: "input_audio", Text: "ignored"},
			},
		},
	}
	text, _, err := CountTokenRealtime(info, ev, "gpt-4o-realtime-preview")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if text <= 0 {
		t.Errorf("conversation message text tokens = %d, want > 0", text)
	}
}

func TestCountTokenRealtime_ResponseDoneCountsTools(t *testing.T) {
	info := &relaycommon.RelayInfo{
		IsFirstRequest: false,
		RealtimeTools: []dto.RealTimeTool{
			{Type: "function", Name: "get_weather", Description: "gets the weather"},
		},
	}
	ev := dto.RealtimeEvent{Type: dto.RealtimeEventTypeResponseDone}
	text, _, err := CountTokenRealtime(info, ev, "gpt-4o-realtime-preview")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Each tool adds a fixed +8 plus its own token count, so > 8.
	if text <= 8 {
		t.Errorf("response-done tool tokens = %d, want > 8", text)
	}
}

func TestCountTokenRealtime_AudioDeltaInvalidErrors(t *testing.T) {
	info := &relaycommon.RelayInfo{OutputAudioFormat: "pcm16"}
	ev := dto.RealtimeEvent{
		Type:  dto.RealtimeEventResponseAudioDelta,
		Delta: "!!!bad-audio!!!",
	}
	_, _, err := CountTokenRealtime(info, ev, "gpt-4o-realtime-preview")
	if err == nil {
		t.Fatal("expected error for undecodable audio delta")
	}
}

// ─── EstimateRequestToken guard branches ──────────────────────────────────

func TestEstimateRequestToken_CountTokenDisabled(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = false
	defer func() { constant.CountToken = prev }()

	c := createTestGinContext()
	got, err := EstimateRequestToken(c, &types.TokenCountMeta{}, &relaycommon.RelayInfo{})
	if err != nil || got != 0 {
		t.Errorf("disabled CountToken => (%d, %v), want (0, nil)", got, err)
	}
}

func TestEstimateRequestToken_NilMetaErrors(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	defer func() { constant.CountToken = prev }()

	c := createTestGinContext()
	_, err := EstimateRequestToken(c, nil, &relaycommon.RelayInfo{})
	if err == nil {
		t.Fatal("expected error for nil meta")
	}
}

func TestEstimateRequestToken_RealtimeReturnsZero(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	defer func() { constant.CountToken = prev }()

	c := createTestGinContext()
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIRealtime}
	got, err := EstimateRequestToken(c, &types.TokenCountMeta{}, info)
	if err != nil || got != 0 {
		t.Errorf("realtime format => (%d, %v), want (0, nil)", got, err)
	}
}

func TestEstimateRequestToken_TextOnlyOpenAIFormat(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	defer func() { constant.CountToken = prev }()

	c := createTestGinContext()
	c.Set(string(constant.ContextKeyOriginalModel), "gpt-3.5-turbo")

	meta := &types.TokenCountMeta{
		CombineText:   "count the tokens in this message please",
		MessagesCount: 1,
		ToolsCount:    0,
		NameCount:     0,
	}
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI}

	got, err := EstimateRequestToken(c, meta, info)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// OpenAI format adds messages*3 + 3 framing tokens on top of the text tokens,
	// so the total must exceed the framing floor.
	if got <= 3+1*3 {
		t.Errorf("openai text estimate = %d, want > framing floor %d", got, 3+1*3)
	}
}

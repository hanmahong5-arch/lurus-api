package app

// token_counter_more_test.go — covers EstimateRequestToken's guard branches and
// the data-URI media classification + per-file-type token switch. These figures
// drive pre-consume pricing, so the arithmetic is asserted exactly.

import (
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

func TestEstimateRequestToken_NilMetaAndRealtime(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = prev })

	c := createTestGinContext()

	// nil meta → error.
	if _, err := EstimateRequestToken(c, nil, &relaycommon.RelayInfo{}); err == nil {
		t.Fatal("expected error for nil meta")
	}

	// realtime format → short-circuits to 0.
	got, err := EstimateRequestToken(c, &types.TokenCountMeta{},
		&relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIRealtime})
	if err != nil || got != 0 {
		t.Errorf("realtime => (%d,%v), want (0,nil)", got, err)
	}
}

func TestEstimateRequestToken_DataURIMediaClassification(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = prev })

	c := createTestGinContext()
	// A non-OpenAI text model makes the image path take the flat 520 branch
	// (no decode), keeping the total deterministic.
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gemini-1.5-pro")

	meta := &types.TokenCountMeta{
		TokenType:     types.TokenTypeTextNumber, // rune-count, deterministic
		CombineText:   "ab",                      // 2 runes
		ToolsCount:    1,
		MessagesCount: 2,
		NameCount:     1,
		Files: []*types.FileMeta{
			{OriginData: "data:image/png;base64,AAAA"},
			{OriginData: "data:audio/mp3;base64,AAAA"},
			{OriginData: "data:video/mp4;base64,AAAA"},
			{OriginData: "data:application/pdf;base64,AAAA"},
		},
	}

	got, err := EstimateRequestToken(c, meta, &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI})
	if err != nil {
		t.Fatalf("EstimateRequestToken: %v", err)
	}

	// text(2) + openai-format(tools 1*8 + msgs 2*3 + names 1*3 + 3 = 20)
	// + image(520, non-OpenAI model) + audio(256) + video(8192) + file(4096).
	const want = 2 + 20 + 520 + 256 + 8192 + 4096
	if got != want {
		t.Errorf("token estimate = %d, want %d", got, want)
	}

	// The computed prompt-token count is stashed on the context for downstream
	// billing.
	if v := common.GetContextKeyInt(c, constant.ContextKeyPromptTokens); v != want {
		t.Errorf("context prompt tokens = %d, want %d", v, want)
	}
}

func TestCountAudioToken_EmptyAndInvalid(t *testing.T) {
	// Empty audio → 0 tokens, no error.
	if n, err := CountAudioTokenInput("", "pcm16"); err != nil || n != 0 {
		t.Errorf("empty input => (%d,%v), want (0,nil)", n, err)
	}
	if n, err := CountAudioTokenOutput("", "pcm16"); err != nil || n != 0 {
		t.Errorf("empty output => (%d,%v), want (0,nil)", n, err)
	}
	// Undecodable audio → error surfaces from parseAudio.
	if _, err := CountAudioTokenInput("!!!not-base64!!!", "pcm16"); err == nil {
		t.Error("expected error for undecodable input audio")
	}
	if _, err := CountAudioTokenOutput("!!!not-base64!!!", "pcm16"); err == nil {
		t.Error("expected error for undecodable output audio")
	}
}

func TestCountTokenRealtime_InputAudioBufferAppendInvalid(t *testing.T) {
	info := &relaycommon.RelayInfo{InputAudioFormat: "pcm16"}
	ev := dto.RealtimeEvent{
		Type:  dto.RealtimeEventInputAudioBufferAppend,
		Audio: "!!!bad-audio!!!",
	}
	if _, _, err := CountTokenRealtime(info, ev, "gpt-4o-realtime-preview"); err == nil {
		t.Fatal("expected error for undecodable input-audio-buffer append")
	}
}

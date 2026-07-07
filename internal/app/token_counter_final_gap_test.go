package app

// token_counter_final_gap_test.go — remaining hermetic token-counter arms: the
// successful audio-duration returns (input/output), the realtime audio-delta
// success paths, getImageToken's ParsedData (pre-decoded) branch, the multipart
// parse-error arm, and an image whose bytes fail to decode inside the media loop.

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	constant2 "github.com/LurusTech/lurus-hub/internal/adapter/provider/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

func TestCountAudioToken_ValidDurations(t *testing.T) {
	// 480 bytes of pcm16 @ 24kHz => 240 samples => 0.01s; both helpers must
	// return a non-negative token count from the real duration math.
	b64 := base64.StdEncoding.EncodeToString(make([]byte, 4800))
	in, err := CountAudioTokenInput(b64, "pcm16")
	if err != nil {
		t.Fatalf("CountAudioTokenInput: %v", err)
	}
	out, err := CountAudioTokenOutput(b64, "pcm16")
	if err != nil {
		t.Fatalf("CountAudioTokenOutput: %v", err)
	}
	if in < 0 || out < 0 {
		t.Errorf("audio tokens = (in %d, out %d), want non-negative", in, out)
	}
}

func TestCountTokenRealtime_AudioDeltaSuccess(t *testing.T) {
	audio := base64.StdEncoding.EncodeToString(make([]byte, 2400))
	info := &relaycommon.RelayInfo{OutputAudioFormat: "pcm16", InputAudioFormat: "pcm16"}

	_, aud, err := CountTokenRealtime(info, dto.RealtimeEvent{
		Type:  dto.RealtimeEventResponseAudioDelta,
		Delta: audio,
	}, "gpt-4o-realtime-preview")
	if err != nil {
		t.Fatalf("audio delta: %v", err)
	}
	if aud < 0 {
		t.Errorf("audio-delta tokens = %d, want >= 0", aud)
	}

	_, aud2, err := CountTokenRealtime(info, dto.RealtimeEvent{
		Type:  dto.RealtimeEventInputAudioBufferAppend,
		Audio: audio,
	}, "gpt-4o-realtime-preview")
	if err != nil {
		t.Fatalf("input audio append: %v", err)
	}
	if aud2 < 0 {
		t.Errorf("input-append tokens = %d, want >= 0", aud2)
	}
}

func TestGetImageToken_ParsedDataBranch(t *testing.T) {
	enableMediaToken(t, true)

	// Pre-parsed base64 image data takes the ParsedData decode branch.
	raw := pngDataURL(t, 100, 80) // data:image/png;base64,....
	b64 := raw[len("data:image/png;base64,"):]
	fm := &types.FileMeta{
		Detail:     "high",
		ParsedData: &types.LocalFileData{Base64Data: b64},
	}
	got, err := getImageToken(fm, "gpt-4o", true)
	if err != nil {
		t.Fatalf("getImageToken ParsedData: %v", err)
	}
	if got <= 0 {
		t.Errorf("tokens = %d, want > 0 from pre-parsed image", got)
	}
}

func TestEstimateRequestToken_MultipartParseError(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = prev })

	// Audio mode but the body is not a valid multipart form => parse error.
	c := createTestGinContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=nope")

	info := &relaycommon.RelayInfo{RelayMode: constant2.RelayModeAudioTranscription}
	if _, err := EstimateRequestToken(c, &types.TokenCountMeta{}, info); err == nil {
		t.Fatal("expected a multipart parse error")
	}
}

func TestEstimateRequestToken_ImageDecodeErrorInLoop(t *testing.T) {
	prev := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = prev })
	enableMediaToken(t, true)
	allowLocalFetch(t)
	raiseDownloadCap(t)

	// Server advertises image/png but serves non-image bytes: classification says
	// image, but getImageToken's decode fails => the loop's error arm runs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is definitely not a png"))
	}))
	defer srv.Close()

	c := createTestGinContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")

	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTextNumber,
		Files:     []*types.FileMeta{{OriginData: srv.URL}},
	}
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAI, IsStream: true}

	if _, err := EstimateRequestToken(c, meta, info); err == nil {
		t.Fatal("expected an image-decode error surfaced from the media loop")
	}
}

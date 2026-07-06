package app

// token_counter_gap_test.go — closes the remaining getImageToken /
// EstimateRequestToken branches: the "count-media-in-non-stream disabled" floor,
// the invalid-base64 decode error, the debug-logged tile path, and the
// end-to-end HTTP media-fetch → classify → per-type token accounting path
// (image via a real httptest server, and video via content-type).

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

// enableMediaToken turns on local media-token counting for the test.
func enableMediaToken(t *testing.T, notStream bool) {
	t.Helper()
	pm, pns := constant.GetMediaToken, constant.GetMediaTokenNotStream
	constant.GetMediaToken = true
	constant.GetMediaTokenNotStream = notStream
	t.Cleanup(func() { constant.GetMediaToken = pm; constant.GetMediaTokenNotStream = pns })
}

// TestGetImageToken_NonStreamMediaFloor drives the "not counting media in
// non-stream mode" floor: GetMediaToken on but GetMediaTokenNotStream off with a
// non-stream request returns 3×baseTokens without decoding.
func TestGetImageToken_NonStreamMediaFloor(t *testing.T) {
	enableMediaToken(t, false) // GetMediaTokenNotStream = false

	fm := &types.FileMeta{OriginData: pngDataURL(t, 64, 64), Detail: "high"}
	got, err := getImageToken(fm, "gpt-4o", false) // stream=false
	if err != nil {
		t.Fatalf("getImageToken: %v", err)
	}
	if got != 3*85 {
		t.Errorf("got = %d, want %d (3×base floor, media-in-nonstream disabled)", got, 3*85)
	}
}

// TestGetImageToken_InvalidBase64Errors drives the decode-error arm: a data URI
// whose payload isn't a valid image must surface an error, not a bogus count.
func TestGetImageToken_InvalidBase64Errors(t *testing.T) {
	enableMediaToken(t, true)

	fm := &types.FileMeta{OriginData: "data:image/png;base64,!!!!not-base64!!!!", Detail: "high"}
	if _, err := getImageToken(fm, "gpt-4o", true); err == nil {
		t.Fatal("expected an error decoding invalid base64 image data")
	}
}

// TestGetImageToken_DebugTilePath exercises the tile computation with debug
// logging on, so the DebugEnabled log line in the tile path runs.
func TestGetImageToken_DebugTilePath(t *testing.T) {
	enableMediaToken(t, true)
	prevDebug := common.DebugEnabled
	common.DebugEnabled = true
	t.Cleanup(func() { common.DebugEnabled = prevDebug })

	fm := &types.FileMeta{OriginData: pngDataURL(t, 800, 600), Detail: "high"}
	got, err := getImageToken(fm, "gpt-4o", true)
	if err != nil {
		t.Fatalf("getImageToken: %v", err)
	}
	if got <= 85 {
		t.Errorf("tile tokens = %d, want > base (real decode + tiles)", got)
	}
}

// TestEstimateRequestToken_HTTPImageFetched drives the full media path: an HTTP
// image URL is fetched, sniffed as image/png, classified as an image, and its
// per-image token count (via getImageToken's http decode) is added. Proves the
// fetch→classify→count wiring end to end.
func TestEstimateRequestToken_HTTPImageFetched(t *testing.T) {
	prevCount := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = prevCount })
	enableMediaToken(t, true)
	allowLocalFetch(t)
	raiseDownloadCap(t)

	srv := pngServer(t, "image/png")
	defer srv.Close()

	c := createTestGinContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o")

	meta := &types.TokenCountMeta{
		TokenType:   types.TokenTypeTextNumber,
		CombineText: "hello world",
		Files:       []*types.FileMeta{{OriginData: srv.URL}},
	}
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		IsStream:    true,
	}

	got, err := EstimateRequestToken(c, meta, info)
	if err != nil {
		t.Fatalf("EstimateRequestToken: %v", err)
	}
	// text runes (11) + OpenAI framing (3) + a positive image token contribution.
	if got <= 14 {
		t.Errorf("total tokens = %d, want > 14 (text framing + image tokens)", got)
	}
}

// TestEstimateRequestToken_HTTPVideoClassified drives the non-image branch of
// the classifier: an HTTP URL served as video/mp4 is classified FileTypeVideo
// and contributes the fixed video token cost.
func TestEstimateRequestToken_HTTPVideoClassified(t *testing.T) {
	prevCount := constant.CountToken
	constant.CountToken = true
	t.Cleanup(func() { constant.CountToken = prevCount })
	enableMediaToken(t, true)
	allowLocalFetch(t)
	raiseDownloadCap(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake mp4 body"))
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

	got, err := EstimateRequestToken(c, meta, info)
	if err != nil {
		t.Fatalf("EstimateRequestToken: %v", err)
	}
	// OpenAI framing (3) + video cost (4096*2). No text.
	if got != 3+4096*2 {
		t.Errorf("total tokens = %d, want %d (framing + video cost)", got, 3+4096*2)
	}
}

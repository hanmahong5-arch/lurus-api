package app

// file_decoder_gap_test.go — covers GetFileBase64FromUrl's application/
// octet-stream MIME-recovery arm (guess the type from the URL's file extension)
// and getImageToken's patch-model downscale-to-cap branch (an image whose 32×32
// patch count exceeds the 1536 cap must be scaled and clamped).

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/types"
)

func TestGetFileBase64FromUrl_OctetStreamMimeFromURLExt(t *testing.T) {
	allowLocalFetch(t)
	raiseDownloadCap(t)

	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	body := buf.Bytes()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := createTestGinContext()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	// URL ends in ".png" so the octet-stream response is re-typed from the ext.
	got, err := GetFileBase64FromUrl(c, srv.URL+"/asset.png", "unit")
	if err != nil {
		t.Fatalf("GetFileBase64FromUrl: %v", err)
	}
	if got.MimeType != "image/png" {
		t.Errorf("MimeType = %q, want image/png (guessed from URL .png ext)", got.MimeType)
	}
	if got.Base64Data == "" {
		t.Error("Base64Data is empty, want the encoded file bytes")
	}
}

func TestGetImageToken_PatchModelExceedsCap(t *testing.T) {
	enableMediaToken(t, true)

	// A 2048×2048 image => ceil(2048/32)^2 = 64^2 = 4096 patches, above the 1536
	// cap, so the downscale-and-clamp branch runs.
	fm := &types.FileMeta{OriginData: pngDataURL(t, 2048, 2048), Detail: "high"}
	got, err := getImageToken(fm, "gpt-4.1-mini", true)
	if err != nil {
		t.Fatalf("getImageToken: %v", err)
	}
	// Capped patch count (1536) × 1.62 multiplier ≈ 2488; assert it's clamped
	// near the cap rather than the raw 4096×1.62.
	if got <= 0 || got > 2489 {
		t.Errorf("patch tokens = %d, want a positive value clamped at the 1536-patch cap", got)
	}
}

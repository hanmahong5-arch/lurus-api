package app

// file_decoder_sniff_gap_test.go — drives GetFileTypeFromUrl's fall-through
// chain: an octet-stream response whose Content-Disposition filename carries an
// unknown extension must break past the disposition guess, skip the (extension-
// less) URL, and finally recover the type by content-sniffing the body bytes.

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetFileTypeFromUrl_UnknownDispositionThenContentSniff(t *testing.T) {
	allowLocalFetch(t)

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	body := buf.Bytes()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Unknown extension => the disposition-based guess yields octet-stream and
		// the code breaks out to try the next strategy.
		w.Header().Set("Content-Disposition", `attachment; filename="payload.zzz"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := createTestGinContext()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	// URL has no file extension, so recovery must come from content sniffing.
	got, err := GetFileTypeFromUrl(c, srv.URL+"/blob")
	if err != nil {
		t.Fatalf("GetFileTypeFromUrl: %v", err)
	}
	if got != "image/png" {
		t.Errorf("sniffed type = %q, want image/png (recovered from body bytes)", got)
	}
}

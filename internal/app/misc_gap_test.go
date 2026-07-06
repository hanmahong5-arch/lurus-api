package app

// misc_gap_test.go — small hermetic closers: the math-symbol Unicode-range arms
// of the token estimator, the '@' weighting branch, GetImageFromUrl's
// download-error / non-200 arms, and DecodeBase64FileData's mime-without-colon
// branch.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsMathSymbol_UnicodeRanges(t *testing.T) {
	cases := []struct {
		name string
		r    rune
		want bool
	}{
		{"mathematical_operators_2200_22FF", rune(0x22C0), true}, // ⋀ (not in the literal set)
		{"supplemental_operators_2A00_2AFF", rune(0x2A00), true}, // ⨀
		{"math_alphanumeric_1D400_1D7FF", rune(0x1D400), true},   // 𝐀
		{"literal_set_member", '≤', true},                        // from the explicit string
		{"plain_letter_not_math", 'a', false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isMathSymbol(tc.r); got != tc.want {
				t.Errorf("isMathSymbol(%U) = %v, want %v", tc.r, got, tc.want)
			}
		})
	}
}

func TestEstimateTokenByModel_AtSignWeighted(t *testing.T) {
	// An '@' is weighted distinctly from other symbols; a string containing one
	// must exercise that branch and produce a positive estimate.
	if got := EstimateTokenByModel("deepseek-chat", "user@example.com"); got <= 0 {
		t.Errorf("EstimateTokenByModel with '@' = %d, want > 0", got)
	}
}

func TestGetImageFromUrl_DownloadError(t *testing.T) {
	allowLocalFetch(t)
	if _, _, err := GetImageFromUrl(deadURL(t)); err == nil {
		t.Fatal("expected a download error for a dead endpoint")
	}
}

func TestGetImageFromUrl_Non200(t *testing.T) {
	allowLocalFetch(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	if _, _, err := GetImageFromUrl(srv.URL); err == nil {
		t.Fatal("expected an error for a non-200 image response")
	}
}

func TestDecodeBase64FileData_MimeWithoutColon(t *testing.T) {
	// A "mime;base64,DATA" header (no leading "data:") has a ';' but the segment
	// before it carries no ':', so the function falls back to image decoding and
	// prefixes the detected format.
	dataURL := pngDataURL(t, 4, 4)                   // data:image/png;base64,....
	stripped := strings.TrimPrefix(dataURL, "data:") // image/png;base64,....
	mimeType, b64, err := DecodeBase64FileData(stripped)
	if err != nil {
		t.Fatalf("DecodeBase64FileData: %v", err)
	}
	if !strings.HasPrefix(mimeType, "image/") || b64 == "" {
		t.Errorf("decoded = (%q, len %d), want image/* mime + non-empty data", mimeType, len(b64))
	}
}

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func gateTestRouter(cfg ReleaseGateConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/releases/:id/download/:artifact_id",
		ReleaseDownloadGateWith(cfg),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true, "reached": true}) },
	)
	return r
}

func doGate(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestReleaseDownloadGate(t *testing.T) {
	const gatedProduct = "lurus-switch"
	gatedSet := map[string]bool{gatedProduct: true}

	// id 1 = gated product, id 2 = free product, anything else = not found.
	lookup := func(_ context.Context, id int64) (string, bool) {
		switch id {
		case 1:
			return gatedProduct, true
		case 2:
			return "lurus-www", true
		default:
			return "", false
		}
	}
	anon := func(_ *gin.Context) (int64, bool) { return 0, false }
	authed := func(_ *gin.Context) (int64, bool) { return 42, true }
	entitled := func(_ context.Context, _ int64, _ string) (bool, error) { return true, nil }
	notEntitled := func(_ context.Context, _ int64, _ string) (bool, error) { return false, nil }
	entErr := func(_ context.Context, _ int64, _ string) (bool, error) { return false, errors.New("platform down") }

	tests := []struct {
		name        string
		cfg         ReleaseGateConfig
		path        string
		wantStatus  int
		wantReached bool
		wantCode    string // expected error_code; "" when the download is allowed through
	}{
		{
			name:        "feature off — public download untouched even with deny deps",
			cfg:         ReleaseGateConfig{GatedProducts: nil, LookupProductID: lookup, ResolveAccount: anon, CheckEntitled: notEntitled},
			path:        "/api/releases/1/download/1",
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:        "ungated product — anonymous still allowed",
			cfg:         ReleaseGateConfig{GatedProducts: gatedSet, LookupProductID: lookup, ResolveAccount: anon, CheckEntitled: notEntitled},
			path:        "/api/releases/2/download/1",
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:        "gated + anonymous → 401 fail-closed",
			cfg:         ReleaseGateConfig{GatedProducts: gatedSet, LookupProductID: lookup, ResolveAccount: anon, CheckEntitled: entitled},
			path:        "/api/releases/1/download/1",
			wantStatus:  http.StatusUnauthorized,
			wantReached: false,
			wantCode:    "DOWNLOAD_AUTH_REQUIRED",
		},
		{
			name:        "gated + authed + no entitlement → 403",
			cfg:         ReleaseGateConfig{GatedProducts: gatedSet, LookupProductID: lookup, ResolveAccount: authed, CheckEntitled: notEntitled},
			path:        "/api/releases/1/download/1",
			wantStatus:  http.StatusForbidden,
			wantReached: false,
			wantCode:    "NO_ENTITLEMENT",
		},
		{
			name:        "gated + entitlement system error → 403 fail-closed",
			cfg:         ReleaseGateConfig{GatedProducts: gatedSet, LookupProductID: lookup, ResolveAccount: authed, CheckEntitled: entErr},
			path:        "/api/releases/1/download/1",
			wantStatus:  http.StatusForbidden,
			wantReached: false,
			wantCode:    "ENTITLEMENT_CHECK_FAILED",
		},
		{
			name:        "gated + authed + entitled → download allowed",
			cfg:         ReleaseGateConfig{GatedProducts: gatedSet, LookupProductID: lookup, ResolveAccount: authed, CheckEntitled: entitled},
			path:        "/api/releases/1/download/1",
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:        "malformed release id → defer to handler",
			cfg:         ReleaseGateConfig{GatedProducts: gatedSet, LookupProductID: lookup, ResolveAccount: anon, CheckEntitled: notEntitled},
			path:        "/api/releases/abc/download/1",
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:        "release not found → defer to handler (404 path)",
			cfg:         ReleaseGateConfig{GatedProducts: gatedSet, LookupProductID: lookup, ResolveAccount: anon, CheckEntitled: notEntitled},
			path:        "/api/releases/999/download/1",
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doGate(gateTestRouter(tt.cfg), tt.path)
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", w.Code, tt.wantStatus, w.Body.String())
			}
			var body map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			if reached := body["reached"] == true; reached != tt.wantReached {
				t.Fatalf("handler reached = %v, want %v (body=%s)", reached, tt.wantReached, w.Body.String())
			}
			if tt.wantCode != "" && body["error_code"] != tt.wantCode {
				t.Fatalf("error_code = %v, want %q (body=%s)", body["error_code"], tt.wantCode, w.Body.String())
			}
		})
	}
}

func TestParseGatedProducts(t *testing.T) {
	got := parseGatedProducts(" lurus-switch , ,lurus-creator ")
	if len(got) != 2 || !got["lurus-switch"] || !got["lurus-creator"] {
		t.Fatalf("parseGatedProducts = %v, want {lurus-switch, lurus-creator}", got)
	}
	if len(parseGatedProducts("")) != 0 {
		t.Fatalf("empty input should yield empty set")
	}
	if len(parseGatedProducts(" , , ")) != 0 {
		t.Fatalf("whitespace/comma-only input should yield empty set")
	}
}

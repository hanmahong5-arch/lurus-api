package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mountReleaseGate wires ReleaseDownloadGateWith on a route with an :id param.
func mountReleaseGate(cfg ReleaseGateConfig) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/releases/:id/download", ReleaseDownloadGateWith(cfg), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func getRelease(r *gin.Engine, id string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/releases/"+id+"/download", nil))
	return w
}

func TestReleaseGate_FeatureOff_Passes(t *testing.T) {
	r := mountReleaseGate(ReleaseGateConfig{}) // no gated products
	if w := getRelease(r, "1"); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when no products gated", w.Code)
	}
}

func TestReleaseGate_MalformedID_Passes(t *testing.T) {
	r := mountReleaseGate(ReleaseGateConfig{
		GatedProducts: map[string]bool{"lurus-switch": true},
		LookupProductID: func(context.Context, int64) (string, bool) {
			t.Fatal("lookup must not be called for malformed id")
			return "", false
		},
	})
	if w := getRelease(r, "not-an-int"); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (defer to handler validation)", w.Code)
	}
}

func TestReleaseGate_ReleaseNotFound_Passes(t *testing.T) {
	r := mountReleaseGate(ReleaseGateConfig{
		GatedProducts:   map[string]bool{"lurus-switch": true},
		LookupProductID: func(context.Context, int64) (string, bool) { return "", false },
	})
	if w := getRelease(r, "5"); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (missing release defers to 404)", w.Code)
	}
}

func TestReleaseGate_FreeProduct_Passes(t *testing.T) {
	r := mountReleaseGate(ReleaseGateConfig{
		GatedProducts:   map[string]bool{"lurus-switch": true},
		LookupProductID: func(context.Context, int64) (string, bool) { return "free-tool", true },
	})
	if w := getRelease(r, "5"); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for non-gated product", w.Code)
	}
}

func TestReleaseGate_GatedAnonymous_401(t *testing.T) {
	r := mountReleaseGate(ReleaseGateConfig{
		GatedProducts:   map[string]bool{"lurus-switch": true},
		LookupProductID: func(context.Context, int64) (string, bool) { return "lurus-switch", true },
		ResolveAccount:  func(*gin.Context) (int64, bool) { return 0, false },
	})
	if w := getRelease(r, "5"); w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for anonymous access to gated product", w.Code)
	}
}

func TestReleaseGate_GatedEntitlementError_403(t *testing.T) {
	r := mountReleaseGate(ReleaseGateConfig{
		GatedProducts:   map[string]bool{"lurus-switch": true},
		LookupProductID: func(context.Context, int64) (string, bool) { return "lurus-switch", true },
		ResolveAccount:  func(*gin.Context) (int64, bool) { return 42, true },
		CheckEntitled: func(context.Context, int64, string) (bool, error) {
			return false, errors.New("platform down")
		},
	})
	if w := getRelease(r, "5"); w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (fail-closed on indeterminate entitlement)", w.Code)
	}
}

func TestReleaseGate_GatedNotEntitled_403(t *testing.T) {
	r := mountReleaseGate(ReleaseGateConfig{
		GatedProducts:   map[string]bool{"lurus-switch": true},
		LookupProductID: func(context.Context, int64) (string, bool) { return "lurus-switch", true },
		ResolveAccount:  func(*gin.Context) (int64, bool) { return 42, true },
		CheckEntitled:   func(context.Context, int64, string) (bool, error) { return false, nil },
	})
	if w := getRelease(r, "5"); w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for account without entitlement", w.Code)
	}
}

func TestReleaseGate_GatedEntitled_Passes(t *testing.T) {
	r := mountReleaseGate(ReleaseGateConfig{
		GatedProducts:   map[string]bool{"lurus-switch": true},
		LookupProductID: func(context.Context, int64) (string, bool) { return "lurus-switch", true },
		ResolveAccount:  func(*gin.Context) (int64, bool) { return 42, true },
		CheckEntitled:   func(context.Context, int64, string) (bool, error) { return true, nil },
	})
	if w := getRelease(r, "5"); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for entitled account; body=%s", w.Code, w.Body.String())
	}
}

func TestParseGatedProducts_ExtraCases(t *testing.T) {
	got := parseGatedProducts(" lurus-switch , lurus-creator ,, ")
	if len(got) != 2 || !got["lurus-switch"] || !got["lurus-creator"] {
		t.Errorf("parseGatedProducts = %v, want {lurus-switch,lurus-creator}", got)
	}
	if len(parseGatedProducts("")) != 0 {
		t.Errorf("empty string must yield empty set")
	}
}

func TestReleaseDownloadGate_WiresConfig(t *testing.T) {
	// Exercise the production constructor (reads env, wires prod deps). With
	// no RELEASE_GATED_PRODUCTS set the gate is off and every request passes.
	r := gin.New()
	r.GET("/releases/:id/download", ReleaseDownloadGate(), func(c *gin.Context) { c.Status(http.StatusOK) })
	if w := getRelease(r, "1"); w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when RELEASE_GATED_PRODUCTS unset", w.Code)
	}
}

func TestResolveOIDCAccountID_ContextShortCircuit(t *testing.T) {
	// When identity_account_id is already in context (set by an earlier
	// middleware), resolveOIDCAccountID returns it without touching JWKS.
	c, _ := newTestContext(http.MethodGet, "/x", "", "")
	c.Set("identity_account_id", int64(1234))
	id, ok := resolveOIDCAccountID(c)
	if !ok || id != 1234 {
		t.Errorf("resolveOIDCAccountID = (%d,%v), want (1234,true)", id, ok)
	}
}

func TestResolveOIDCAccountID_OIDCDisabled_False(t *testing.T) {
	// oidcEnabled is false in unit scope and no context id → anonymous.
	c, _ := newTestContext(http.MethodGet, "/x", "", "")
	if id, ok := resolveOIDCAccountID(c); ok || id != 0 {
		t.Errorf("resolveOIDCAccountID = (%d,%v), want (0,false) with OIDC disabled", id, ok)
	}
}

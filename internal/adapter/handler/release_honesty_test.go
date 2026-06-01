package handler

// release_honesty_test.go — honesty lock for GOAL-14.
//
// Claim under audit:
//
//	"gated product anonymous GET → ReleaseDownloadGate returns 401/403;
//	 entitlement indeterminate → DENY; stub client is not a real platform."
//
// The middleware package already proves the core gate truth table
// (release_gate_test.go: 8 cases). This file lives in the HANDLER package and
// locks the part that matters to GOAL-14's specific edge list and is NOT
// covered there:
//
//   - entitlement system TIMEOUT (context.DeadlineExceeded) → DENY 403, not a
//     silent allow (the existing entErr case uses a generic error; a deadline
//     is the realistic platform-slow failure mode and §4.1 wants the
//     fail-closed claim proven against the actual hazard)
//   - 越权 product_id: a caller entitled to product X must NOT be admitted to
//     gated product Y — the gate must pass the *requested* product to
//     CheckEntitled, not a caller-controlled one
//   - presigned-URL no-leak: when the gate denies, the downstream handler that
//     would mint the MinIO presigned URL MUST NOT run, so no Location header /
//     URL ever reaches an unentitled caller
//   - retry idempotency: two identical denied requests produce identical
//     denials with no side effect (the gate is a pure read)
//
// The subject is middleware.ReleaseDownloadGateWith wired in front of a stub
// that stands in for handler.DownloadArtifact's presigned-redirect; the
// assertions are on HTTP status + the ABSENCE of the redirect, independent of
// the gate's internal text. Not a tautology.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/middleware"
	"github.com/gin-gonic/gin"
)

const honestyGatedProduct = "lurus-switch"

// presignedURLSentinel is what the downstream "download" handler would emit —
// in production this is a 302 Location to a MinIO presigned URL. If the gate
// fails open, this string leaks to the caller; if it fails closed, it must not.
const presignedURLSentinel = "https://minio.internal/lurus-releases/switch-v1.dmg?X-Amz-Signature=LEAKED"

// buildGatedDownloadRouter wires the production gate (ReleaseDownloadGateWith)
// in front of a stub download handler that mints a presigned redirect. A
// process-wide counter records whether the stub ran, so we can assert the
// presigned URL is never generated on a denied request.
func buildGatedDownloadRouter(cfg middleware.ReleaseGateConfig, downloadRan *atomic.Int32) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/releases/:id/download/:artifact_id",
		middleware.ReleaseDownloadGateWith(cfg),
		func(c *gin.Context) {
			downloadRan.Add(1)
			// Mirror handler.DownloadArtifact's terminal step: a 302 to the
			// presigned URL. This is the thing that must never run when denied.
			c.Redirect(http.StatusFound, presignedURLSentinel)
		},
	)
	return r
}

func getGated(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// TestReleaseDownload_AnonymousGated_FailClosed is the GOAL-14 headline: an
// anonymous GET on a gated product is rejected (401) and the presigned-URL
// handler is never reached, so no download URL leaks.
func TestReleaseDownload_AnonymousGated_FailClosed(t *testing.T) {
	gated := map[string]bool{honestyGatedProduct: true}
	lookup := func(_ context.Context, id int64) (string, bool) {
		if id == 1 {
			return honestyGatedProduct, true
		}
		return "", false
	}
	anon := func(_ *gin.Context) (int64, bool) { return 0, false }
	entitled := func(_ context.Context, _ int64, _ string) (bool, error) { return true, nil }

	var ran atomic.Int32
	r := buildGatedDownloadRouter(middleware.ReleaseGateConfig{
		GatedProducts:   gated,
		LookupProductID: lookup,
		ResolveAccount:  anon,
		CheckEntitled:   entitled, // would allow, but anon must be rejected before this runs
	}, &ran)

	w := getGated(r, "/api/releases/1/download/1")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body=%s)", w.Code, w.Body.String())
	}
	if ran.Load() != 0 {
		t.Fatalf("download handler ran %d times on anonymous denial — presigned URL minted for an unauthenticated caller", ran.Load())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Fatalf("presigned URL LEAKED via Location header on denial: %q", loc)
	}
}

// TestReleaseDownload_EntitlementTimeout_DeniesClosed proves the fail-closed
// contract against the realistic hazard: the entitlement call exceeds its
// deadline. A context.DeadlineExceeded must produce 403, never an allow.
func TestReleaseDownload_EntitlementTimeout_DeniesClosed(t *testing.T) {
	gated := map[string]bool{honestyGatedProduct: true}
	lookup := func(_ context.Context, _ int64) (string, bool) { return honestyGatedProduct, true }
	authed := func(_ *gin.Context) (int64, bool) { return 42, true }
	timeoutCheck := func(ctx context.Context, _ int64, _ string) (bool, error) {
		// Simulate a platform call that blows its deadline.
		dctx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()
		<-dctx.Done()
		return false, dctx.Err() // context.DeadlineExceeded
	}

	var ran atomic.Int32
	r := buildGatedDownloadRouter(middleware.ReleaseGateConfig{
		GatedProducts:   gated,
		LookupProductID: lookup,
		ResolveAccount:  authed,
		CheckEntitled:   timeoutCheck,
	}, &ran)

	w := getGated(r, "/api/releases/1/download/1")

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 on entitlement timeout (fail-closed)", w.Code)
	}
	if ran.Load() != 0 {
		t.Fatalf("download handler ran on entitlement timeout — fail-OPEN bug")
	}
}

// TestReleaseDownload_CrossProductEntitlement_NoEscalation is the 越权 edge: a
// caller whose entitlement is ONLY for product X must be denied a gated
// product Y. The gate must pass the requested product (resolved from the
// release id) to CheckEntitled — not let the caller smuggle in a product they
// happen to own. We assert by recording the productID the gate actually checks.
func TestReleaseDownload_CrossProductEntitlement_NoEscalation(t *testing.T) {
	const ownedProduct = "lurus-creator" // what the caller IS entitled to
	const gatedTarget = honestyGatedProduct
	gated := map[string]bool{gatedTarget: true}

	// Release id 1 resolves to the gated target the caller does NOT own.
	lookup := func(_ context.Context, id int64) (string, bool) {
		if id == 1 {
			return gatedTarget, true
		}
		return "", false
	}
	authed := func(_ *gin.Context) (int64, bool) { return 7, true }

	var checkedProduct atomic.Value
	checkedProduct.Store("")
	// Entitled ONLY for ownedProduct; everything else denied.
	check := func(_ context.Context, _ int64, productID string) (bool, error) {
		checkedProduct.Store(productID)
		return productID == ownedProduct, nil
	}

	var ran atomic.Int32
	r := buildGatedDownloadRouter(middleware.ReleaseGateConfig{
		GatedProducts:   gated,
		LookupProductID: lookup,
		ResolveAccount:  authed,
		CheckEntitled:   check,
	}, &ran)

	w := getGated(r, "/api/releases/1/download/1")

	if got := checkedProduct.Load().(string); got != gatedTarget {
		t.Fatalf("gate checked entitlement for %q, want the REQUESTED product %q (privilege-escalation gap if it checks the wrong product)", got, gatedTarget)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 — caller entitled to %q must not download %q", w.Code, ownedProduct, gatedTarget)
	}
	if ran.Load() != 0 {
		t.Fatalf("download handler ran on cross-product request — privilege escalation")
	}
}

// TestReleaseDownload_RetryIdempotent_NoLeakOnReplay locks that the gate is a
// pure read: replaying an identical denied request yields the identical denial
// with no presigned URL ever minted. A gate that cached an "allow" after the
// first miss, or that mutated state, would diverge on the second call.
func TestReleaseDownload_RetryIdempotent_NoLeakOnReplay(t *testing.T) {
	gated := map[string]bool{honestyGatedProduct: true}
	lookup := func(_ context.Context, _ int64) (string, bool) { return honestyGatedProduct, true }
	authed := func(_ *gin.Context) (int64, bool) { return 5, true }
	notEntitled := func(_ context.Context, _ int64, _ string) (bool, error) { return false, nil }

	var ran atomic.Int32
	r := buildGatedDownloadRouter(middleware.ReleaseGateConfig{
		GatedProducts:   gated,
		LookupProductID: lookup,
		ResolveAccount:  authed,
		CheckEntitled:   notEntitled,
	}, &ran)

	for attempt := 1; attempt <= 3; attempt++ {
		w := getGated(r, "/api/releases/1/download/1")
		if w.Code != http.StatusForbidden {
			t.Fatalf("attempt %d: status = %d, want 403 (gate must be deterministic on retry)", attempt, w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "" {
			t.Fatalf("attempt %d: presigned URL leaked on retry: %q", attempt, loc)
		}
	}
	if ran.Load() != 0 {
		t.Fatalf("download handler ran %d times across 3 denied retries — non-idempotent gate", ran.Load())
	}
}

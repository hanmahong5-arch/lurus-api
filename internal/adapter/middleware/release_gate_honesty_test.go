package middleware

// release_gate_honesty_test.go — honesty lock for PROC-19.
//
// Claim under audit (commit 97355628):
//
//	"newhub release entitlement gate: config-driven fail-closed download
//	 entitlement gate, default OFF; 8/8 hermetic tests."
//
// The existing release_gate_test.go covers the canonical 8-row truth table.
// This file does NOT re-run those rows. It locks the two PROC-19 properties
// that the existing table asserts only implicitly, and that §4.1 cares about
// most for a fail-closed security control:
//
//   - DEFAULT OFF really means OFF: with an empty GatedProducts set the gate is
//     a pure pass-through even when EVERY dependency is wired to deny — proving
//     the feature cannot accidentally block public downloads when unconfigured.
//   - fail-closed on a SYSTEM ERROR (not just "no entitlement"): an entitlement
//     backend that errors must yield 403, and the downstream handler must NOT
//     run. Distinct from the existing entErr row in that it also asserts the
//     handler-not-reached side effect and uses context.DeadlineExceeded — the
//     realistic "platform slow" failure, the exact case a fail-OPEN bug would
//     turn into a silent allow.
//
// Subject = ReleaseDownloadGateWith; assertions are HTTP status + a
// handler-reached flag, independent of the gate's response text. Not a
// tautology.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// buildGate wires the gate in front of a sentinel handler that records whether
// it ran, so "fail-closed" can be asserted as "handler never reached".
func buildGate(cfg ReleaseGateConfig, reached *atomic.Int32) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/releases/:id/download/:artifact_id",
		ReleaseDownloadGateWith(cfg),
		func(c *gin.Context) {
			reached.Add(1)
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)
	return r
}

func hit(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

// TestReleaseGate_DefaultOff_PublicDownloadUnaffected proves the "default OFF"
// safety property: an empty GatedProducts set passes everything through, even
// with deny-everything dependencies. A regression that treated empty-set as
// "gate all" would break every public download — this lock catches it.
func TestReleaseGate_DefaultOff_PublicDownloadUnaffected(t *testing.T) {
	denyAll := ReleaseGateConfig{
		GatedProducts:   nil, // OFF
		LookupProductID: func(context.Context, int64) (string, bool) { return "lurus-switch", true },
		ResolveAccount:  func(*gin.Context) (int64, bool) { return 0, false }, // anonymous
		CheckEntitled:   func(context.Context, int64, string) (bool, error) { return false, nil },
	}
	var reached atomic.Int32
	r := buildGate(denyAll, &reached)

	w := hit(r, "/api/releases/1/download/1")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (feature OFF must pass through)", w.Code)
	}
	if reached.Load() != 1 {
		t.Fatalf("handler reached %d times, want 1 (OFF must not block)", reached.Load())
	}
}

// TestReleaseGate_FailsClosedOnEntitlementSystemError_honesty drives the gate
// with an entitlement backend that times out (context.DeadlineExceeded). The
// gate MUST return 403 and the download handler must not run — proving the
// fail-CLOSED contract against the realistic platform-slow failure mode.
func TestReleaseGate_FailsClosedOnEntitlementSystemError_honesty(t *testing.T) {
	timeoutBackend := func(ctx context.Context, _ int64, _ string) (bool, error) {
		dctx, cancel := context.WithTimeout(ctx, time.Millisecond)
		defer cancel()
		<-dctx.Done()
		return false, dctx.Err()
	}
	cfg := ReleaseGateConfig{
		GatedProducts:   map[string]bool{"lurus-switch": true},
		LookupProductID: func(context.Context, int64) (string, bool) { return "lurus-switch", true },
		ResolveAccount:  func(*gin.Context) (int64, bool) { return 99, true },
		CheckEntitled:   timeoutBackend,
	}
	var reached atomic.Int32
	r := buildGate(cfg, &reached)

	w := hit(r, "/api/releases/1/download/1")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 on entitlement system error (fail-closed)", w.Code)
	}
	if reached.Load() != 0 {
		t.Fatalf("download handler ran on entitlement error — fail-OPEN bug (security regression)")
	}
}

// TestReleaseGate_MalformedAndMissingDeferToHandler locks the "defer" edges:
// a malformed release id and a not-found release both pass through to the
// handler (which owns 400/404), rather than the gate inventing a status. This
// keeps input-validation and existence checks in one place (single source) and
// prevents the gate from leaking "this id exists / doesn't" via its own status.
func TestReleaseGate_MalformedAndMissingDeferToHandler(t *testing.T) {
	cfg := ReleaseGateConfig{
		GatedProducts: map[string]bool{"lurus-switch": true},
		LookupProductID: func(_ context.Context, id int64) (string, bool) {
			if id == 1 {
				return "lurus-switch", true
			}
			return "", false // not found
		},
		ResolveAccount: func(*gin.Context) (int64, bool) { return 0, false },
		CheckEntitled:  func(context.Context, int64, string) (bool, error) { return false, nil },
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{"malformed id", "/api/releases/not-a-number/download/1"},
		{"not found id", "/api/releases/999/download/1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reached atomic.Int32
			r := buildGate(cfg, &reached)
			w := hit(r, tc.path)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (gate must defer %s to the handler)", w.Code, tc.name)
			}
			if reached.Load() != 1 {
				t.Fatalf("handler reached %d times, want 1 (gate should defer, not abort)", reached.Load())
			}
		})
	}
}

package router

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/gin-gonic/gin"
)

// TestSetApiRouter_SecureVerificationWiring asserts the secure-verification
// feature is wired at the route layer to match the frontend contract
// (web/src/services/secureVerification.js): the verify endpoints exist and
// channel-key reveal is a POST gated route, with the old unprotected GET form
// removed (it both mismatched the frontend's POST and lacked the step-up gate).
func TestSetApiRouter_SecureVerificationWiring(t *testing.T) {
	common.RedisEnabled = false
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	SetApiRouter(engine)

	has := func(method, path string) bool {
		for _, rt := range engine.Routes() {
			if rt.Method == method && rt.Path == path {
				return true
			}
		}
		return false
	}

	cases := []struct {
		method, path string
		want         bool
		why          string
	}{
		{"POST", "/api/verify", true, "frontend posts here to pass session verification"},
		{"GET", "/api/verify/status", true, "verification status companion endpoint"},
		{"POST", "/api/channel/:id/key", true, "frontend posts here to reveal a channel key"},
		{"GET", "/api/channel/:id/key", false, "old unprotected GET must be gone (method mismatch + no step-up gate)"},
	}
	for _, c := range cases {
		if got := has(c.method, c.path); got != c.want {
			t.Errorf("route %s %s present=%v, want %v — %s", c.method, c.path, got, c.want, c.why)
		}
	}
}

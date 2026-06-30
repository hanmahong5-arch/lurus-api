package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// runSecureGate exercises SecureVerificationRequired behind a session-injector
// that optionally sets the authenticated user id and the secure_verified_at
// session value. No DB or Redis is touched (RedisEnabled is disabled by the
// middleware package's auth_test init).
func runSecureGate(setID, setSession bool, sessionVal interface{}) *httptest.ResponseRecorder {
	r := gin.New()
	r.Use(gin.Recovery())
	store := cookie.NewStore([]byte("secure-verify-test-secret"))
	r.Use(sessions.Sessions("session", store))
	r.Use(func(c *gin.Context) {
		if setID {
			c.Set("id", 1)
		}
		if setSession {
			s := sessions.Default(c)
			s.Set(SecureVerificationSessionKey, sessionVal)
			_ = s.Save()
		}
		c.Next()
	})
	r.GET("/gated", SecureVerificationRequired(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/gated", nil))
	return w
}

func secureGateBodyCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal body %q: %v", w.Body.String(), err)
	}
	return resp.Code
}

// Not logged in (no id in context) -> 401, before any verification check.
func TestSecureVerificationRequired_NotLoggedIn_401(t *testing.T) {
	w := runSecureGate(false, false, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// Logged in but never verified -> 403 with the code the frontend keys off.
func TestSecureVerificationRequired_NoVerification_403Required(t *testing.T) {
	w := runSecureGate(true, false, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if code := secureGateBodyCode(t, w); code != "VERIFICATION_REQUIRED" {
		t.Errorf("code = %q, want VERIFICATION_REQUIRED", code)
	}
}

// Fresh verification timestamp -> request passes through to the handler.
func TestSecureVerificationRequired_Fresh_Passes(t *testing.T) {
	w := runSecureGate(true, true, time.Now().Unix())
	if w.Code != http.StatusOK || w.Body.String() != "ok" {
		t.Fatalf("status=%d body=%q, want 200 ok", w.Code, w.Body.String())
	}
}

// Timestamp older than the window -> 403 VERIFICATION_EXPIRED.
func TestSecureVerificationRequired_Expired_403Expired(t *testing.T) {
	old := time.Now().Unix() - int64(SecureVerificationTimeout) - 100
	w := runSecureGate(true, true, old)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if code := secureGateBodyCode(t, w); code != "VERIFICATION_EXPIRED" {
		t.Errorf("code = %q, want VERIFICATION_EXPIRED", code)
	}
}

// elapsed == timeout is treated as expired (the check is >=), guarding the boundary.
func TestSecureVerificationRequired_BoundaryAtTimeout_403Expired(t *testing.T) {
	boundary := time.Now().Unix() - int64(SecureVerificationTimeout)
	w := runSecureGate(true, true, boundary)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 at the exact timeout boundary", w.Code)
	}
	if code := secureGateBodyCode(t, w); code != "VERIFICATION_EXPIRED" {
		t.Errorf("code = %q, want VERIFICATION_EXPIRED", code)
	}
}

// A non-int64 timestamp (corrupt session) -> 403 VERIFICATION_INVALID.
func TestSecureVerificationRequired_InvalidType_403Invalid(t *testing.T) {
	w := runSecureGate(true, true, "not-an-int64")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if code := secureGateBodyCode(t, w); code != "VERIFICATION_INVALID" {
		t.Errorf("code = %q, want VERIFICATION_INVALID", code)
	}
}

// OptionalSecureVerification never blocks; it only annotates the context.
func TestOptionalSecureVerification_SetsFlag(t *testing.T) {
	run := func(setSession bool, val interface{}) (int, bool) {
		r := gin.New()
		r.Use(gin.Recovery())
		store := cookie.NewStore([]byte("secure-verify-test-secret"))
		r.Use(sessions.Sessions("session", store))
		r.Use(func(c *gin.Context) {
			c.Set("id", 1)
			if setSession {
				s := sessions.Default(c)
				s.Set(SecureVerificationSessionKey, val)
				_ = s.Save()
			}
			c.Next()
		})
		var seen bool
		r.GET("/opt", OptionalSecureVerification(), func(c *gin.Context) {
			seen = c.GetBool("secure_verified")
			c.String(http.StatusOK, "ok")
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/opt", nil))
		return w.Code, seen
	}

	if code, verified := run(true, time.Now().Unix()); code != http.StatusOK || !verified {
		t.Errorf("fresh: code=%d verified=%v, want 200 true", code, verified)
	}
	if code, verified := run(false, nil); code != http.StatusOK || verified {
		t.Errorf("absent: code=%d verified=%v, want 200 false", code, verified)
	}
}

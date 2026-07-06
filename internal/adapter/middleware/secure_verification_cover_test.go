package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// mountOptionalSecure wires OptionalSecureVerification behind a session store,
// seeding an "id" and optionally a verification timestamp.
func mountOptionalSecure(userID int, verifiedAt interface{}) (*gin.Engine, *bool) {
	verified := new(bool)
	r := gin.New()
	r.Use(gin.Recovery())
	store := cookie.NewStore([]byte("secure-cover-secret"))
	r.Use(sessions.Sessions("session", store))
	r.Use(func(c *gin.Context) {
		if userID != 0 {
			c.Set("id", userID)
		}
		s := sessions.Default(c)
		if verifiedAt != nil {
			s.Set(SecureVerificationSessionKey, verifiedAt)
			_ = s.Save()
		}
		c.Next()
	})
	r.GET("/s", OptionalSecureVerification(), func(c *gin.Context) {
		*verified = c.GetBool("secure_verified")
		c.Status(http.StatusOK)
	})
	return r, verified
}

func runSecure(r *gin.Engine) {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/s", nil))
}

func TestOptionalSecureVerification_NoUser(t *testing.T) {
	r, v := mountOptionalSecure(0, nil)
	runSecure(r)
	if *v {
		t.Errorf("secure_verified = true, want false for anonymous")
	}
}

func TestOptionalSecureVerification_NoSessionMark(t *testing.T) {
	r, v := mountOptionalSecure(7, nil)
	runSecure(r)
	if *v {
		t.Errorf("secure_verified = true, want false when no verification mark")
	}
}

func TestOptionalSecureVerification_FreshMark(t *testing.T) {
	r, v := mountOptionalSecure(7, time.Now().Unix())
	runSecure(r)
	if !*v {
		t.Errorf("secure_verified = false, want true for fresh verification")
	}
}

func TestOptionalSecureVerification_ExpiredMark(t *testing.T) {
	// A verification timestamp well beyond the timeout window is treated as
	// unverified (and the stale mark is cleared).
	old := time.Now().Unix() - SecureVerificationTimeout - 100
	r, v := mountOptionalSecure(7, old)
	runSecure(r)
	if *v {
		t.Errorf("secure_verified = true, want false for expired verification")
	}
}

func TestOptionalSecureVerification_WrongType(t *testing.T) {
	r, v := mountOptionalSecure(7, "not-an-int64")
	runSecure(r)
	if *v {
		t.Errorf("secure_verified = true, want false for malformed mark type")
	}
}

package middleware

import (
	"strings"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
	zita "github.com/hanmahong5-arch/zita-sdk-go"
)

// OptionalZitaIdentity is a NON-BLOCKING SDK identity injector for the v2
// console. Unlike zita.Client.AuthMiddleware — which aborts 401 the moment
// the lurus_session cookie is absent or invalid — this middleware resolves
// the cookie (or Bearer token) when present, stuffs the validated
// *zita.Identity into the gin context, and ALWAYS calls c.Next().
//
// Rationale: v2 data routes authenticate via middleware.UserAuth(), which
// historically only honored a gin session or an Authorization bearer. A user
// who logged in through the platform SDK bridge carries a lurus_session
// COOKIE that neither path read, so every data fetch 401'd even though the
// user was authenticated (ADR-0011 Layer C gap). authHelper now consults the
// identity this middleware injects as one accepted auth path; a missing or
// invalid cookie is not an error here — it just means authHelper falls back
// to the gin session / bearer paths and ultimately to a clean 401.
func OptionalZitaIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		if common.ZitaClient == nil {
			c.Next()
			return
		}
		token := readZitaSessionToken(c)
		if token == "" {
			c.Next()
			return
		}
		if id, err := common.ZitaClient.ValidateSession(token); err == nil && id != nil && id.AccountID > 0 {
			c.Set(zita.ContextKey, id)
		}
		c.Next()
	}
}

// readZitaSessionToken mirrors the SDK's unexported readSessionToken: the
// lurus_session cookie wins, falling back to an Authorization: Bearer token.
func readZitaSessionToken(c *gin.Context) string {
	if cookie, err := c.Cookie(zita.SessionCookieName); err == nil {
		if cookie = strings.TrimSpace(cookie); cookie != "" {
			return cookie
		}
	}
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") || strings.HasPrefix(header, "bearer ") {
		return strings.TrimSpace(header[len("Bearer "):])
	}
	return ""
}

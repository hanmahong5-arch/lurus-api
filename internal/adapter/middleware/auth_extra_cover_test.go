package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// seedUserToken creates an enabled user + token and returns (userID, key, token).
// The caller may mutate the returned token and re-save for branch-specific setups.
func seedUserToken(t *testing.T) (int, string, *repo.Token) {
	t.Helper()
	user := &repo.User{
		Username: "authuser", DisplayName: "Auth User", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Email: "auth@local", TenantId: "default", Quota: 1_000_000,
	}
	if err := repo.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	key := common.GetRandomString(48)
	tok := &repo.Token{
		UserId: user.Id, TenantId: "default", Key: key, Status: common.TokenStatusEnabled,
		Name: "auth-token", CreatedTime: common.GetTimestamp(), AccessedTime: common.GetTimestamp(),
		ExpiredTime: -1, UnlimitedQuota: true,
	}
	if err := repo.DB.Create(tok).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	return user.Id, key, tok
}

func mountTokenAuth(path string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	pass := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }
	r.POST(path, TokenAuth(), pass)
	r.GET(path, TokenAuth(), pass)
	return r
}

func TestTokenAuth_ValidToken_Passes(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	_, key, _ := seedUserToken(t)
	r := mountTokenAuth("/v1/chat/completions")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-"+key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for valid token; body=%s", w.Code, w.Body.String())
	}
}

func TestTokenAuth_InvalidToken_401(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	r := mountTokenAuth("/v1/chat/completions")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-nonexistenttoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for invalid token", w.Code)
	}
}

func TestTokenAuth_DisabledUser_403(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	uid, key, _ := seedUserToken(t)
	// Disable the user after token creation.
	if err := repo.DB.Model(&repo.User{}).Where("id = ?", uid).Update("status", common.UserStatusDisabled).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}
	r := mountTokenAuth("/v1/chat/completions")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-"+key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for banned user; body=%s", w.Code, w.Body.String())
	}
}

func TestTokenAuth_IPRestriction_Rejects(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	_, key, tok := seedUserToken(t)
	// Restrict to a CIDR that excludes the httptest client IP (192.0.2.1).
	allow := "10.0.0.0/8"
	tok.AllowIps = &allow
	if err := repo.DB.Save(tok).Error; err != nil {
		t.Fatalf("save token: %v", err)
	}
	r := mountTokenAuth("/v1/chat/completions")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-"+key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for client IP outside token allowlist; body=%s", w.Code, w.Body.String())
	}
}

func TestTokenAuth_DeprecatedGroup_Rejects(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	_, key, tok := seedUserToken(t)
	// A token group that is neither in the user's usable groups nor in the
	// configured GroupRatio must be rejected (403). With empty ratio settings
	// in unit scope, any concrete group is unusable.
	tok.Group = "phantom-group"
	if err := repo.DB.Save(tok).Error; err != nil {
		t.Fatalf("save token: %v", err)
	}
	r := mountTokenAuth("/v1/chat/completions")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-"+key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for unusable token group; body=%s", w.Code, w.Body.String())
	}
}

func TestTokenAuth_AnthropicHeaderExtraction(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	_, key, _ := seedUserToken(t)
	// /v1/messages accepts the key via the x-api-key header (Anthropic style).
	r := mountTokenAuth("/v1/messages")
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	req.Header.Set("x-api-key", "sk-"+key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for x-api-key auth; body=%s", w.Code, w.Body.String())
	}
}

func TestTokenAuth_GeminiQueryKeyExtraction(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	_, key, _ := seedUserToken(t)
	// Gemini path accepts ?key= in the query string.
	r := mountTokenAuth("/v1beta/models/gemini-2.0-flash")
	req := httptest.NewRequest(http.MethodGet, "/v1beta/models/gemini-2.0-flash?key=sk-"+key, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for gemini query-key auth; body=%s", w.Code, w.Body.String())
	}
}

func TestTokenAuth_WebSocketProtocolExtraction(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	_, key, _ := seedUserToken(t)
	r := mountTokenAuth("/v1/realtime")
	req := httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "realtime, openai-insecure-api-key.sk-"+key+", openai-beta.realtime-v1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for websocket-protocol key auth; body=%s", w.Code, w.Body.String())
	}
}

// -------------------- PlaygroundAuth --------------------

func TestPlaygroundAuth_BearerDelegatesToTokenAuth(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	_, key, _ := seedUserToken(t)
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/pg", PlaygroundAuth(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodPost, "/pg", nil)
	req.Header.Set("Authorization", "Bearer sk-"+key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (Bearer delegates to TokenAuth); body=%s", w.Code, w.Body.String())
	}
}

func TestPlaygroundAuth_SessionResolvesFirstToken(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	uid, _, _ := seedUserToken(t)
	r := gin.New()
	r.Use(gin.Recovery())
	store := cookie.NewStore([]byte("pg-sess-secret"))
	r.Use(sessions.Sessions("session", store))
	r.Use(func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("id", uid)
		_ = s.Save()
		c.Next()
	})
	r.POST("/pg", PlaygroundAuth(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodPost, "/pg", nil) // no Authorization → session path
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (session resolves user's first token); body=%s", w.Code, w.Body.String())
	}
}

func TestPlaygroundAuth_SessionInvalidType_401(t *testing.T) {
	_, cleanup := setupCoverDB(t)
	defer cleanup()
	r := gin.New()
	r.Use(gin.Recovery())
	store := cookie.NewStore([]byte("pg-bad-secret"))
	r.Use(sessions.Sessions("session", store))
	r.Use(func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("id", "not-an-int") // wrong type
		_ = s.Save()
		c.Next()
	})
	r.POST("/pg", PlaygroundAuth(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodPost, "/pg", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for non-integer session id", w.Code)
	}
}

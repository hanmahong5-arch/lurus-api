package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var scopeTestDBCounter atomic.Int64

// setupScopeTestRouter is the integration harness for TokenAuth scope
// enforcement. It builds an isolated sqlite-backed Token row and wraps the
// real TokenAuth() middleware around a no-op 200 handler. Anything other
// than 200 means the middleware rejected the request — which is what we
// want to measure: not whether HasScope/PathToRequiredScope are correct in
// isolation (those are covered by token_scope_test.go and types/token_scope_test.go)
// but whether TokenAuth wires them together correctly against a real DB row.
//
// Token.Group is left empty so the group-ratio block in TokenAuth is
// skipped — that block requires ratio_setting state that the wider service
// boot sequence prepares, not the middleware unit. The scope branch we
// care about runs BEFORE the group block, so this slim setup faithfully
// exercises the scope path.
func setupScopeTestRouter(t *testing.T, tokenScopes string) (*gin.Engine, string, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbName := fmt.Sprintf("file:scope_test_%d?mode=memory&cache=shared", scopeTestDBCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	tables := []interface{}{&repo.User{}, &repo.Token{}}
	for _, tbl := range tables {
		if err := db.AutoMigrate(tbl); err != nil {
			// SQLite uses global index names; composite indexes shared across
			// models produce harmless "already exists" errors during migrate.
			if !strings.Contains(err.Error(), "already exists") {
				t.Fatalf("migrate %T: %v", tbl, err)
			}
		}
	}

	prevDB := repo.DB
	prevSQLite := common.UsingSQLite
	prevPG := common.UsingPostgreSQL
	prevRedis := common.RedisEnabled

	repo.DB = db
	repo.InitCol()
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMapRWMutex.Unlock()

	user := &repo.User{
		Username:    "scopetestuser",
		DisplayName: "Scope Test User",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Email:       "scopetest@local",
		TenantId:    "default",
		Quota:       1_000_000,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	tokenKey := common.GetRandomString(48)
	token := &repo.Token{
		UserId:         user.Id,
		TenantId:       "default",
		Key:            tokenKey,
		Status:         common.TokenStatusEnabled,
		Name:           "scope-test",
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		// Group intentionally empty — see godoc above.
		Scopes: tokenScopes,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	pass := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"success": true}) }
	// Mount one route per scope branch so each test probe hits the real
	// TokenAuth → PathToRequiredScope → HasScope sequence.
	r.POST("/v1/chat/completions", TokenAuth(), pass)
	r.POST("/v1/messages", TokenAuth(), pass)
	r.POST("/v1/images/generations", TokenAuth(), pass)
	r.POST("/v1/embeddings", TokenAuth(), pass)
	r.POST("/v1/audio/speech", TokenAuth(), pass)
	r.POST("/v1/audio/music", TokenAuth(), pass)
	r.POST("/mj/submit/imagine", TokenAuth(), pass)
	r.POST("/suno/submit/music", TokenAuth(), pass)
	r.GET("/v1/models", TokenAuth(), pass) // info: no scope check

	cleanup := func() {
		repo.DB = prevDB
		common.UsingSQLite = prevSQLite
		common.UsingPostgreSQL = prevPG
		common.RedisEnabled = prevRedis
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return r, tokenKey, cleanup
}

func doProbe(t *testing.T, r *gin.Engine, method, path, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestTokenAuth_ScopeEmpty_AllowsAllPaths is the backward-compat guarantee:
// every Token row issued before migration 015 has Scopes="" and must keep
// working on every relay endpoint. Breaking this rejects production traffic
// the moment migration 015 ships, so it is a P0 gate.
func TestTokenAuth_ScopeEmpty_AllowsAllPaths(t *testing.T) {
	r, key, cleanup := setupScopeTestRouter(t, "")
	defer cleanup()

	paths := []string{
		"/v1/chat/completions",
		"/v1/messages",
		"/v1/images/generations",
		"/v1/embeddings",
		"/v1/audio/speech",
		"/v1/audio/music",
		"/mj/submit/imagine",
		"/suno/submit/music",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			w := doProbe(t, r, http.MethodPost, p, key)
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 (empty scopes must allow %s); body=%s",
					w.Code, p, w.Body.String())
			}
		})
	}
}

// TestTokenAuth_ScopeChat_AllowsChatPath proves the positive match path —
// scope present in allowlist → 200.
func TestTokenAuth_ScopeChat_AllowsChatPath(t *testing.T) {
	r, key, cleanup := setupScopeTestRouter(t, "chat")
	defer cleanup()

	w := doProbe(t, r, http.MethodPost, "/v1/chat/completions", key)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (chat scope must allow chat path); body=%s",
			w.Code, w.Body.String())
	}
}

// TestTokenAuth_ScopeChat_RejectsImagePath is the headline scope enforcement
// guarantee: a chat-only token must NOT be able to generate images. If this
// regresses, every customer who issued a scope-restricted token gets silently
// full permissions — the worst possible failure mode for an enterprise feature.
func TestTokenAuth_ScopeChat_RejectsImagePath(t *testing.T) {
	r, key, cleanup := setupScopeTestRouter(t, "chat")
	defer cleanup()

	w := doProbe(t, r, http.MethodPost, "/v1/images/generations", key)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (chat-only token must reject image path); body=%s",
			w.Code, w.Body.String())
	}
}

// TestTokenAuth_ScopeMultiple_AllowsListedRejectsUnlisted exercises the
// multi-scope case end-to-end through the same TokenAuth path the relay
// router uses in production.
func TestTokenAuth_ScopeMultiple_AllowsListedRejectsUnlisted(t *testing.T) {
	r, key, cleanup := setupScopeTestRouter(t, "chat,image")
	defer cleanup()

	allowed := []string{"/v1/chat/completions", "/v1/images/generations"}
	rejected := []string{"/v1/embeddings", "/v1/audio/speech", "/mj/submit/imagine"}

	for _, p := range allowed {
		t.Run("allow "+p, func(t *testing.T) {
			w := doProbe(t, r, http.MethodPost, p, key)
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 (chat+image must allow %s); body=%s",
					w.Code, p, w.Body.String())
			}
		})
	}
	for _, p := range rejected {
		t.Run("reject "+p, func(t *testing.T) {
			w := doProbe(t, r, http.MethodPost, p, key)
			if w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want 403 (chat+image must reject %s); body=%s",
					w.Code, p, w.Body.String())
			}
		})
	}
}

// TestTokenAuth_ScopeAudio_TaskPathStillRejected verifies the ordering in
// PathToRequiredScope — /v1/audio/music is a TASK scope, not AUDIO, so an
// audio-only token must NOT smuggle through to async music generation.
func TestTokenAuth_ScopeAudio_TaskPathStillRejected(t *testing.T) {
	r, key, cleanup := setupScopeTestRouter(t, "audio")
	defer cleanup()

	w := doProbe(t, r, http.MethodPost, "/v1/audio/music", key)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (audio scope must NOT grant /v1/audio/music — that is the task scope); body=%s",
			w.Code, w.Body.String())
	}
}

// TestTokenAuth_NoScopeRequired_ScopedTokenStillPasses guards the info-
// endpoint exception: GET /v1/models has no required scope, so even a
// chat-only token must still be able to list models. Otherwise UIs that
// list models before issuing a chat request break.
func TestTokenAuth_NoScopeRequired_ScopedTokenStillPasses(t *testing.T) {
	r, key, cleanup := setupScopeTestRouter(t, "chat")
	defer cleanup()

	w := doProbe(t, r, http.MethodGet, "/v1/models", key)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (info endpoint must not require scope); body=%s",
			w.Code, w.Body.String())
	}
}

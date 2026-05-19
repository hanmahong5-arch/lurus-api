package handler

import (
	"encoding/json"
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

// Chat is a single-model thin wrapper over the same self-HTTP loopback
// that the Playground fan-out uses. The hermetic setup mirrors
// playground_fanout_test.go so the only difference is the route handler
// under test and the seeded tenant (Chat enforces GetTenantBySlug,
// Playground does not).

var chatTestDBCounter atomic.Int64

type chatCtx struct {
	router       *gin.Engine
	db           *gorm.DB
	userID       int
	tenantSlug   string
	mockUpstream *httptest.Server
	upstreamHits atomic.Int32
	cleanup      func()
}

func setupChatRouter(t *testing.T, upstreamHandler http.HandlerFunc) *chatCtx {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:chat%d?mode=memory&cache=shared", chatTestDBCounter.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, tbl := range []interface{}{
		&repo.User{}, &repo.Token{}, &repo.Tenant{}, &repo.Option{},
	} {
		if err := db.AutoMigrate(tbl); err != nil && !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("auto migrate %T: %v", tbl, err)
		}
	}

	prevDB := repo.DB
	prevLogDB := repo.LOG_DB
	prevSQLite := common.UsingSQLite
	prevPG := common.UsingPostgreSQL
	prevRedis := common.RedisEnabled
	prevBase := playgroundUpstreamBaseURL

	repo.DB = db
	repo.LOG_DB = db
	repo.InitCol()
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	user := &repo.User{
		Username: "chat-tester", DisplayName: "Chat Tester",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Email: "chat@test.local",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	tenant := &repo.Tenant{Id: "acme", Slug: "acme", Name: "Acme Co"}
	if err := db.Create(tenant).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}

	ctx := &chatCtx{
		db:         db,
		userID:     user.Id,
		tenantSlug: "acme",
	}

	ctx.mockUpstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx.upstreamHits.Add(1)
		upstreamHandler(w, r)
	}))
	playgroundUpstreamBaseURL = ctx.mockUpstream.URL

	router := gin.New()
	mockAuth := func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Next()
	}
	router.POST("/api/v2/:tenant_slug/chat/send", mockAuth, ChatSend)
	ctx.router = router

	ctx.cleanup = func() {
		ctx.mockUpstream.Close()
		playgroundUpstreamBaseURL = prevBase
		repo.DB = prevDB
		repo.LOG_DB = prevLogDB
		common.UsingSQLite = prevSQLite
		common.UsingPostgreSQL = prevPG
		common.RedisEnabled = prevRedis
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	t.Cleanup(ctx.cleanup)
	return ctx
}

func postChat(ctx *chatCtx, slug string, body interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v2/"+slug+"/chat/send",
		strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx.router.ServeHTTP(w, req)
	return w
}

func chatEchoUpstream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model    string              `json:"model"`
		Messages []map[string]string `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	// Echo last user message + turn count so multi-turn tests can assert
	// the full history was forwarded.
	lastUser := ""
	turns := 0
	for _, m := range req.Messages {
		if m["role"] == "user" {
			lastUser = m["content"]
			turns++
		}
	}
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]string{
				"role":    "assistant",
				"content": fmt.Sprintf("%s [turn %d via %s]", lastUser, turns, req.Model),
			}},
		},
		"usage": map[string]int{"prompt_tokens": 12, "completion_tokens": 24},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ─── Tests ──────────────────────────────────────────────────────────────────

// 1. Happy path — single turn → 200 with {message, usage, latency_ms}.
//    Multi-turn history is faithfully forwarded to the upstream.
func TestV2Chat_HappyPath(t *testing.T) {
	ctx := setupChatRouter(t, chatEchoUpstream)

	w := postChat(ctx, ctx.tenantSlug, map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
			{"role": "assistant", "content": "hi"},
			{"role": "user", "content": "how are you"},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if got := ctx.upstreamHits.Load(); got != 1 {
		t.Errorf("upstream hits = %d, want 1", got)
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	msg := data["message"].(map[string]interface{})
	if msg["role"] != "assistant" {
		t.Errorf("role = %v, want assistant", msg["role"])
	}
	wantContent := "how are you [turn 2 via gpt-4o]"
	if msg["content"] != wantContent {
		t.Errorf("content = %q, want %q (multi-turn history must be forwarded)", msg["content"], wantContent)
	}
	usage := data["usage"].(map[string]interface{})
	if usage["prompt_tokens"].(float64) != 12 {
		t.Errorf("prompt_tokens = %v, want 12", usage["prompt_tokens"])
	}
	if data["latency_ms"] == nil {
		t.Error("latency_ms missing from response")
	}
}

// 2. Tenant 404 — unknown slug short-circuits before upstream.
func TestV2Chat_TenantNotFound(t *testing.T) {
	ctx := setupChatRouter(t, chatEchoUpstream)

	w := postChat(ctx, "no-such-tenant", map[string]interface{}{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if got := ctx.upstreamHits.Load(); got != 0 {
		t.Errorf("upstream hits = %d, want 0 (tenant guard must short-circuit)", got)
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error_code"] != "TENANT_NOT_FOUND" {
		t.Errorf("error_code = %v, want TENANT_NOT_FOUND", resp["error_code"])
	}
}

// 3. Upstream error surfaces as 502 with the upstream's error_code,
//    not as 200-with-empty-content. Wave 2 v1: chat returns a single
//    failure (vs Playground's per-column isolation) since there's only
//    one column.
func TestV2Chat_UpstreamErrorSurfaces(t *testing.T) {
	ctx := setupChatRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "upstream model unavailable",
				"code":    "MODEL_UNAVAILABLE",
			},
		})
	})

	w := postChat(ctx, ctx.tenantSlug, map[string]interface{}{
		"model":    "broken-model",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error_code"] != "MODEL_UNAVAILABLE" {
		t.Errorf("error_code = %v, want MODEL_UNAVAILABLE", resp["error_code"])
	}
}

// 4. Validation — empty messages array → 400, no upstream call.
func TestV2Chat_EmptyMessages(t *testing.T) {
	ctx := setupChatRouter(t, chatEchoUpstream)

	w := postChat(ctx, ctx.tenantSlug, map[string]interface{}{
		"model":    "gpt-4o",
		"messages": []map[string]string{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if got := ctx.upstreamHits.Load(); got != 0 {
		t.Errorf("upstream hits = %d, want 0", got)
	}
}

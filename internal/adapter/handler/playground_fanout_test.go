package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Test infrastructure for PlaygroundFanOut. We stub the upstream
// /v1/chat/completions with httptest.NewServer and point
// playgroundUpstreamBaseURL at it, so the handler runs hermetically
// without standing up the full relay stack.

var fanoutTestDBCounter atomic.Int64

type fanoutCtx struct {
	router        *gin.Engine
	db            *gorm.DB
	userID        int
	tenantSlug    string
	mockUpstream  *httptest.Server
	upstreamCalls atomic.Int32
	cleanup       func()
}

func setupFanoutRouter(t *testing.T, upstreamHandler http.HandlerFunc) *fanoutCtx {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:fanout%d?mode=memory&cache=shared", fanoutTestDBCounter.Add(1))
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

	// Seed user.
	user := &repo.User{
		Username: "pg-tester", DisplayName: "Playground Tester",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Email: "pg@test.local",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	ctx := &fanoutCtx{
		db:         db,
		userID:     user.Id,
		tenantSlug: "acme",
	}

	// Mock upstream — wraps caller-supplied handler so we can count calls.
	ctx.mockUpstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx.upstreamCalls.Add(1)
		upstreamHandler(w, r)
	}))
	playgroundUpstreamBaseURL = ctx.mockUpstream.URL

	router := gin.New()
	// Mock UserAuth: just set c.Set("id", user.Id) per request.
	mockAuth := func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Next()
	}
	router.POST("/api/v2/:tenant_slug/playground/run", mockAuth, PlaygroundFanOut)
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

func postFanout(ctx *fanoutCtx, body interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v2/"+ctx.tenantSlug+"/playground/run",
		strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	ctx.router.ServeHTTP(w, req)
	return w
}

func parseFanout(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("parse body: %v — raw: %s", err, w.Body.String())
	}
	return out
}

// echoUpstream returns an OpenAI-shaped response with content = the user's
// message + " [from {model}]". Lets tests assert per-model routing.
func echoUpstream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model    string                  `json:"model"`
		Messages []map[string]string     `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	userMsg := ""
	for _, m := range req.Messages {
		if m["role"] == "user" {
			userMsg = m["content"]
		}
	}
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]string{"role": "assistant", "content": userMsg + " [from " + req.Model + "]"}},
		},
		"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 20},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ─── Tests ──────────────────────────────────────────────────────────────────

// 1. Happy path — 3 models in parallel, each gets its own row keyed by
//    model name, content reflects per-model echo. Asserts fan-out actually
//    fanned (3 upstream calls, not 1).
func TestPlaygroundFanOut_HappyPath(t *testing.T) {
	ctx := setupFanoutRouter(t, echoUpstream)

	w := postFanout(ctx, map[string]interface{}{
		"system": "You are concise.",
		"user":   "What is 2+2?",
		"models": []string{"gpt-4o", "claude-3.5-sonnet", "gemini-1.5-pro"},
		"params": map[string]float64{"temperature": 0.7, "top_p": 1.0},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if got := ctx.upstreamCalls.Load(); got != 3 {
		t.Errorf("upstream calls = %d, want 3 (true fan-out)", got)
	}

	resp := parseFanout(t, w)
	items := resp["data"].(map[string]interface{})["items"].([]interface{})
	if len(items) != 3 {
		t.Fatalf("items len = %d, want 3", len(items))
	}
	// Order MUST match request order (frontend depends on column index).
	expected := []string{"gpt-4o", "claude-3.5-sonnet", "gemini-1.5-pro"}
	for i, it := range items {
		m := it.(map[string]interface{})
		if m["model"] != expected[i] {
			t.Errorf("items[%d].model = %v, want %s", i, m["model"], expected[i])
		}
		want := "What is 2+2? [from " + expected[i] + "]"
		if m["content"] != want {
			t.Errorf("items[%d].content = %v, want %q", i, m["content"], want)
		}
		if m["prompt_tokens"].(float64) != 10 {
			t.Errorf("items[%d].prompt_tokens = %v, want 10", i, m["prompt_tokens"])
		}
	}
}

// 2. Per-column errors are isolated — one model fails, others still return
//    content. Asserts the fan-out doesn't first-failure-aborts-everything.
func TestPlaygroundFanOut_PerColumnErrorIsolation(t *testing.T) {
	ctx := setupFanoutRouter(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string              `json:"model"`
			Messages []map[string]string `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model == "broken-model" {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{"message": "upstream down", "code": "BAD_GATEWAY"},
			})
			return
		}
		// Replay echoUpstream against the already-parsed body.
		userMsg := ""
		for _, m := range req.Messages {
			if m["role"] == "user" {
				userMsg = m["content"]
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": userMsg + " [from " + req.Model + "]"}},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 20},
		})
	})

	w := postFanout(ctx, map[string]interface{}{
		"user":   "ping",
		"models": []string{"gpt-4o", "broken-model", "gemini-1.5-pro"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("fan-out HTTP code = %d, want 200 (per-column errors don't fail the call)", w.Code)
	}

	items := parseFanout(t, w)["data"].(map[string]interface{})["items"].([]interface{})

	col0 := items[0].(map[string]interface{})
	if col0["error_code"] != nil && col0["error_code"] != "" {
		t.Errorf("col0 should have no error, got %v", col0["error_code"])
	}
	if !strings.Contains(col0["content"].(string), "[from gpt-4o]") {
		t.Errorf("col0 content missing model echo: %v", col0["content"])
	}

	col1 := items[1].(map[string]interface{})
	if col1["error_code"] != "BAD_GATEWAY" {
		t.Errorf("col1 error_code = %v, want BAD_GATEWAY", col1["error_code"])
	}
	if col1["error_message"] != "upstream down" {
		t.Errorf("col1 error_message = %v, want 'upstream down'", col1["error_message"])
	}

	col2 := items[2].(map[string]interface{})
	if col2["error_code"] != nil && col2["error_code"] != "" {
		t.Errorf("col2 should have no error, got %v", col2["error_code"])
	}
}

// 3. system message is optional — request without `system` still works,
//    upstream gets a messages array of length 1 (user only).
func TestPlaygroundFanOut_SystemOptional(t *testing.T) {
	var capturedMessages [][]map[string]string
	var mu sync.Mutex

	ctx := setupFanoutRouter(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []map[string]string `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		capturedMessages = append(capturedMessages, req.Messages)
		mu.Unlock()
		echoUpstreamReplay(w, req.Messages)
	})

	w := postFanout(ctx, map[string]interface{}{
		"user":   "no system here",
		"models": []string{"gpt-4o"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(capturedMessages) != 1 {
		t.Fatalf("captured calls = %d, want 1", len(capturedMessages))
	}
	if len(capturedMessages[0]) != 1 {
		t.Errorf("messages len = %d, want 1 (system absent → only user)", len(capturedMessages[0]))
	}
	if capturedMessages[0][0]["role"] != "user" {
		t.Errorf("messages[0].role = %v, want user", capturedMessages[0][0]["role"])
	}
}

// echoUpstreamReplay is like echoUpstream but takes already-parsed
// messages — used by tests that want to capture the messages slice.
func echoUpstreamReplay(w http.ResponseWriter, messages []map[string]string) {
	content := ""
	for _, m := range messages {
		if m["role"] == "user" {
			content = m["content"]
		}
	}
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]string{"content": content + " [ack]"}},
		},
		"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 8},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// 4. Validation — missing required `user` field → 400 INVALID_REQUEST,
//    no upstream calls fired.
func TestPlaygroundFanOut_MissingUserField(t *testing.T) {
	ctx := setupFanoutRouter(t, echoUpstream)
	w := postFanout(ctx, map[string]interface{}{
		"models": []string{"gpt-4o"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if got := ctx.upstreamCalls.Load(); got != 0 {
		t.Errorf("upstream calls = %d, want 0 (validation must short-circuit)", got)
	}
	resp := parseFanout(t, w)
	if resp["error_code"] != "INVALID_REQUEST" {
		t.Errorf("error_code = %v, want INVALID_REQUEST", resp["error_code"])
	}
}

// 5. Validation — empty models array → 400. Same short-circuit guarantee.
func TestPlaygroundFanOut_EmptyModels(t *testing.T) {
	ctx := setupFanoutRouter(t, echoUpstream)
	w := postFanout(ctx, map[string]interface{}{
		"user":   "hi",
		"models": []string{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if got := ctx.upstreamCalls.Load(); got != 0 {
		t.Errorf("upstream calls = %d, want 0", got)
	}
}

// 6. Latency_ms is populated and reasonable (>= 0, < 5s on a 100ms-stalled
//    upstream — generous bound to avoid CI flakes).
func TestPlaygroundFanOut_LatencyMeasured(t *testing.T) {
	ctx := setupFanoutRouter(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		echoUpstream(w, r)
	})

	w := postFanout(ctx, map[string]interface{}{
		"user":   "measure me",
		"models": []string{"gpt-4o"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	col0 := parseFanout(t, w)["data"].(map[string]interface{})["items"].([]interface{})[0].(map[string]interface{})
	latency := col0["latency_ms"].(float64)
	if latency < 50 {
		t.Errorf("latency_ms = %v, want >= 50 (upstream stalls 50ms)", latency)
	}
	if latency > 5000 {
		t.Errorf("latency_ms = %v, suspiciously large", latency)
	}
}

// 7. Token resolution — user with NO existing token gets one auto-created
//    on first call (mirrors PlaygroundAuth). Subsequent calls reuse it.
func TestPlaygroundFanOut_AutoCreatesDefaultToken(t *testing.T) {
	ctx := setupFanoutRouter(t, echoUpstream)

	// Pre-check: user has no tokens.
	var pre int64
	ctx.db.Model(&repo.Token{}).Where("user_id = ?", ctx.userID).Count(&pre)
	if pre != 0 {
		t.Fatalf("seed user should start with 0 tokens, got %d", pre)
	}

	w := postFanout(ctx, map[string]interface{}{
		"user":   "auto-create me a token",
		"models": []string{"gpt-4o"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}

	var post int64
	ctx.db.Model(&repo.Token{}).Where("user_id = ?", ctx.userID).Count(&post)
	if post != 1 {
		t.Errorf("after fan-out, token count = %d, want 1 (auto-create)", post)
	}

	// Second call should reuse, not create a 2nd.
	w2 := postFanout(ctx, map[string]interface{}{
		"user":   "second call",
		"models": []string{"gpt-4o"},
	})
	if w2.Code != http.StatusOK {
		t.Fatalf("2nd status = %d", w2.Code)
	}
	var post2 int64
	ctx.db.Model(&repo.Token{}).Where("user_id = ?", ctx.userID).Count(&post2)
	if post2 != 1 {
		t.Errorf("after 2nd call, token count = %d, want 1 (reuse)", post2)
	}
}

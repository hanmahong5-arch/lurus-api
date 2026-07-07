package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	relayconstant "github.com/LurusTech/lurus-hub/internal/adapter/provider/constant"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"

	"github.com/gin-gonic/gin"
)

// ─── response_capture pure branches ──────────────────────────────────────────

// TestBufferedResponseWriter_BytesNilBuffer covers the nil-buffer guard in
// Bytes(): a zero-value BufferedResponseWriter (buf == nil) must return nil
// rather than panicking.
func TestBufferedResponseWriter_BytesNilBuffer(t *testing.T) {
	var b BufferedResponseWriter // buf is nil
	if got := b.Bytes(); got != nil {
		t.Errorf("Bytes() on nil buffer = %v, want nil", got)
	}
}

// TestBufferedResponseWriter_AppendCappedSecondWriteFullBuffer covers the
// remaining<=0 early-return branch of appendCapped: once the buffer is filled
// to cap, a subsequent Write forwards all bytes but adds nothing to the buffer
// and only sets the truncated flag.
func TestBufferedResponseWriter_AppendCappedSecondWriteFullBuffer(t *testing.T) {
	inner, rec := newTestWriter(t)
	const cap = 8
	w := NewBufferedResponseWriter(inner, cap)

	// First write exactly fills the buffer to cap (no truncation yet).
	first := bytes.Repeat([]byte("A"), cap)
	if _, err := w.Write(first); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	if w.Truncated() {
		t.Fatalf("buffer should not be truncated after exactly filling to cap")
	}
	// Second write hits remaining<=0: forwards to client, buffer stays at cap,
	// truncated flips true.
	second := []byte("BBBB")
	if _, err := w.Write(second); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	if w.buf.Len() != cap {
		t.Errorf("buffer len = %d, want capped at %d", w.buf.Len(), cap)
	}
	if !w.Truncated() {
		t.Error("Truncated() should be true after writing past a full buffer")
	}
	// Every byte still reached the client.
	if rec.Body.Len() != len(first)+len(second) {
		t.Errorf("client saw %d bytes, want %d", rec.Body.Len(), len(first)+len(second))
	}
}

// ─── ClaudeHelper config branches ────────────────────────────────────────────

// TestClaudeHelper_DefaultMaxTokensWhenZero covers the MaxTokens==0 default
// branch: a request with MaxTokens 0 must have a positive default injected so
// the outgoing Anthropic body carries a non-zero max_tokens.
func TestClaudeHelper_DefaultMaxTokensWhenZero(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "claude-defmax", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(claudeMessagesBody))
	}))
	defer srv.Close()

	claudeReq := &dto.ClaudeRequest{
		Model:    "claude-3-5-sonnet",
		Messages: []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
		// MaxTokens intentionally omitted (0) to hit the default branch.
	}
	c, info := wireClaudeUpstream(t, srv.URL, "claude-3-5-sonnet", claudeReq)
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := ClaudeHelper(c, info); err != nil {
		t.Fatalf("ClaudeHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("max_tokens")) {
		t.Errorf("expected default max_tokens in outgoing body: %s", sawBody)
	}
}

// TestClaudeHelper_SystemPromptOverrideStringPrepend covers the
// SystemPromptOverride branch where the caller already supplies a STRING
// system prompt: the channel prompt must be prepended without dropping the
// caller's system text.
func TestClaudeHelper_SystemPromptOverrideStringPrepend(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "claude-sysovr-str", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(claudeMessagesBody))
	}))
	defer srv.Close()

	claudeReq := &dto.ClaudeRequest{
		Model:     "claude-3-5-sonnet",
		MaxTokens: 16,
		System:    "CALLER-SYSTEM",
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
	}
	c, info := wireClaudeUpstream(t, srv.URL, "claude-3-5-sonnet", claudeReq)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		SystemPrompt:         "CHANNEL-SYSTEM",
		SystemPromptOverride: true,
	})
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := ClaudeHelper(c, info); err != nil {
		t.Fatalf("ClaudeHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("CHANNEL-SYSTEM")) {
		t.Errorf("channel system prompt not prepended: %s", sawBody)
	}
	if !bytes.Contains(sawBody, []byte("CALLER-SYSTEM")) {
		t.Errorf("caller system prompt was dropped: %s", sawBody)
	}
}

// TestClaudeHelper_SystemPromptOverrideParsedPrepend covers the
// SystemPromptOverride branch where the caller supplies a STRUCTURED (array)
// system prompt: the channel prompt is prepended as a new text block ahead of
// the caller's blocks.
func TestClaudeHelper_SystemPromptOverrideParsedPrepend(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "claude-sysovr-arr", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(claudeMessagesBody))
	}))
	defer srv.Close()

	sysBlock := dto.ClaudeMediaMessage{Type: dto.ContentTypeText}
	sysBlock.SetText("CALLER-BLOCK")
	claudeReq := &dto.ClaudeRequest{
		Model:     "claude-3-5-sonnet",
		MaxTokens: 16,
		System:    []dto.ClaudeMediaMessage{sysBlock},
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
	}
	c, info := wireClaudeUpstream(t, srv.URL, "claude-3-5-sonnet", claudeReq)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		SystemPrompt:         "CHANNEL-BLOCK",
		SystemPromptOverride: true,
	})
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := ClaudeHelper(c, info); err != nil {
		t.Fatalf("ClaudeHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("CHANNEL-BLOCK")) {
		t.Errorf("channel system block not prepended: %s", sawBody)
	}
	if !bytes.Contains(sawBody, []byte("CALLER-BLOCK")) {
		t.Errorf("caller system block was dropped: %s", sawBody)
	}
}

// TestClaudeHelper_PassThroughBody covers the PassThroughBodyEnabled branch:
// the raw request body is forwarded verbatim rather than converted.
func TestClaudeHelper_PassThroughBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "claude-pass", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rawBody := `{"model":"claude-3-5-sonnet","max_tokens":16,"messages":[{"role":"user","content":"claude-passthrough-marker"}]}`
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(claudeMessagesBody))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader([]byte(rawBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-ant-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "claude-3-5-sonnet")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
	c.Set("token_name", "tkn")

	claudeReq := &dto.ClaudeRequest{
		Model:     "claude-3-5-sonnet",
		MaxTokens: 16,
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
	}
	info := &relaycommon.RelayInfo{
		Request:         claudeReq,
		OriginModelName: "claude-3-5-sonnet",
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RequestURLPath:  "/v1/messages",
		StartTime:       time.Now(),
		UserId:          u.Id,
	}

	if err := ClaudeHelper(c, info); err != nil {
		t.Fatalf("ClaudeHelper passthrough returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("claude-passthrough-marker")) {
		t.Errorf("expected raw body forwarded verbatim, got: %s", sawBody)
	}
}

// TestClaudeHelper_UpstreamErrorClassified covers the non-2xx error branch of
// ClaudeHelper against a native Anthropic channel (the OpenAI-channel error
// test does not exercise the claude adaptor's DoResponse error path).
func TestClaudeHelper_UpstreamErrorClassified(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()
	srv := newErr500Server(t)

	claudeReq := &dto.ClaudeRequest{
		Model:     "claude-3-5-sonnet",
		MaxTokens: 16,
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
	}
	c, info := wireClaudeUpstream(t, srv.URL, "claude-3-5-sonnet", claudeReq)
	c.Set("token_name", "tkn")

	if err := ClaudeHelper(c, info); err == nil {
		t.Fatalf("expected upstream 500 error from native claude channel, got nil")
	}
}

// ─── GeminiHelper config branches ────────────────────────────────────────────

const geminiWireModel = "gemini-2.5-flash"

// wireGeminiNative wires a gin.Context + RelayInfo to a native Gemini channel
// pointing at srvURL. Callers supply the channel settings and request.
func wireGeminiNative(t *testing.T, srvURL string, chSetting dto.ChannelSettings, req *dto.GeminiChatRequest, userID int) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/"+geminiWireModel+":generateContent", nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srvURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "gm-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, geminiWireModel)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, chSetting)
	c.Set("token_name", "tkn")
	info := &relaycommon.RelayInfo{
		Request:         req,
		OriginModelName: geminiWireModel,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RequestURLPath:  "/v1beta/models/" + geminiWireModel + ":generateContent",
		StartTime:       time.Now(),
		UserId:          userID,
	}
	return c, info
}

// TestGeminiHelper_SystemPromptOverrideMerge covers the SystemInstructions
// merge branch: when the caller already supplies a system instruction and the
// channel enables override, the channel prompt is prepended to the caller's.
func TestGeminiHelper_SystemPromptOverrideMerge(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "gemini-sysovr", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(geminiGenerateBody))
	}))
	defer srv.Close()

	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}},
		SystemInstructions: &dto.GeminiChatContent{
			Parts: []dto.GeminiPart{{Text: "CALLER-SI"}},
		},
	}
	c, info := wireGeminiNative(t, srv.URL, dto.ChannelSettings{
		SystemPrompt:         "CHANNEL-SI",
		SystemPromptOverride: true,
	}, req, u.Id)

	if err := GeminiHelper(c, info); err != nil {
		t.Fatalf("GeminiHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("CHANNEL-SI")) {
		t.Errorf("channel system instruction not merged in: %s", sawBody)
	}
	if !bytes.Contains(sawBody, []byte("CALLER-SI")) {
		t.Errorf("caller system instruction was dropped: %s", sawBody)
	}
}

// TestGeminiHelper_PassThroughBody covers the PassThroughBodyEnabled branch of
// GeminiHelper: the raw body is forwarded verbatim.
func TestGeminiHelper_PassThroughBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "gemini-pass", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rawBody := `{"contents":[{"parts":[{"text":"gemini-passthrough-marker"}]}]}`
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(geminiGenerateBody))
	}))
	defer srv.Close()

	req := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}}}
	c, info := wireGeminiNative(t, srv.URL, dto.ChannelSettings{PassThroughBodyEnabled: true}, req, u.Id)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/"+geminiWireModel+":generateContent", bytes.NewReader([]byte(rawBody)))
	c.Request.Header.Set("Content-Type", "application/json")

	if err := GeminiHelper(c, info); err != nil {
		t.Fatalf("GeminiHelper passthrough returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("gemini-passthrough-marker")) {
		t.Errorf("expected raw body forwarded verbatim, got: %s", sawBody)
	}
}

// TestGeminiHelper_UpstreamErrorNativeChannel covers the non-200 error branch
// of GeminiHelper against a native Gemini channel.
func TestGeminiHelper_UpstreamErrorNativeChannel(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()
	srv := newErr500Server(t)

	req := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}}}
	c, info := wireGeminiNative(t, srv.URL, dto.ChannelSettings{}, req, 0)

	if err := GeminiHelper(c, info); err == nil {
		t.Fatalf("expected upstream 500 error from native gemini channel, got nil")
	}
}

// ─── TextHelper system-prompt override branch ────────────────────────────────

// TestTextHelper_SystemPromptOverridePrepend covers the override branch where
// the caller already supplies a system message: the channel prompt is prepended
// to the existing system message rather than added as a new one.
func TestTextHelper_SystemPromptOverridePrepend(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "text-sysovr", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatCompletionBody))
	}))
	defer srv.Close()

	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-4o-mini",
		Messages: []dto.Message{
			{Role: "system", Content: "CALLER-SYS"},
			{Role: "user", Content: "hi"},
		},
	}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "gpt-4o-mini", req)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		SystemPrompt:         "CHANNEL-SYS",
		SystemPromptOverride: true,
	})
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := TextHelper(c, info); err != nil {
		t.Fatalf("TextHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("CHANNEL-SYS")) {
		t.Errorf("channel system prompt not prepended: %s", sawBody)
	}
	if !bytes.Contains(sawBody, []byte("CALLER-SYS")) {
		t.Errorf("caller system message dropped: %s", sawBody)
	}
}

// ─── ParamOverride branch across helpers ─────────────────────────────────────

// TestTextHelper_ParamOverride covers the ParamOverride application branch:
// a channel-level param override rewrites a field in the converted body.
func TestTextHelper_ParamOverride(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "text-override", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatCompletionBody))
	}))
	defer srv.Close()

	req := &dto.GeneralOpenAIRequest{Model: "gpt-4o-mini", Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "gpt-4o-mini", req)
	c.Set("token_name", "tkn")
	info.UserId = u.Id
	// ParamOverride is promoted from the embedded *ChannelMeta, which InitChannelMeta
	// (inside TextHelper) populates from this context key.
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"temperature": 0.42})

	if err := TextHelper(c, info); err != nil {
		t.Fatalf("TextHelper param override returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("0.42")) {
		t.Errorf("param override temperature not applied to outgoing body: %s", sawBody)
	}
}

// TestEmbeddingHelper_ParamOverride covers the ParamOverride branch in
// EmbeddingHelper.
func TestEmbeddingHelper_ParamOverride(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "emb-override", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"text-embedding-3-small","usage":{"prompt_tokens":5,"total_tokens":5}}`))
	}))
	defer srv.Close()

	embReq := &dto.EmbeddingRequest{Model: "text-embedding-3-small", Input: "hello"}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/embeddings", relayconstant.RelayModeEmbeddings, "text-embedding-3-small", embReq)
	c.Set("token_name", "tkn")
	info.UserId = u.Id
	// ParamOverride is promoted from the embedded *ChannelMeta, which InitChannelMeta
	// (inside EmbeddingHelper) populates from this context key.
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"dimensions": 256})

	if err := EmbeddingHelper(c, info); err != nil {
		t.Fatalf("EmbeddingHelper param override returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("256")) {
		t.Errorf("param override dimensions not applied: %s", sawBody)
	}
}

// ─── DoResponse parse-error branch (2xx body that fails to parse) ─────────────

// TestEmbeddingHelper_MalformedUpstreamBody covers the DoResponse error branch:
// a 200 upstream returning a body that the OpenAI handler cannot parse must
// surface a non-nil error rather than a hollow success.
func TestEmbeddingHelper_MalformedUpstreamBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`}{ this is not valid json`))
	}))
	defer srv.Close()

	embReq := &dto.EmbeddingRequest{Model: "text-embedding-3-small", Input: "hello"}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/embeddings", relayconstant.RelayModeEmbeddings, "text-embedding-3-small", embReq)
	c.Set("token_name", "tkn")

	if err := EmbeddingHelper(c, info); err == nil {
		t.Fatalf("expected DoResponse parse error for malformed 200 body, got nil")
	}
}

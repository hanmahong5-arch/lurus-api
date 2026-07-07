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

// TestImageHelper_SizeQualityLogAndTokenDefaults drives ImageHelper with a sized
// HD request against an upstream that reports zero usage tokens. This covers the
// token-default fill-in (TotalTokens/PromptTokens ← request.N), the HD quality
// branch, and the size log-content branch that the default happy path skips.
func TestImageHelper_SizeQualityLogAndTokenDefaults(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "img-size", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Zero-usage image response forces the token-default branch.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"created":1,"data":[{"url":"https://cdn/img.png"}]}`))
	}))
	defer srv.Close()

	imgReq := &dto.ImageRequest{Model: "dall-e-3", Prompt: "a cat", N: 1, Size: "1024x1024", Quality: "hd"}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/images/generations", relayconstant.RelayModeImagesGenerations, "dall-e-3", imgReq)
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := ImageHelper(c, info); err != nil {
		t.Fatalf("ImageHelper sized HD returned error: %v", err.Error())
	}
	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1 (post-consume ran)", refreshed.RequestCount)
	}
}

// TestClaudeHelper_ParamOverride covers the ParamOverride branch of ClaudeHelper.
func TestClaudeHelper_ParamOverride(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "claude-ovr", Quota: 10_000_000}
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
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
	}
	c, info := wireClaudeUpstream(t, srv.URL, "claude-3-5-sonnet", claudeReq)
	c.Set("token_name", "tkn")
	info.UserId = u.Id
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"top_k": 7})

	if err := ClaudeHelper(c, info); err != nil {
		t.Fatalf("ClaudeHelper param override returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("top_k")) {
		t.Errorf("param override top_k not applied to claude body: %s", sawBody)
	}
}

// TestClaudeHelper_SystemPromptOverrideEmptyStringSystem covers the override
// branch where the caller's string system prompt is empty: the channel prompt
// replaces it wholesale.
func TestClaudeHelper_SystemPromptOverrideEmptyStringSystem(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "claude-empty-str", Quota: 10_000_000}
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
		System:    "   ", // whitespace-only → treated as empty after trim
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
	}
	c, info := wireClaudeUpstream(t, srv.URL, "claude-3-5-sonnet", claudeReq)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		SystemPrompt:         "CHANNEL-ONLY",
		SystemPromptOverride: true,
	})
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := ClaudeHelper(c, info); err != nil {
		t.Fatalf("ClaudeHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("CHANNEL-ONLY")) {
		t.Errorf("channel system prompt not applied over empty caller system: %s", sawBody)
	}
}

// TestClaudeHelper_SystemPromptOverrideEmptyArraySystem covers the override
// branch where the caller supplies an EMPTY structured system array: the channel
// prompt becomes the sole system block.
func TestClaudeHelper_SystemPromptOverrideEmptyArraySystem(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "claude-empty-arr", Quota: 10_000_000}
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
		System:    []dto.ClaudeMediaMessage{}, // empty structured system
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
	}
	c, info := wireClaudeUpstream(t, srv.URL, "claude-3-5-sonnet", claudeReq)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		SystemPrompt:         "CHANNEL-SOLE-BLOCK",
		SystemPromptOverride: true,
	})
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := ClaudeHelper(c, info); err != nil {
		t.Fatalf("ClaudeHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("CHANNEL-SOLE-BLOCK")) {
		t.Errorf("channel system block not set for empty caller system array: %s", sawBody)
	}
}

// TestGeminiHelper_SystemInstructionEmptyPartsFilled covers the branch where the
// caller supplies a SystemInstructions object with an EMPTY Parts slice: the
// channel prompt fills the empty parts.
func TestGeminiHelper_SystemInstructionEmptyPartsFilled(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "gemini-empty-parts", Quota: 10_000_000}
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
		Contents:           []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}},
		SystemInstructions: &dto.GeminiChatContent{Parts: []dto.GeminiPart{}}, // present but empty
	}
	c, info := wireGeminiNative(t, srv.URL, dto.ChannelSettings{SystemPrompt: "CHANNEL-FILL"}, req, u.Id)

	if err := GeminiHelper(c, info); err != nil {
		t.Fatalf("GeminiHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("CHANNEL-FILL")) {
		t.Errorf("channel prompt did not fill empty system instruction parts: %s", sawBody)
	}
}

// TestGeminiHelper_SystemInstructionOverrideAppends covers the override branch
// where every existing system-instruction part is empty-text: the merge loop
// finds nothing to prepend into and appends the channel prompt as a new part.
func TestGeminiHelper_SystemInstructionOverrideAppends(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "gemini-append", Quota: 10_000_000}
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

	// Two parts, one with inline data (empty Text) so the merge loop skips it and
	// falls through to the append branch.
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}},
		SystemInstructions: &dto.GeminiChatContent{
			Parts: []dto.GeminiPart{{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: "AAAA"}}},
		},
	}
	c, info := wireGeminiNative(t, srv.URL, dto.ChannelSettings{
		SystemPrompt:         "CHANNEL-APPEND",
		SystemPromptOverride: true,
	}, req, u.Id)

	if err := GeminiHelper(c, info); err != nil {
		t.Fatalf("GeminiHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("CHANNEL-APPEND")) {
		t.Errorf("channel prompt not appended when all existing parts were non-text: %s", sawBody)
	}
}

// TestTextHelper_StreamOptionsIncludeUsage covers the StreamOptions handling
// branch of TextHelper: when the caller supplies StreamOptions, the includeUsage
// flag is derived from it before dispatch over the streaming path.
func TestTextHelper_StreamOptionsIncludeUsage(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	prevTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 60
	defer func() { constant.StreamingTimeout = prevTimeout }()

	u := &repo.User{Username: "text-streamopts", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := newStreamServer(t)
	req := &dto.GeneralOpenAIRequest{
		Model:         "gpt-4o-mini",
		Stream:        true,
		StreamOptions: &dto.StreamOptions{IncludeUsage: true},
		Messages:      []dto.Message{{Role: "user", Content: "hi"}},
	}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "gpt-4o-mini", req)
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := TextHelper(c, info); err != nil {
		t.Fatalf("TextHelper stream+options returned error: %v", err.Error())
	}
	if !info.ShouldIncludeUsage {
		t.Errorf("expected ShouldIncludeUsage=true derived from caller StreamOptions")
	}
}

// TestClaudeHelper_MalformedUpstreamBody covers the DoResponse parse-error branch
// of ClaudeHelper against a native Anthropic channel returning an unparseable
// 200 body.
func TestClaudeHelper_MalformedUpstreamBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`}{ not valid claude json`))
	}))
	defer srv.Close()

	claudeReq := &dto.ClaudeRequest{
		Model:     "claude-3-5-sonnet",
		MaxTokens: 16,
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
	}
	c, info := wireClaudeUpstream(t, srv.URL, "claude-3-5-sonnet", claudeReq)
	c.Set("token_name", "tkn")

	if err := ClaudeHelper(c, info); err == nil {
		t.Fatalf("expected DoResponse parse error for malformed 200 claude body, got nil")
	}
}

// TestTextHelper_MalformedUpstreamBody covers the DoResponse parse-error branch
// of TextHelper against a 200 upstream returning an unparseable chat body.
func TestTextHelper_MalformedUpstreamBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`}{ not valid chat json`))
	}))
	defer srv.Close()

	req := &dto.GeneralOpenAIRequest{Model: "gpt-4o-mini", Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "gpt-4o-mini", req)
	c.Set("token_name", "tkn")

	if err := TextHelper(c, info); err == nil {
		t.Fatalf("expected DoResponse parse error for malformed 200 chat body, got nil")
	}
}

// TestGeminiEmbeddingHandler_MalformedUpstreamBody covers the DoResponse
// parse-error branch of GeminiEmbeddingHandler.
func TestGeminiEmbeddingHandler_MalformedUpstreamBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`}{ not valid embedding json`))
	}))
	defer srv.Close()

	body := []byte(`{"model":"text-embedding-004","content":{"parts":[{"text":"hello"}]}}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/text-embedding-004:embedContent", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "gm-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "text-embedding-004")

	info := &relaycommon.RelayInfo{OriginModelName: "text-embedding-004", RequestURLPath: "/v1/embeddings", StartTime: time.Now()}
	if err := GeminiEmbeddingHandler(c, info); err == nil {
		t.Fatalf("expected DoResponse parse error for malformed 200 gemini embedding body, got nil")
	}
}

// TestResponsesHelper_AudioModelConsumesAudioQuota covers the audio-billing
// branch of ResponsesHelper: a gpt-4o-audio model routes post-consume through
// PostAudioConsumeQuota rather than the standard quota path.
func TestResponsesHelper_AudioModelConsumesAudioQuota(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "resp-audio", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responsesBody))
	}))
	defer srv.Close()

	req := &dto.OpenAIResponsesRequest{Model: "gpt-4o-audio-preview", Input: []byte(`"hi"`)}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/responses", relayconstant.RelayModeResponses, "gpt-4o-audio-preview", req)
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := ResponsesHelper(c, info); err != nil {
		t.Fatalf("ResponsesHelper audio model returned error: %v", err.Error())
	}
	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1 (audio post-consume ran)", refreshed.RequestCount)
	}
}

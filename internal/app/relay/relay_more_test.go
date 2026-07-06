package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	relayconstant "github.com/LurusTech/lurus-hub/internal/adapter/provider/constant"
	"github.com/LurusTech/lurus-hub/internal/adapter/repo"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/model_setting"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/system_setting"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"

	"github.com/gin-gonic/gin"
)

// ─── ClaudeHelper happy path (non-stream) ────────────────────────────────────

// wireClaudeUpstream wires a gin.Context + RelayInfo to an Anthropic-format
// upstream, so InitChannelMeta resolves ApiType=Anthropic and GetAdaptor returns
// the real claude adaptor — the request is genuinely converted (native Claude
// passthrough) and the response is parsed by the claude DoResponse.
func wireClaudeUpstream(t *testing.T, srvURL, model string, request dto.Request) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srvURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-ant-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, model)

	info := &relaycommon.RelayInfo{
		Request:         request,
		OriginModelName: model,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RequestURLPath:  "/v1/messages",
		StartTime:       time.Now(),
	}
	return c, info
}

const claudeMessagesBody = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-3-5-sonnet","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":3}}`

// TestClaudeHelper_HappyPath drives the Claude relay success tail against a real
// Anthropic-format upstream. The claude adaptor genuinely runs
// ConvertClaudeRequest → marshal → RemoveDisabledFields → DoRequest (with
// x-api-key auth) → DoResponse → PostClaudeConsumeQuota. The observable contract
// is the seeded user's request count being incremented exactly once.
func TestClaudeHelper_HappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "claude-happy", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawAuth string
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The claude adaptor authenticates with the x-api-key header.
		sawAuth = r.Header.Get("x-api-key")
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
	// Channel-level system prompt must be injected into the outgoing request.
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{SystemPrompt: "You are a pirate."})
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := ClaudeHelper(c, info); err != nil {
		t.Fatalf("ClaudeHelper happy path returned error: %v", err.Error())
	}
	if sawAuth != "sk-ant-key" {
		t.Errorf("upstream x-api-key = %q, want channel key sk-ant-key", sawAuth)
	}
	if !bytes.Contains(sawBody, []byte("You are a pirate.")) {
		t.Errorf("channel system prompt not injected into upstream body: %s", sawBody)
	}

	var refreshed repo.User
	if dbErr := repo.DB.First(&refreshed, u.Id).Error; dbErr != nil {
		t.Fatalf("reload user: %v", dbErr)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1 (Claude post-consume ran)", refreshed.RequestCount)
	}
}

// TestClaudeHelper_ThinkingAdapter drives the extended-thinking adapter branch:
// a "-thinking" suffixed model with the adapter enabled injects a thinking
// config and bumps max tokens before dispatch. Asserts the outgoing request
// carries the thinking block and the call succeeds.
func TestClaudeHelper_ThinkingAdapter(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "claude-think", Quota: 10_000_000}
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
		Model:     "claude-3-5-sonnet-thinking",
		MaxTokens: 16,
		Messages:  []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}},
	}
	c, info := wireClaudeUpstream(t, srv.URL, "claude-3-5-sonnet-thinking", claudeReq)
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := ClaudeHelper(c, info); err != nil {
		t.Fatalf("ClaudeHelper thinking adapter returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("thinking")) {
		t.Errorf("expected thinking config injected into upstream body: %s", sawBody)
	}
}

// ─── Stream happy paths (text/event-stream branch) ───────────────────────────

const chatCompletionStreamBody = "data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hi\"}}]}\n\n" +
	"data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":3,\"total_tokens\":8}}\n\n" +
	"data: [DONE]\n\n"

func newStreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatCompletionStreamBody))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestTextHelper_StreamHappyPath drives the streaming branch (info.IsStream set
// from the upstream text/event-stream Content-Type) for TextHelper through the
// OpenAI streaming DoResponse (OaiStreamHandler) and the post-consume tail.
func TestTextHelper_StreamHappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	// StreamScannerHandler builds a time.NewTicker(StreamingTimeout*s); the
	// hermetic test tier leaves it 0 which panics, so give it a real value.
	prevTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 60
	defer func() { constant.StreamingTimeout = prevTimeout }()

	u := &repo.User{Username: "stream-text", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := newStreamServer(t)
	req := &dto.GeneralOpenAIRequest{Model: "gpt-4o-mini", Stream: true, Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "gpt-4o-mini", req)
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := TextHelper(c, info); err != nil {
		t.Fatalf("TextHelper stream happy path returned error: %v", err.Error())
	}
	if !info.IsStream {
		t.Errorf("expected info.IsStream=true after event-stream response")
	}
	var refreshed repo.User
	if dbErr := repo.DB.First(&refreshed, u.Id).Error; dbErr != nil {
		t.Fatalf("reload user: %v", dbErr)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1 (stream post-consume ran)", refreshed.RequestCount)
	}
}

// ─── postConsumeQuota rich pricing math ──────────────────────────────────────

// TestPostConsumeQuota_RichPricingComputesQuota drives the full decimal billing
// computation in postConsumeQuota: prompt/cache/image/cache-creation token
// ratios, a chat search-preview web-search surcharge, and an extra OtherRatios
// multiplier. It asserts the user is actually debited (UsedQuota>0) and metered.
func TestPostConsumeQuota_RichPricingComputesQuota(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	u := &repo.User{Username: "pc-rich", Quota: 1_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	ch := &repo.Channel{Type: constant.ChannelTypeOpenAI}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}

	info := newRelayInfo(u.Id, ch.Id, constant.APITypeOpenAI)
	// search-preview chat model triggers the web-search surcharge branch.
	info.OriginModelName = "gpt-4o-search-preview"
	info.PriceData = types.PriceData{
		ModelRatio:         2,
		CompletionRatio:    3,
		CacheRatio:         0.5,
		ImageRatio:         1.5,
		CacheCreationRatio: 1.2,
		OtherRatios:        map[string]float64{"surge": 2},
		GroupRatioInfo:     types.GroupRatioInfo{GroupRatio: 1},
	}
	info.FinalPreConsumedQuota = 0

	c, _ := newJSONContext(http.MethodPost, "/", nil)
	c.Set("token_name", "tkn")
	c.Set("chat_completion_web_search_context_size", "high")

	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         10,
			ImageTokens:          5,
			CachedCreationTokens: 3,
		},
	}
	postConsumeQuota(c, info, usage)

	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", refreshed.RequestCount)
	}
	if refreshed.UsedQuota <= 0 {
		t.Errorf("UsedQuota = %d, want >0 (rich pricing debited real quota)", refreshed.UsedQuota)
	}
}

// ─── TextHelper channel-setting branches ─────────────────────────────────────

// TestTextHelper_SystemPromptAndWebSearch drives the channel system-prompt
// injection branch (prepends a system message into the converted OpenAI request)
// and the web-search-options context capture, then dispatches to a 200 upstream.
func TestTextHelper_SystemPromptAndWebSearch(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "text-sys", Quota: 10_000_000}
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
		Model:            "gpt-4o-mini",
		Messages:         []dto.Message{{Role: "user", Content: "hi"}},
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "high"},
	}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "gpt-4o-mini", req)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{SystemPrompt: "Be terse."})
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := TextHelper(c, info); err != nil {
		t.Fatalf("TextHelper returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("Be terse.")) {
		t.Errorf("system prompt not prepended to upstream messages: %s", sawBody)
	}
	if got := c.GetString("chat_completion_web_search_context_size"); got != "high" {
		t.Errorf("web search context size ctx = %q, want high", got)
	}
}

// TestTextHelper_PassThroughBody drives the PassThroughBodyEnabled branch which
// forwards the raw request body verbatim instead of converting it.
func TestTextHelper_PassThroughBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "text-pass", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rawBody := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"passthrough-marker"}]}`
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatCompletionBody))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(rawBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-test-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o-mini")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
	c.Set("token_name", "tkn")

	req := &dto.GeneralOpenAIRequest{Model: "gpt-4o-mini", Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	info := &relaycommon.RelayInfo{
		Request:         req,
		OriginModelName: "gpt-4o-mini",
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RequestURLPath:  "/v1/chat/completions",
		StartTime:       time.Now(),
		UserId:          u.Id,
	}

	if err := TextHelper(c, info); err != nil {
		t.Fatalf("TextHelper passthrough returned error: %v", err.Error())
	}
	// The raw (unconverted) body must have been forwarded verbatim.
	if !bytes.Contains(sawBody, []byte("passthrough-marker")) {
		t.Errorf("expected raw body forwarded, got: %s", sawBody)
	}
}

// ─── GeminiHelper happy path (native Gemini channel) ─────────────────────────

const geminiGenerateBody = `{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3,"totalTokenCount":8}}`

// TestGeminiHelper_NativeHappyPath drives GeminiHelper against a real
// Gemini-format upstream (channel type Gemini → gemini adaptor). This exercises
// ConvertGeminiRequest (native passthrough), the generateContent dispatch, the
// gemini DoResponse parse, and the post-consume tail.
func TestGeminiHelper_NativeHappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "gemini-native", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawKey string
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawKey = r.Header.Get("x-goog-api-key")
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(geminiGenerateBody))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "gm-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gemini-2.5-flash")
	// Channel-level system prompt exercises the SystemInstructions injection.
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{SystemPrompt: "You are a pirate."})
	c.Set("token_name", "tkn")

	req := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}}}
	info := &relaycommon.RelayInfo{
		Request:         req,
		OriginModelName: "gemini-2.5-flash",
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RequestURLPath:  "/v1beta/models/gemini-2.5-flash:generateContent",
		StartTime:       time.Now(),
	}
	info.UserId = u.Id

	if err := GeminiHelper(c, info); err != nil {
		t.Fatalf("GeminiHelper native happy path returned error: %v", err.Error())
	}
	if sawKey != "gm-key" {
		t.Errorf("upstream x-goog-api-key = %q, want gm-key", sawKey)
	}
	if !bytes.Contains(sawBody, []byte("You are a pirate.")) {
		t.Errorf("channel system prompt not injected into gemini body: %s", sawBody)
	}
	var refreshed repo.User
	if dbErr := repo.DB.First(&refreshed, u.Id).Error; dbErr != nil {
		t.Fatalf("reload user: %v", dbErr)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1 (Gemini post-consume ran)", refreshed.RequestCount)
	}
}

// TestGeminiEmbeddingHandler_HappyPath drives the native Gemini embedding
// success tail: parse single embedContent request → dispatch → NativeGemini
// embedding DoResponse → post-consume billing.
func TestGeminiEmbeddingHandler_HappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "gemini-emb", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"embedding":{"values":[0.1,0.2,0.3]}}`))
	}))
	defer srv.Close()

	body := []byte(`{"model":"text-embedding-004","content":{"parts":[{"text":"hello"}]}}`)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/text-embedding-004:embedContent", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "gm-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "text-embedding-004")
	c.Set("token_name", "tkn")

	info := &relaycommon.RelayInfo{
		OriginModelName: "text-embedding-004",
		RequestURLPath:  "/v1beta/models/text-embedding-004:embedContent",
		StartTime:       time.Now(),
	}
	info.UserId = u.Id

	if err := GeminiEmbeddingHandler(c, info); err != nil {
		t.Fatalf("GeminiEmbeddingHandler happy path returned error: %v", err.Error())
	}
	if info.IsGeminiBatchEmbedding {
		t.Errorf("single embedContent must not set batch flag")
	}
	// The embedding DoResponse converts the upstream reply to an OpenAI-format
	// embedding list and writes it to the client; assert that reached the recorder.
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"object":"list"`)) {
		t.Errorf("expected OpenAI-format embedding response written, got: %s", rec.Body.String())
	}
}

// ─── RelayMidjourneySubmit change/origin-resolution happy path ────────────────

// TestRelayMidjourneySubmit_ChangeResolvesOriginChannel drives an UPSCALE
// (Change) action that references an existing SUCCESS task. This exercises the
// origin-task lookup + origin-channel resolution branch (base_url/key rebinding)
// before dispatching to the fake MJ upstream.
func TestRelayMidjourneySubmit_ChangeResolvesOriginChannel(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "mj-change", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"result":"mj-upscaled","description":"ok"}`))
	}))
	defer srv.Close()

	baseURL := srv.URL
	ch := &repo.Channel{Type: 1, Status: common.ChannelStatusEnabled, BaseURL: &baseURL, Key: "sk-mj"}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	if err := repo.MjInsert(&repo.Midjourney{MjId: "origin-1", UserId: u.Id, ChannelId: ch.Id, Status: "SUCCESS", Prompt: "a castle"}); err != nil {
		t.Fatalf("seed origin mj: %v", err)
	}

	c, rec := mjPostContext(t, "/mj/submit/change", []byte(`{"taskId":"origin-1","action":"UPSCALE","index":1}`))
	c.Set("token_name", "tkn")
	info := &relaycommon.RelayInfo{UserId: u.Id}
	info.RelayMode = relayconstant.RelayModeMidjourneyChange

	if resp := RelayMidjourneySubmit(c, info); resp != nil {
		t.Fatalf("expected nil (success), got %+v", resp)
	}
	if rec.Body.Len() == 0 {
		t.Errorf("expected response body written")
	}
	saved := repo.GetByOnlyMJId("mj-upscaled")
	if saved == nil {
		t.Fatalf("expected persisted upscale task mj-upscaled")
	}
	// The change action inherits the origin task's prompt.
	if saved.Prompt != "a castle" {
		t.Errorf("prompt = %q, want inherited 'a castle'", saved.Prompt)
	}
}

// ─── GeminiHelper thinking-adapter branch ────────────────────────────────────

// TestGeminiHelper_ThinkingAdapter enables the Gemini thinking adapter and sends
// a request without a thinking config, exercising the ThinkingAdaptor injection
// branch before dispatch.
func TestGeminiHelper_ThinkingAdapter(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	gs := model_setting.GetGeminiSettings()
	prev := gs.ThinkingAdapterEnabled
	gs.ThinkingAdapterEnabled = true
	defer func() { gs.ThinkingAdapterEnabled = prev }()

	u := &repo.User{Username: "gemini-think", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(geminiGenerateBody))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "gm-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gemini-2.5-flash")
	c.Set("token_name", "tkn")

	req := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}}}
	info := &relaycommon.RelayInfo{
		Request:         req,
		OriginModelName: "gemini-2.5-flash",
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RequestURLPath:  "/v1beta/models/gemini-2.5-flash:generateContent",
		StartTime:       time.Now(),
	}
	info.UserId = u.Id

	if err := GeminiHelper(c, info); err != nil {
		t.Fatalf("GeminiHelper thinking adapter returned error: %v", err.Error())
	}
	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", refreshed.RequestCount)
	}
}

// ─── RerankHelper happy path (Jina channel) ──────────────────────────────────

// TestRerankHelper_HappyPath drives a rerank request against a Jina-format
// upstream (channel type Jina → jina adaptor → RerankHandler). It proves the
// convert → dispatch → parse → write → post-consume path for the rerank format.
func TestRerankHelper_HappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "rerank-user", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"rerank-1","results":[{"index":0,"relevance_score":0.9},{"index":1,"relevance_score":0.4}],"usage":{"total_tokens":12}}`))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeJina)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "jina-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "rerank-1")
	c.Set("token_name", "tkn")

	req := &dto.RerankRequest{Model: "rerank-1", Query: "q", Documents: []any{"d1", "d2"}}
	info := &relaycommon.RelayInfo{
		Request:         req,
		OriginModelName: "rerank-1",
		RelayMode:       relayconstant.RelayModeRerank,
		RequestURLPath:  "/v1/rerank",
		StartTime:       time.Now(),
	}
	info.UserId = u.Id

	if err := RerankHelper(c, info); err != nil {
		t.Fatalf("RerankHelper happy path returned error: %v", err.Error())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("relevance_score")) {
		t.Errorf("expected rerank results written to client, got: %s", rec.Body.String())
	}
	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1 (rerank post-consume ran)", refreshed.RequestCount)
	}
}

// ─── RelayMidjourneyImage ────────────────────────────────────────────────────

func TestRelayMidjourneyImage_TaskNotFound(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	c, rec := mjGinContext(t, nil)
	c.Params = gin.Params{{Key: "id", Value: "no-such-task"}}
	RelayMidjourneyImage(c)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400 for missing task", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("midjourney_task_not_found")) {
		t.Errorf("body = %s, want midjourney_task_not_found", rec.Body.String())
	}
}

func TestRelayMidjourneyImage_StreamsImage(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	// Allow the loopback upstream through SSRF protection for this test only.
	fs := system_setting.GetFetchSetting()
	prevSSRF, prevPriv := fs.EnableSSRFProtection, fs.AllowPrivateIp
	fs.EnableSSRFProtection = false
	fs.AllowPrivateIp = true
	defer func() { fs.EnableSSRFProtection = prevSSRF; fs.AllowPrivateIp = prevPriv }()

	imgBytes := []byte("\xff\xd8\xff\xe0fake-jpeg-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(imgBytes)
	}))
	defer srv.Close()

	if err := repo.MjInsert(&repo.Midjourney{MjId: "img-1", UserId: 4, ImageUrl: srv.URL}); err != nil {
		t.Fatalf("seed mj: %v", err)
	}

	c, rec := mjGinContext(t, nil)
	c.Params = gin.Params{{Key: "id", Value: "img-1"}}
	RelayMidjourneyImage(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if !bytes.Equal(rec.Body.Bytes(), imgBytes) {
		t.Errorf("streamed body mismatch: got %q", rec.Body.Bytes())
	}
}

func TestRelayMidjourneyImage_UpstreamNon200(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	fs := system_setting.GetFetchSetting()
	prevSSRF := fs.EnableSSRFProtection
	fs.EnableSSRFProtection = false
	defer func() { fs.EnableSSRFProtection = prevSSRF }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("gone"))
	}))
	defer srv.Close()

	if err := repo.MjInsert(&repo.Midjourney{MjId: "img-404", UserId: 4, ImageUrl: srv.URL}); err != nil {
		t.Fatalf("seed mj: %v", err)
	}

	c, rec := mjGinContext(t, nil)
	c.Params = gin.Params{{Key: "id", Value: "img-404"}}
	RelayMidjourneyImage(c)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 propagated from upstream", rec.Code)
	}
}

// ─── RelaySwapFace happy path (fake MJ upstream + billing) ───────────────────

func TestRelaySwapFace_HappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "swap-user", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"result":"swap-ok","description":"submitted"}`))
	}))
	defer srv.Close()

	c, rec := mjPostContext(t, "/mj/submit/swap_face", []byte(`{"sourceBase64":"c3Jj","targetBase64":"dGd0"}`))
	c.Set("base_url", srv.URL)
	c.Set("token_name", "tkn")

	info := &relaycommon.RelayInfo{UserId: u.Id, IsPlayground: true}
	if resp := RelaySwapFace(c, info); resp != nil {
		t.Fatalf("expected nil (success), got %+v", resp)
	}
	if rec.Body.Len() == 0 {
		t.Errorf("expected response body written")
	}

	saved := repo.GetByOnlyMJId("swap-ok")
	if saved == nil {
		t.Fatalf("expected persisted swap-face task")
	}
	if saved.Action != constant.MjActionSwapFace || saved.UserId != u.Id {
		t.Errorf("unexpected persisted task: %+v", saved)
	}
	// The success defer must run PostConsumeQuota + request-count metering.
	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1 (swap-face billing ran)", refreshed.RequestCount)
	}
	if refreshed.UsedQuota <= 0 {
		t.Errorf("UsedQuota = %d, want >0 (quota consumed)", refreshed.UsedQuota)
	}
}

// ─── RelayMidjourneySubmit additional branches ───────────────────────────────

func TestRelayMidjourneySubmit_QuotaNotEnough(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	// User with almost no quota → the per-call MJ price (>0) exceeds it.
	u := &repo.User{Username: "mj-poor", Quota: 1}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	c, _ := mjPostContext(t, "/mj/submit/imagine", []byte(`{"prompt":"a lone tree"}`))
	c.Set("base_url", "http://127.0.0.1:0") // never reached: quota check precedes dispatch
	info := &relaycommon.RelayInfo{UserId: u.Id}
	info.RelayMode = relayconstant.RelayModeMidjourneyImagine

	resp := RelayMidjourneySubmit(c, info)
	if resp == nil || resp.Description != "quota_not_enough" {
		t.Fatalf("expected quota_not_enough, got %+v", resp)
	}
}

func TestRelayMidjourneySubmit_Code21RewritesToSuccess(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "mj-21", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":21,"description":"task exists","result":"mj-21id","properties":{"status":"SUCCESS","imageUrl":"https://cdn/done.png"}}`))
	}))
	defer srv.Close()

	c, rec := mjPostContext(t, "/mj/submit/imagine", []byte(`{"prompt":"a rocket"}`))
	c.Set("base_url", srv.URL)
	c.Set("token_name", "tkn")
	info := &relaycommon.RelayInfo{UserId: u.Id}
	info.RelayMode = relayconstant.RelayModeMidjourneyImagine

	if resp := RelayMidjourneySubmit(c, info); resp != nil {
		t.Fatalf("expected nil, got %+v", resp)
	}
	// Code 21 → success: persisted with SUCCESS status + upstream image url.
	saved := repo.GetByOnlyMJId("mj-21id")
	if saved == nil {
		t.Fatalf("expected persisted task mj-21id")
	}
	if saved.Status != "SUCCESS" || saved.ImageUrl != "https://cdn/done.png" {
		t.Errorf("expected SUCCESS/imageUrl persisted, got %+v", saved)
	}
	// Response body is rewritten from code 21 → code 1.
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":1`)) {
		t.Errorf("expected code rewritten to 1, got %s", rec.Body.String())
	}
}

func TestRelayMidjourneySubmit_Code22RewritesBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "mj-22", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":22,"description":"queued","result":"mj-22id"}`))
	}))
	defer srv.Close()

	c, rec := mjPostContext(t, "/mj/submit/imagine", []byte(`{"prompt":"a river"}`))
	c.Set("base_url", srv.URL)
	c.Set("token_name", "tkn")
	info := &relaycommon.RelayInfo{UserId: u.Id}
	info.RelayMode = relayconstant.RelayModeMidjourneyImagine

	if resp := RelayMidjourneySubmit(c, info); resp != nil {
		t.Fatalf("expected nil, got %+v", resp)
	}
	if repo.GetByOnlyMJId("mj-22id") == nil {
		t.Fatalf("expected persisted task mj-22id")
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"code":1`)) {
		t.Errorf("expected code 22 rewritten to 1 in body, got %s", rec.Body.String())
	}
}

func TestRelayMidjourneySubmit_ErrorCodeSkipsBilling(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "mj-err", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Code 4 = submit error → task persisted with fail reason, quota NOT consumed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":4,"description":"banned prompt","result":""}`))
	}))
	defer srv.Close()

	c, _ := mjPostContext(t, "/mj/submit/imagine", []byte(`{"prompt":"blocked"}`))
	c.Set("base_url", srv.URL)
	c.Set("token_name", "tkn")
	info := &relaycommon.RelayInfo{UserId: u.Id}
	info.RelayMode = relayconstant.RelayModeMidjourneyImagine

	if resp := RelayMidjourneySubmit(c, info); resp != nil {
		t.Fatalf("expected nil (task recorded), got %+v", resp)
	}
	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.RequestCount != 0 || refreshed.UsedQuota != 0 {
		t.Errorf("error-code submit must not bill: RequestCount=%d UsedQuota=%d", refreshed.RequestCount, refreshed.UsedQuota)
	}
}

// ─── RelayTaskSubmit error / quota branches ──────────────────────────────────

func TestRelayTaskSubmit_QuotaNotEnough(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "task-poor", Quota: 1}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	c, _ := taskSubmitContext(t, http.MethodPost, "/suno/submit/music", "music", []byte(`{"prompt":"lofi"}`))
	c.Set("platform", string(constant.TaskPlatformSuno))
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "http://127.0.0.1:0")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-suno")
	info := &relaycommon.RelayInfo{UserId: u.Id}

	taskErr := RelayTaskSubmit(c, info)
	if taskErr == nil || taskErr.Code != "quota_not_enough" {
		t.Fatalf("expected quota_not_enough, got %+v", taskErr)
	}
}

func TestRelayTaskSubmit_UpstreamErrorReleasesQuota(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "task-5xx", Quota: 100_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream down"}`))
	}))
	defer srv.Close()

	c, _ := taskSubmitContext(t, http.MethodPost, "/suno/submit/music", "music", []byte(`{"prompt":"lofi"}`))
	c.Set("platform", string(constant.TaskPlatformSuno))
	c.Set("token_name", "tkn")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-suno")
	info := &relaycommon.RelayInfo{UserId: u.Id}

	taskErr := RelayTaskSubmit(c, info)
	if taskErr == nil {
		t.Fatalf("expected upstream failure error, got nil")
	}
	// Upstream failure → quota released (never consumed), no task persisted.
	var refreshed repo.User
	if err := repo.DB.First(&refreshed, u.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.RequestCount != 0 || refreshed.UsedQuota != 0 {
		t.Errorf("upstream-error submit must not bill: RequestCount=%d UsedQuota=%d", refreshed.RequestCount, refreshed.UsedQuota)
	}
}

// ─── videoFetchByIDRespBodyBuilder OpenAI-video conversion branch ─────────────

func TestVideoFetchByIDRespBodyBuilder_OpenAIVideoPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()

	ch := &repo.Channel{Type: constant.ChannelTypeOpenAI}
	if err := repo.DB.Create(ch).Error; err != nil {
		t.Fatalf("seed channel: %v", err)
	}
	// Sora (OpenAI channel type) task; /v1/videos/ RequestURI triggers the
	// OpenAIVideoConverter branch which returns task.Data verbatim.
	seedTask(t, &repo.Task{
		TaskID:    "vid-openai",
		UserId:    12,
		ChannelId: ch.Id,
		Platform:  constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeOpenAI)),
		Action:    "video",
		Status:    repo.TaskStatusSuccess,
		Data:      []byte(`{"id":"vid-openai","status":"completed"}`),
	})

	c, _ := newJSONContext(http.MethodGet, "/v1/videos/vid-openai", nil)
	c.Request.RequestURI = "/v1/videos/vid-openai"
	c.Set("id", 12)
	c.Params = gin.Params{{Key: "task_id", Value: "vid-openai"}}

	body, taskErr := videoFetchByIDRespBodyBuilder(c)
	if taskErr != nil {
		t.Fatalf("unexpected taskErr: %+v", taskErr)
	}
	if !bytes.Contains(body, []byte("vid-openai")) {
		t.Errorf("expected converted video data passthrough, got %s", body)
	}
}

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

// ─── ResponsesHelper branches ────────────────────────────────────────────────

// TestResponsesHelper_PassThroughBody covers the PassThroughBodyEnabled branch
// of ResponsesHelper: the raw body is forwarded verbatim.
func TestResponsesHelper_PassThroughBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "resp-pass", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rawBody := `{"model":"gpt-4o-mini","input":"responses-passthrough-marker"}`
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responsesBody))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(rawBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-test-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-4o-mini")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
	c.Set("token_name", "tkn")

	req := &dto.OpenAIResponsesRequest{Model: "gpt-4o-mini", Input: []byte(`"hi"`)}
	info := &relaycommon.RelayInfo{
		Request:         req,
		OriginModelName: "gpt-4o-mini",
		RelayMode:       relayconstant.RelayModeResponses,
		RequestURLPath:  "/v1/responses",
		StartTime:       time.Now(),
		UserId:          u.Id,
	}

	if err := ResponsesHelper(c, info); err != nil {
		t.Fatalf("ResponsesHelper passthrough returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("responses-passthrough-marker")) {
		t.Errorf("expected raw body forwarded verbatim, got: %s", sawBody)
	}
}

// TestResponsesHelper_ParamOverride covers the ParamOverride branch of
// ResponsesHelper.
func TestResponsesHelper_ParamOverride(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "resp-override", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responsesBody))
	}))
	defer srv.Close()

	req := &dto.OpenAIResponsesRequest{Model: "gpt-4o-mini", Input: []byte(`"hi"`)}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/responses", relayconstant.RelayModeResponses, "gpt-4o-mini", req)
	c.Set("token_name", "tkn")
	info.UserId = u.Id
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"temperature": 0.33})

	if err := ResponsesHelper(c, info); err != nil {
		t.Fatalf("ResponsesHelper param override returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("0.33")) {
		t.Errorf("param override not applied to responses body: %s", sawBody)
	}
}

// TestResponsesHelper_MalformedUpstreamBody covers the DoResponse parse-error
// branch of ResponsesHelper: a 200 body that cannot be parsed surfaces an error.
func TestResponsesHelper_MalformedUpstreamBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`}{ not valid json`))
	}))
	defer srv.Close()

	req := &dto.OpenAIResponsesRequest{Model: "gpt-4o-mini", Input: []byte(`"hi"`)}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/responses", relayconstant.RelayModeResponses, "gpt-4o-mini", req)
	c.Set("token_name", "tkn")

	if err := ResponsesHelper(c, info); err == nil {
		t.Fatalf("expected DoResponse parse error for malformed 200 body, got nil")
	}
}

// ─── ImageHelper branches ────────────────────────────────────────────────────

// TestImageHelper_ParamOverride covers the ParamOverride branch of ImageHelper.
func TestImageHelper_ParamOverride(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "img-override", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(imageGenBody))
	}))
	defer srv.Close()

	imgReq := &dto.ImageRequest{Model: "dall-e-3", Prompt: "a cat", N: 1}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/images/generations", relayconstant.RelayModeImagesGenerations, "dall-e-3", imgReq)
	c.Set("token_name", "tkn")
	info.UserId = u.Id
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"style": "vivid"})

	if err := ImageHelper(c, info); err != nil {
		t.Fatalf("ImageHelper param override returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("vivid")) {
		t.Errorf("param override style not applied to image body: %s", sawBody)
	}
}

// TestImageHelper_MalformedUpstreamBody covers the DoResponse parse-error branch
// of ImageHelper.
func TestImageHelper_MalformedUpstreamBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`}{ not valid json`))
	}))
	defer srv.Close()

	imgReq := &dto.ImageRequest{Model: "dall-e-3", Prompt: "a cat", N: 1}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/images/generations", relayconstant.RelayModeImagesGenerations, "dall-e-3", imgReq)
	c.Set("token_name", "tkn")

	if err := ImageHelper(c, info); err == nil {
		t.Fatalf("expected DoResponse parse error for malformed 200 image body, got nil")
	}
}

// ─── GeminiHelper malformed body ─────────────────────────────────────────────

// TestGeminiHelper_MalformedUpstreamBody covers the DoResponse parse-error
// branch of GeminiHelper against a native Gemini channel.
func TestGeminiHelper_MalformedUpstreamBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`}{ not valid gemini json`))
	}))
	defer srv.Close()

	req := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}}}
	c, info := wireGeminiNative(t, srv.URL, dto.ChannelSettings{}, req, 0)

	if err := GeminiHelper(c, info); err == nil {
		t.Fatalf("expected DoResponse parse error for malformed 200 gemini body, got nil")
	}
}

// ─── GeminiEmbeddingHandler batch + param override ───────────────────────────

// TestGeminiEmbeddingHandler_BatchHappyPath drives the batch embedding success
// tail (batchEmbedContents) end-to-end, proving the batch parse + dispatch +
// DoResponse + post-consume path.
func TestGeminiEmbeddingHandler_BatchHappyPath(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "gemini-batch", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"embeddings":[{"values":[0.1,0.2]},{"values":[0.3,0.4]}]}`))
	}))
	defer srv.Close()

	body := []byte(`{"requests":[{"model":"text-embedding-004","content":{"parts":[{"text":"a"}]}},{"model":"text-embedding-004","content":{"parts":[{"text":"b"}]}}]}`)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/text-embedding-004:batchEmbedContents", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "gm-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "text-embedding-004")
	c.Set("token_name", "tkn")

	info := &relaycommon.RelayInfo{
		OriginModelName: "text-embedding-004",
		RequestURLPath:  "/v1beta/models/text-embedding-004:batchEmbedContents",
		StartTime:       time.Now(),
		UserId:          u.Id,
	}

	if err := GeminiEmbeddingHandler(c, info); err != nil {
		t.Fatalf("GeminiEmbeddingHandler batch happy path returned error: %v", err.Error())
	}
	if !info.IsGeminiBatchEmbedding {
		t.Errorf("batch path must set IsGeminiBatchEmbedding=true")
	}
}

// TestGeminiEmbeddingHandler_ParamOverride covers the param-override merge branch
// of GeminiEmbeddingHandler (which merges overrides into the request map).
func TestGeminiEmbeddingHandler_ParamOverride(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "gemini-emb-ovr", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"embedding":{"values":[0.1,0.2,0.3]}}`))
	}))
	defer srv.Close()

	body := []byte(`{"model":"text-embedding-004","content":{"parts":[{"text":"hello"}]}}`)
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/text-embedding-004:embedContent", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeGemini)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "gm-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "text-embedding-004")
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"outputDimensionality": 128})
	c.Set("token_name", "tkn")

	info := &relaycommon.RelayInfo{
		OriginModelName: "text-embedding-004",
		RequestURLPath:  "/v1beta/models/text-embedding-004:embedContent",
		StartTime:       time.Now(),
		UserId:          u.Id,
	}

	if err := GeminiEmbeddingHandler(c, info); err != nil {
		t.Fatalf("GeminiEmbeddingHandler param override returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("outputDimensionality")) {
		t.Errorf("param override not merged into gemini embedding body: %s", sawBody)
	}
}

// ─── RerankHelper malformed body ─────────────────────────────────────────────

// TestRerankHelper_MalformedUpstreamBody covers the DoResponse parse-error
// branch of RerankHelper against a Jina channel.
func TestRerankHelper_MalformedUpstreamBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`}{ not valid rerank json`))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
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

	if err := RerankHelper(c, info); err == nil {
		t.Fatalf("expected DoResponse parse error for malformed 200 rerank body, got nil")
	}
}

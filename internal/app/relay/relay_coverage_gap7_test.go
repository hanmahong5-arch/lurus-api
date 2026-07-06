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

const rerankHappyBody = `{"model":"rerank-1","results":[{"index":0,"relevance_score":0.9}],"usage":{"total_tokens":12}}`

func wireJinaRerank(t *testing.T, srvURL string, req *dto.RerankRequest, userID int) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeJina)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srvURL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "jina-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "rerank-1")
	c.Set("token_name", "tkn")
	info := &relaycommon.RelayInfo{
		Request:         req,
		OriginModelName: "rerank-1",
		RelayMode:       relayconstant.RelayModeRerank,
		RequestURLPath:  "/v1/rerank",
		StartTime:       time.Now(),
		UserId:          userID,
	}
	return c, info
}

// TestRerankHelper_ParamOverride covers the ParamOverride branch of RerankHelper.
func TestRerankHelper_ParamOverride(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "rerank-ovr", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rerankHappyBody))
	}))
	defer srv.Close()

	req := &dto.RerankRequest{Model: "rerank-1", Query: "q", Documents: []any{"d1", "d2"}}
	c, info := wireJinaRerank(t, srv.URL, req, u.Id)
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]any{"top_n": 5})

	if err := RerankHelper(c, info); err != nil {
		t.Fatalf("RerankHelper param override returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("top_n")) {
		t.Errorf("param override top_n not applied to rerank body: %s", sawBody)
	}
}

// TestRerankHelper_PassThroughBody covers the PassThroughBodyEnabled branch of
// RerankHelper: the raw body is forwarded verbatim.
func TestRerankHelper_PassThroughBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "rerank-pass", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rawBody := `{"model":"rerank-1","query":"q","documents":["rerank-passthrough-marker"]}`
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rerankHappyBody))
	}))
	defer srv.Close()

	req := &dto.RerankRequest{Model: "rerank-1", Query: "q", Documents: []any{"d1"}}
	c, info := wireJinaRerank(t, srv.URL, req, u.Id)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", bytes.NewReader([]byte(rawBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})

	if err := RerankHelper(c, info); err != nil {
		t.Fatalf("RerankHelper passthrough returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("rerank-passthrough-marker")) {
		t.Errorf("expected raw body forwarded verbatim, got: %s", sawBody)
	}
}

// TestImageHelper_PassThroughBody covers the PassThroughBodyEnabled branch of
// ImageHelper: the raw body is forwarded verbatim.
func TestImageHelper_PassThroughBody(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "img-pass", Quota: 10_000_000}
	if err := repo.DB.Create(u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rawBody := `{"model":"dall-e-3","prompt":"image-passthrough-marker","n":1}`
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(imageGenBody))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader([]byte(rawBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, srv.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-test-key")
	common.SetContextKey(c, constant.ContextKeyChannelId, 1)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "dall-e-3")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: true})
	c.Set("token_name", "tkn")

	imgReq := &dto.ImageRequest{Model: "dall-e-3", Prompt: "a cat", N: 1}
	info := &relaycommon.RelayInfo{
		Request:         imgReq,
		OriginModelName: "dall-e-3",
		RelayMode:       relayconstant.RelayModeImagesGenerations,
		RequestURLPath:  "/v1/images/generations",
		StartTime:       time.Now(),
		UserId:          u.Id,
	}

	if err := ImageHelper(c, info); err != nil {
		t.Fatalf("ImageHelper passthrough returned error: %v", err.Error())
	}
	if !bytes.Contains(sawBody, []byte("image-passthrough-marker")) {
		t.Errorf("expected raw body forwarded verbatim, got: %s", sawBody)
	}
}

// TestTextHelper_SystemPromptOverrideMediaContent covers the override branch
// where the caller's existing system message carries STRUCTURED media content
// (not a plain string): the channel prompt is prepended as a new text content
// block ahead of the caller's blocks.
func TestTextHelper_SystemPromptOverrideMediaContent(t *testing.T) {
	cleanup := setupRelayDB(t)
	defer cleanup()
	app.InitHttpClient()

	u := &repo.User{Username: "text-media-ovr", Quota: 10_000_000}
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
			{Role: "system", Content: []dto.MediaContent{{Type: dto.ContentTypeText, Text: "CALLER-MEDIA-SYS"}}},
			{Role: "user", Content: "hi"},
		},
	}
	c, info := wireOpenAIUpstream(t, srv.URL, "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "gpt-4o-mini", req)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		SystemPrompt:         "CHANNEL-MEDIA-SYS",
		SystemPromptOverride: true,
	})
	c.Set("token_name", "tkn")
	info.UserId = u.Id

	if err := TextHelper(c, info); err != nil {
		t.Fatalf("TextHelper media-content override returned error: %v", err.Error())
	}
	// The media-content override branch prepends the channel prompt as a
	// structured text block; the resulting system content must be an ARRAY of
	// content blocks (not a plain string), proving the ParseContent branch ran.
	if !bytes.Contains(sawBody, []byte(`[{"text":"CHANNEL-MEDIA-SYS"`)) {
		t.Errorf("expected channel prompt prepended as a structured content block, got: %s", sawBody)
	}
}

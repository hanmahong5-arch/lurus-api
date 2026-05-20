package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/tavily"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// withTavilyStub points the package-level webSearchClient at a Client
// that talks to a stub server, bypassing the env-var lookup so tests
// don't need TAVILY_API_KEY. Returns a cleanup func.
func withTavilyStub(t *testing.T, handler http.HandlerFunc) (cleanup func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	c, err := tavily.New("test-key", tavily.WithEndpoint(ts.URL))
	if err != nil {
		t.Fatalf("tavily.New: %v", err)
	}
	resetWebSearchClientForTesting()
	webSearchClient = c
	// Mark Once as done by calling Do with a no-op — but resetWebSearchClientForTesting
	// already resets it. We need to flip the once so resolveWebSearchClient
	// returns our injected client without consulting env.
	webSearchClientOnce.Do(func() {})
	return func() {
		ts.Close()
		resetWebSearchClientForTesting()
	}
}

func newReqCtx(t *testing.T, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/lutu/search", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func TestPostWebSearch_RejectsMissingQuery(t *testing.T) {
	defer withTavilyStub(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("tavily should not have been called")
	})()

	c, w := newReqCtx(t, map[string]any{}) // no "query"
	PostWebSearch(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "query is required") {
		t.Errorf("body missing message: %s", w.Body.String())
	}
}

func TestPostWebSearch_HappyPath(t *testing.T) {
	defer withTavilyStub(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"answer": "Bitcoin trades around $50,000 today.",
			"results": [
				{"title":"BTC Live","url":"https://example.com/btc","content":"$50k","score":0.95}
			]
		}`))
	})()

	c, w := newReqCtx(t, map[string]any{"query": "btc price"})
	PostWebSearch(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp WebSearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Answer == "" {
		t.Errorf("expected non-empty answer")
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].URL != "https://example.com/btc" {
		t.Errorf("url mismatch: %q", resp.Results[0].URL)
	}
}

func TestPostWebSearch_SearchDisabledWhenNoKey(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "")
	resetWebSearchClientForTesting()

	c, w := newReqCtx(t, map[string]any{"query": "x"})
	PostWebSearch(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", w.Code)
	}
	if !strings.Contains(w.Body.String(), "search_disabled") {
		t.Errorf("expected search_disabled error: %s", w.Body.String())
	}
}

func TestPostWebSearch_UpstreamErrorBecomes502(t *testing.T) {
	defer withTavilyStub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"tavily exploded"}`))
	})()

	c, w := newReqCtx(t, map[string]any{"query": "x"})
	PostWebSearch(c)

	if w.Code != http.StatusBadGateway {
		t.Errorf("code = %d, want 502", w.Code)
	}
	if !strings.Contains(w.Body.String(), "upstream_error") {
		t.Errorf("expected upstream_error tag: %s", w.Body.String())
	}
}

func TestPostWebSearch_HonorsDepthAdvanced(t *testing.T) {
	var receivedBody []byte
	defer withTavilyStub(t, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		receivedBody = buf
		_, _ = w.Write([]byte(`{"answer":"x","results":[]}`))
	})()

	c, w := newReqCtx(t, map[string]any{"query": "x", "depth": "advanced"})
	PostWebSearch(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	var sent map[string]any
	_ = json.Unmarshal(receivedBody, &sent)
	if sent["search_depth"] != "advanced" {
		t.Errorf("expected advanced depth in upstream, got %v", sent["search_depth"])
	}
}

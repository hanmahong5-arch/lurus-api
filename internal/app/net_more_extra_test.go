package app

// net_more_extra_test.go — remaining network-adjacent helpers: the
// non-stream read-deadline wrapper, the worker-mode download path, and the
// Midjourney upstream request builder. All use in-memory httptest servers.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/setting"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/system_setting"
	"github.com/gin-gonic/gin"
)

// ─── http_client.go: WrapNonStreamReadDeadline ────────────────────────────

func TestWrapNonStreamReadDeadline_ReadsAndCloses(t *testing.T) {
	if WrapNonStreamReadDeadline(nil) != nil {
		t.Error("nil body must pass through as nil")
	}

	src := io.NopCloser(strings.NewReader("hello world"))
	wrapped := WrapNonStreamReadDeadline(src)

	got, err := io.ReadAll(wrapped)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("read body = %q, want hello world", got)
	}
	if err := wrapped.Close(); err != nil {
		t.Errorf("close: %v", err)
	}
}

// ─── http_client.go: checkRedirect ────────────────────────────────────────

func TestCheckRedirect(t *testing.T) {
	// A public redirect target with SSRF relaxed and few hops => allowed.
	fs := system_setting.GetFetchSetting()
	prevSSRF := fs.EnableSSRFProtection
	fs.EnableSSRFProtection = false
	t.Cleanup(func() { fs.EnableSSRFProtection = prevSSRF })

	req := httptest.NewRequest(http.MethodGet, "https://example.com/next", nil)
	if err := checkRedirect(req, nil); err != nil {
		t.Errorf("checkRedirect allowed target => %v, want nil", err)
	}

	// More than 10 hops => stopped.
	via := make([]*http.Request, 10)
	if err := checkRedirect(req, via); err == nil {
		t.Error("expected 'stopped after 10 redirects' error")
	}

	// SSRF on + loopback target => blocked.
	fs.EnableSSRFProtection = true
	fs.AllowPrivateIp = false
	loop := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1/x", nil)
	if err := checkRedirect(loop, nil); err == nil {
		t.Error("expected redirect to loopback to be blocked")
	}
}

// ─── download.go: worker-mode path ────────────────────────────────────────

func TestDoDownloadRequest_WorkerMode(t *testing.T) {
	allowLocalFetch(t)

	var gotWorkerBody WorkerRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotWorkerBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("via-worker"))
	}))
	defer srv.Close()

	// Enable worker mode by pointing WorkerUrl at the test server.
	prevURL := system_setting.WorkerUrl
	prevAllow := system_setting.WorkerAllowHttpImageRequestEnabled
	system_setting.WorkerUrl = srv.URL
	system_setting.WorkerAllowHttpImageRequestEnabled = true // allow http target url
	t.Cleanup(func() {
		system_setting.WorkerUrl = prevURL
		system_setting.WorkerAllowHttpImageRequestEnabled = prevAllow
	})

	resp, err := DoDownloadRequest("http://example.com/file.png", "unit")
	if err != nil {
		t.Fatalf("DoDownloadRequest worker: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	// The worker request must carry the origin URL it was asked to fetch.
	if gotWorkerBody.URL != "http://example.com/file.png" {
		t.Errorf("worker request URL = %q, want the origin url", gotWorkerBody.URL)
	}
}

// ─── midjourney.go: DoMidjourneyHttpRequest ───────────────────────────────

func TestDoMidjourneyHttpRequest_PostForwardsBody(t *testing.T) {
	allowLocalFetch(t)
	if GetHttpClient() == nil {
		InitHttpClient()
	}

	var gotBody map[string]interface{}
	var gotSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSecret = r.Header.Get("mj-api-secret")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"description":"submitted","result":"123"}`))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reqBody := `{"prompt":"a cat --fast","accountFilter":{"x":1}}`
	c.Request = httptest.NewRequest(http.MethodPost, "/mj/submit/imagine", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	// Provide an upstream channel key so the mj-api-secret header is set.
	c.Set("channel_key", "Bearer mj-secret-123")

	mjResp, respBody, err := DoMidjourneyHttpRequest(c, 5*time.Second, srv.URL)
	if err != nil {
		t.Fatalf("DoMidjourneyHttpRequest: %v", err)
	}
	if mjResp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", mjResp.StatusCode)
	}
	if len(respBody) == 0 {
		t.Error("expected non-empty response body")
	}
	if gotSecret != "mj-secret-123" {
		t.Errorf("mj-api-secret = %q, want mj-secret-123 (Bearer stripped)", gotSecret)
	}
	// The prompt is forwarded (default MjModeClearEnabled is off, so --fast stays).
	if _, ok := gotBody["prompt"]; !ok {
		t.Error("forwarded body missing prompt")
	}
}

func TestDoMidjourneyHttpRequest_ModeClearStripsFlags(t *testing.T) {
	allowLocalFetch(t)
	if GetHttpClient() == nil {
		InitHttpClient()
	}
	prevClear := setting.MjModeClearEnabled
	setting.MjModeClearEnabled = true
	t.Cleanup(func() { setting.MjModeClearEnabled = prevClear })

	var gotPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if p, ok := body["prompt"].(string); ok {
			gotPrompt = p
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1}`))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/mj/submit/imagine",
		bytes.NewBufferString(`{"prompt":"a cat --fast --relax --turbo"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	if _, _, err := DoMidjourneyHttpRequest(c, 5*time.Second, srv.URL); err != nil {
		t.Fatalf("DoMidjourneyHttpRequest: %v", err)
	}
	if strings.Contains(gotPrompt, "--fast") || strings.Contains(gotPrompt, "--relax") || strings.Contains(gotPrompt, "--turbo") {
		t.Errorf("prompt %q should have mode flags stripped", gotPrompt)
	}
}

func TestDoMidjourneyHttpRequest_KeepsAccountFilterWhenEnabled(t *testing.T) {
	allowLocalFetch(t)
	if GetHttpClient() == nil {
		InitHttpClient()
	}
	prevAF := setting.MjAccountFilterEnabled
	prevNotify := setting.MjNotifyEnabled
	setting.MjAccountFilterEnabled = true
	setting.MjNotifyEnabled = true
	t.Cleanup(func() {
		setting.MjAccountFilterEnabled = prevAF
		setting.MjNotifyEnabled = prevNotify
	})

	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1}`))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/mj/submit/imagine",
		bytes.NewBufferString(`{"prompt":"x","accountFilter":{"k":1},"notifyHook":"h"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	if _, _, err := DoMidjourneyHttpRequest(c, 5*time.Second, srv.URL); err != nil {
		t.Fatalf("DoMidjourneyHttpRequest: %v", err)
	}
	// With both features enabled, accountFilter and notifyHook are preserved.
	if _, ok := body["accountFilter"]; !ok {
		t.Error("accountFilter should be preserved when MjAccountFilterEnabled is true")
	}
	if _, ok := body["notifyHook"]; !ok {
		t.Error("notifyHook should be preserved when MjNotifyEnabled is true")
	}
}

func TestDoMidjourneyHttpRequest_GetNoBody(t *testing.T) {
	allowLocalFetch(t)
	if GetHttpClient() == nil {
		InitHttpClient()
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":1,"description":"ok"}`))
	}))
	defer srv.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// GET request: DoMidjourneyHttpRequest must skip body decode entirely.
	c.Request = httptest.NewRequest(http.MethodGet, "/mj/task/123/fetch", nil)

	mjResp, _, err := DoMidjourneyHttpRequest(c, 5*time.Second, srv.URL)
	if err != nil {
		t.Fatalf("DoMidjourneyHttpRequest GET: %v", err)
	}
	if mjResp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", mjResp.StatusCode)
	}
}

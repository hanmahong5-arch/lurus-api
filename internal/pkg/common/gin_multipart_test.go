package common

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/gin-gonic/gin"
)

func TestParseBoundary(t *testing.T) {
	if _, err := parseBoundary(""); err == nil {
		t.Error("empty content type should error (boundary not found)")
	}
	if _, err := parseBoundary("application/json"); err == nil {
		t.Error("non-multipart should error (boundary not found)")
	}
	if _, err := parseBoundary("multipart/form-data; charset=utf-8"); err == nil {
		t.Error("missing boundary param should error")
	}
	b, err := parseBoundary("multipart/form-data; boundary=abc123")
	if err != nil || b != "abc123" {
		t.Errorf("boundary = %q err=%v", b, err)
	}
}

func TestMultipartMemoryLimit(t *testing.T) {
	orig := constant.MaxFileDownloadMB
	t.Cleanup(func() { constant.MaxFileDownloadMB = orig })

	constant.MaxFileDownloadMB = 0
	if got := multipartMemoryLimit(); got != 32<<20 {
		t.Errorf("zero config should default to 32MB, got %d", got)
	}
	constant.MaxFileDownloadMB = 8
	if got := multipartMemoryLimit(); got != 8<<20 {
		t.Errorf("8MB config, got %d", got)
	}
}

func TestParseFormData(t *testing.T) {
	var dst struct {
		Name  string   `json:"name"`
		Tags  []string `json:"tags"`
		Model string   `json:"model"`
	}
	// url.Values with a repeated key -> slice, single key -> string.
	err := parseFormData([]byte("name=neo&tags=a&tags=b&model=gpt-4"), &dst)
	if err != nil {
		t.Fatalf("parseFormData error: %v", err)
	}
	if dst.Name != "neo" || dst.Model != "gpt-4" || len(dst.Tags) != 2 {
		t.Errorf("unexpected: %+v", dst)
	}
}

func TestUnmarshalBodyReusable_FormURLEncoded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origMax := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 10
	t.Cleanup(func() { constant.MaxRequestBodyMB = origMax })

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("model=gpt-4&stream=true"))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var dst struct {
		Model  string `json:"model"`
		Stream string `json:"stream"`
	}
	if err := UnmarshalBodyReusable(c, &dst); err != nil {
		t.Fatalf("error: %v", err)
	}
	if dst.Model != "gpt-4" || dst.Stream != "true" {
		t.Errorf("unexpected form decode: %+v", dst)
	}
}

func TestParseAndUnmarshalMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	origMax := constant.MaxRequestBodyMB
	constant.MaxRequestBodyMB = 10
	t.Cleanup(func() { constant.MaxRequestBodyMB = origMax })

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("model", "whisper-1")
	_ = mw.WriteField("language", "en")
	_ = mw.Close()

	// ParseMultipartFormReusable returns the parsed form.
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body.Bytes()))
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	form, err := ParseMultipartFormReusable(c)
	if err != nil {
		t.Fatalf("ParseMultipartFormReusable error: %v", err)
	}
	if form.Value["model"][0] != "whisper-1" {
		t.Errorf("form model = %v", form.Value["model"])
	}

	// UnmarshalBodyReusable multipart branch decodes into a struct.
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body.Bytes()))
	c2.Request.Header.Set("Content-Type", mw.FormDataContentType())
	var dst struct {
		Model    string `json:"model"`
		Language string `json:"language"`
	}
	if err := UnmarshalBodyReusable(c2, &dst); err != nil {
		t.Fatalf("multipart unmarshal error: %v", err)
	}
	if dst.Model != "whisper-1" || dst.Language != "en" {
		t.Errorf("unexpected multipart decode: %+v", dst)
	}
}

func TestInMemoryRateLimiter(t *testing.T) {
	l := &InMemoryRateLimiter{}
	l.Init(0) // no background sweeper

	key := "user-1"
	// First 3 within the window are allowed.
	for i := 0; i < 3; i++ {
		if !l.Request(key, 3, 60) {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	// 4th within the same window (duration 60s) is rejected.
	if l.Request(key, 3, 60) {
		t.Error("4th request over the limit should be rejected")
	}
	// A different key has its own bucket.
	if !l.Request("user-2", 3, 60) {
		t.Error("independent key should be allowed")
	}
	// With duration 0, the oldest entry is always evictable so it stays allowed.
	if !l.Request(key, 3, 0) {
		t.Error("zero-duration window should allow (slide oldest out)")
	}
}

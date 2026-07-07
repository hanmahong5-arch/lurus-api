package helper

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"

	"github.com/gin-gonic/gin"
)

// writeCtx returns a gin context wired to a fresh recorder plus a minimal request
// so the stream writers have a live http.Flusher and request context.
func writeCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	return c, w
}

func TestFlushWriter(t *testing.T) {
	t.Run("nil context returns nil", func(t *testing.T) {
		if err := FlushWriter(nil); err != nil {
			t.Errorf("nil ctx: %v", err)
		}
	})

	t.Run("flushes recorder", func(t *testing.T) {
		c, w := writeCtx()
		if err := FlushWriter(c); err != nil {
			t.Errorf("unexpected err: %v", err)
		}
		if !w.Flushed {
			t.Error("recorder should be flushed")
		}
	})

	t.Run("cancelled request context errors", func(t *testing.T) {
		c, _ := writeCtx()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c.Request = c.Request.WithContext(ctx)
		if err := FlushWriter(c); err == nil || !strings.Contains(err.Error(), "request context done") {
			t.Errorf("expected context-done error, got %v", err)
		}
	})
}

func TestStringData(t *testing.T) {
	t.Run("writes data frame", func(t *testing.T) {
		c, w := writeCtx()
		if err := StringData(c, "hello"); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !strings.Contains(w.Body.String(), "data: hello") {
			t.Errorf("body = %q, want data frame", w.Body.String())
		}
	})

	t.Run("nil context errors", func(t *testing.T) {
		if err := StringData(nil, "x"); err == nil {
			t.Error("expected error for nil context")
		}
	})

	t.Run("cancelled context errors", func(t *testing.T) {
		c, _ := writeCtx()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c.Request = c.Request.WithContext(ctx)
		if err := StringData(c, "x"); err == nil {
			t.Error("expected error for cancelled context")
		}
	})
}

func TestObjectData(t *testing.T) {
	t.Run("marshals and writes", func(t *testing.T) {
		c, w := writeCtx()
		if err := ObjectData(c, map[string]string{"k": "v"}); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !strings.Contains(w.Body.String(), `"k":"v"`) {
			t.Errorf("body = %q", w.Body.String())
		}
	})

	t.Run("nil object errors", func(t *testing.T) {
		c, _ := writeCtx()
		if err := ObjectData(c, nil); err == nil {
			t.Error("expected error for nil object")
		}
	})
}

func TestPingData(t *testing.T) {
	t.Run("writes ping comment", func(t *testing.T) {
		c, w := writeCtx()
		if err := PingData(c); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !strings.Contains(w.Body.String(), ": PING") {
			t.Errorf("body = %q, want PING", w.Body.String())
		}
	})

	t.Run("nil context errors", func(t *testing.T) {
		if err := PingData(nil); err == nil {
			t.Error("expected error for nil context")
		}
	})
}

func TestDone(t *testing.T) {
	c, w := writeCtx()
	Done(c)
	if !strings.Contains(w.Body.String(), "[DONE]") {
		t.Errorf("body = %q, want [DONE]", w.Body.String())
	}
}

func TestClaudeDataAndChunk(t *testing.T) {
	t.Run("ClaudeData writes event + data", func(t *testing.T) {
		c, w := writeCtx()
		if err := ClaudeData(c, dto.ClaudeResponse{Type: "message_start"}); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		body := w.Body.String()
		if !strings.Contains(body, "event: message_start") || !strings.Contains(body, "data:") {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("ClaudeChunkData writes provided data", func(t *testing.T) {
		c, w := writeCtx()
		ClaudeChunkData(c, dto.ClaudeResponse{Type: "content_block_delta"}, `{"x":1}`)
		body := w.Body.String()
		if !strings.Contains(body, "event: content_block_delta") || !strings.Contains(body, `{"x":1}`) {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("ResponseChunkData writes event + data", func(t *testing.T) {
		c, w := writeCtx()
		ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "response.output_text.delta"}, `{"y":2}`)
		body := w.Body.String()
		if !strings.Contains(body, "event: response.output_text.delta") || !strings.Contains(body, `{"y":2}`) {
			t.Errorf("body = %q", body)
		}
	})
}

func TestSetEventStreamHeaders(t *testing.T) {
	t.Run("sets SSE headers once", func(t *testing.T) {
		c, w := writeCtx()
		c.Set(common.RequestIdKey, "req-99")
		SetEventStreamHeaders(c)
		if w.Header().Get("Content-Type") != "text/event-stream" {
			t.Errorf("content-type = %q", w.Header().Get("Content-Type"))
		}
		if w.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("cache-control = %q", w.Header().Get("Cache-Control"))
		}
		if w.Header().Get("X-Request-Id") != "req-99" {
			t.Errorf("x-request-id = %q", w.Header().Get("X-Request-Id"))
		}
	})

	t.Run("idempotent - second call is a no-op guard", func(t *testing.T) {
		c, w := writeCtx()
		SetEventStreamHeaders(c)
		// mutate then call again; guard should prevent overwrite path from running
		w.Header().Set("Content-Type", "sentinel")
		SetEventStreamHeaders(c)
		if w.Header().Get("Content-Type") != "sentinel" {
			t.Errorf("headers should not be reset on second call, got %q", w.Header().Get("Content-Type"))
		}
	})
}

func TestGetLocalRealtimeID(t *testing.T) {
	c, _ := writeCtx()
	c.Set(common.RequestIdKey, "abc")
	if got := GetLocalRealtimeID(c); got != "evt_abc" {
		t.Errorf("GetLocalRealtimeID = %q, want evt_abc", got)
	}
}

func TestGenerateFinalUsageResponseWithExtension(t *testing.T) {
	usage := dto.Usage{PromptTokens: 10}
	ext := ComputeLurusExtension(&relaycommon.RelayInfo{UserQuota: 1000000}, &usage, 500)
	resp := GenerateFinalUsageResponse("id", 1, "gpt-4", usage, ext)
	if resp.Usage == nil || resp.Usage.XLurus == nil {
		t.Fatal("expected XLurus extension attached to usage")
	}
	if resp.Usage.XLurus.BillingMode != "legacy" {
		t.Errorf("billing mode = %q, want legacy", resp.Usage.XLurus.BillingMode)
	}
}

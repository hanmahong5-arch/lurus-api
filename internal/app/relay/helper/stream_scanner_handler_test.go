package helper

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"

	"github.com/gin-gonic/gin"
)

func init() {
	// StreamScannerHandler builds a time.NewTicker(StreamingTimeout*time.Second);
	// a zero value would panic, so ensure a positive, comfortably-large timeout.
	if constant.StreamingTimeout <= 0 {
		constant.StreamingTimeout = 60
	}
}

func newStreamCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	return c, w
}

func respFromString(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestStreamScannerHandler_NilGuards(t *testing.T) {
	c, _ := newStreamCtx()
	info := &relaycommon.RelayInfo{}
	// nil resp / nil handler must return immediately without panic.
	StreamScannerHandler(c, nil, info, func(string) bool { return true })
	resp := respFromString("data: x\n")
	defer func() { _ = resp.Body.Close() }()
	StreamScannerHandler(c, resp, info, nil)
}

func TestStreamScannerHandler_ParsesFramesUntilDone(t *testing.T) {
	c, _ := newStreamCtx()
	info := &relaycommon.RelayInfo{}

	body := "data: {\"a\":1}\n\n" +
		"event: ignored\n" +
		"data: {\"b\":2}\n\n" +
		"data: [DONE]\n\n" +
		"data: {\"c\":3}\n\n" // after DONE must not be delivered

	var (
		mu        sync.Mutex
		collected []string
	)
	sawDone := false
	resp := respFromString(body)
	defer func() { _ = resp.Body.Close() }()
	StreamScannerHandler(c, resp, info, func(data string) bool {
		mu.Lock()
		collected = append(collected, data)
		mu.Unlock()
		return true
	}, &sawDone)

	if !sawDone {
		t.Error("sawTerminalMarker should be true when [DONE] observed")
	}
	if len(collected) != 2 {
		t.Fatalf("collected = %v, want 2 frames before DONE", collected)
	}
	if collected[0] != `{"a":1}` || collected[1] != `{"b":2}` {
		t.Errorf("collected = %v, want [{\"a\":1} {\"b\":2}]", collected)
	}
}

func TestStreamScannerHandler_HandlerFalseStops(t *testing.T) {
	c, _ := newStreamCtx()
	info := &relaycommon.RelayInfo{}

	body := "data: one\n\ndata: two\n\ndata: three\n\n"

	var (
		mu        sync.Mutex
		collected []string
	)
	resp := respFromString(body)
	defer func() { _ = resp.Body.Close() }()
	StreamScannerHandler(c, resp, info, func(data string) bool {
		mu.Lock()
		collected = append(collected, data)
		n := len(collected)
		mu.Unlock()
		return n < 1 // stop after the very first frame
	})

	mu.Lock()
	defer mu.Unlock()
	if len(collected) != 1 {
		t.Fatalf("collected = %v, want exactly 1 frame before stop", collected)
	}
	if collected[0] != "one" {
		t.Errorf("first frame = %q, want one", collected[0])
	}
}

func TestStreamScannerHandler_EOFWithoutDone(t *testing.T) {
	c, _ := newStreamCtx()
	info := &relaycommon.RelayInfo{}

	// no [DONE] terminator -> clean EOF, sawTerminalMarker stays false.
	body := "data: alpha\n\ndata: beta\n\n"

	var count int
	var mu sync.Mutex
	sawDone := false
	resp := respFromString(body)
	defer func() { _ = resp.Body.Close() }()
	StreamScannerHandler(c, resp, info, func(string) bool {
		mu.Lock()
		count++
		mu.Unlock()
		return true
	}, &sawDone)

	if sawDone {
		t.Error("sawTerminalMarker should be false without [DONE]")
	}
	mu.Lock()
	defer mu.Unlock()
	if count != 2 {
		t.Errorf("delivered %d frames, want 2", count)
	}
}

func TestStreamScannerHandler_SkipsShortAndNonDataLines(t *testing.T) {
	c, _ := newStreamCtx()
	info := &relaycommon.RelayInfo{}

	// lines under 6 chars, comment/event lines and empty data are all skipped.
	body := ": ping\n" +
		"id: 1\n" +
		"data:\n" +
		"event: message\n" +
		"data: real\n\n" +
		"data: [DONE]\n\n"

	var (
		mu        sync.Mutex
		collected []string
	)
	resp := respFromString(body)
	defer func() { _ = resp.Body.Close() }()
	StreamScannerHandler(c, resp, info, func(data string) bool {
		mu.Lock()
		collected = append(collected, data)
		mu.Unlock()
		return true
	})

	mu.Lock()
	defer mu.Unlock()
	if len(collected) != 1 || collected[0] != "real" {
		t.Errorf("collected = %v, want [real]", collected)
	}
}

func TestGetScannerBufferSize(t *testing.T) {
	prev := constant.StreamScannerMaxBufferMB
	t.Cleanup(func() { constant.StreamScannerMaxBufferMB = prev })

	constant.StreamScannerMaxBufferMB = 4
	if got := getScannerBufferSize(); got != 4<<20 {
		t.Errorf("getScannerBufferSize = %d, want %d", got, 4<<20)
	}

	constant.StreamScannerMaxBufferMB = 0
	// falls back to config value (a positive default)
	if got := getScannerBufferSize(); got <= 0 {
		t.Errorf("getScannerBufferSize fallback = %d, want > 0", got)
	}
}

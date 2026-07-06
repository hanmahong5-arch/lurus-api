package helper

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/config"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/operation_setting"
)

func respFromReader(body io.Reader) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(body),
		Header:     make(http.Header),
	}
}

// TestStreamScannerHandler_PingGoroutine drives the keep-alive ping path: with
// ping enabled and a very short interval, at least one ": PING" frame must be
// emitted to the client while the scanner is blocked waiting for upstream data.
func TestStreamScannerHandler_PingGoroutine(t *testing.T) {
	gs := operation_setting.GetGeneralSetting()
	prevEnabled, prevSeconds := gs.PingIntervalEnabled, gs.PingIntervalSeconds
	gs.PingIntervalEnabled = true
	gs.PingIntervalSeconds = 0 // <=0 -> falls back to config Relay.PingInterval
	t.Cleanup(func() {
		gs.PingIntervalEnabled = prevEnabled
		gs.PingIntervalSeconds = prevSeconds
	})

	relayCfg := config.Get()
	prevPing := relayCfg.Relay.PingInterval
	relayCfg.Relay.PingInterval = 5 * time.Millisecond
	t.Cleanup(func() { relayCfg.Relay.PingInterval = prevPing })

	c, w := newStreamCtx()
	info := &relaycommon.RelayInfo{} // DisablePing false -> ping active

	// io.Pipe lets us hold the stream open long enough for ping ticks to fire.
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.WriteString(pw, "data: {\"a\":1}\n\n")
		// keep the connection open so several ping intervals elapse
		time.Sleep(40 * time.Millisecond)
		_, _ = io.WriteString(pw, "data: [DONE]\n\n")
		_ = pw.Close()
	}()

	var (
		mu        sync.Mutex
		collected []string
	)
	StreamScannerHandler(c, respFromReader(pr), info, func(data string) bool {
		mu.Lock()
		collected = append(collected, data)
		mu.Unlock()
		return true
	})

	if !strings.Contains(w.Body.String(), ": PING") {
		t.Errorf("expected at least one ': PING' keep-alive frame, body = %q", w.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(collected) != 1 || collected[0] != `{"a":1}` {
		t.Errorf("collected = %v, want [{\"a\":1}]", collected)
	}
}

// TestStreamScannerHandler_DebugEnabled exercises the DebugEnabled println
// branches (setup banner, per-frame echo, [DONE] and goroutine-exit traces).
func TestStreamScannerHandler_DebugEnabled(t *testing.T) {
	prev := common.DebugEnabled
	common.DebugEnabled = true
	t.Cleanup(func() { common.DebugEnabled = prev })

	c, _ := newStreamCtx()
	info := &relaycommon.RelayInfo{}

	body := "data: {\"a\":1}\n\ndata: [DONE]\n\n"
	sawDone := false
	StreamScannerHandler(c, respFromString(body), info, func(string) bool { return true }, &sawDone)
	if !sawDone {
		t.Error("expected [DONE] terminator to be observed")
	}
}

// TestStreamScannerHandler_DataHandlerTimeout forces the per-write timeout branch
// by making the data handler block far longer than the configured WriteTimeout.
func TestStreamScannerHandler_DataHandlerTimeout(t *testing.T) {
	relayCfg := config.Get()
	prevWrite := relayCfg.Relay.WriteTimeout
	relayCfg.Relay.WriteTimeout = 5 * time.Millisecond
	t.Cleanup(func() { relayCfg.Relay.WriteTimeout = prevWrite })

	c, _ := newStreamCtx()
	info := &relaycommon.RelayInfo{}

	// handlerEntered is written from the data-handler goroutine (StreamScannerHandler
	// dispatches the handler via common.SafeGo) and read here after the timeout branch
	// returns while that goroutine may still be sleeping — use atomic to stay race-free
	// under the -race gate.
	var handlerEntered atomic.Int32
	body := "data: {\"slow\":1}\n\ndata: {\"never\":2}\n\n"
	StreamScannerHandler(c, respFromString(body), info, func(string) bool {
		handlerEntered.Add(1)
		time.Sleep(60 * time.Millisecond) // exceeds WriteTimeout -> timeout branch
		return true
	})

	// The scanner must bail out after the first (timed-out) frame, so the second
	// frame is never delivered.
	if got := handlerEntered.Load(); got != 1 {
		t.Errorf("handler entered %d times, want 1 (timeout should stop the loop)", got)
	}
}

// TestStreamScannerHandler_ScannerTooLongError drives the scanner.Err() != EOF
// branch by feeding a single line larger than the scanner's max buffer.
func TestStreamScannerHandler_ScannerTooLongError(t *testing.T) {
	relayCfg := config.Get()
	prevInit, prevMax := relayCfg.Relay.StreamScannerInitialBuffer, relayCfg.Relay.StreamScannerMaxBuffer
	relayCfg.Relay.StreamScannerInitialBuffer = 64
	relayCfg.Relay.StreamScannerMaxBuffer = 128
	t.Cleanup(func() {
		relayCfg.Relay.StreamScannerInitialBuffer = prevInit
		relayCfg.Relay.StreamScannerMaxBuffer = prevMax
	})

	c, _ := newStreamCtx()
	info := &relaycommon.RelayInfo{}

	// A single line far exceeding 128 bytes triggers bufio.ErrTooLong (not EOF).
	huge := "data: " + strings.Repeat("x", 5000) + "\n\n"
	var delivered int
	StreamScannerHandler(c, respFromString(huge), info, func(string) bool {
		delivered++
		return true
	})

	if delivered != 0 {
		t.Errorf("delivered %d frames, want 0 (over-long line should error out)", delivered)
	}
}

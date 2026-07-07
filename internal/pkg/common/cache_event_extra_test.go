package common

import (
	"net/http/httptest"
	"testing"
)

// TestBillingCache_RedisDisabled asserts the wallet-balance cache degrades
// safely (no panic, cache-miss semantics) when Redis is unavailable — the
// common configuration where these helpers must fail closed, not crash.
func TestBillingCache_RedisDisabled(t *testing.T) {
	origEnabled, origRDB := RedisEnabled, RDB
	RedisEnabled = false
	RDB = nil
	t.Cleanup(func() { RedisEnabled, RDB = origEnabled, origRDB })

	if bal, ok := GetCachedWalletBalance(123); ok || bal != 0 {
		t.Errorf("no-redis Get should miss, got (%v,%v)", bal, ok)
	}
	// Set / Invalidate must be no-ops (no panic).
	SetCachedWalletBalance(123, 500)
	InvalidateCachedWalletBalance(123)

	// Without a cached balance, the fast-path skip must be denied.
	if ShouldSkipPreAuth(123, 1.0) {
		t.Error("ShouldSkipPreAuth must be false when no cached balance exists")
	}
}

// TestCustomEvent_Render verifies the SSE encoder writes the event data and the
// text/event-stream content type through to the ResponseWriter.
func TestCustomEvent_Render(t *testing.T) {
	rec := httptest.NewRecorder()
	ev := &CustomEvent{Data: "hello-world"}
	if err := ev.Render(rec); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content type = %q, want text/event-stream", ct)
	}
	if body := rec.Body.String(); body != "hello-world" {
		t.Errorf("body = %q, want hello-world", body)
	}

	// Data starting with "data" appends the SSE terminator "\n\n".
	rec2 := httptest.NewRecorder()
	ev2 := &CustomEvent{Data: "data: payload"}
	if err := ev2.Render(rec2); err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if got := rec2.Body.String(); got != "data: payload\n\n" {
		t.Errorf("body = %q, want trailing \\n\\n", got)
	}
}

// TestCustomEvent_WriteContentType_PreservesCacheControl ensures an existing
// Cache-Control header is not overwritten.
func TestCustomEvent_WriteContentType_PreservesCacheControl(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Cache-Control", "max-age=60")
	ev := &CustomEvent{}
	ev.WriteContentType(rec)
	if got := rec.Header().Get("Cache-Control"); got != "max-age=60" {
		t.Errorf("Cache-Control overwritten: %q", got)
	}
	// Default no-cache set when absent.
	rec2 := httptest.NewRecorder()
	ev.WriteContentType(rec2)
	if got := rec2.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("default Cache-Control = %q, want no-cache", got)
	}
}

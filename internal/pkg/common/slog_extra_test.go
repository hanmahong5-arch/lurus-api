package common

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestSlogConfigFromEnv(t *testing.T) {
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "debug")
	cfg := SlogConfigFromEnv()
	if !cfg.JSONFormat || cfg.Level != slog.LevelDebug {
		t.Errorf("json/debug env not applied: %+v", cfg)
	}

	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("LOG_LEVEL", "warn")
	cfg = SlogConfigFromEnv()
	if cfg.JSONFormat || cfg.Level != slog.LevelWarn {
		t.Errorf("text/warn env not applied: %+v", cfg)
	}

	t.Setenv("LOG_FORMAT", "")
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("GIN_MODE", "release")
	cfg = SlogConfigFromEnv()
	if !cfg.JSONFormat || cfg.Level != slog.LevelError {
		t.Errorf("release auto-json/error not applied: %+v", cfg)
	}
}

func TestSlogTextHandler_LogsWithContext(t *testing.T) {
	buf := &bytes.Buffer{}
	InitSlog(&SlogConfig{Level: slog.LevelDebug, Writer: buf, ErrWriter: buf, TimeFormat: "15:04:05"})
	t.Cleanup(func() { slogLogger.Store(nil) })

	ctx := WithRequestID(context.Background(), "req-77")
	LogInfo(ctx, "hello", "k", "v")
	out := buf.String()
	if !strings.Contains(out, "hello") || !strings.Contains(out, "req-77") || !strings.Contains(out, "k=v") {
		t.Errorf("text handler output missing fields: %q", out)
	}

	// Warn/Error route to errWriter (same buffer) with the WARN/ERR tags.
	buf.Reset()
	LogWarnf(ctx, "warn-%d", 5)
	if !strings.Contains(buf.String(), "WARN") || !strings.Contains(buf.String(), "warn-5") {
		t.Errorf("warn output: %q", buf.String())
	}
	buf.Reset()
	LogError(ctx, "boom")
	if !strings.Contains(buf.String(), "ERR") {
		t.Errorf("error output: %q", buf.String())
	}
}

func TestSlogWithSourceAndDebugf(t *testing.T) {
	buf := &bytes.Buffer{}
	InitSlog(&SlogConfig{Level: slog.LevelDebug, Writer: buf, ErrWriter: buf, TimeFormat: "15:04:05"})
	t.Cleanup(func() { slogLogger.Store(nil) })

	LogWithSource(context.Background(), slog.LevelInfo, "with-source")
	if !strings.Contains(buf.String(), "source=") || !strings.Contains(buf.String(), "with-source") {
		t.Errorf("LogWithSource missing source attr: %q", buf.String())
	}

	// LogDebugf only emits when DebugEnabled.
	buf.Reset()
	origDebug := DebugEnabled
	t.Cleanup(func() { DebugEnabled = origDebug })
	DebugEnabled = false
	LogDebugf(context.Background(), "dbg-%d", 1)
	if strings.Contains(buf.String(), "dbg-1") {
		t.Error("LogDebugf should be suppressed when DebugEnabled=false")
	}
	DebugEnabled = true
	LogDebugf(context.Background(), "dbg-%d", 2)
	if !strings.Contains(buf.String(), "dbg-2") {
		t.Errorf("LogDebugf should emit when DebugEnabled=true: %q", buf.String())
	}
}

func TestSlogTextHandler_WithAttrsAndGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	l := InitSlog(&SlogConfig{Level: slog.LevelInfo, Writer: buf, ErrWriter: buf, TimeFormat: "15:04:05"})
	t.Cleanup(func() { slogLogger.Store(nil) })

	// The custom text handler formats records itself, so WithAttrs merely wraps
	// the base handler (constructor coverage); the line still emits the message.
	l.With("component", "relay").Info("attr-line")
	if !strings.Contains(buf.String(), "attr-line") {
		t.Errorf("WithAttrs-wrapped log missing message: %q", buf.String())
	}
	// WithGroup must not panic and still emit the message.
	buf.Reset()
	l.WithGroup("grp").Info("group-line", "n", 1)
	if !strings.Contains(buf.String(), "group-line") {
		t.Errorf("WithGroup output missing: %q", buf.String())
	}
}

func TestSlogJSONHandler_ContextInjection(t *testing.T) {
	buf := &bytes.Buffer{}
	l := InitSlog(&SlogConfig{Level: slog.LevelInfo, Writer: buf, ErrWriter: buf, JSONFormat: true})
	t.Cleanup(func() { slogLogger.Store(nil) })

	ctx := WithUserID(WithRequestID(context.Background(), "rid-1"), "user-42")
	LogInfo(ctx, "json-msg")
	out := buf.String()
	if !strings.Contains(out, `"request_id":"rid-1"`) || !strings.Contains(out, `"user_id":"user-42"`) {
		t.Errorf("json contextHandler did not inject fields: %q", out)
	}

	// contextHandler.WithAttrs / WithGroup / Enabled paths.
	if !l.Handler().Enabled(ctx, slog.LevelError) {
		t.Error("error level should be enabled at info threshold")
	}
	buf.Reset()
	l.With("svc", "x").WithGroup("g").Info("wrapped")
	if !strings.Contains(buf.String(), "wrapped") {
		t.Errorf("wrapped json log missing: %q", buf.String())
	}
}

func TestBackground(t *testing.T) {
	if Background() == nil {
		t.Error("Background returned nil context")
	}
}

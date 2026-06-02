package common

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestInitSlog_DefaultConfig(t *testing.T) {
	// Reset slog state for testing
	slogLogger.Store(nil)

	InitSlog(nil)

	if slogLogger.Load() == nil {
		t.Error("slogLogger should not be nil after InitSlog")
	}
}

func TestInitSlog_CustomConfig(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:      slog.LevelDebug,
		Writer:     buf,
		ErrWriter:  buf,
		AddSource:  false,
		JSONFormat: false,
		TimeFormat: "15:04:05",
	}

	InitSlog(cfg)

	if slogLogger.Load() == nil {
		t.Error("slogLogger should not be nil after InitSlog with custom config")
	}
}

func TestInitSlog_JSONFormat(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:      slog.LevelInfo,
		Writer:     buf,
		ErrWriter:  buf,
		JSONFormat: true,
	}

	InitSlog(cfg)

	if slogLogger.Load() == nil {
		t.Error("slogLogger should not be nil after InitSlog with JSON format")
	}
}

func TestGetSlogLogger(t *testing.T) {
	slogLogger.Store(nil)

	logger := GetSlogLogger()
	if logger == nil {
		t.Error("GetSlogLogger should return a non-nil logger")
	}

	// Calling again should return the same logger
	logger2 := GetSlogLogger()
	if logger != logger2 {
		t.Error("GetSlogLogger should return the same logger instance")
	}
}

func TestSetSlogLevel(t *testing.T) {
	slogLogger.Store(nil)

	InitSlog(nil)

	// Should not panic
	SetSlogLevel(slog.LevelDebug)
	SetSlogLevel(slog.LevelWarn)
	SetSlogLevel(slog.LevelError)
	SetSlogLevel(slog.LevelInfo)
}

func TestLogInfo(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelInfo,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	ctx := context.Background()
	LogInfo(ctx, "test info message")

	output := buf.String()
	if !strings.Contains(output, "test info message") {
		t.Errorf("LogInfo output should contain the message, got: %s", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("LogInfo output should contain INFO level, got: %s", output)
	}
}

func TestLogError(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelInfo,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	ctx := context.Background()
	LogError(ctx, "test error message")

	output := buf.String()
	if !strings.Contains(output, "test error message") {
		t.Errorf("LogError output should contain the message, got: %s", output)
	}
	if !strings.Contains(output, "ERR") {
		t.Errorf("LogError output should contain ERR level, got: %s", output)
	}
}

func TestLogWarn(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelInfo,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	ctx := context.Background()
	LogWarn(ctx, "test warn message")

	output := buf.String()
	if !strings.Contains(output, "test warn message") {
		t.Errorf("LogWarn output should contain the message, got: %s", output)
	}
	if !strings.Contains(output, "WARN") {
		t.Errorf("LogWarn output should contain WARN level, got: %s", output)
	}
}

func TestLogDebug_Enabled(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelDebug,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	// Enable debug
	oldDebug := DebugEnabled
	DebugEnabled = true
	defer func() { DebugEnabled = oldDebug }()

	ctx := context.Background()
	LogDebug(ctx, "test debug message")

	output := buf.String()
	if !strings.Contains(output, "test debug message") {
		t.Errorf("LogDebug output should contain the message when debug enabled, got: %s", output)
	}
}

func TestLogDebug_Disabled(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelDebug,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	// Disable debug
	oldDebug := DebugEnabled
	DebugEnabled = false
	defer func() { DebugEnabled = oldDebug }()

	ctx := context.Background()
	LogDebug(ctx, "test debug message")

	output := buf.String()
	if strings.Contains(output, "test debug message") {
		t.Errorf("LogDebug should not output when debug disabled, got: %s", output)
	}
}

func TestLogWithContext_RequestID(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelInfo,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	ctx := context.WithValue(context.Background(), RequestIdKey, "test-request-123")
	LogInfo(ctx, "test message with request ID")

	output := buf.String()
	if !strings.Contains(output, "test-request-123") {
		t.Errorf("Log output should contain request ID, got: %s", output)
	}
}

func TestLogInfof(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelInfo,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	ctx := context.Background()
	LogInfof(ctx, "formatted %s %d", "message", 42)

	output := buf.String()
	if !strings.Contains(output, "formatted message 42") {
		t.Errorf("LogInfof output should contain formatted message, got: %s", output)
	}
}

func TestLogErrorf(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelInfo,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	ctx := context.Background()
	LogErrorf(ctx, "error: %s", "something went wrong")

	output := buf.String()
	if !strings.Contains(output, "error: something went wrong") {
		t.Errorf("LogErrorf output should contain formatted message, got: %s", output)
	}
}

func TestWithRequestID(t *testing.T) {
	ctx := WithRequestID(context.Background(), "my-request-id")

	val := ctx.Value(RequestIdKey)
	if val != "my-request-id" {
		t.Errorf("WithRequestID should set request ID, got: %v", val)
	}
}

func TestTimer(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelDebug,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	// Enable debug for timer output
	oldDebug := DebugEnabled
	DebugEnabled = true
	defer func() { DebugEnabled = oldDebug }()

	ctx := context.Background()
	done := Timer(ctx, "test-operation")

	// Simulate some work
	// Nothing actually needs to happen

	done()

	output := buf.String()
	if !strings.Contains(output, "test-operation") {
		t.Errorf("Timer output should contain operation name, got: %s", output)
	}
	if !strings.Contains(output, "took") {
		t.Errorf("Timer output should contain 'took', got: %s", output)
	}
}

func TestDefaultSlogConfig(t *testing.T) {
	cfg := DefaultSlogConfig()

	if cfg.Level != slog.LevelInfo {
		t.Errorf("Default level should be Info, got: %v", cfg.Level)
	}
	if cfg.Writer == nil {
		t.Error("Default writer should not be nil")
	}
	if cfg.ErrWriter == nil {
		t.Error("Default errWriter should not be nil")
	}
	if cfg.JSONFormat {
		t.Error("Default should not use JSON format")
	}
	if cfg.AddSource {
		t.Error("Default should not add source")
	}
}

func TestSetSlogWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	SetSlogWriter(buf)

	// Should not panic
	slogMu.RLock()
	if slogWriter != buf {
		t.Error("SetSlogWriter should update slogWriter")
	}
	slogMu.RUnlock()
}

func TestSetSlogErrWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	SetSlogErrWriter(buf)

	// Should not panic
	slogMu.RLock()
	if slogErrWriter != buf {
		t.Error("SetSlogErrWriter should update slogErrWriter")
	}
	slogMu.RUnlock()
}

func TestConcurrentLogging(t *testing.T) {
	slogLogger.Store(nil)

	buf := &bytes.Buffer{}
	cfg := &SlogConfig{
		Level:     slog.LevelInfo,
		Writer:    buf,
		ErrWriter: buf,
	}
	InitSlog(cfg)

	ctx := context.Background()
	var wg sync.WaitGroup

	// Run concurrent logging
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			LogInfo(ctx, "concurrent message", "n", n)
		}(i)
	}

	wg.Wait()

	// Should not panic and should produce output
	output := buf.String()
	if len(output) == 0 {
		t.Error("Concurrent logging should produce output")
	}
}

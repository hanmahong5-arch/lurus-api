package tracing_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/LurusTech/lurus-hub/internal/pkg/tracing"
)

// TestChannelTypeToGenAISystem verifies the mapping from newhub channel type
// integers to OTel GenAI system name strings.
func TestChannelTypeToGenAISystem_KnownTypes(t *testing.T) {
	tests := []struct {
		channelType int
		want        string
	}{
		{1, "openai"},      // ChannelTypeOpenAI
		{14, "anthropic"},  // ChannelTypeAnthropic
		{24, "google"},     // ChannelTypeGemini
		{43, "deepseek"},   // ChannelTypeDeepSeek
		{3, "azure"},       // ChannelTypeAzure
		{20, "openrouter"}, // ChannelTypeOpenRouter
		{999, "unknown"},
	}
	for _, tc := range tests {
		got := tracing.ChannelTypeToGenAISystem(tc.channelType)
		if got != tc.want {
			t.Errorf("ChannelTypeToGenAISystem(%d) = %q, want %q", tc.channelType, got, tc.want)
		}
	}
}

// TestStartLLMSpan_CreatesSpanWithAttributes verifies that StartLLMSpan
// returns a span named "gen_ai.chat" with gen_ai.system and
// gen_ai.request.model attributes set.
func TestStartLLMSpan_CreatesSpanWithAttributes(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	tr := tp.Tracer("test")

	ctx := context.Background()
	_, span := tracing.StartLLMSpanWithTracer(ctx, tr, "openai", "gpt-4o")
	span.End()

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one recorded span")
	}

	var found sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == "gen_ai.chat" {
			found = s
			break
		}
	}
	if found == nil {
		names := make([]string, len(spans))
		for i, s := range spans {
			names[i] = s.Name()
		}
		t.Fatalf("no span named 'gen_ai.chat' found; got: %v", names)
	}

	assertAttr(t, found.Attributes(), semconv.GenAiSystemKey.String("openai"))
	assertAttr(t, found.Attributes(), semconv.GenAiRequestModelKey.String("gpt-4o"))
}

// TestSetGenAIAttributes_HappyPath verifies that usage attributes and finish
// reasons are set, and span status remains Unset (success).
func TestSetGenAIAttributes_HappyPath(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	tr := tp.Tracer("test")

	ctx := context.Background()
	_, span := tr.Start(ctx, "gen_ai.chat")

	tracing.SetGenAIAttributes(span, 100, 50, 0.0012, []string{"stop"}, nil)
	span.End()

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	s := spans[0]

	attrs := s.Attributes()
	assertAttr(t, attrs, semconv.GenAiUsagePromptTokensKey.Int(100))
	assertAttr(t, attrs, semconv.GenAiUsageCompletionTokensKey.Int(50))
	assertAttr(t, attrs, attribute.Key("gen_ai.usage.cost").Float64(0.0012))
	// finish reasons stored as string slice attribute
	assertAttrKey(t, attrs, attribute.Key("gen_ai.response.finish_reasons"))

	if s.Status().Code != codes.Unset {
		t.Errorf("expected status Unset for success path, got %v", s.Status().Code)
	}
}

// TestSetGenAIAttributes_ErrorPath verifies that upstream errors are recorded
// on the span and the status is set to Error.
func TestSetGenAIAttributes_ErrorPath(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	tr := tp.Tracer("test")

	ctx := context.Background()
	_, span := tr.Start(ctx, "gen_ai.chat")

	upstreamErr := errors.New("upstream 500 internal server error")
	tracing.SetGenAIAttributes(span, 0, 0, 0, nil, upstreamErr)
	span.End()

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	s := spans[0]

	if s.Status().Code != codes.Error {
		t.Errorf("expected status Error, got %v", s.Status().Code)
	}
	if len(s.Events()) == 0 {
		t.Error("expected at least one event (RecordError), got none")
	}
}

// assertAttr checks that the expected key-value attribute appears in the slice.
func assertAttr(t *testing.T, attrs []attribute.KeyValue, want attribute.KeyValue) {
	t.Helper()
	for _, a := range attrs {
		if a.Key == want.Key && a.Value == want.Value {
			return
		}
	}
	t.Errorf("attribute %v=%v not found; span attributes: %v", want.Key, want.Value.AsInterface(), attrs)
}

// assertAttrKey checks that any attribute with the given key appears.
func assertAttrKey(t *testing.T, attrs []attribute.KeyValue, key attribute.Key) {
	t.Helper()
	for _, a := range attrs {
		if a.Key == key {
			return
		}
	}
	t.Errorf("attribute key %v not found; span attributes: %v", key, attrs)
}

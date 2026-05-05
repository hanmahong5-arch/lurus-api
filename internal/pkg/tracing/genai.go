package tracing

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// genAISystemMap maps newhub channel type integers to OTel gen_ai.system values.
// Covers the most common providers; unknown types fall back to "unknown".
var genAISystemMap = map[int]string{
	1:  "openai",      // ChannelTypeOpenAI
	3:  "azure",       // ChannelTypeAzure
	14: "anthropic",   // ChannelTypeAnthropic
	24: "google",      // ChannelTypeGemini
	41: "google",      // ChannelTypeVertexAI (Google)
	43: "deepseek",    // ChannelTypeDeepSeek
	33: "aws",         // ChannelTypeAws (Bedrock)
	20: "openrouter",  // ChannelTypeOpenRouter
}

// ChannelTypeToGenAISystem converts a newhub channel type int to the
// OTel semantic convention gen_ai.system string value.
func ChannelTypeToGenAISystem(channelType int) string {
	if s, ok := genAISystemMap[channelType]; ok {
		return s
	}
	return "unknown"
}

// StartLLMSpan creates a child span named "gen_ai.chat" under the context
// and sets gen_ai.system and gen_ai.request.model. Uses the global Tracer().
// Always safe to call: when OTel is uninitialised the noop tracer is returned.
func StartLLMSpan(ctx context.Context, system, model string) (context.Context, trace.Span) {
	return StartLLMSpanWithTracer(ctx, Tracer(), system, model)
}

// StartLLMSpanWithTracer is the testable variant that accepts an explicit Tracer.
func StartLLMSpanWithTracer(ctx context.Context, tr trace.Tracer, system, model string) (context.Context, trace.Span) {
	ctx, span := tr.Start(ctx, "gen_ai.chat",
		trace.WithSpanKind(trace.SpanKindClient),
	)
	span.SetAttributes(
		semconv.GenAiSystemKey.String(system),
		semconv.GenAiRequestModelKey.String(model),
	)
	return ctx, span
}

// SetGenAIAttributes records usage metrics and finish reasons on the span.
// If err is non-nil the span is marked as error and the error is recorded.
// Safe to call with a noop span (no-ops automatically).
func SetGenAIAttributes(span trace.Span, inputTokens, outputTokens int, costCNY float64, finishReasons []string, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	attrs := []attribute.KeyValue{
		semconv.GenAiUsagePromptTokensKey.Int(inputTokens),
		semconv.GenAiUsageCompletionTokensKey.Int(outputTokens),
		attribute.Key("gen_ai.usage.cost").Float64(costCNY),
	}
	if len(finishReasons) > 0 {
		attrs = append(attrs, semconv.GenAiResponseFinishReasonsKey.StringSlice(finishReasons))
	}
	span.SetAttributes(attrs...)
}

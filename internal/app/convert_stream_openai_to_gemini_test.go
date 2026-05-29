package app

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

// Tests for StreamResponseOpenAI2Gemini — streaming response conversion for the
// /v1beta/models/* relay path. Previously 0% covered. Pins the leading-empty-
// chunk skip and the usage-source / finish-reason / tool-call handling.

func streamChoice(content string, finishReason *string) dto.ChatCompletionsStreamResponseChoice {
	d := dto.ChatCompletionsStreamResponseChoiceDelta{}
	if content != "" {
		d.SetContentString(content)
	}
	return dto.ChatCompletionsStreamResponseChoice{Delta: d, FinishReason: finishReason}
}

func streamToolChoice(name, args string) dto.ChatCompletionsStreamResponseChoice {
	return dto.ChatCompletionsStreamResponseChoice{
		Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
			ToolCalls: []dto.ToolCallResponse{
				{ID: "c1", Function: dto.FunctionResponse{Name: name, Arguments: args}},
			},
		},
	}
}

func TestStreamResponseOpenAI2Gemini_EmptyChunkReturnsNil(t *testing.T) {
	// Leading OpenAI stream chunks carry an empty delta and no finish reason;
	// they must be skipped (nil) rather than emitted as empty Gemini chunks.
	in := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{streamChoice("", nil)},
	}
	if out := StreamResponseOpenAI2Gemini(in, geminiInfo()); out != nil {
		t.Errorf("empty chunk -> %+v, want nil", out)
	}
}

func TestStreamResponseOpenAI2Gemini_TextDelta(t *testing.T) {
	in := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{streamChoice("hello", nil)},
	}
	out := StreamResponseOpenAI2Gemini(in, geminiInfo())
	if out == nil {
		t.Fatal("text delta -> nil, want a response")
	}
	if len(out.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1", len(out.Candidates))
	}
	parts := out.Candidates[0].Content.Parts
	if len(parts) != 1 || parts[0].Text != "hello" {
		t.Errorf("parts = %+v, want one text 'hello'", parts)
	}
	// No finish reason on a mid-stream content chunk.
	if out.Candidates[0].FinishReason != nil {
		t.Errorf("FinishReason = %v, want nil mid-stream", out.Candidates[0].FinishReason)
	}
}

func TestStreamResponseOpenAI2Gemini_FinishReasonOnlyNotSkipped(t *testing.T) {
	stop := "stop"
	in := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{streamChoice("", &stop)},
	}
	out := StreamResponseOpenAI2Gemini(in, geminiInfo())
	if out == nil {
		t.Fatal("finish-reason chunk -> nil, want a response (must not be skipped)")
	}
	if fr := out.Candidates[0].FinishReason; fr == nil || *fr != "STOP" {
		t.Errorf("FinishReason = %v, want STOP", fr)
	}
	if len(out.Candidates[0].Content.Parts) != 0 {
		t.Errorf("parts = %+v, want none (empty content)", out.Candidates[0].Content.Parts)
	}
}

func TestStreamResponseOpenAI2Gemini_UsageFromResponse(t *testing.T) {
	in := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{streamChoice("x", nil)},
		Usage:   &dto.Usage{PromptTokens: 11, CompletionTokens: 5, TotalTokens: 16},
	}
	out := StreamResponseOpenAI2Gemini(in, geminiInfo())
	if out.UsageMetadata.PromptTokenCount != 11 ||
		out.UsageMetadata.CandidatesTokenCount != 5 ||
		out.UsageMetadata.TotalTokenCount != 16 {
		t.Errorf("usage = %+v, want prompt=11 cand=5 total=16 (from response Usage)", out.UsageMetadata)
	}
}

func TestStreamResponseOpenAI2Gemini_UsageFromEstimateWhenNoUsage(t *testing.T) {
	info := geminiInfo()
	info.SetEstimatePromptTokens(15)
	in := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{streamChoice("x", nil)},
	}
	out := StreamResponseOpenAI2Gemini(in, info)
	if out.UsageMetadata.PromptTokenCount != 15 || out.UsageMetadata.TotalTokenCount != 15 {
		t.Errorf("usage = %+v, want prompt=15 total=15 (from estimate when no Usage)", out.UsageMetadata)
	}
	if out.UsageMetadata.CandidatesTokenCount != 0 {
		t.Errorf("CandidatesTokenCount = %d, want 0 when no Usage", out.UsageMetadata.CandidatesTokenCount)
	}
}

func TestStreamResponseOpenAI2Gemini_ToolCallDelta(t *testing.T) {
	in := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{streamToolChoice("get_weather", `{"city":"NYC"}`)},
	}
	out := StreamResponseOpenAI2Gemini(in, geminiInfo())
	if out == nil {
		t.Fatal("tool-call delta -> nil, want a response")
	}
	parts := out.Candidates[0].Content.Parts
	if len(parts) != 1 || parts[0].FunctionCall == nil {
		t.Fatalf("parts = %+v, want one function call", parts)
	}
	fc := parts[0].FunctionCall
	if fc.FunctionName != "get_weather" {
		t.Errorf("FunctionName = %q, want get_weather", fc.FunctionName)
	}
	args, ok := fc.Arguments.(map[string]interface{})
	if !ok || args["city"] != "NYC" {
		t.Errorf("Arguments = %#v, want map{city:NYC}", fc.Arguments)
	}
}

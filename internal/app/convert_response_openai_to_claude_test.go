package app

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

// Tests for ResponseOpenAI2Claude — the response-side conversion behind the
// /v1/messages (RelayFormatClaude) relay path (non-streaming). Together with
// the ClaudeToOpenAIRequest tests this pins the full request->response
// round-trip the Anthropic-native endpoint relies on. Previously 0% covered.

func textChoice(content, finishReason string) dto.OpenAITextResponseChoice {
	m := dto.Message{Role: "assistant"}
	m.SetStringContent(content)
	return dto.OpenAITextResponseChoice{Message: m, FinishReason: finishReason}
}

func toolChoice(id, name, args string) dto.OpenAITextResponseChoice {
	m := dto.Message{Role: "assistant"}
	m.SetToolCalls([]dto.ToolCallRequest{
		{ID: id, Type: "function", Function: dto.FunctionRequest{Name: name, Arguments: args}},
	})
	return dto.OpenAITextResponseChoice{Message: m, FinishReason: "tool_calls"}
}

func TestResponseOpenAI2Claude_TextResponse(t *testing.T) {
	in := &dto.OpenAITextResponse{
		Id:      "resp_1",
		Model:   "gpt-4o",
		Choices: []dto.OpenAITextResponseChoice{textChoice("Hello there", "stop")},
		Usage:   dto.Usage{PromptTokens: 10, CompletionTokens: 4},
	}

	out := ResponseOpenAI2Claude(in, nonOpenRouterInfo())
	if out.Id != "resp_1" || out.Model != "gpt-4o" {
		t.Errorf("Id/Model = %q/%q, want resp_1/gpt-4o", out.Id, out.Model)
	}
	if out.Type != "message" || out.Role != "assistant" {
		t.Errorf("Type/Role = %q/%q, want message/assistant", out.Type, out.Role)
	}
	if out.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want end_turn", out.StopReason)
	}
	if len(out.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(out.Content))
	}
	if out.Content[0].Type != "text" || out.Content[0].GetText() != "Hello there" {
		t.Errorf("Content[0] = {%q,%q}, want {text, Hello there}", out.Content[0].Type, out.Content[0].GetText())
	}
	if out.Usage == nil || out.Usage.InputTokens != 10 || out.Usage.OutputTokens != 4 {
		t.Errorf("Usage = %+v, want {InputTokens:10, OutputTokens:4}", out.Usage)
	}
}

func TestResponseOpenAI2Claude_ToolCallParsedAsMap(t *testing.T) {
	in := &dto.OpenAITextResponse{
		Id:      "resp_2",
		Model:   "gpt-4o",
		Choices: []dto.OpenAITextResponseChoice{toolChoice("call_1", "get_weather", `{"city":"NYC"}`)},
		Usage:   dto.Usage{PromptTokens: 8, CompletionTokens: 12},
	}

	out := ResponseOpenAI2Claude(in, nonOpenRouterInfo())
	if out.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want tool_use", out.StopReason)
	}
	if len(out.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(out.Content))
	}
	block := out.Content[0]
	if block.Type != "tool_use" || block.Id != "call_1" || block.Name != "get_weather" {
		t.Errorf("block = {%q,%q,%q}, want {tool_use, call_1, get_weather}", block.Type, block.Id, block.Name)
	}
	input, ok := block.Input.(map[string]interface{})
	if !ok {
		t.Fatalf("Input type = %T, want map[string]interface{} (valid JSON args must parse to an object)", block.Input)
	}
	if input["city"] != "NYC" {
		t.Errorf("Input[city] = %v, want NYC", input["city"])
	}
}

// TestResponseOpenAI2Claude_InvalidToolArgsFallsBackToRawString pins the
// fallback at convert.go: when a tool call's Arguments are not valid JSON, the
// raw string is preserved as Input rather than dropped or erroring.
func TestResponseOpenAI2Claude_InvalidToolArgsFallsBackToRawString(t *testing.T) {
	in := &dto.OpenAITextResponse{
		Id:      "resp_3",
		Model:   "gpt-4o",
		Choices: []dto.OpenAITextResponseChoice{toolChoice("call_2", "do_thing", "not-json")},
	}

	out := ResponseOpenAI2Claude(in, nonOpenRouterInfo())
	if len(out.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(out.Content))
	}
	raw, ok := out.Content[0].Input.(string)
	if !ok || raw != "not-json" {
		t.Errorf("Input = %#v, want raw string \"not-json\" (invalid JSON must fall back to the raw arguments)", out.Content[0].Input)
	}
}

func TestResponseOpenAI2Claude_EmptyChoices(t *testing.T) {
	in := &dto.OpenAITextResponse{Id: "resp_4", Model: "gpt-4o"}
	out := ResponseOpenAI2Claude(in, nonOpenRouterInfo())
	if len(out.Content) != 0 {
		t.Errorf("len(Content) = %d, want 0 for empty choices", len(out.Content))
	}
	if out.StopReason != "" {
		t.Errorf("StopReason = %q, want empty for empty choices", out.StopReason)
	}
	// Type/Role are still set so the envelope is well-formed.
	if out.Type != "message" || out.Role != "assistant" {
		t.Errorf("Type/Role = %q/%q, want message/assistant", out.Type, out.Role)
	}
}

func TestResponseOpenAI2Claude_MultipleChoices(t *testing.T) {
	in := &dto.OpenAITextResponse{
		Id:    "resp_5",
		Model: "gpt-4o",
		Choices: []dto.OpenAITextResponseChoice{
			textChoice("first", "stop"),
			textChoice("second", "length"),
		},
	}

	out := ResponseOpenAI2Claude(in, nonOpenRouterInfo())
	if len(out.Content) != 2 {
		t.Fatalf("len(Content) = %d, want 2 (one block per choice)", len(out.Content))
	}
	if out.Content[0].GetText() != "first" || out.Content[1].GetText() != "second" {
		t.Errorf("contents = [%q,%q], want [first, second]", out.Content[0].GetText(), out.Content[1].GetText())
	}
	// stopReason reflects the LAST choice's finish reason (length -> max_tokens).
	if out.StopReason != "max_tokens" {
		t.Errorf("StopReason = %q, want max_tokens (from last choice)", out.StopReason)
	}
}

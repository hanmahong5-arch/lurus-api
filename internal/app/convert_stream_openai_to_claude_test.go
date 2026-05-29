package app

import (
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

// Tests for StreamResponseOpenAI2Claude — streaming response conversion for the
// /v1/messages (RelayFormatClaude) relay path. Previously 0% covered. Covers
// the first-chunk branch (SendResponseCount==1): message_start emission and the
// text / tool_use / thinking content blocks, plus the Done short-circuit and
// the finish-in-first-chunk stop sequence.

func claudeStreamInfo(estimatePrompt int) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		SendResponseCount: 1,
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{},
	}
	info.SetEstimatePromptTokens(estimatePrompt)
	return info
}

func TestStreamResponseOpenAI2Claude_DoneReturnsNil(t *testing.T) {
	info := claudeStreamInfo(0)
	info.ClaudeConvertInfo.Done = true
	if out := StreamResponseOpenAI2Claude(&dto.ChatCompletionsStreamResponse{}, info); out != nil {
		t.Errorf("Done -> %+v, want nil (no further events after stream completed)", out)
	}
}

func TestStreamResponseOpenAI2Claude_FirstChunkText(t *testing.T) {
	in := &dto.ChatCompletionsStreamResponse{
		Id:      "msg_1",
		Model:   "claude-sonnet",
		Choices: []dto.ChatCompletionsStreamResponseChoice{streamChoice("Hello", nil)},
	}
	out := StreamResponseOpenAI2Claude(in, claudeStreamInfo(12))
	if len(out) != 3 {
		t.Fatalf("got %d events, want 3 (message_start, content_block_start, content_block_delta)", len(out))
	}
	if out[0].Type != "message_start" || out[0].Message == nil {
		t.Fatalf("event[0] = %q, want message_start with Message", out[0].Type)
	}
	if out[0].Message.Id != "msg_1" || out[0].Message.Role != "assistant" || out[0].Message.Type != "message" {
		t.Errorf("message_start.Message = {%q,%q,%q}, want {msg_1, assistant, message}",
			out[0].Message.Id, out[0].Message.Role, out[0].Message.Type)
	}
	if out[0].Message.Usage == nil || out[0].Message.Usage.InputTokens != 12 {
		t.Errorf("message_start usage input = %v, want 12 (estimate)", out[0].Message.Usage)
	}
	if out[1].Type != "content_block_start" || out[1].ContentBlock == nil || out[1].ContentBlock.Type != "text" {
		t.Errorf("event[1] = %q/%+v, want content_block_start text", out[1].Type, out[1].ContentBlock)
	}
	if out[2].Type != "content_block_delta" || out[2].Delta == nil || out[2].Delta.Type != "text_delta" || out[2].Delta.GetText() != "Hello" {
		t.Errorf("event[2] = %q/%+v, want content_block_delta text_delta 'Hello'", out[2].Type, out[2].Delta)
	}
}

func TestStreamResponseOpenAI2Claude_FirstChunkToolUse(t *testing.T) {
	in := &dto.ChatCompletionsStreamResponse{
		Id:      "msg_2",
		Model:   "claude-sonnet",
		Choices: []dto.ChatCompletionsStreamResponseChoice{streamToolChoice("get_weather", `{"city":"NYC"}`)},
	}
	out := StreamResponseOpenAI2Claude(in, claudeStreamInfo(0))
	if len(out) != 3 {
		t.Fatalf("got %d events, want 3 (message_start, content_block_start tool_use, input_json_delta)", len(out))
	}
	if out[0].Type != "message_start" {
		t.Errorf("event[0] = %q, want message_start", out[0].Type)
	}
	if out[1].Type != "content_block_start" || out[1].ContentBlock == nil ||
		out[1].ContentBlock.Type != "tool_use" || out[1].ContentBlock.Name != "get_weather" || out[1].ContentBlock.Id != "c1" {
		t.Errorf("event[1] = %q/%+v, want content_block_start tool_use get_weather/c1", out[1].Type, out[1].ContentBlock)
	}
	if out[2].Type != "content_block_delta" || out[2].Delta == nil || out[2].Delta.Type != "input_json_delta" {
		t.Fatalf("event[2] = %q/%+v, want content_block_delta input_json_delta", out[2].Type, out[2].Delta)
	}
	if out[2].Delta.PartialJson == nil || *out[2].Delta.PartialJson != `{"city":"NYC"}` {
		t.Errorf("input_json_delta partial = %v, want the raw arguments", out[2].Delta.PartialJson)
	}
}

func TestStreamResponseOpenAI2Claude_FirstChunkReasoning(t *testing.T) {
	rc := "let me think"
	in := &dto.ChatCompletionsStreamResponse{
		Id:    "msg_3",
		Model: "claude-sonnet",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ReasoningContent: &rc}},
		},
	}
	out := StreamResponseOpenAI2Claude(in, claudeStreamInfo(0))
	if len(out) != 3 {
		t.Fatalf("got %d events, want 3 (message_start, content_block_start thinking, thinking_delta)", len(out))
	}
	if out[1].Type != "content_block_start" || out[1].ContentBlock == nil || out[1].ContentBlock.Type != "thinking" {
		t.Errorf("event[1] = %q/%+v, want content_block_start thinking", out[1].Type, out[1].ContentBlock)
	}
	if out[2].Type != "content_block_delta" || out[2].Delta == nil || out[2].Delta.Type != "thinking_delta" {
		t.Errorf("event[2] = %q/%+v, want content_block_delta thinking_delta", out[2].Type, out[2].Delta)
	}
}

func TestStreamResponseOpenAI2Claude_FirstChunkWithFinishEmitsStopSequence(t *testing.T) {
	stop := "stop"
	in := &dto.ChatCompletionsStreamResponse{
		Id:      "msg_4",
		Model:   "claude-sonnet",
		Choices: []dto.ChatCompletionsStreamResponseChoice{streamChoice("Hi", &stop)},
		Usage:   &dto.Usage{PromptTokens: 9, CompletionTokens: 2},
	}
	info := claudeStreamInfo(0)
	out := StreamResponseOpenAI2Claude(in, info)
	// message_start, content_block_start, content_block_delta, content_block_stop,
	// message_delta (usage + stop_reason), message_stop
	if len(out) != 6 {
		t.Fatalf("got %d events, want 6 (full first-chunk-with-finish sequence): %+v", len(out), eventTypes(out))
	}
	if out[3].Type != "content_block_stop" {
		t.Errorf("event[3] = %q, want content_block_stop", out[3].Type)
	}
	if out[4].Type != "message_delta" || out[4].Delta == nil ||
		out[4].Delta.StopReason == nil || *out[4].Delta.StopReason != "end_turn" {
		t.Errorf("event[4] = %q/%+v, want message_delta stop_reason end_turn", out[4].Type, out[4].Delta)
	}
	if out[4].Usage == nil || out[4].Usage.InputTokens != 9 || out[4].Usage.OutputTokens != 2 {
		t.Errorf("message_delta usage = %+v, want input=9 output=2", out[4].Usage)
	}
	if out[5].Type != "message_stop" {
		t.Errorf("event[5] = %q, want message_stop", out[5].Type)
	}
	if !info.ClaudeConvertInfo.Done {
		t.Error("ClaudeConvertInfo.Done = false, want true after a terminal first chunk")
	}
}

func eventTypes(rs []*dto.ClaudeResponse) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Type
	}
	return out
}

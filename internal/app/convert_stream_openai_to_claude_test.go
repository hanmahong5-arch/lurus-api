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

func assertEventTypes(t *testing.T, out []*dto.ClaudeResponse, want []string) {
	t.Helper()
	got := eventTypes(out)
	if len(got) != len(want) {
		t.Fatalf("got %d events %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q (full got %v / want %v)", i, got[i], want[i], got, want)
		}
	}
}

// feedClaudeStream drives a full OpenAI->Claude stream conversion across several
// chunks, mimicking the handler's per-chunk dispatch. The first chunk is the
// "first response" (SendResponseCount==1, triggers message_start); the rest go
// through the subsequent-chunk state machine.
func feedClaudeStream(info *relaycommon.RelayInfo, chunks ...*dto.ChatCompletionsStreamResponse) []*dto.ClaudeResponse {
	var all []*dto.ClaudeResponse
	for i, ch := range chunks {
		info.SendResponseCount = i + 1
		all = append(all, StreamResponseOpenAI2Claude(ch, info)...)
	}
	return all
}

func chunk(choices ...dto.ChatCompletionsStreamResponseChoice) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{Id: "msg", Model: "claude-sonnet", Choices: choices}
}

// Text streamed over two delta chunks then a terminal chunk: the second text
// delta must NOT re-open a content block (LastMessagesType already text).
func TestStreamResponseOpenAI2Claude_TextContinuationNoReopen(t *testing.T) {
	stop := "stop"
	info := claudeStreamInfo(5)
	done := chunk(streamChoice("", &stop))
	done.Usage = &dto.Usage{PromptTokens: 5, CompletionTokens: 4}
	out := feedClaudeStream(info,
		chunk(streamChoice("Hello", nil)),
		chunk(streamChoice(" world", nil)),
		done,
	)
	assertEventTypes(t, out, []string{
		"message_start", "content_block_start", "content_block_delta", // chunk 1
		"content_block_delta", // chunk 2 — continuation, no new content_block_start
		"content_block_stop", "message_delta", "message_stop", // chunk 3 — terminal
	})
	if out[3].Delta == nil || out[3].Delta.GetText() != " world" {
		t.Errorf("continuation delta = %+v, want text_delta ' world'", out[3].Delta)
	}
	md := out[5]
	if md.Delta == nil || md.Delta.StopReason == nil || *md.Delta.StopReason != "end_turn" {
		t.Errorf("message_delta stop_reason = %+v, want end_turn", md.Delta)
	}
	if md.Usage == nil || md.Usage.InputTokens != 5 || md.Usage.OutputTokens != 4 {
		t.Errorf("message_delta usage = %+v, want input=5 output=4", md.Usage)
	}
	if !info.ClaudeConvertInfo.Done {
		t.Error("Done = false after terminal chunk, want true")
	}
	// A further chunk after Done short-circuits to nil.
	if extra := StreamResponseOpenAI2Claude(chunk(streamChoice("late", nil)), info); extra != nil {
		t.Errorf("post-Done chunk -> %+v, want nil", extra)
	}
}

// Switching from a text block to a tool_use block must close the text block
// (content_block_stop) and bump the index before opening the tool block.
func TestStreamResponseOpenAI2Claude_TextThenToolClosesTextBlock(t *testing.T) {
	info := claudeStreamInfo(0)
	out := feedClaudeStream(info,
		chunk(streamChoice("Let me check", nil)),
		chunk(streamToolChoice("get_weather", `{"city":"NYC"}`)),
	)
	assertEventTypes(t, out, []string{
		"message_start", "content_block_start", "content_block_delta", // text
		"content_block_stop",  // close the text block
		"content_block_start", // open tool_use
		"content_block_delta", // input_json_delta
	})
	if out[4].ContentBlock == nil || out[4].ContentBlock.Type != "tool_use" || out[4].ContentBlock.Name != "get_weather" {
		t.Errorf("tool block = %+v, want tool_use get_weather", out[4].ContentBlock)
	}
	// Text block is index 0; the tool block is opened at the bumped index 1.
	if out[3].Index == nil || *out[3].Index != 0 {
		t.Errorf("content_block_stop index = %v, want 0", out[3].Index)
	}
	if out[4].Index == nil || *out[4].Index != 1 {
		t.Errorf("tool content_block_start index = %v, want 1 (bumped)", out[4].Index)
	}
}

// Switching from a thinking block to a text block also closes the prior block
// and bumps the index.
func TestStreamResponseOpenAI2Claude_ThinkingThenTextClosesThinkingBlock(t *testing.T) {
	rc := "pondering"
	info := claudeStreamInfo(0)
	out := feedClaudeStream(info,
		chunk(dto.ChatCompletionsStreamResponseChoice{Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ReasoningContent: &rc}}),
		chunk(streamChoice("Answer", nil)),
	)
	assertEventTypes(t, out, []string{
		"message_start", "content_block_start", "content_block_delta", // thinking
		"content_block_stop",  // close thinking
		"content_block_start", // open text
		"content_block_delta", // text_delta
	})
	if out[4].ContentBlock == nil || out[4].ContentBlock.Type != "text" {
		t.Errorf("event[4] block = %+v, want text", out[4].ContentBlock)
	}
	if out[5].Delta == nil || out[5].Delta.Type != "text_delta" || out[5].Delta.GetText() != "Answer" {
		t.Errorf("event[5] delta = %+v, want text_delta 'Answer'", out[5].Delta)
	}
}

// An empty mid-stream delta (no content, no reasoning, no finish) yields no
// events — it must not emit an empty content_block_delta.
func TestStreamResponseOpenAI2Claude_EmptyMidStreamDeltaIsDropped(t *testing.T) {
	info := claudeStreamInfo(0)
	out := feedClaudeStream(info,
		chunk(streamChoice("Hi", nil)),
		chunk(streamChoice("", nil)), // empty, not terminal
	)
	assertEventTypes(t, out, []string{
		"message_start", "content_block_start", "content_block_delta", // chunk 1 only
	})
	if info.ClaudeConvertInfo.Done {
		t.Error("Done = true after a non-terminal empty chunk, want false")
	}
}

// Tool arguments stream as fragments: a first chunk with the tool name (no args
// yet), then chunks carrying only argument fragments (no name) which must emit
// input_json_delta without re-opening the block, then a terminal tool_calls
// chunk mapping to stop_reason tool_use.
func TestStreamResponseOpenAI2Claude_ToolArgsStreamedInFragments(t *testing.T) {
	toolCalls := "tool_calls"
	info := claudeStreamInfo(0)
	frag := func(name, args string) dto.ChatCompletionsStreamResponseChoice {
		return dto.ChatCompletionsStreamResponseChoice{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{
					{ID: "c1", Index: intPtr(0), Function: dto.FunctionResponse{Name: name, Arguments: args}},
				},
			},
		}
	}
	terminal := chunk(streamChoice("", &toolCalls))
	terminal.Usage = &dto.Usage{PromptTokens: 6, CompletionTokens: 9}
	out := feedClaudeStream(info,
		chunk(streamToolChoice("search", "")), // first chunk: name only, no args
		chunk(frag("", `{"q":`)),              // arg fragment, no name -> no new block
		chunk(frag("", `"cats"}`)),            // arg fragment continuation
		terminal,                              // finish
	)
	assertEventTypes(t, out, []string{
		"message_start", "content_block_start", // chunk 1: tool_use opened, no args delta
		"content_block_delta", // chunk 2: input_json_delta
		"content_block_delta", // chunk 3: input_json_delta
		"content_block_stop", "message_delta", "message_stop", // terminal
	})
	if out[2].Delta == nil || out[2].Delta.Type != "input_json_delta" || out[2].Delta.PartialJson == nil || *out[2].Delta.PartialJson != `{"q":` {
		t.Errorf("event[2] = %+v, want input_json_delta '{\"q\":'", out[2].Delta)
	}
	md := out[5]
	if md.Delta == nil || md.Delta.StopReason == nil || *md.Delta.StopReason != "tool_use" {
		t.Errorf("terminal stop_reason = %+v, want tool_use", md.Delta)
	}
}

// Multiple tool calls in a single subsequent chunk WITHOUT explicit indices:
// the block index is derived as base+i so each tool gets its own block.
func TestStreamResponseOpenAI2Claude_MultipleToolCallsNoIndex(t *testing.T) {
	info := claudeStreamInfo(0)
	multi := dto.ChatCompletionsStreamResponseChoice{
		Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
			ToolCalls: []dto.ToolCallResponse{
				{ID: "a", Function: dto.FunctionResponse{Name: "f0", Arguments: `{}`}},
				{ID: "b", Function: dto.FunctionResponse{Name: "f1", Arguments: `{}`}},
			},
		},
	}
	out := feedClaudeStream(info,
		chunk(streamToolChoice("f_first", `{}`)), // first-chunk tool -> LastMessagesType=Tools
		chunk(multi),
	)
	assertEventTypes(t, out, []string{
		"message_start", "content_block_start", "content_block_delta", // chunk 1
		"content_block_start", "content_block_delta", // chunk 2 tool 0
		"content_block_start", "content_block_delta", // chunk 2 tool 1
	})
	if out[3].Index == nil || out[5].Index == nil || *out[3].Index == *out[5].Index {
		t.Errorf("two tool blocks share index (%v, %v), want distinct base+i", out[3].Index, out[5].Index)
	}
}

func intPtr(i int) *int { return &i }

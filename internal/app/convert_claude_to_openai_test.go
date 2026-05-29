package app

import (
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

// Tests for ClaudeToOpenAIRequest — the request-side conversion behind the
// /v1/messages (RelayFormatClaude) relay path. Previously 0% covered. These
// are characterization + edge-case tests: they pin the current contract so a
// future refactor can't silently change how the headline Anthropic-native
// endpoint maps onto OpenAI-format upstreams.

func sp(s string) *string { return &s }

func nonOpenRouterInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: "claude-3-5-sonnet",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-4o",
		},
	}
}

func TestClaudeToOpenAIRequest_BasicStringMessage(t *testing.T) {
	temp := 0.7
	req := dto.ClaudeRequest{
		Model:       "claude-3-5-sonnet",
		MaxTokens:   100,
		Temperature: &temp,
		TopP:        0.9,
		Stream:      true,
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: "hello"},
		},
	}

	out, err := ClaudeToOpenAIRequest(req, nonOpenRouterInfo())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Model != "claude-3-5-sonnet" {
		t.Errorf("Model = %q, want claude-3-5-sonnet", out.Model)
	}
	if out.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", out.MaxTokens)
	}
	if out.Temperature == nil || *out.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", out.Temperature)
	}
	if out.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", out.TopP)
	}
	if !out.Stream {
		t.Error("Stream = false, want true")
	}
	if len(out.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(out.Messages))
	}
	if out.Messages[0].Role != "user" {
		t.Errorf("Messages[0].Role = %q, want user", out.Messages[0].Role)
	}
	if got := out.Messages[0].StringContent(); got != "hello" {
		t.Errorf("Messages[0] content = %q, want hello", got)
	}
}

func TestClaudeToOpenAIRequest_SystemStringPrepended(t *testing.T) {
	req := dto.ClaudeRequest{
		Model:    "claude-3-5-sonnet",
		System:   "You are helpful.",
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	}

	out, err := ClaudeToOpenAIRequest(req, nonOpenRouterInfo())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2 (system + user)", len(out.Messages))
	}
	if out.Messages[0].Role != "system" {
		t.Errorf("Messages[0].Role = %q, want system", out.Messages[0].Role)
	}
	if got := out.Messages[0].StringContent(); got != "You are helpful." {
		t.Errorf("system content = %q, want 'You are helpful.'", got)
	}
}

func TestClaudeToOpenAIRequest_StopSequences(t *testing.T) {
	t.Run("none -> Stop nil", func(t *testing.T) {
		out, _ := ClaudeToOpenAIRequest(dto.ClaudeRequest{
			Messages: []dto.ClaudeMessage{{Role: "user", Content: "x"}},
		}, nonOpenRouterInfo())
		if out.Stop != nil {
			t.Errorf("Stop = %v, want nil", out.Stop)
		}
	})

	t.Run("one -> string", func(t *testing.T) {
		out, _ := ClaudeToOpenAIRequest(dto.ClaudeRequest{
			StopSequences: []string{"END"},
			Messages:      []dto.ClaudeMessage{{Role: "user", Content: "x"}},
		}, nonOpenRouterInfo())
		s, ok := out.Stop.(string)
		if !ok || s != "END" {
			t.Errorf("Stop = %#v, want string \"END\"", out.Stop)
		}
	})

	t.Run("many -> slice", func(t *testing.T) {
		out, _ := ClaudeToOpenAIRequest(dto.ClaudeRequest{
			StopSequences: []string{"A", "B"},
			Messages:      []dto.ClaudeMessage{{Role: "user", Content: "x"}},
		}, nonOpenRouterInfo())
		arr, ok := out.Stop.([]string)
		if !ok || len(arr) != 2 || arr[0] != "A" || arr[1] != "B" {
			t.Errorf("Stop = %#v, want []string{A,B}", out.Stop)
		}
	})
}

func TestClaudeToOpenAIRequest_ToolsConverted(t *testing.T) {
	req := dto.ClaudeRequest{
		Model: "claude-3-5-sonnet",
		Tools: []dto.Tool{
			{
				Name:        "get_weather",
				Description: "Get the weather",
				InputSchema: map[string]interface{}{"type": "object"},
			},
		},
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "weather?"}},
	}

	out, err := ClaudeToOpenAIRequest(req, nonOpenRouterInfo())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(out.Tools))
	}
	tool := out.Tools[0]
	if tool.Type != "function" {
		t.Errorf("tool.Type = %q, want function", tool.Type)
	}
	if tool.Function.Name != "get_weather" {
		t.Errorf("tool name = %q, want get_weather", tool.Function.Name)
	}
	if tool.Function.Description != "Get the weather" {
		t.Errorf("tool description = %q, want 'Get the weather'", tool.Function.Description)
	}
}

func TestClaudeToOpenAIRequest_MultimodalTextAndImage(t *testing.T) {
	req := dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{
				Role: "user",
				Content: []dto.ClaudeMediaMessage{
					{Type: "text", Text: sp("describe this")},
					{Type: "image", Source: &dto.ClaudeMessageSource{
						Type: "base64", MediaType: "image/png", Data: "QUJD",
					}},
				},
			},
		},
	}

	out, err := ClaudeToOpenAIRequest(req, nonOpenRouterInfo())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(out.Messages))
	}
	parts := out.Messages[0].ParseContent()
	if len(parts) != 2 {
		t.Fatalf("len(content parts) = %d, want 2", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "describe this" {
		t.Errorf("part[0] = {%q,%q}, want {text, describe this}", parts[0].Type, parts[0].Text)
	}
	if parts[1].Type != "image_url" {
		t.Fatalf("part[1].Type = %q, want image_url", parts[1].Type)
	}
	if img := parts[1].GetImageMedia(); img == nil || img.Url != "data:image/png;base64,QUJD" {
		t.Errorf("image url = %v, want data:image/png;base64,QUJD", img)
	}
}

func TestClaudeToOpenAIRequest_ToolResultBecomesToolMessage(t *testing.T) {
	req := dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{
				Role: "user",
				Content: []dto.ClaudeMediaMessage{
					{Type: "tool_result", ToolUseId: "toolu_1", Content: "sunny, 25C"},
				},
			},
		},
	}

	out, err := ClaudeToOpenAIRequest(req, nonOpenRouterInfo())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The tool_result is emitted as a standalone role:"tool" message.
	var toolMsg *dto.Message
	for i := range out.Messages {
		if out.Messages[i].Role == "tool" {
			toolMsg = &out.Messages[i]
			break
		}
	}
	if toolMsg == nil {
		t.Fatalf("no role:tool message produced; messages=%d", len(out.Messages))
	}
	if toolMsg.ToolCallId != "toolu_1" {
		t.Errorf("ToolCallId = %q, want toolu_1", toolMsg.ToolCallId)
	}
	if got := toolMsg.StringContent(); got != "sunny, 25C" {
		t.Errorf("tool content = %q, want 'sunny, 25C'", got)
	}
}

// TestClaudeToOpenAIRequest_TextWithToolUse_DropsText pins a known fidelity
// limitation (flagged with a NOTE at convert.go's media/tool branch): when a
// single assistant message carries BOTH a text block and a tool_use block, the
// current conversion keeps only the tool call and DROPS the text. OpenAI's
// format does allow an assistant message with both content and tool_calls, so
// this loses the model's stated intent across a tool round-trip. This test
// documents (does not endorse) the behavior; changing it would alter the
// upstream request shape and must be validated against the target providers
// before flipping.
func TestClaudeToOpenAIRequest_TextWithToolUse_DropsText(t *testing.T) {
	req := dto.ClaudeRequest{
		Messages: []dto.ClaudeMessage{
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{Type: "text", Text: sp("Let me check the weather.")},
					{Type: "tool_use", Id: "toolu_1", Name: "get_weather",
						Input: map[string]interface{}{"city": "NYC"}},
				},
			},
		},
	}

	out, err := ClaudeToOpenAIRequest(req, nonOpenRouterInfo())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(out.Messages))
	}
	msg := out.Messages[0]
	calls := msg.ParseToolCalls()
	if len(calls) != 1 || calls[0].Function.Name != "get_weather" {
		t.Fatalf("tool calls = %#v, want one get_weather call", calls)
	}
	// Current (limitation) behavior: the "Let me check the weather." text is gone.
	if got := msg.StringContent(); got != "" {
		t.Logf("NOTE: assistant text now preserved alongside tool_use (%q) — "+
			"if this is an intentional fix, update this test to assert it", got)
		t.Errorf("expected text dropped (current behavior) but got %q; "+
			"behavior changed — re-validate against upstream providers", got)
	}
}

func TestClaudeToOpenAIRequest_ThinkingAddsModelSuffix(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "claude-3-7-sonnet-thinking",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			UpstreamModelName: "gpt-4o",
		},
	}
	req := dto.ClaudeRequest{
		Model:    "gpt-4o",
		Thinking: &dto.Thinking{Type: "enabled"},
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "x"}},
	}

	out, err := ClaudeToOpenAIRequest(req, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Model != "gpt-4o-thinking" {
		t.Errorf("Model = %q, want gpt-4o-thinking (suffix appended when OriginModelName ends in -thinking)", out.Model)
	}
}

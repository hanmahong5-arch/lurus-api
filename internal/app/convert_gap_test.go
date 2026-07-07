package app

// convert_gap_test.go — closes the non-OpenRouter array-system concatenation
// arm of ClaudeToOpenAIRequest: multiple system text blocks are flattened into a
// single string system message (as opposed to the structured-media path taken
// for OpenRouter-Claude upstreams).

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

func TestClaudeToOpenAIRequest_NonOpenRouterArraySystemConcat(t *testing.T) {
	req := dto.ClaudeRequest{
		Model: "claude-3-5-sonnet",
		System: []dto.ClaudeMediaMessage{
			{Type: "text", Text: sp("You are")},
			{Type: "text", Text: sp(" careful and helpful.")},
		},
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hi"}},
	}

	out, err := ClaudeToOpenAIRequest(req, nonOpenRouterInfo())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Messages) < 2 {
		t.Fatalf("len(Messages) = %d, want >=2 (system + user)", len(out.Messages))
	}
	if out.Messages[0].Role != "system" {
		t.Errorf("Messages[0].Role = %q, want system", out.Messages[0].Role)
	}
	// Non-OpenRouter path flattens the array into one concatenated string.
	if got := out.Messages[0].StringContent(); got != "You are careful and helpful." {
		t.Errorf("system content = %q, want the concatenated string", got)
	}
}

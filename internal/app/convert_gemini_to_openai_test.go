package app

import (
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

// Tests for GeminiToOpenAIRequest — request-side conversion behind the
// /v1beta/models/* (RelayFormatGemini) relay path. Previously 0% covered.
// Includes the regression for the StopSequences slice-bounds panic.

func geminiInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		IsStream:   false,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4o"},
	}
}

func TestGeminiToOpenAIRequest_BasicTextMessage(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{{Text: "hello"}}},
		},
	}

	out, err := GeminiToOpenAIRequest(req, geminiInfo())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o (from UpstreamModelName)", out.Model)
	}
	if len(out.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(out.Messages))
	}
	if out.Messages[0].Role != "user" {
		t.Errorf("Role = %q, want user", out.Messages[0].Role)
	}
	if got := out.Messages[0].StringContent(); got != "hello" {
		t.Errorf("content = %q, want hello (single text part -> string content)", got)
	}
}

func TestGeminiToOpenAIRequest_RoleMapped(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "model", Parts: []dto.GeminiPart{{Text: "I am the model"}}},
		},
	}
	out, _ := GeminiToOpenAIRequest(req, geminiInfo())
	if len(out.Messages) != 1 || out.Messages[0].Role != "assistant" {
		t.Errorf("role mapping: got %+v, want one message role=assistant", out.Messages)
	}
}

func TestGeminiToOpenAIRequest_GenerationConfig(t *testing.T) {
	temp := 0.5
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "x"}}}},
		GenerationConfig: dto.GeminiChatGenerationConfig{
			Temperature:     &temp,
			TopP:            0.8,
			TopK:            40,
			MaxOutputTokens: 256,
			CandidateCount:  2,
		},
	}
	out, _ := GeminiToOpenAIRequest(req, geminiInfo())
	if out.Temperature == nil || *out.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want 0.5", out.Temperature)
	}
	if out.TopP != 0.8 {
		t.Errorf("TopP = %v, want 0.8", out.TopP)
	}
	if out.TopK != 40 {
		t.Errorf("TopK = %d, want 40", out.TopK)
	}
	if out.MaxTokens != 256 {
		t.Errorf("MaxTokens = %d, want 256", out.MaxTokens)
	}
	if out.N != 2 {
		t.Errorf("N = %d, want 2", out.N)
	}
}

// TestGeminiToOpenAIRequest_StopSequences is the regression for a slice-bounds
// panic: the conversion did `StopSequences[:4]` whenever any stop sequence was
// present, which panics ("slice bounds out of range [:4]") for the common case
// of 1-3 sequences — crashing the relay handler on a valid request. Now it
// truncates only when there are more than 4.
func TestGeminiToOpenAIRequest_StopSequences(t *testing.T) {
	cases := []struct {
		name    string
		stops   []string
		wantLen int
	}{
		{"one", []string{"A"}, 1},
		{"three", []string{"A", "B", "C"}, 3},
		{"exactly four", []string{"A", "B", "C", "D"}, 4},
		{"five truncated to four", []string{"A", "B", "C", "D", "E"}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &dto.GeminiChatRequest{
				Contents:         []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "x"}}}},
				GenerationConfig: dto.GeminiChatGenerationConfig{StopSequences: tc.stops},
			}
			// Must not panic.
			out, err := GeminiToOpenAIRequest(req, geminiInfo())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			stops, ok := out.Stop.([]string)
			if !ok {
				t.Fatalf("Stop type = %T, want []string", out.Stop)
			}
			if len(stops) != tc.wantLen {
				t.Errorf("len(Stop) = %d, want %d", len(stops), tc.wantLen)
			}
		})
	}
}

func TestGeminiToOpenAIRequest_NoStopSequencesLeavesStopNil(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "x"}}}},
	}
	out, _ := GeminiToOpenAIRequest(req, geminiInfo())
	if out.Stop != nil {
		t.Errorf("Stop = %#v, want nil when no stop sequences", out.Stop)
	}
}

func TestGeminiToOpenAIRequest_SystemInstructionsPrepended(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
		SystemInstructions: &dto.GeminiChatContent{
			Parts: []dto.GeminiPart{{Text: "You are terse."}},
		},
	}
	out, _ := GeminiToOpenAIRequest(req, geminiInfo())
	if len(out.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2 (system + user)", len(out.Messages))
	}
	if out.Messages[0].Role != "system" || out.Messages[0].StringContent() != "You are terse." {
		t.Errorf("system message = {%q,%q}, want {system, You are terse.}",
			out.Messages[0].Role, out.Messages[0].StringContent())
	}
}

func TestGeminiToOpenAIRequest_InlineImage(t *testing.T) {
	req := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{Role: "user", Parts: []dto.GeminiPart{
				{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: "QUJD"}},
			}},
		},
	}
	out, _ := GeminiToOpenAIRequest(req, geminiInfo())
	if len(out.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want 1", len(out.Messages))
	}
	parts := out.Messages[0].ParseContent()
	if len(parts) != 1 || parts[0].Type != "image_url" {
		t.Fatalf("parts = %+v, want one image_url", parts)
	}
	if img := parts[0].GetImageMedia(); img == nil || img.Url != "data:image/png;base64,QUJD" {
		t.Errorf("image url = %v, want data:image/png;base64,QUJD", img)
	}
}

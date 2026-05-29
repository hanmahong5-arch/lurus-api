package app

import (
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
)

// Tests for ResponseOpenAI2Gemini — response-side conversion for the
// /v1beta/models/* (RelayFormatGemini) relay path. Previously 0% covered.
// Completes the Gemini request->response round-trip under regression test.
// (textChoice/toolChoice helpers live in convert_response_openai_to_claude_test.go.)

func TestResponseOpenAI2Gemini_TextResponse(t *testing.T) {
	in := &dto.OpenAITextResponse{
		Id:      "r1",
		Model:   "gpt-4o",
		Choices: []dto.OpenAITextResponseChoice{textChoice("Hi there", "stop")},
		Usage:   dto.Usage{PromptTokens: 7, CompletionTokens: 3},
	}

	out := ResponseOpenAI2Gemini(in, nonOpenRouterInfo())
	if len(out.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1", len(out.Candidates))
	}
	cand := out.Candidates[0]
	if cand.Content.Role != "model" {
		t.Errorf("Content.Role = %q, want model", cand.Content.Role)
	}
	if len(cand.Content.Parts) != 1 || cand.Content.Parts[0].Text != "Hi there" {
		t.Errorf("parts = %+v, want one text 'Hi there'", cand.Content.Parts)
	}
	if cand.FinishReason == nil || *cand.FinishReason != "STOP" {
		t.Errorf("FinishReason = %v, want STOP", cand.FinishReason)
	}
	if out.UsageMetadata.PromptTokenCount != 7 ||
		out.UsageMetadata.CandidatesTokenCount != 3 ||
		out.UsageMetadata.TotalTokenCount != 10 {
		t.Errorf("usage = %+v, want prompt=7 cand=3 total=10", out.UsageMetadata)
	}
}

func TestResponseOpenAI2Gemini_FinishReasonMapping(t *testing.T) {
	cases := map[string]string{
		"stop":           "STOP",
		"length":         "MAX_TOKENS",
		"content_filter": "SAFETY",
		"tool_calls":     "STOP",
		"weird":          "STOP", // default
	}
	for openaiReason, want := range cases {
		t.Run(openaiReason, func(t *testing.T) {
			in := &dto.OpenAITextResponse{
				Choices: []dto.OpenAITextResponseChoice{textChoice("x", openaiReason)},
			}
			out := ResponseOpenAI2Gemini(in, nonOpenRouterInfo())
			if len(out.Candidates) != 1 {
				t.Fatalf("len(Candidates) = %d, want 1", len(out.Candidates))
			}
			if got := out.Candidates[0].FinishReason; got == nil || *got != want {
				t.Errorf("FinishReason for %q = %v, want %q", openaiReason, got, want)
			}
		})
	}
}

func TestResponseOpenAI2Gemini_ToolCallArgs(t *testing.T) {
	t.Run("valid json -> parsed map", func(t *testing.T) {
		in := &dto.OpenAITextResponse{
			Choices: []dto.OpenAITextResponseChoice{toolChoice("c1", "get_weather", `{"city":"NYC"}`)},
		}
		out := ResponseOpenAI2Gemini(in, nonOpenRouterInfo())
		fc := out.Candidates[0].Content.Parts[0].FunctionCall
		if fc == nil || fc.FunctionName != "get_weather" {
			t.Fatalf("FunctionCall = %+v, want name get_weather", fc)
		}
		args, ok := fc.Arguments.(map[string]interface{})
		if !ok || args["city"] != "NYC" {
			t.Errorf("Arguments = %#v, want map{city:NYC}", fc.Arguments)
		}
	})

	t.Run("invalid json -> wrapped under arguments key", func(t *testing.T) {
		in := &dto.OpenAITextResponse{
			Choices: []dto.OpenAITextResponseChoice{toolChoice("c2", "do_thing", "not-json")},
		}
		out := ResponseOpenAI2Gemini(in, nonOpenRouterInfo())
		fc := out.Candidates[0].Content.Parts[0].FunctionCall
		args, ok := fc.Arguments.(map[string]interface{})
		if !ok || args["arguments"] != "not-json" {
			t.Errorf("Arguments = %#v, want map{arguments:not-json} fallback", fc.Arguments)
		}
	})

	t.Run("empty args -> empty map", func(t *testing.T) {
		in := &dto.OpenAITextResponse{
			Choices: []dto.OpenAITextResponseChoice{toolChoice("c3", "noargs", "")},
		}
		out := ResponseOpenAI2Gemini(in, nonOpenRouterInfo())
		fc := out.Candidates[0].Content.Parts[0].FunctionCall
		args, ok := fc.Arguments.(map[string]interface{})
		if !ok || len(args) != 0 {
			t.Errorf("Arguments = %#v, want empty map", fc.Arguments)
		}
	})
}

func TestResponseOpenAI2Gemini_EmptyTextProducesNoParts(t *testing.T) {
	in := &dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{textChoice("", "stop")},
	}
	out := ResponseOpenAI2Gemini(in, nonOpenRouterInfo())
	if len(out.Candidates) != 1 {
		t.Fatalf("len(Candidates) = %d, want 1", len(out.Candidates))
	}
	if len(out.Candidates[0].Content.Parts) != 0 {
		t.Errorf("parts = %+v, want none for empty text", out.Candidates[0].Content.Parts)
	}
}

func TestResponseOpenAI2Gemini_MultipleCandidates(t *testing.T) {
	in := &dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{
			textChoice("a", "stop"),
			textChoice("b", "stop"),
		},
	}
	out := ResponseOpenAI2Gemini(in, nonOpenRouterInfo())
	if len(out.Candidates) != 2 {
		t.Fatalf("len(Candidates) = %d, want 2", len(out.Candidates))
	}
	if out.Candidates[0].Content.Parts[0].Text != "a" || out.Candidates[1].Content.Parts[0].Text != "b" {
		t.Errorf("candidate texts = [%q,%q], want [a,b]",
			out.Candidates[0].Content.Parts[0].Text, out.Candidates[1].Content.Parts[0].Text)
	}
}

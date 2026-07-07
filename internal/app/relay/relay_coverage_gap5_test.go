package relay

import (
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	relayconstant "github.com/LurusTech/lurus-hub/internal/adapter/provider/constant"
	"github.com/LurusTech/lurus-hub/internal/app"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"

	"github.com/gin-gonic/gin"
)

// TestHandlers_DoRequestConnectionError drives each relay helper against an
// unreachable upstream (a closed port) so adaptor.DoRequest returns a transport
// error BEFORE any response is received. This exercises the DoRequestFailed
// error branch of every helper — distinct from the non-2xx branch, which the
// 500-upstream test already covers. Each helper must surface a non-nil
// NewAPIError rather than a panic or hollow success.
func TestHandlers_DoRequestConnectionError(t *testing.T) {
	app.InitHttpClient()

	// Port 1 is reserved and never listening → immediate connection refused.
	const deadURL = "http://127.0.0.1:1"

	textReq := &dto.GeneralOpenAIRequest{Model: "gpt-4o-mini", Messages: []dto.Message{{Role: "user", Content: "hi"}}}
	claudeReq := &dto.ClaudeRequest{Model: "claude-3-5-sonnet", MaxTokens: 16, Messages: []dto.ClaudeMessage{{Role: "user", Content: []byte(`[{"type":"text","text":"hi"}]`)}}}
	geminiReq := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Parts: []dto.GeminiPart{{Text: "hi"}}}}}
	respReq := &dto.OpenAIResponsesRequest{Model: "gpt-4o-mini", Input: []byte(`"hi"`)}
	imgReq := &dto.ImageRequest{Model: "dall-e-3", Prompt: "a cat", N: 1}
	embReq := &dto.EmbeddingRequest{Model: "text-embedding-3-small", Input: "hello"}
	rerankReq := &dto.RerankRequest{Model: "rerank-1", Query: "q", Documents: []any{"d1", "d2"}}
	audioReq := &dto.AudioRequest{Model: "tts-1", Input: "hello world", Voice: "alloy"}

	tests := []struct {
		name  string
		path  string
		mode  int
		model string
		req   dto.Request
		call  func(*gin.Context, *relaycommon.RelayInfo) *types.NewAPIError
	}{
		{"Text", "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "gpt-4o-mini", textReq, TextHelper},
		{"Claude", "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "claude-3-5-sonnet", claudeReq, ClaudeHelper},
		{"Gemini", "/v1/chat/completions", relayconstant.RelayModeChatCompletions, "gemini-2.5-flash", geminiReq, GeminiHelper},
		{"Responses", "/v1/responses", relayconstant.RelayModeResponses, "gpt-4o-mini", respReq, ResponsesHelper},
		{"Image", "/v1/images/generations", relayconstant.RelayModeImagesGenerations, "dall-e-3", imgReq, ImageHelper},
		{"Embedding", "/v1/embeddings", relayconstant.RelayModeEmbeddings, "text-embedding-3-small", embReq, EmbeddingHelper},
		{"Rerank", "/v1/rerank", relayconstant.RelayModeRerank, "rerank-1", rerankReq, RerankHelper},
		{"Audio", "/v1/audio/speech", relayconstant.RelayModeAudioSpeech, "tts-1", audioReq, AudioHelper},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, info := wireOpenAIUpstream(t, deadURL, tc.path, tc.mode, tc.model, tc.req)
			c.Set("token_name", "tkn")
			err := tc.call(c, info)
			if err == nil {
				t.Fatalf("%s: expected DoRequest connection error, got nil", tc.name)
			}
		})
	}
}

package helper

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/pkg/constant"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"

	"github.com/gin-gonic/gin"
)

func init() {
	// The request-body guard reads a max size (MB) that defaults to 0 outside a
	// booted server; give it a sane limit so body reads succeed under test.
	if constant.MaxRequestBodyMB <= 0 {
		constant.MaxRequestBodyMB = 50
	}
}

// newJSONCtx builds a gin.Context whose request carries a JSON body, matching
// the way relay handlers hand requests to the validators.
func newJSONCtx(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c, w
}

func TestGetAndValidateTextRequest(t *testing.T) {
	t.Run("valid chat completion", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/chat/completions", `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)
		req, err := GetAndValidateTextRequest(c, 1 /*RelayModeChatCompletions*/)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if req.Model != "gpt-4" {
			t.Errorf("model = %q, want gpt-4", req.Model)
		}
	})

	t.Run("chat completion missing messages rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/chat/completions", `{"model":"gpt-4"}`)
		_, err := GetAndValidateTextRequest(c, 1)
		if err == nil || !strings.Contains(err.Error(), "messages is required") {
			t.Fatalf("expected messages-required error, got %v", err)
		}
	})

	t.Run("missing model rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/chat/completions", `{"messages":[{"role":"user","content":"hi"}]}`)
		_, err := GetAndValidateTextRequest(c, 1)
		if err == nil || !strings.Contains(err.Error(), "model is required") {
			t.Fatalf("expected model-required error, got %v", err)
		}
	})

	t.Run("max_tokens over limit rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/chat/completions", `{"model":"gpt-4","max_tokens":2000000000,"messages":[{"role":"user","content":"hi"}]}`)
		_, err := GetAndValidateTextRequest(c, 1)
		if err == nil || !strings.Contains(err.Error(), "max_tokens is invalid") {
			t.Fatalf("expected max_tokens error, got %v", err)
		}
	})

	t.Run("completions missing prompt rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/completions", `{"model":"gpt-4","prompt":""}`)
		_, err := GetAndValidateTextRequest(c, 2 /*RelayModeCompletions*/)
		if err == nil || !strings.Contains(err.Error(), "prompt is required") {
			t.Fatalf("expected prompt-required error, got %v", err)
		}
	})

	t.Run("completions with prompt ok", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/completions", `{"model":"gpt-4","prompt":"once"}`)
		if _, err := GetAndValidateTextRequest(c, 2); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("moderations default model applied", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/moderations", `{"input":"text"}`)
		req, err := GetAndValidateTextRequest(c, 4 /*RelayModeModerations*/)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if req.Model != "text-moderation-latest" {
			t.Errorf("default model = %q, want text-moderation-latest", req.Model)
		}
	})

	t.Run("moderations missing input rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/moderations", `{"model":"m"}`)
		_, err := GetAndValidateTextRequest(c, 4)
		if err == nil || !strings.Contains(err.Error(), "input is required") {
			t.Fatalf("expected input-required error, got %v", err)
		}
	})

	t.Run("edits missing instruction rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/edits", `{"model":"m"}`)
		_, err := GetAndValidateTextRequest(c, 7 /*RelayModeEdits*/)
		if err == nil || !strings.Contains(err.Error(), "instruction is required") {
			t.Fatalf("expected instruction-required error, got %v", err)
		}
	})

	t.Run("invalid web_search_options rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/chat/completions", `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"web_search_options":{"search_context_size":"huge"}}`)
		_, err := GetAndValidateTextRequest(c, 1)
		if err == nil || !strings.Contains(err.Error(), "search_context_size") {
			t.Fatalf("expected search_context_size error, got %v", err)
		}
	})

	t.Run("empty web_search_options size defaulted to medium", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/chat/completions", `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"web_search_options":{}}`)
		req, err := GetAndValidateTextRequest(c, 1)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if req.WebSearchOptions.SearchContextSize != "medium" {
			t.Errorf("default size = %q, want medium", req.WebSearchOptions.SearchContextSize)
		}
	})

	t.Run("malformed json rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/chat/completions", `{not json`)
		if _, err := GetAndValidateTextRequest(c, 1); err == nil {
			t.Fatal("expected json parse error")
		}
	})
}

func TestGetAndValidateClaudeRequest(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/messages", `{"model":"claude-3","messages":[{"role":"user","content":"hi"}]}`)
		req, err := GetAndValidateClaudeRequest(c)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if req.Model != "claude-3" {
			t.Errorf("model = %q", req.Model)
		}
	})

	t.Run("missing messages rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/messages", `{"model":"claude-3"}`)
		_, err := GetAndValidateClaudeRequest(c)
		if err == nil || !strings.Contains(err.Error(), "messages is required") {
			t.Fatalf("expected messages error, got %v", err)
		}
	})

	t.Run("missing model rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/messages", `{"messages":[{"role":"user","content":"hi"}]}`)
		_, err := GetAndValidateClaudeRequest(c)
		if err == nil || !strings.Contains(err.Error(), "model is required") {
			t.Fatalf("expected model error, got %v", err)
		}
	})

	t.Run("bad json rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/messages", `nope`)
		if _, err := GetAndValidateClaudeRequest(c); err == nil {
			t.Fatal("expected json error")
		}
	})
}

func TestGetAndValidateGeminiRequest(t *testing.T) {
	t.Run("valid contents", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1beta/models/gemini-pro:generateContent", `{"contents":[{"parts":[{"text":"hi"}]}]}`)
		if _, err := GetAndValidateGeminiRequest(c); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("empty contents rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1beta/models/gemini-pro:generateContent", `{}`)
		_, err := GetAndValidateGeminiRequest(c)
		if err == nil || !strings.Contains(err.Error(), "contents is required") {
			t.Fatalf("expected contents error, got %v", err)
		}
	})

	t.Run("bad json rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1beta/models/x:generateContent", `bad`)
		if _, err := GetAndValidateGeminiRequest(c); err == nil {
			t.Fatal("expected json error")
		}
	})

	t.Run("embedding parses", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1beta/models/text-embedding:embedContent", `{"content":{"parts":[{"text":"hi"}]}}`)
		if _, err := GetAndValidateGeminiEmbeddingRequest(c); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("batch embedding parses", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1beta/models/text-embedding:batchEmbedContents", `{"requests":[]}`)
		if _, err := GetAndValidateGeminiBatchEmbeddingRequest(c); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})
}

func TestGetAndValidateRerankRequest(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/rerank", `{"model":"r","query":"q","documents":["a","b"]}`)
		if _, err := GetAndValidateRerankRequest(c); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("empty query rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/rerank", `{"model":"r","documents":["a"]}`)
		_, err := GetAndValidateRerankRequest(c)
		if err == nil || !strings.Contains(err.Error(), "query is empty") {
			t.Fatalf("expected query error, got %v", err)
		}
	})

	t.Run("empty documents rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/rerank", `{"model":"r","query":"q"}`)
		_, err := GetAndValidateRerankRequest(c)
		if err == nil || !strings.Contains(err.Error(), "documents is empty") {
			t.Fatalf("expected documents error, got %v", err)
		}
	})

	t.Run("bad json rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/rerank", `bad`)
		if _, err := GetAndValidateRerankRequest(c); err == nil {
			t.Fatal("expected json error")
		}
	})
}

func TestGetAndValidateEmbeddingRequest(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/embeddings", `{"model":"e","input":"hello"}`)
		if _, err := GetAndValidateEmbeddingRequest(c, 3 /*RelayModeEmbeddings*/); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("empty input rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/embeddings", `{"model":"e"}`)
		_, err := GetAndValidateEmbeddingRequest(c, 3)
		if err == nil || !strings.Contains(err.Error(), "input is empty") {
			t.Fatalf("expected input error, got %v", err)
		}
	})

	t.Run("moderations default model", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/moderations", `{"input":"x"}`)
		req, err := GetAndValidateEmbeddingRequest(c, 4 /*RelayModeModerations*/)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if req.Model != "omni-moderation-latest" {
			t.Errorf("default model = %q", req.Model)
		}
	})

	t.Run("embeddings model from path param", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/embeddings", `{"input":"x"}`)
		c.Params = gin.Params{{Key: "model", Value: "text-embed-3"}}
		req, err := GetAndValidateEmbeddingRequest(c, 3)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if req.Model != "text-embed-3" {
			t.Errorf("param model = %q", req.Model)
		}
	})
}

func TestGetAndValidateResponsesRequest(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/responses", `{"model":"gpt-4","input":"hi"}`)
		if _, err := GetAndValidateResponsesRequest(c); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("missing model rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/responses", `{"input":"hi"}`)
		_, err := GetAndValidateResponsesRequest(c)
		if err == nil || !strings.Contains(err.Error(), "model is required") {
			t.Fatalf("expected model error, got %v", err)
		}
	})

	t.Run("missing input rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/responses", `{"model":"gpt-4"}`)
		_, err := GetAndValidateResponsesRequest(c)
		if err == nil || !strings.Contains(err.Error(), "input is required") {
			t.Fatalf("expected input error, got %v", err)
		}
	})

	t.Run("bad json rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/responses", `bad`)
		if _, err := GetAndValidateResponsesRequest(c); err == nil {
			t.Fatal("expected json error")
		}
	})
}

func TestGetAndValidAudioRequest(t *testing.T) {
	t.Run("speech valid", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/audio/speech", `{"model":"tts-1","input":"hi"}`)
		if _, err := GetAndValidAudioRequest(c, 25 /*RelayModeAudioSpeech*/); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("speech missing model rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/audio/speech", `{"input":"hi"}`)
		_, err := GetAndValidAudioRequest(c, 25)
		if err == nil || !strings.Contains(err.Error(), "model is required") {
			t.Fatalf("expected model error, got %v", err)
		}
	})

	t.Run("transcription default response format applied", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/audio/transcriptions", `{"model":"whisper-1"}`)
		req, err := GetAndValidAudioRequest(c, 26 /*RelayModeAudioTranscription*/)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if req.ResponseFormat != "json" {
			t.Errorf("response format = %q, want json", req.ResponseFormat)
		}
	})

	t.Run("transcription missing model rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/audio/transcriptions", `{}`)
		_, err := GetAndValidAudioRequest(c, 26)
		if err == nil || !strings.Contains(err.Error(), "model is required") {
			t.Fatalf("expected model error, got %v", err)
		}
	})
}

func TestGetAndValidOpenAIImageRequest(t *testing.T) {
	t.Run("dall-e-3 defaults", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/images/generations", `{"model":"dall-e-3","prompt":"a cat"}`)
		req, err := GetAndValidOpenAIImageRequest(c, 5 /*RelayModeImagesGenerations*/)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if req.Size != "1024x1024" {
			t.Errorf("size default = %q", req.Size)
		}
		if req.Quality != "standard" {
			t.Errorf("quality default = %q", req.Quality)
		}
		if req.N != 1 {
			t.Errorf("N default = %d", req.N)
		}
	})

	t.Run("missing model rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/images/generations", `{"prompt":"a cat"}`)
		_, err := GetAndValidOpenAIImageRequest(c, 5)
		if err == nil || !strings.Contains(err.Error(), "model is required") {
			t.Fatalf("expected model error, got %v", err)
		}
	})

	t.Run("multiplication sign in size rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/images/generations", `{"model":"dall-e-3","size":"1024×1024"}`)
		_, err := GetAndValidOpenAIImageRequest(c, 5)
		if err == nil || !strings.Contains(err.Error(), "multiplication sign") {
			t.Fatalf("expected multiplication-sign error, got %v", err)
		}
	})

	t.Run("dall-e-2 bad size rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/images/generations", `{"model":"dall-e-2","size":"999x999"}`)
		_, err := GetAndValidOpenAIImageRequest(c, 5)
		if err == nil || !strings.Contains(err.Error(), "256x256") {
			t.Fatalf("expected dall-e-2 size error, got %v", err)
		}
	})

	t.Run("dall-e-3 bad size rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/images/generations", `{"model":"dall-e-3","size":"800x800"}`)
		_, err := GetAndValidOpenAIImageRequest(c, 5)
		if err == nil || !strings.Contains(err.Error(), "1024x1024") {
			t.Fatalf("expected dall-e-3 size error, got %v", err)
		}
	})

	t.Run("gpt-image-1 quality defaulted", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/images/generations", `{"model":"gpt-image-1"}`)
		req, err := GetAndValidOpenAIImageRequest(c, 5)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if req.Quality != "auto" {
			t.Errorf("quality = %q, want auto", req.Quality)
		}
	})
}

func TestGetAndValidateRequestDispatch(t *testing.T) {
	tests := []struct {
		name    string
		format  types.RelayFormat
		path    string
		body    string
		wantErr bool
		model   string
	}{
		{"openai", types.RelayFormatOpenAI, "/v1/chat/completions", `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`, false, "gpt-4"},
		{"claude", types.RelayFormatClaude, "/v1/messages", `{"model":"claude-3","messages":[{"role":"user","content":"hi"}]}`, false, "claude-3"},
		{"gemini", types.RelayFormatGemini, "/v1beta/models/gemini-pro:generateContent", `{"contents":[{"parts":[{"text":"hi"}]}]}`, false, ""},
		{"gemini embed", types.RelayFormatGemini, "/v1beta/models/text:embedContent", `{"content":{"parts":[{"text":"x"}]}}`, false, ""},
		{"gemini batch", types.RelayFormatGemini, "/v1beta/models/text:batchEmbedContents", `{"requests":[]}`, false, ""},
		{"responses", types.RelayFormatOpenAIResponses, "/v1/responses", `{"model":"gpt-4","input":"hi"}`, false, "gpt-4"},
		{"image", types.RelayFormatOpenAIImage, "/v1/images/generations", `{"model":"dall-e-3","prompt":"a"}`, false, "dall-e-3"},
		{"embedding", types.RelayFormatEmbedding, "/v1/embeddings", `{"model":"e","input":"x"}`, false, "e"},
		{"rerank", types.RelayFormatRerank, "/v1/rerank", `{"model":"r","query":"q","documents":["a"]}`, false, "r"},
		{"audio", types.RelayFormatOpenAIAudio, "/v1/audio/speech", `{"model":"tts-1","input":"hi"}`, false, "tts-1"},
		{"openai reject", types.RelayFormatOpenAI, "/v1/chat/completions", `{"model":"gpt-4"}`, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newJSONCtx("POST", tt.path, tt.body)
			req, err := GetAndValidateRequest(c, tt.format)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if req == nil {
				t.Fatal("expected non-nil request")
			}
		})
	}
}

func TestGetAndValidateRequestRealtimeAndUnsupported(t *testing.T) {
	t.Run("realtime returns base request", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/v1/realtime", ``)
		req, err := GetAndValidateRequest(c, types.RelayFormatOpenAIRealtime)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if _, ok := req.(*dto.BaseRequest); !ok {
			t.Fatalf("expected *dto.BaseRequest, got %T", req)
		}
	})

	t.Run("unsupported format rejected", func(t *testing.T) {
		c, _ := newJSONCtx("POST", "/x", ``)
		_, err := GetAndValidateRequest(c, types.RelayFormat("bogus-format"))
		if err == nil || !strings.Contains(err.Error(), "unsupported relay format") {
			t.Fatalf("expected unsupported-format error, got %v", err)
		}
	})
}

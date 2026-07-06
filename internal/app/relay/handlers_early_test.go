package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"

	"github.com/gin-gonic/gin"
)

// newHandlerContext returns a gin context with a real request so InitChannelMeta
// and downstream context reads don't nil-deref.
func newHandlerContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c, rec
}

// Each relay helper begins by type-asserting info.Request to the concrete DTO it
// serves. Feeding a mismatched request type must surface an InvalidRequest error
// (skip-retry) rather than proceeding — this is the guard that stops one relay
// format from being fed a foreign request DTO. We assert the error class per
// handler.
func assertInvalidRequest(t *testing.T, name string, err *types.NewAPIError) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected invalid-request error, got nil", name)
	}
}

func TestHandlers_RejectWrongRequestType(t *testing.T) {
	// A single "foreign" request DTO that none of the specific handlers accept.
	foreign := &dto.EmbeddingRequest{Model: "x"}

	tests := []struct {
		name string
		call func(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError
		// req is the deliberately-wrong request type for this handler.
		req dto.Request
	}{
		{"TextHelper", TextHelper, foreign},
		{"RerankHelper", RerankHelper, &dto.ImageRequest{}},
		{"AudioHelper", AudioHelper, foreign},
		{"ImageHelper", ImageHelper, foreign},
		{"ClaudeHelper", ClaudeHelper, foreign},
		{"GeminiHelper", GeminiHelper, foreign},
		{"ResponsesHelper", ResponsesHelper, foreign},
		// EmbeddingHelper expects *dto.EmbeddingRequest, so feed it a non-embedding req.
		{"EmbeddingHelper", EmbeddingHelper, &dto.ImageRequest{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newHandlerContext()
			info := &relaycommon.RelayInfo{Request: tc.req, OriginModelName: "m"}
			err := tc.call(c, info)
			assertInvalidRequest(t, tc.name, err)
			if err.GetErrorCode() != types.ErrorCodeInvalidRequest {
				t.Errorf("%s: error code = %v, want ErrorCodeInvalidRequest", tc.name, err.GetErrorCode())
			}
		})
	}
}

// imagePromptForEvent is the concise-prompt extractor for the NATS image event.
func TestImagePromptForEvent(t *testing.T) {
	if got := imagePromptForEvent(nil); got != "" {
		t.Errorf("nil req => %q, want empty", got)
	}
	if got := imagePromptForEvent(&dto.ImageRequest{Prompt: "a cat"}); got != "a cat" {
		t.Errorf("=> %q, want 'a cat'", got)
	}
}

// isNoThinkingRequest returns true only when an explicit zero thinking budget is
// set — the signal that the caller opted out of extended thinking.
func TestIsNoThinkingRequest(t *testing.T) {
	zero := 0
	nonZero := 512

	req := &dto.GeminiChatRequest{}
	if isNoThinkingRequest(req) {
		t.Errorf("no thinking config => want false")
	}

	req.GenerationConfig.ThinkingConfig = &dto.GeminiThinkingConfig{ThinkingBudget: &zero}
	if !isNoThinkingRequest(req) {
		t.Errorf("budget 0 => want true")
	}

	req.GenerationConfig.ThinkingConfig = &dto.GeminiThinkingConfig{ThinkingBudget: &nonZero}
	if isNoThinkingRequest(req) {
		t.Errorf("budget 512 => want false")
	}
}

func TestTrimModelThinking(t *testing.T) {
	cases := []struct{ in, want string }{
		{"gemini-2.5-flash", "gemini-2.5-flash"},
		{"gemini-2.5-flash-nothinking", "gemini-2.5-flash"},
		{"gemini-2.5-flash-thinking", "gemini-2.5-flash"},
		{"gemini-2.5-flash-thinking-1024", "gemini-2.5-flash-thinking"},
	}
	for _, tc := range cases {
		if got := trimModelThinking(tc.in); got != tc.want {
			t.Errorf("trimModelThinking(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

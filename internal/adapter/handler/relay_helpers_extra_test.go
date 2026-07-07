package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/LurusTech/lurus-hub/internal/adapter/provider/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/dto"
	"github.com/LurusTech/lurus-hub/internal/pkg/types"
	"github.com/gin-gonic/gin"
)

func newTestCtx() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, w
}

// errWithStatus builds a NewAPIError carrying a specific upstream status code.
func errWithStatus(status int) *types.NewAPIError {
	return types.NewErrorWithStatusCode(errors.New("boom"), types.ErrorCodeBadResponseStatusCode, status)
}

func TestShouldRetry_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		err         *types.NewAPIError
		retryTimes  int
		setSpecific bool
		want        bool
	}{
		{name: "nil_error", err: nil, retryTimes: 3, want: false},
		{name: "channel_error_always_retry", err: types.NewError(errors.New("x"), types.ErrorCodeChannelNoAvailableKey), retryTimes: 3, want: true},
		{name: "skip_retry_error", err: types.NewError(errors.New("x"), types.ErrorCodeBadResponseStatusCode, types.ErrOptionWithSkipRetry()), retryTimes: 3, want: false},
		{name: "no_retries_left", err: errWithStatus(500), retryTimes: 0, want: false},
		{name: "specific_channel_pinned", err: errWithStatus(500), retryTimes: 3, setSpecific: true, want: false},
		{name: "too_many_requests_429", err: errWithStatus(http.StatusTooManyRequests), retryTimes: 3, want: true},
		{name: "redirect_307", err: errWithStatus(307), retryTimes: 3, want: true},
		{name: "server_500", err: errWithStatus(500), retryTimes: 3, want: true},
		{name: "gateway_timeout_504_no_retry", err: errWithStatus(504), retryTimes: 3, want: false},
		{name: "cf_524_no_retry", err: errWithStatus(524), retryTimes: 3, want: false},
		{name: "bad_request_400", err: errWithStatus(http.StatusBadRequest), retryTimes: 3, want: false},
		{name: "request_timeout_408", err: errWithStatus(408), retryTimes: 3, want: false},
		{name: "success_2xx", err: errWithStatus(200), retryTimes: 3, want: false},
		{name: "other_4xx_default_retry", err: errWithStatus(http.StatusForbidden), retryTimes: 3, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestCtx()
			if tt.setSpecific {
				c.Set("specific_channel_id", 7)
			}
			got := shouldRetry(c, tt.err, tt.retryTimes)
			if got != tt.want {
				t.Errorf("shouldRetry(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestShouldRetryTaskRelay_TableDriven(t *testing.T) {
	mk := func(status int, local bool) *dto.TaskError {
		return &dto.TaskError{StatusCode: status, LocalError: local, Error: errors.New("boom")}
	}
	tests := []struct {
		name        string
		taskErr     *dto.TaskError
		retryTimes  int
		setSpecific bool
		want        bool
	}{
		{name: "nil_error", taskErr: nil, retryTimes: 3, want: false},
		{name: "no_retries_left", taskErr: mk(500, false), retryTimes: 0, want: false},
		{name: "specific_channel_pinned", taskErr: mk(500, false), retryTimes: 3, setSpecific: true, want: false},
		{name: "too_many_requests_429", taskErr: mk(http.StatusTooManyRequests, false), retryTimes: 3, want: true},
		{name: "redirect_307", taskErr: mk(307, false), retryTimes: 3, want: true},
		{name: "server_500", taskErr: mk(500, false), retryTimes: 3, want: true},
		{name: "gateway_timeout_504_no_retry", taskErr: mk(504, false), retryTimes: 3, want: false},
		{name: "cf_524_no_retry", taskErr: mk(524, false), retryTimes: 3, want: false},
		{name: "bad_request_400", taskErr: mk(http.StatusBadRequest, false), retryTimes: 3, want: false},
		{name: "request_timeout_408", taskErr: mk(408, false), retryTimes: 3, want: false},
		{name: "local_error_no_retry", taskErr: mk(502, true), retryTimes: 3, want: true}, // 502 is 5xx -> retry before LocalError check
		{name: "local_error_generic_no_retry", taskErr: mk(0, true), retryTimes: 3, want: false},
		{name: "success_2xx", taskErr: mk(200, false), retryTimes: 3, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestCtx()
			if tt.setSpecific {
				c.Set("specific_channel_id", 7)
			}
			got := shouldRetryTaskRelay(c, 1, tt.taskErr, tt.retryTimes)
			if got != tt.want {
				t.Errorf("shouldRetryTaskRelay(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetChannel_NoChannelMeta(t *testing.T) {
	t.Run("auto_ban_true", func(t *testing.T) {
		c, _ := newTestCtx()
		c.Set("auto_ban", true)
		c.Set("channel_id", 42)
		c.Set("channel_type", 3)
		c.Set("channel_name", "primary")
		info := &relaycommon.RelayInfo{} // ChannelMeta nil
		ch, apiErr := getChannel(c, info, nil)
		if apiErr != nil {
			t.Fatalf("unexpected error: %v", apiErr)
		}
		if ch.Id != 42 || ch.Type != 3 || ch.Name != "primary" {
			t.Errorf("channel fields not sourced from ctx: %+v", ch)
		}
		if ch.AutoBan == nil || *ch.AutoBan != 1 {
			t.Errorf("expected AutoBan=1, got %v", ch.AutoBan)
		}
	})

	t.Run("auto_ban_false", func(t *testing.T) {
		c, _ := newTestCtx()
		c.Set("auto_ban", false)
		c.Set("channel_id", 9)
		info := &relaycommon.RelayInfo{}
		ch, apiErr := getChannel(c, info, nil)
		if apiErr != nil {
			t.Fatalf("unexpected error: %v", apiErr)
		}
		if ch.AutoBan == nil || *ch.AutoBan != 0 {
			t.Errorf("expected AutoBan=0, got %v", ch.AutoBan)
		}
	})
}

func TestAddUsedChannel(t *testing.T) {
	c, _ := newTestCtx()
	addUsedChannel(c, 1)
	addUsedChannel(c, 2)
	addUsedChannel(c, 3)
	got := c.GetStringSlice("use_channel")
	want := []string{"1", "2", "3"}
	if len(got) != len(want) {
		t.Fatalf("use_channel = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("use_channel[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFastTokenCountMetaForPricing(t *testing.T) {
	t.Run("nil_request", func(t *testing.T) {
		meta := fastTokenCountMetaForPricing(nil)
		if meta == nil || meta.MaxTokens != 0 {
			t.Errorf("expected empty meta, got %+v", meta)
		}
	})

	t.Run("openai_max_completion_tokens_wins", func(t *testing.T) {
		req := &dto.GeneralOpenAIRequest{MaxTokens: 100, MaxCompletionTokens: 250}
		meta := fastTokenCountMetaForPricing(req)
		if meta.MaxTokens != 250 {
			t.Errorf("MaxTokens = %d, want 250", meta.MaxTokens)
		}
		if meta.TokenType != types.TokenTypeTokenizer {
			t.Errorf("TokenType = %q, want tokenizer", meta.TokenType)
		}
	})

	t.Run("openai_max_tokens_wins", func(t *testing.T) {
		req := &dto.GeneralOpenAIRequest{MaxTokens: 400, MaxCompletionTokens: 50}
		meta := fastTokenCountMetaForPricing(req)
		if meta.MaxTokens != 400 {
			t.Errorf("MaxTokens = %d, want 400", meta.MaxTokens)
		}
	})

	t.Run("claude_request", func(t *testing.T) {
		req := &dto.ClaudeRequest{MaxTokens: 777}
		meta := fastTokenCountMetaForPricing(req)
		if meta.MaxTokens != 777 {
			t.Errorf("MaxTokens = %d, want 777", meta.MaxTokens)
		}
	})

	t.Run("responses_request", func(t *testing.T) {
		req := &dto.OpenAIResponsesRequest{MaxOutputTokens: 321}
		meta := fastTokenCountMetaForPricing(req)
		if meta.MaxTokens != 321 {
			t.Errorf("MaxTokens = %d, want 321", meta.MaxTokens)
		}
	})
}

func TestRelayNotImplemented(t *testing.T) {
	c, w := newTestCtx()
	RelayNotImplemented(c)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", w.Code)
	}
	if !containsSub(w.Body.String(), "api_not_implemented") {
		t.Errorf("body missing api_not_implemented: %s", w.Body.String())
	}
}

func TestRelayNotFound(t *testing.T) {
	c, w := newTestCtx()
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/nonexistent", nil)
	RelayNotFound(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	if !containsSub(w.Body.String(), "/v1/nonexistent") {
		t.Errorf("body should echo the invalid URL: %s", w.Body.String())
	}
}

func containsSub(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

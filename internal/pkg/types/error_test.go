package types

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestIsUpstreamFailure(t *testing.T) {
	tests := []struct {
		name string
		err  *NewAPIError
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{
			name: "channel:* code (network/key issue) → upstream",
			err:  &NewAPIError{errorCode: ErrorCodeChannelNoAvailableKey, StatusCode: 503},
			want: true,
		},
		{
			name: "client cancellation → not upstream",
			err:  &NewAPIError{Err: context.Canceled, StatusCode: 0},
			want: false,
		},
		{
			name: "wrapped client cancellation → not upstream",
			err:  &NewAPIError{Err: fmt.Errorf("wrap: %w", context.Canceled), StatusCode: 0},
			want: false,
		},
		{name: "500 → upstream", err: &NewAPIError{StatusCode: 500}, want: true},
		{name: "502 → upstream", err: &NewAPIError{StatusCode: 502}, want: true},
		{name: "503 → upstream", err: &NewAPIError{StatusCode: 503}, want: true},
		{name: "504 (gateway timeout) → upstream", err: &NewAPIError{StatusCode: 504}, want: true},
		{name: "524 (CF origin timeout) → upstream", err: &NewAPIError{StatusCode: 524}, want: true},
		{name: "408 (request timeout) → upstream", err: &NewAPIError{StatusCode: 408}, want: true},
		{name: "400 (bad request) → user error", err: &NewAPIError{StatusCode: 400}, want: false},
		{name: "401 (unauthorized) → user error", err: &NewAPIError{StatusCode: 401}, want: false},
		{name: "403 (forbidden) → user error", err: &NewAPIError{StatusCode: 403}, want: false},
		{name: "404 (not found) → user error", err: &NewAPIError{StatusCode: 404}, want: false},
		{name: "429 (rate limit) → user error", err: &NewAPIError{StatusCode: 429}, want: false},
		{
			name: "no status, no cancellation (raw network err) → fail-safe upstream",
			err:  &NewAPIError{Err: errors.New("dial tcp: timeout"), StatusCode: 0},
			want: true,
		},
		{
			name: "channel:* takes precedence over 4xx-shaped status",
			err:  &NewAPIError{errorCode: ErrorCodeChannelInvalidKey, StatusCode: 401},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUpstreamFailure(tt.err); got != tt.want {
				t.Errorf("IsUpstreamFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}

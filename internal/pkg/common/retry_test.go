package common

import (
	"errors"
	"testing"
	"time"
)

// fastConfig keeps backoff sleeps negligible so the retry tests stay -short-safe.
func fastConfig(maxAttempts int) RetryConfig {
	return RetryConfig{
		MaxAttempts: maxAttempts,
		BaseDelay:   time.Millisecond,
		MaxDelay:    2 * time.Millisecond,
	}
}

func TestRetryConnect_SuccessFirst(t *testing.T) {
	calls := 0
	err := RetryConnect("t", fastConfig(5), func(error) bool { return true }, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected fn called once, got %d", calls)
	}
}

func TestRetryConnect_SuccessAfterN(t *testing.T) {
	calls := 0
	// Fail twice, succeed on the 3rd attempt. nil shouldRetry == always retry.
	err := RetryConnect("t", fastConfig(5), nil, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil after retries, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
}

func TestRetryConnect_ExhaustWrapsLastError(t *testing.T) {
	sentinel := errors.New("still down")
	calls := 0
	err := RetryConnect("postgres SQL_DSN", fastConfig(4), nil, func() error {
		calls++
		return sentinel
	})
	if err == nil {
		t.Fatal("expected error on exhaustion")
	}
	if calls != 4 {
		t.Fatalf("expected 4 attempts, got %d", calls)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
}

func TestRetryConnect_FailFastOnNonRetryable(t *testing.T) {
	sentinel := errors.New("bad config")
	calls := 0
	err := RetryConnect("t", fastConfig(5), func(error) bool { return false }, func() error {
		calls++
		return sentinel
	})
	if err == nil {
		t.Fatal("expected error")
	}
	// shouldRetry=false must stop after the FIRST attempt, not burn the budget.
	if calls != 1 {
		t.Fatalf("expected fail-fast after 1 attempt, got %d", calls)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
}

func TestBackoffDelay_CappedBounded(t *testing.T) {
	baseDelay := time.Second
	maxDelay := 10 * time.Second
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 10 * time.Second},   // 16s would exceed maxDelay → capped
		{100, 10 * time.Second}, // overflow-safe: never exceeds maxDelay, never wraps negative
	}
	for _, tc := range cases {
		if got := backoffDelay(baseDelay, maxDelay, tc.attempt); got != tc.want {
			t.Errorf("backoffDelay(attempt=%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestRetryConnect_ZeroConfigNormalized(t *testing.T) {
	// A zero RetryConfig must normalize to exactly 1 attempt (never 0, never an
	// infinite loop) and must not sleep — so this returns immediately.
	calls := 0
	start := time.Now()
	err := RetryConnect("t", RetryConfig{}, nil, func() error {
		calls++
		return errors.New("down")
	})
	if err == nil {
		t.Fatal("expected error from single normalized attempt")
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 attempt from zero config, got %d", calls)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("zero-config single attempt should not sleep, took %v", elapsed)
	}
}

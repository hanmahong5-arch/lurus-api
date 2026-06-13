package common

import (
	"fmt"
	"time"
)

// RetryConfig parameterizes RetryConnect's bounded exponential backoff.
// A zero/negative field is normalized to a safe default (see RetryConnect) so a
// partially-filled config can never produce a zero-attempt or zero-delay loop.
type RetryConfig struct {
	MaxAttempts int           // total attempts including the first; <1 normalized to 1
	BaseDelay   time.Duration // sleep before the 2nd attempt; <=0 normalized to 1s
	MaxDelay    time.Duration // ceiling for any single backoff sleep; <=0 normalized to BaseDelay
}

// RetryConnect runs fn with capped exponential backoff, returning nil on the
// first success. It is the boot-time connect guard for transient dependency
// unavailability — e.g. a DB/Redis pod that is not yet Ready when this pod
// starts. A single gorm.Open/ping that fails on a cold cluster otherwise
// crashes the process and burns a K8s restart; bounded retry lets the cluster
// self-heal instead.
//
// shouldRetry gates fail-fast: when it returns false for an error, RetryConnect
// stops immediately and wraps that error — a permanent/config error must not be
// retried into the full budget. On exhaustion the last error is wrapped with
// the attempt count. Each failed attempt is logged via SysLog.
//
// This helper NEVER calls FatalLog: the terminal decision stays at the call
// site (InitDB/InitLogDB/InitRedisClient keep their FatalLog after exhaustion).
// The budget is bounded, so a genuinely-misconfigured dependency only delays
// that inevitable terminal error by the worst-case backoff sum.
//
// Shape mirrors internal/pkg/search.RetryWithBackoff, generalized with a
// configurable budget + fail-fast predicate. It lives in common (not search)
// because common must NOT import search — search already imports common, so the
// reverse would cycle — and both repo and redis are common consumers.
func RetryConnect(what string, cfg RetryConfig, shouldRetry func(error) bool, fn func() error) error {
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = cfg.BaseDelay
	}

	var err error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		err = fn()
		if err == nil {
			if attempt > 1 {
				SysLog(fmt.Sprintf("%s: connected on attempt %d/%d", what, attempt, cfg.MaxAttempts))
			}
			return nil
		}
		if shouldRetry != nil && !shouldRetry(err) {
			return fmt.Errorf("%s: non-retryable error on attempt %d/%d: %w", what, attempt, cfg.MaxAttempts, err)
		}
		if attempt == cfg.MaxAttempts {
			break
		}
		delay := backoffDelay(cfg.BaseDelay, cfg.MaxDelay, attempt)
		SysLog(fmt.Sprintf("%s: attempt %d/%d failed, retrying in %v: %v", what, attempt, cfg.MaxAttempts, delay, err))
		time.Sleep(delay)
	}
	return fmt.Errorf("%s: giving up after %d attempts: %w", what, cfg.MaxAttempts, err)
}

// backoffDelay returns the capped exponential backoff to sleep AFTER a 1-based
// attempt fails: baseDelay * 2^(attempt-1), clamped to maxDelay. The doubling
// loop is overflow-safe — once a double reaches or exceeds maxDelay (or wraps
// non-positive on an absurd attempt count) it returns maxDelay immediately.
func backoffDelay(baseDelay, maxDelay time.Duration, attempt int) time.Duration {
	delay := baseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= maxDelay || delay <= 0 {
			return maxDelay
		}
	}
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

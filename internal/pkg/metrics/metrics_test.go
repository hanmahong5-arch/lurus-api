package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordRelayRequest(t *testing.T) {
	// Record a successful request
	RecordRelayRequest("openai", "gpt-4", "success", 0.5)

	// Verify counter incremented
	count := testutil.ToFloat64(RelayRequestsTotal.WithLabelValues("openai", "gpt-4", "success"))
	if count != 1 {
		t.Errorf("expected count 1, got %f", count)
	}

	// Record an error request
	RecordRelayRequest("openai", "gpt-4", "error", 1.0)
	errorCount := testutil.ToFloat64(RelayRequestsTotal.WithLabelValues("openai", "gpt-4", "error"))
	if errorCount != 1 {
		t.Errorf("expected error count 1, got %f", errorCount)
	}
}

func TestRecordTokens(t *testing.T) {
	RecordTokens("claude", "claude-3", 100, 50)

	inputCount := testutil.ToFloat64(TokensProcessed.WithLabelValues("claude", "claude-3", "input"))
	if inputCount != 100 {
		t.Errorf("expected input tokens 100, got %f", inputCount)
	}

	outputCount := testutil.ToFloat64(TokensProcessed.WithLabelValues("claude", "claude-3", "output"))
	if outputCount != 50 {
		t.Errorf("expected output tokens 50, got %f", outputCount)
	}
}

func TestRecordQuotaConsumed(t *testing.T) {
	RecordQuotaConsumed("tenant1", "user1", 1000)

	quota := testutil.ToFloat64(QuotaConsumed.WithLabelValues("tenant1", "user1"))
	if quota != 1000 {
		t.Errorf("expected quota 1000, got %f", quota)
	}
}

func TestActiveConnections(t *testing.T) {
	initial := testutil.ToFloat64(ActiveConnections)

	ActiveConnections.Inc()
	after := testutil.ToFloat64(ActiveConnections)
	if after != initial+1 {
		t.Errorf("expected %f, got %f", initial+1, after)
	}

	ActiveConnections.Dec()
	final := testutil.ToFloat64(ActiveConnections)
	if final != initial {
		t.Errorf("expected %f, got %f", initial, final)
	}
}

func TestRecordChannelError(t *testing.T) {
	channelID := "ch-test-1"
	channelName := "test-channel"
	provider := "openai"

	// Reset to known state
	ResetChannelErrors(channelID, channelName, provider)

	// Record multiple errors
	RecordChannelError(channelID, channelName, provider, "timeout")
	RecordChannelError(channelID, channelName, provider, "timeout")
	RecordChannelError(channelID, channelName, provider, "rate_limit")

	// Verify consecutive errors count
	consecutive := testutil.ToFloat64(ChannelConsecutiveErrors.WithLabelValues(channelID, channelName, provider))
	if consecutive != 3 {
		t.Errorf("expected consecutive errors 3, got %f", consecutive)
	}

	// Verify total errors
	timeoutErrors := testutil.ToFloat64(ChannelErrorsTotal.WithLabelValues(channelID, channelName, provider, "timeout"))
	if timeoutErrors != 2 {
		t.Errorf("expected timeout errors 2, got %f", timeoutErrors)
	}

	rateLimitErrors := testutil.ToFloat64(ChannelErrorsTotal.WithLabelValues(channelID, channelName, provider, "rate_limit"))
	if rateLimitErrors != 1 {
		t.Errorf("expected rate_limit errors 1, got %f", rateLimitErrors)
	}
}

func TestResetChannelErrors(t *testing.T) {
	channelID := "ch-test-2"
	channelName := "test-channel-2"
	provider := "claude"

	// Add some errors
	RecordChannelError(channelID, channelName, provider, "connection")
	RecordChannelError(channelID, channelName, provider, "connection")

	// Reset
	ResetChannelErrors(channelID, channelName, provider)

	// Verify consecutive errors is 0
	consecutive := testutil.ToFloat64(ChannelConsecutiveErrors.WithLabelValues(channelID, channelName, provider))
	if consecutive != 0 {
		t.Errorf("expected consecutive errors 0 after reset, got %f", consecutive)
	}
}

func TestSetChannelHealth(t *testing.T) {
	channelID := "ch-health-1"
	channelName := "health-test"
	provider := "gemini"

	// Set healthy
	SetChannelHealth(channelID, channelName, provider, true)
	health := testutil.ToFloat64(ChannelHealth.WithLabelValues(channelID, channelName, provider))
	if health != 1 {
		t.Errorf("expected health 1 for healthy, got %f", health)
	}

	// Set unhealthy
	SetChannelHealth(channelID, channelName, provider, false)
	health = testutil.ToFloat64(ChannelHealth.WithLabelValues(channelID, channelName, provider))
	if health != 0 {
		t.Errorf("expected health 0 for unhealthy, got %f", health)
	}
}

// TestPoolExhaustedRejections verifies that RecordPoolExhaustedRejection increments
// the counter with the expected label combination. Simulates the pool_balance_check
// middleware rejecting a relay request for an exhausted pool.
func TestPoolExhaustedRejections(t *testing.T) {
	tenantID := "tenant-pool-test-1"
	poolKind := "relay"

	before := testutil.ToFloat64(PoolExhaustedRejections.WithLabelValues(tenantID, poolKind))

	RecordPoolExhaustedRejection(tenantID, poolKind)
	RecordPoolExhaustedRejection(tenantID, poolKind)

	after := testutil.ToFloat64(PoolExhaustedRejections.WithLabelValues(tenantID, poolKind))
	if after-before != 2 {
		t.Errorf("expected 2 increments, got delta %f", after-before)
	}

	// Different tenant — must be independent
	otherTenant := "tenant-pool-test-2"
	RecordPoolExhaustedRejection(otherTenant, poolKind)
	otherCount := testutil.ToFloat64(PoolExhaustedRejections.WithLabelValues(otherTenant, poolKind))
	if otherCount != 1 {
		t.Errorf("expected 1 for other tenant, got %f", otherCount)
	}
	// Ensure first tenant unchanged
	if testutil.ToFloat64(PoolExhaustedRejections.WithLabelValues(tenantID, poolKind)) != after {
		t.Errorf("first tenant count changed unexpectedly")
	}
}

// TestBillingDebitAmountCNY verifies that RecordBillingDebit populates the histogram.
// HistogramVec.WithLabelValues returns Observer (not Collector), so we test via
// testutil.CollectAndCount on the parent vec which counts metric families produced.
func TestBillingDebitAmountCNY(t *testing.T) {
	tenantID := "tenant-billing-test-3" // unique label to avoid cross-test interference

	// Before any observation the vec may or may not expose this label set.
	// Prime the label set so the next CollectAndCount reflects our observations.
	RecordBillingDebit(tenantID, 0.05)
	RecordBillingDebit(tenantID, 1.50)
	RecordBillingDebit(tenantID, 10.0)

	// The HistogramVec itself is a Collector; CollectAndCount counts the number of
	// unique metric time series it emits. After three observations the vec must have
	// at least one time series with this tenant label.
	count := testutil.CollectAndCount(BillingDebitAmountCNY)
	if count == 0 {
		t.Errorf("expected at least one metric series from BillingDebitAmountCNY, got 0")
	}

	// Cross-tenant isolation: a second tenant gets its own time series.
	otherTenant := "tenant-billing-test-4"
	RecordBillingDebit(otherTenant, 0.01)

	countAfter := testutil.CollectAndCount(BillingDebitAmountCNY)
	if countAfter <= count {
		t.Errorf("expected a new series after recording other tenant, count %d -> %d", count, countAfter)
	}
}

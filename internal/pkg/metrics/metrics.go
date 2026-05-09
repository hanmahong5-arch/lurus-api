// Package metrics provides Prometheus metrics for the API gateway.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	namespace = "lurus"
	subsystem = "gateway"
)

var (
	// RequestsTotal counts total requests by method, path, and status
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// RequestDuration measures request latency in seconds
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// RelayRequestsTotal counts relay requests by provider and model
	RelayRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "relay_requests_total",
			Help:      "Total number of relay requests to upstream providers",
		},
		[]string{"provider", "model", "status"},
	)

	// RelayDuration measures upstream API latency
	RelayDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "relay_duration_seconds",
			Help:      "Upstream provider API latency in seconds",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"provider", "model"},
	)

	// ChannelSelectDuration measures channel selection latency
	ChannelSelectDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "channel_select_duration_seconds",
			Help:      "Channel selection latency in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25},
		},
	)

	// TokensProcessed counts tokens processed
	TokensProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "tokens_processed_total",
			Help:      "Total tokens processed (input + output)",
		},
		[]string{"provider", "model", "type"}, // type: input, output
	)

	// QuotaConsumed tracks quota consumption
	QuotaConsumed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "quota_consumed_total",
			Help:      "Total quota consumed",
		},
		[]string{"tenant_id", "user_id"},
	)

	// RetryAttempts counts retry attempts
	RetryAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "retry_attempts_total",
			Help:      "Total retry attempts",
		},
		[]string{"provider", "reason"},
	)

	// ActiveConnections tracks current active connections
	ActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "active_connections",
			Help:      "Number of active connections",
		},
	)

	// ChannelHealth tracks channel availability
	ChannelHealth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "channel_health",
			Help:      "Channel health status (1=healthy, 0=unhealthy)",
		},
		[]string{"channel_id", "channel_name", "provider"},
	)

	// CacheHits tracks cache hit/miss ratio
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "cache_hits_total",
			Help:      "Cache hit/miss counts",
		},
		[]string{"cache_type", "result"}, // result: hit, miss
	)

	// ChannelConsecutiveErrors tracks consecutive errors per channel
	// Reset to 0 on successful request, incremented on each error
	ChannelConsecutiveErrors = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "channel_consecutive_errors",
			Help:      "Consecutive error count per channel (resets on success)",
		},
		[]string{"channel_id", "channel_name", "provider"},
	)

	// ChannelErrorsTotal tracks total channel errors
	ChannelErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "channel_errors_total",
			Help:      "Total errors per channel",
		},
		[]string{"channel_id", "channel_name", "provider", "error_type"},
	)

	// CircuitBreakerState tracks per-channel breaker state (0=closed, 1=open, 2=half_open)
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "circuit_breaker_state",
			Help:      "Circuit breaker state per channel (0=closed, 1=open, 2=half_open)",
		},
		[]string{"channel_id"},
	)

	// CircuitBreakerTrips counts how many times each breaker has been tripped open
	CircuitBreakerTrips = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "circuit_breaker_trips_total",
			Help:      "Total times a channel circuit breaker tripped to Open",
		},
		[]string{"channel_id"},
	)

	// CircuitBreakerRejections counts requests rejected by open breakers
	CircuitBreakerRejections = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "circuit_breaker_rejections_total",
			Help:      "Total requests rejected by open circuit breaker",
		},
		[]string{"channel_id"},
	)

	// RelayOverheadDuration measures newhub-side latency before the first
	// upstream attempt (requestStart → first channel_select). SLO target:
	// how much delay newhub itself adds. Excludes upstream wall time.
	RelayOverheadDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "relay_overhead_duration_seconds",
			Help:      "Newhub overhead before first upstream call (excludes upstream wall time)",
			Buckets:   []float64{.0005, .001, .002, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	// RelayTotalDuration measures end-to-end Relay() handler duration including
	// retries and upstream wall time. Labeled per provider/model so SLOs can be
	// sliced by which upstream is degrading.
	RelayTotalDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "relay_total_duration_seconds",
			Help:      "End-to-end Relay() handler duration including retries and upstream wall time",
			Buckets:   []float64{.05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"provider", "model", "status"},
	)
)

// RecordRelayRequest records a relay request with its outcome
func RecordRelayRequest(provider, model, status string, durationSec float64) {
	RelayRequestsTotal.WithLabelValues(provider, model, status).Inc()
	RelayDuration.WithLabelValues(provider, model).Observe(durationSec)
}

// RecordTokens records token usage
func RecordTokens(provider, model string, inputTokens, outputTokens int) {
	if inputTokens > 0 {
		TokensProcessed.WithLabelValues(provider, model, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		TokensProcessed.WithLabelValues(provider, model, "output").Add(float64(outputTokens))
	}
}

// RecordQuotaConsumed records quota consumption
func RecordQuotaConsumed(tenantID, userID string, quota int64) {
	QuotaConsumed.WithLabelValues(tenantID, userID).Add(float64(quota))
}

// RecordChannelError increments consecutive error count for a channel
func RecordChannelError(channelID, channelName, provider, errorType string) {
	ChannelConsecutiveErrors.WithLabelValues(channelID, channelName, provider).Inc()
	ChannelErrorsTotal.WithLabelValues(channelID, channelName, provider, errorType).Inc()
}

// ResetChannelErrors resets consecutive error count on successful request
func ResetChannelErrors(channelID, channelName, provider string) {
	ChannelConsecutiveErrors.WithLabelValues(channelID, channelName, provider).Set(0)
}

// SetChannelHealth sets channel health status (1=healthy, 0=unhealthy)
func SetChannelHealth(channelID, channelName, provider string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	ChannelHealth.WithLabelValues(channelID, channelName, provider).Set(value)
}

// RecordCircuitBreakerState sets the breaker state gauge for a channel.
func RecordCircuitBreakerState(channelID string, state int) {
	CircuitBreakerState.WithLabelValues(channelID).Set(float64(state))
}

// RecordCircuitBreakerTrip increments the trip counter.
func RecordCircuitBreakerTrip(channelID string) {
	CircuitBreakerTrips.WithLabelValues(channelID).Inc()
}

// RecordCircuitBreakerRejection increments the rejection counter.
func RecordCircuitBreakerRejection(channelID string) {
	CircuitBreakerRejections.WithLabelValues(channelID).Inc()
}

// RecordRelayOverhead records newhub-side overhead before first upstream call.
// SLO target: P99 < 50ms.
func RecordRelayOverhead(durationSec float64) {
	RelayOverheadDuration.Observe(durationSec)
}

// RecordRelayTotal records end-to-end Relay() handler duration.
// status must be "success" or "error".
func RecordRelayTotal(provider, model, status string, durationSec float64) {
	RelayTotalDuration.WithLabelValues(provider, model, status).Observe(durationSec)
}

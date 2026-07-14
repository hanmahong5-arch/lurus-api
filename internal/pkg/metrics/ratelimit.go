package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RateLimitedTotal counts relay requests rejected by the business rate
// limiter (middleware.BusinessRateLimit). Labels:
//
//	scope — which bucket rejected the request: "token" (per-token window)
//	        or "tenant" (aggregate window across the tenant's tokens)
//	type  — which limit tripped: "rpm" today; "tpm" reserved for the
//	        tokens-per-minute limit (columns exist since migration 023,
//	        enforcement pending — see middleware/business_rate_limit.go).
//
// Deliberately named without the lurus_gateway prefix so dashboards can
// address it as newhub_rate_limited_total alongside the newhub alert rules.
var RateLimitedTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "newhub",
		Name:      "rate_limited_total",
		Help:      "Relay requests rejected by the business rate limiter, by scope (token|tenant) and type (rpm|tpm)",
	},
	[]string{"scope", "type"},
)

// RecordRateLimited increments the business rate-limit rejection counter.
// scope is "token" or "tenant"; limitType is "rpm" or "tpm".
func RecordRateLimited(scope, limitType string) {
	RateLimitedTotal.WithLabelValues(scope, limitType).Inc()
}

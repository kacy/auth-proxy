package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds application-level metrics.
// HTTP request metrics are handled by the middleware package.
type Metrics struct {
	// Upstream metrics
	UpstreamRequestsTotal   *prometheus.CounterVec
	UpstreamRequestDuration *prometheus.HistogramVec
	UpstreamErrors          *prometheus.CounterVec

	// Attestation metrics
	AttestationAttemptsTotal *prometheus.CounterVec
	AttestationSuccessTotal  *prometheus.CounterVec
	AttestationFailuresTotal *prometheus.CounterVec
}

func New() *Metrics {
	return &Metrics{
		UpstreamRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_upstream_requests_total",
				Help: "Total number of requests to upstream (Supabase)",
			},
			[]string{"endpoint", "status"},
		),
		UpstreamRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "auth_proxy_upstream_request_duration_seconds",
				Help:    "Upstream request duration in seconds",
				Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"endpoint"},
		),
		UpstreamErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_upstream_errors_total",
				Help: "Total number of upstream errors",
			},
			[]string{"endpoint", "error_type"},
		),
		AttestationAttemptsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_attestation_attempts_total",
				Help: "Total number of attestation verification attempts",
			},
			[]string{"platform"},
		),
		AttestationSuccessTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_attestation_success_total",
				Help: "Total number of successful attestation verifications",
			},
			[]string{"platform"},
		),
		AttestationFailuresTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_attestation_failures_total",
				Help: "Total number of failed attestation verifications",
			},
			[]string{"platform", "reason"},
		),
	}
}

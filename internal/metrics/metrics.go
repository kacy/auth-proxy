package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the auth-proxy service.
type Metrics struct {
	// gRPC metrics
	GRPCRequestsTotal    *prometheus.CounterVec
	GRPCRequestDuration  *prometheus.HistogramVec
	GRPCRequestsInFlight prometheus.Gauge

	// Authentication metrics
	AuthAttemptsTotal *prometheus.CounterVec
	AuthSuccessTotal  *prometheus.CounterVec
	AuthFailuresTotal *prometheus.CounterVec
	AuthLatency       *prometheus.HistogramVec

	// GoTrue client metrics
	GoTrueRequestsTotal   *prometheus.CounterVec
	GoTrueRequestDuration *prometheus.HistogramVec
	GoTrueErrors          *prometheus.CounterVec

	// Business metrics
	SignupsTotal *prometheus.CounterVec
	LoginsTotal  *prometheus.CounterVec

	// Attestation metrics
	AttestationAttemptsTotal *prometheus.CounterVec
	AttestationSuccessTotal  *prometheus.CounterVec
	AttestationFailuresTotal *prometheus.CounterVec
}

// New creates and registers all Prometheus metrics.
func New() *Metrics {
	return &Metrics{
		GRPCRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_grpc_requests_total",
				Help: "Total number of gRPC requests",
			},
			[]string{"method", "status"},
		),
		GRPCRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "auth_proxy_grpc_request_duration_seconds",
				Help:    "gRPC request duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method"},
		),
		GRPCRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "auth_proxy_grpc_requests_in_flight",
				Help: "Number of gRPC requests currently being processed",
			},
		),
		AuthAttemptsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_auth_attempts_total",
				Help: "Total number of authentication attempts",
			},
			[]string{"provider", "action"},
		),
		AuthSuccessTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_auth_success_total",
				Help: "Total number of successful authentications",
			},
			[]string{"provider", "action"},
		),
		AuthFailuresTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_auth_failures_total",
				Help: "Total number of failed authentications",
			},
			[]string{"provider", "action", "reason"},
		),
		AuthLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "auth_proxy_auth_latency_seconds",
				Help:    "Authentication operation latency in seconds",
				Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"provider", "action"},
		),
		GoTrueRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_gotrue_requests_total",
				Help: "Total number of requests to GoTrue",
			},
			[]string{"endpoint", "status"},
		),
		GoTrueRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "auth_proxy_gotrue_request_duration_seconds",
				Help:    "GoTrue request duration in seconds",
				Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"endpoint"},
		),
		GoTrueErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_gotrue_errors_total",
				Help: "Total number of GoTrue errors",
			},
			[]string{"endpoint", "error_type"},
		),
		SignupsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_signups_total",
				Help: "Total number of user signups",
			},
			[]string{"provider"},
		),
		LoginsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_logins_total",
				Help: "Total number of user logins",
			},
			[]string{"provider"},
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

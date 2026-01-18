package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPMetrics holds Prometheus metrics for HTTP requests.
type HTTPMetrics struct {
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestsInFlight prometheus.Gauge
	responseSize     *prometheus.HistogramVec
}

// NewHTTPMetrics creates and registers HTTP metrics.
func NewHTTPMetrics() *HTTPMetrics {
	return &HTTPMetrics{
		requestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_proxy_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "auth_proxy_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method", "path"},
		),
		requestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "auth_proxy_http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
		),
		responseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "auth_proxy_http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 7), // 100B to 100MB
			},
			[]string{"method", "path"},
		),
	}
}

// Middleware returns the HTTP middleware handler for metrics.
func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Track in-flight requests
		m.requestsInFlight.Inc()
		defer m.requestsInFlight.Dec()

		// Wrap response writer to capture status and size
		recorder := &metricsRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Process request
		next.ServeHTTP(recorder, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		path := normalizePath(r.URL.Path)
		status := strconv.Itoa(recorder.statusCode)

		m.requestsTotal.WithLabelValues(r.Method, path, status).Inc()
		m.requestDuration.WithLabelValues(r.Method, path).Observe(duration)
		m.responseSize.WithLabelValues(r.Method, path).Observe(float64(recorder.written))
	})
}

type metricsRecorder struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (r *metricsRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *metricsRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

// normalizePath normalizes the URL path for metrics labels.
// This prevents high cardinality from dynamic path segments.
func normalizePath(path string) string {
	// Group known auth endpoints
	switch {
	case path == "/health" || path == "/healthz":
		return "/health"
	case path == "/auth/v1/signup" || path == "/signup":
		return "/auth/v1/signup"
	case path == "/auth/v1/token" || path == "/token":
		return "/auth/v1/token"
	case path == "/auth/v1/logout" || path == "/logout":
		return "/auth/v1/logout"
	case path == "/auth/v1/user" || path == "/user":
		return "/auth/v1/user"
	case path == "/auth/v1/recover" || path == "/recover":
		return "/auth/v1/recover"
	case path == "/auth/v1/verify" || path == "/verify":
		return "/auth/v1/verify"
	case path == "/auth/v1/otp" || path == "/otp":
		return "/auth/v1/otp"
	case path == "/attestation/challenge":
		return "/attestation/challenge"
	default:
		// For unknown paths, use a generic label to prevent cardinality explosion
		return "/other"
	}
}

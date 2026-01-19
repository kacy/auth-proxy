package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/kacy/auth-proxy/internal/logging"
	"go.uber.org/zap"
)

// Note: Body sanitization is now handled by logging.SanitizeBody

// LoggingMiddleware logs HTTP requests and responses.
type LoggingMiddleware struct {
	logger      *logging.Logger
	logBodies   bool
	maxBodySize int64
}

// LoggingConfig holds logging middleware configuration.
type LoggingConfig struct {
	LogBodies   bool  // Whether to log request/response bodies
	MaxBodySize int64 // Maximum body size to log (0 = unlimited)
}

// NewLoggingMiddleware creates a new logging middleware.
func NewLoggingMiddleware(logger *logging.Logger, cfg LoggingConfig) *LoggingMiddleware {
	maxSize := cfg.MaxBodySize
	if maxSize == 0 {
		maxSize = 10 * 1024 // 10KB default
	}

	return &LoggingMiddleware{
		logger:      logger,
		logBodies:   cfg.LogBodies,
		maxBodySize: maxSize,
	}
}

// Middleware returns the HTTP middleware handler.
func (m *LoggingMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Capture request body if logging is enabled
		var requestBody []byte
		if m.logBodies && r.Body != nil {
			requestBody, _ = io.ReadAll(io.LimitReader(r.Body, m.maxBodySize))
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Log incoming request
		fields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		}

		if r.URL.RawQuery != "" {
			fields = append(fields, zap.String("query", r.URL.RawQuery))
		}

		// Log request body (sanitized)
		if m.logBodies && len(requestBody) > 0 {
			sanitized := logging.SanitizeBody(requestBody)
			fields = append(fields, zap.String("request_body", sanitized))
		}

		m.logger.Request("incoming request", fields...)

		// Wrap response writer to capture status and body
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
			logBody:        m.logBodies,
			maxSize:        m.maxBodySize,
		}

		// Process request
		next.ServeHTTP(recorder, r)

		// Calculate duration
		duration := time.Since(start)

		// Log response
		responseFields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", recorder.statusCode),
			zap.Duration("duration", duration),
			zap.Int64("response_size", recorder.written),
		}

		if m.logBodies && recorder.body.Len() > 0 {
			sanitized := logging.SanitizeBody(recorder.body.Bytes())
			responseFields = append(responseFields, zap.String("response_body", sanitized))
		}

		if recorder.statusCode >= 400 {
			m.logger.Logger.Warn(logging.EmojiWarning+" request completed with error", responseFields...)
		} else {
			m.logger.Response("request completed", responseFields...)
		}
	})
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	written    int64
	logBody    bool
	maxSize    int64
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.logBody && r.body.Len() < int(r.maxSize) {
		r.body.Write(b)
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

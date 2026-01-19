package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kacy/auth-proxy/internal/logging"
	"go.uber.org/zap"
)

const (
	// APIKeyHeader is the header clients must send with the Supabase anon key.
	APIKeyHeader = "apikey"
)

// APIKeyMiddleware validates that clients send the correct API key.
// This ensures requests come from clients that have your app's Supabase config.
type APIKeyMiddleware struct {
	expectedKey string
	enabled     bool
	logger      *logging.Logger
}

// APIKeyConfig holds configuration for API key validation.
type APIKeyConfig struct {
	// ExpectedKey is the Supabase anon key that clients must provide.
	ExpectedKey string
	// Enabled controls whether API key validation is enforced.
	Enabled bool
}

// NewAPIKeyMiddleware creates a new API key validation middleware.
func NewAPIKeyMiddleware(cfg APIKeyConfig, logger *logging.Logger) *APIKeyMiddleware {
	return &APIKeyMiddleware{
		expectedKey: cfg.ExpectedKey,
		enabled:     cfg.Enabled,
		logger:      logger,
	}
}

// Middleware returns the HTTP middleware handler.
func (m *APIKeyMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if disabled
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip for health checks
		if strings.HasPrefix(r.URL.Path, "/health") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip for challenge endpoint (needed before client can authenticate)
		if r.URL.Path == "/attestation/challenge" {
			next.ServeHTTP(w, r)
			return
		}

		// Get API key from header
		providedKey := r.Header.Get(APIKeyHeader)
		if providedKey == "" {
			m.logger.AuthWarning("request missing API key",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
				zap.String("remote_addr", r.RemoteAddr),
			)
			m.rejectRequest(w, "api_key_required", "API key is required", http.StatusUnauthorized)
			return
		}

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(m.expectedKey)) != 1 {
			m.logger.AuthWarning("invalid API key",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
				zap.String("remote_addr", r.RemoteAddr),
			)
			m.rejectRequest(w, "invalid_api_key", "Invalid API key", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *APIKeyMiddleware) rejectRequest(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

// Package middleware provides HTTP middleware for the auth proxy.
package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kacy/auth-proxy/internal/attestation"
	"github.com/kacy/auth-proxy/internal/logging"
	"go.uber.org/zap"
)

// AttestationHeader is the header name for attestation data.
const (
	AttestationHeader = "X-Attestation"
	PlatformHeader    = "X-Platform"
	KeyIDHeader       = "X-Attestation-Key-ID"
	ChallengeHeader   = "X-Attestation-Challenge"
	AssertionHeader   = "X-Attestation-Assertion"
	ClientDataHeader  = "X-Attestation-Client-Data"
)

// AttestationMiddleware validates device attestation on incoming requests.
type AttestationMiddleware struct {
	verifier *attestation.Verifier
	logger   *logging.Logger
}

// NewAttestationMiddleware creates a new attestation middleware.
func NewAttestationMiddleware(verifier *attestation.Verifier, logger *logging.Logger) *AttestationMiddleware {
	return &AttestationMiddleware{
		verifier: verifier,
		logger:   logger,
	}
}

// Middleware returns the HTTP middleware handler.
func (m *AttestationMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip attestation for health checks
		if strings.HasPrefix(r.URL.Path, "/health") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip if attestation is disabled
		if !m.verifier.IsEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		// Check if this is an initial attestation or an assertion
		if r.Header.Get(AssertionHeader) != "" {
			// iOS assertion flow (subsequent requests)
			if err := m.verifyAssertion(r); err != nil {
				m.handleError(w, err)
				return
			}
		} else if r.Header.Get(AttestationHeader) != "" {
			// Initial attestation flow
			if err := m.verifyAttestation(r); err != nil {
				m.handleError(w, err)
				return
			}
		} else {
			// No attestation provided
			m.logger.AuthWarning("request without attestation",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
			)
			m.handleError(w, attestation.ErrAttestationRequired)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *AttestationMiddleware) verifyAttestation(r *http.Request) error {
	platform := parsePlatform(r.Header.Get(PlatformHeader))
	token := r.Header.Get(AttestationHeader)
	keyID := r.Header.Get(KeyIDHeader)
	challenge := r.Header.Get(ChallengeHeader)

	data := &attestation.AttestationData{
		Platform:  platform,
		Token:     token,
		KeyID:     keyID,
		Challenge: challenge,
	}

	return m.verifier.Verify(r.Context(), data)
}

func (m *AttestationMiddleware) verifyAssertion(r *http.Request) error {
	assertion := r.Header.Get(AssertionHeader)
	keyID := r.Header.Get(KeyIDHeader)
	clientData := r.Header.Get(ClientDataHeader)

	data := &attestation.AssertionData{
		Assertion:  assertion,
		ClientData: []byte(clientData),
		KeyID:      keyID,
	}

	return m.verifier.VerifyAssertion(r.Context(), data)
}

func (m *AttestationMiddleware) handleError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	var statusCode int
	var errorCode string
	var message string

	switch err {
	case attestation.ErrAttestationRequired:
		statusCode = http.StatusUnauthorized
		errorCode = "attestation_required"
		message = "Device attestation is required for this request"
	case attestation.ErrInvalidAttestation:
		statusCode = http.StatusForbidden
		errorCode = "invalid_attestation"
		message = "Device attestation verification failed"
	case attestation.ErrUnsupportedPlatform:
		statusCode = http.StatusBadRequest
		errorCode = "unsupported_platform"
		message = "Unsupported platform for attestation"
	case attestation.ErrKeyNotFound:
		statusCode = http.StatusUnauthorized
		errorCode = "key_not_found"
		message = "Attestation key not found, re-attestation required"
	case attestation.ErrReplayDetected:
		statusCode = http.StatusForbidden
		errorCode = "replay_detected"
		message = "Assertion replay detected"
	case attestation.ErrInvalidAssertion:
		statusCode = http.StatusForbidden
		errorCode = "invalid_assertion"
		message = "Invalid assertion"
	default:
		statusCode = http.StatusInternalServerError
		errorCode = "attestation_error"
		message = "Attestation verification error"
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   errorCode,
		"message": message,
	})
}

func parsePlatform(s string) attestation.Platform {
	switch strings.ToLower(s) {
	case "ios", "apple":
		return attestation.PlatformIOS
	case "android", "google":
		return attestation.PlatformAndroid
	default:
		return attestation.PlatformUnspecified
	}
}

// ChallengeHandler returns an HTTP handler for generating attestation challenges.
// Clients call this endpoint to get a challenge before performing attestation.
func ChallengeHandler(verifier *attestation.Verifier, logger *logging.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get identifier from request (e.g., device ID or temporary session ID)
		var req struct {
			Identifier string `json:"identifier"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "invalid_request",
				"message": "Invalid JSON body",
			})
			return
		}

		if req.Identifier == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "invalid_request",
				"message": "Identifier is required",
			})
			return
		}

		challenge, err := verifier.GenerateChallenge(req.Identifier)
		if err != nil {
			logger.AuthError("failed to generate challenge", zap.Error(err))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "challenge_error",
				"message": "Failed to generate challenge",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"challenge": challenge,
		})
	}
}

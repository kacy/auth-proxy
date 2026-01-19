// Package proxy provides an HTTP reverse proxy for Supabase Auth API.
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/kacy/auth-proxy/internal/logging"
	"github.com/kacy/auth-proxy/internal/metrics"
	"go.uber.org/zap"
)

// Config holds the proxy configuration.
type Config struct {
	TargetURL      string
	AnonKey        string
	Timeout        time.Duration
	MaxRequestBody int64
}

// Proxy handles reverse proxying requests to Supabase Auth.
type Proxy struct {
	config  Config
	target  *url.URL
	proxy   *httputil.ReverseProxy
	logger  *logging.Logger
	metrics *metrics.Metrics
}

// New creates a new HTTP reverse proxy.
func New(cfg Config, logger *logging.Logger, m *metrics.Metrics) (*Proxy, error) {
	target, err := url.Parse(cfg.TargetURL)
	if err != nil {
		return nil, err
	}

	p := &Proxy{
		config:  cfg,
		target:  target,
		logger:  logger,
		metrics: m,
	}

	p.proxy = &httputil.ReverseProxy{
		Director:       p.director,
		ModifyResponse: p.modifyResponse,
		ErrorHandler:   p.errorHandler,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return p, nil
}

// ServeHTTP implements http.Handler.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}

// director modifies the request before forwarding to the target.
func (p *Proxy) director(req *http.Request) {
	// Store original path for logging
	originalPath := req.URL.Path

	// Set target URL
	req.URL.Scheme = p.target.Scheme
	req.URL.Host = p.target.Host
	req.Host = p.target.Host

	// Ensure path starts with /auth/v1 for Supabase Auth API
	if !strings.HasPrefix(req.URL.Path, "/auth/v1") {
		req.URL.Path = "/auth/v1" + req.URL.Path
	}

	// Add required Supabase headers
	req.Header.Set("apikey", p.config.AnonKey)

	// Preserve Authorization header if present (for authenticated requests)
	// The client should send their access token as Authorization: Bearer <token>

	// Remove hop-by-hop headers
	removeHopByHopHeaders(req.Header)

	p.logger.Request("proxying request to Supabase",
		zap.String("original_path", originalPath),
		zap.String("target_path", req.URL.Path),
		zap.String("method", req.Method),
	)
}

// authResponse represents a Supabase auth response with user info.
type authResponse struct {
	User *struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		AppMetadata struct {
			Provider string `json:"provider"`
		} `json:"app_metadata"`
	} `json:"user"`
}

// authPaths are the Supabase Auth API paths that return user info on success.
var authPaths = []string{
	"/auth/v1/token",
	"/auth/v1/signup",
	"/auth/v1/verify",
	"/auth/v1/recover",
	"/auth/v1/user",
}

// isAuthPath checks if the request path is an auth endpoint that returns user info.
func isAuthPath(path string) bool {
	for _, authPath := range authPaths {
		if strings.HasPrefix(path, authPath) {
			return true
		}
	}
	return false
}

// modifyResponse processes the response from the target.
func (p *Proxy) modifyResponse(resp *http.Response) error {
	// Remove hop-by-hop headers from response
	removeHopByHopHeaders(resp.Header)

	path := resp.Request.URL.Path

	// Log response status
	p.logger.Response("received response from Supabase",
		zap.Int("status", resp.StatusCode),
		zap.String("path", path),
	)

	// For successful auth responses, extract and log user info
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && isAuthPath(path) {
		p.logAuthResponse(resp)
	}

	return nil
}

// logAuthResponse extracts user info from auth responses and logs it.
func (p *Proxy) logAuthResponse(resp *http.Response) {
	if resp.Body == nil {
		return
	}

	// Read the body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.logger.Logger.Debug("failed to read auth response body", zap.Error(err))
		return
	}

	// Restore the body for the client
	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	// Parse the response
	var authResp authResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		// Not all responses are JSON or have user info, that's okay
		return
	}

	// Log user info if present
	if authResp.User != nil && authResp.User.ID != "" {
		provider := authResp.User.AppMetadata.Provider
		if provider == "" {
			provider = "email"
		}

		p.logger.OAuthSuccess(
			provider,
			authResp.User.Email,
			authResp.User.ID,
			zap.String("path", resp.Request.URL.Path),
		)
	}
}

// errorHandler handles proxy errors.
func (p *Proxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	p.logger.NetworkError("proxy error",
		zap.Error(err),
		zap.String("path", r.URL.Path),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	w.Write([]byte(`{"error":"upstream service unavailable","code":"bad_gateway"}`))
}

// hopByHopHeaders are headers that should not be forwarded.
var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func removeHopByHopHeaders(header http.Header) {
	for _, h := range hopByHopHeaders {
		header.Del(h)
	}
}

// ResponseRecorder wraps http.ResponseWriter to capture response data.
type ResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
	Body       *bytes.Buffer
}

// NewResponseRecorder creates a new ResponseRecorder.
func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
		Body:           &bytes.Buffer{},
	}
}

// WriteHeader captures the status code.
func (r *ResponseRecorder) WriteHeader(code int) {
	r.StatusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Write captures the response body.
func (r *ResponseRecorder) Write(b []byte) (int, error) {
	r.Body.Write(b)
	return r.ResponseWriter.Write(b)
}

// CopyRequestBody reads and returns the request body, restoring it for later use.
func CopyRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}

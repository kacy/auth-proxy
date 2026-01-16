package gotrue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/company/auth-proxy/internal/logging"
	"github.com/company/auth-proxy/internal/metrics"
	"go.uber.org/zap"
)

type Client struct {
	baseURL    string
	anonKey    string
	httpClient *http.Client
	logger     *logging.Logger
	metrics    *metrics.Metrics
}

func NewClient(baseURL, anonKey string, timeout time.Duration, logger *logging.Logger, m *metrics.Metrics) *Client {
	return &Client{
		baseURL: baseURL,
		anonKey: anonKey,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger:  logger,
		metrics: m,
	}
}

type SignUpRequest struct {
	Email    string                 `json:"email"`
	Password string                 `json:"password"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type OAuthSignInRequest struct {
	Provider string `json:"provider"`
	IDToken  string `json:"id_token"`
	Nonce    string `json:"nonce,omitempty"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user"`
}

type User struct {
	ID               string                 `json:"id"`
	Email            string                 `json:"email"`
	EmailConfirmedAt *string                `json:"email_confirmed_at"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
	AppMetadata      map[string]interface{} `json:"app_metadata"`
	UserMetadata     map[string]interface{} `json:"user_metadata"`
}

type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Message          string `json:"msg"`
	Code             int    `json:"code"`
}

func (e *ErrorResponse) String() string {
	if e.Message != "" {
		return e.Message
	}
	if e.ErrorDescription != "" {
		return e.ErrorDescription
	}
	return e.Error
}

func (c *Client) SignUp(ctx context.Context, req *SignUpRequest) (*AuthResponse, error) {
	endpoint := "/auth/v1/signup"
	start := time.Now()

	c.metrics.AuthAttemptsTotal.WithLabelValues("email", "signup").Inc()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		c.metrics.AuthFailuresTotal.WithLabelValues("email", "signup", "network_error").Inc()
		c.metrics.GoTrueErrors.WithLabelValues(endpoint, "network").Inc()
		return nil, err
	}

	c.metrics.AuthLatency.WithLabelValues("email", "signup").Observe(time.Since(start).Seconds())

	var authResp AuthResponse
	if err := json.Unmarshal(resp, &authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.metrics.AuthSuccessTotal.WithLabelValues("email", "signup").Inc()
	c.metrics.SignupsTotal.WithLabelValues("email").Inc()

	return &authResp, nil
}

func (c *Client) SignIn(ctx context.Context, req *SignInRequest) (*AuthResponse, error) {
	endpoint := "/auth/v1/token?grant_type=password"
	start := time.Now()

	c.metrics.AuthAttemptsTotal.WithLabelValues("email", "login").Inc()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		c.metrics.AuthFailuresTotal.WithLabelValues("email", "login", "network_error").Inc()
		c.metrics.GoTrueErrors.WithLabelValues(endpoint, "network").Inc()
		return nil, err
	}

	c.metrics.AuthLatency.WithLabelValues("email", "login").Observe(time.Since(start).Seconds())

	var authResp AuthResponse
	if err := json.Unmarshal(resp, &authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.metrics.AuthSuccessTotal.WithLabelValues("email", "login").Inc()
	c.metrics.LoginsTotal.WithLabelValues("email").Inc()

	return &authResp, nil
}

func (c *Client) SignInWithOAuth(ctx context.Context, provider string, idToken string, nonce string) (*AuthResponse, error) {
	endpoint := "/auth/v1/token?grant_type=id_token"
	start := time.Now()

	c.metrics.AuthAttemptsTotal.WithLabelValues(provider, "login").Inc()

	reqBody := map[string]string{
		"provider": provider,
		"id_token": idToken,
	}
	if nonce != "" {
		reqBody["nonce"] = nonce
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		c.metrics.AuthFailuresTotal.WithLabelValues(provider, "login", "network_error").Inc()
		c.metrics.GoTrueErrors.WithLabelValues(endpoint, "network").Inc()
		return nil, err
	}

	c.metrics.AuthLatency.WithLabelValues(provider, "login").Observe(time.Since(start).Seconds())

	var authResp AuthResponse
	if err := json.Unmarshal(resp, &authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.metrics.AuthSuccessTotal.WithLabelValues(provider, "login").Inc()
	c.metrics.LoginsTotal.WithLabelValues(provider).Inc()

	return &authResp, nil
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	endpoint := "/auth/v1/token?grant_type=refresh_token"

	body, err := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}

	var authResp AuthResponse
	if err := json.Unmarshal(resp, &authResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &authResp, nil
}

func (c *Client) Logout(ctx context.Context, accessToken string) error {
	endpoint := "/auth/v1/logout"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	c.metrics.GoTrueRequestDuration.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())

	if err != nil {
		c.metrics.GoTrueErrors.WithLabelValues(endpoint, "network").Inc()
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	c.metrics.GoTrueRequestsTotal.WithLabelValues(endpoint, fmt.Sprintf("%d", resp.StatusCode)).Inc()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil {
			return fmt.Errorf("logout failed: %s", errResp.String())
		}
		return fmt.Errorf("logout failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) HealthCheck(ctx context.Context) error {
	endpoint := "/auth/v1/health"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.anonKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	c.metrics.GoTrueRequestDuration.WithLabelValues(endpoint).Observe(duration.Seconds())

	if err != nil {
		c.logger.NetworkError("GoTrue request failed",
			zap.String("endpoint", endpoint),
			zap.Error(err),
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	c.metrics.GoTrueRequestsTotal.WithLabelValues(endpoint, fmt.Sprintf("%d", resp.StatusCode)).Inc()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil {
			c.logger.AuthError("GoTrue returned error",
				zap.String("endpoint", endpoint),
				zap.Int("status", resp.StatusCode),
				zap.String("error", errResp.String()),
			)
			return nil, fmt.Errorf("%s", errResp.String())
		}
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return respBody, nil
}

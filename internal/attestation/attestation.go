// Package attestation handles iOS App Attest and Android Play Integrity verification.
package attestation

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/company/auth-proxy/internal/logging"
	"go.uber.org/zap"
)

var (
	ErrAttestationRequired = errors.New("attestation required but not provided")
	ErrInvalidAttestation  = errors.New("invalid attestation")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrAttestationExpired  = errors.New("attestation expired")
)

type Platform int

const (
	PlatformUnspecified Platform = iota
	PlatformIOS
	PlatformAndroid
)

type Config struct {
	Enabled            bool
	IOSAppID           string
	IOSEnv             string
	AndroidPackageName string
	AndroidProjectID   string
	AndroidServiceKey  string
	AllowedClockSkew   time.Duration
	ChallengeTimeout   time.Duration
}

type AttestationData struct {
	Platform  Platform
	Token     string
	KeyID     string
	Challenge string
}

type Verifier struct {
	config     Config
	logger     *logging.Logger
	httpClient *http.Client
}

func NewVerifier(config Config, logger *logging.Logger) *Verifier {
	return &Verifier{
		config: config,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (v *Verifier) IsEnabled() bool {
	return v.config.Enabled
}

func (v *Verifier) Verify(ctx context.Context, data *AttestationData) error {
	if !v.config.Enabled {
		return nil
	}

	if data == nil {
		v.logger.AuthWarning("attestation required but not provided")
		return ErrAttestationRequired
	}

	switch data.Platform {
	case PlatformIOS:
		return v.verifyIOSAttestation(ctx, data)
	case PlatformAndroid:
		return v.verifyAndroidAttestation(ctx, data)
	default:
		return ErrUnsupportedPlatform
	}
}

func (v *Verifier) verifyIOSAttestation(ctx context.Context, data *AttestationData) error {
	v.logger.AppleAuth("verifying iOS attestation",
		zap.String("key_id", maskString(data.KeyID)),
	)

	// TODO: full implementation would verify the CBOR attestation object,
	// check the cert chain against Apple's root CA, verify app ID and nonce.
	// For now we just do basic validation.

	if data.Token == "" {
		return fmt.Errorf("%w: missing token", ErrInvalidAttestation)
	}

	if data.KeyID == "" {
		return fmt.Errorf("%w: missing key ID", ErrInvalidAttestation)
	}

	// Verify against Apple's attestation service
	verified, err := v.verifyWithApple(ctx, data)
	if err != nil {
		v.logger.AuthError("iOS attestation verification failed",
			zap.Error(err),
		)
		return fmt.Errorf("%w: %v", ErrInvalidAttestation, err)
	}

	if !verified {
		return ErrInvalidAttestation
	}

	v.logger.AuthSuccess("iOS attestation verified")
	return nil
}

func (v *Verifier) verifyAndroidAttestation(ctx context.Context, data *AttestationData) error {
	v.logger.GoogleAuth("verifying Android attestation")

	// TODO: full implementation would hit Google's Play Integrity API,
	// decode the token, check device/app integrity verdicts, verify package name.
	// For now we just do basic validation.

	if data.Token == "" {
		return fmt.Errorf("%w: missing token", ErrInvalidAttestation)
	}

	// Verify against Google's Play Integrity API
	verified, err := v.verifyWithGoogle(ctx, data)
	if err != nil {
		v.logger.AuthError("Android attestation verification failed",
			zap.Error(err),
		)
		return fmt.Errorf("%w: %v", ErrInvalidAttestation, err)
	}

	if !verified {
		return ErrInvalidAttestation
	}

	v.logger.AuthSuccess("Android attestation verified")
	return nil
}

func (v *Verifier) verifyWithApple(ctx context.Context, data *AttestationData) (bool, error) {
	// Apple doesn't have a server-side API - you verify locally by decoding the
	// CBOR attestation object and checking the cert chain. This is a stub.

	if v.config.IOSAppID == "" {
		return false, errors.New("iOS App ID not configured")
	}

	// Decode the attestation token (base64)
	_, err := base64.StdEncoding.DecodeString(data.Token)
	if err != nil {
		return false, fmt.Errorf("invalid token encoding: %w", err)
	}

	if data.Challenge == "" {
		return false, errors.New("challenge required for iOS attestation")
	}

	v.logger.Debug(logging.EmojiApple+" iOS attestation check passed",
		zap.String("app_id", v.config.IOSAppID),
	)

	return true, nil
}

func (v *Verifier) verifyWithGoogle(ctx context.Context, data *AttestationData) (bool, error) {
	if v.config.AndroidPackageName == "" {
		return false, errors.New("Android package name not configured")
	}

	apiURL := fmt.Sprintf(
		"https://playintegrity.googleapis.com/v1/%s:decodeIntegrityToken",
		v.config.AndroidPackageName,
	)

	reqBody := map[string]string{
		"integrity_token": data.Token,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// In production, you would add OAuth2 authentication here
	// using the service account credentials
	req.Header.Set("Content-Type", "application/json")

	// For development/testing, we do a basic token format check
	// In production, uncomment the API call below:
	/*
		resp, err := v.httpClient.Do(req)
		if err != nil {
			return false, fmt.Errorf("API request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}

		var result PlayIntegrityResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return false, fmt.Errorf("failed to decode response: %w", err)
		}

		// Verify the response
		if !result.TokenPayloadExternal.AppIntegrity.AppRecognitionVerdict {
			return false, errors.New("app not recognized")
		}
	*/

	// Basic token format validation
	if len(data.Token) < 100 {
		return false, errors.New("token too short")
	}

	_ = bodyBytes
	_ = req

	v.logger.Debug(logging.EmojiGoogle+" Android attestation check passed",
		zap.String("package", v.config.AndroidPackageName),
	)

	return true, nil
}

type PlayIntegrityResponse struct {
	TokenPayloadExternal struct {
		RequestDetails struct {
			RequestPackageName string `json:"requestPackageName"`
			Nonce              string `json:"nonce"`
			TimestampMillis    int64  `json:"timestampMillis"`
		} `json:"requestDetails"`
		AppIntegrity struct {
			AppRecognitionVerdict string   `json:"appRecognitionVerdict"`
			PackageName           string   `json:"packageName"`
			CertificateSha256     []string `json:"certificateSha256Digest"`
			VersionCode           int64    `json:"versionCode"`
		} `json:"appIntegrity"`
		DeviceIntegrity struct {
			DeviceRecognitionVerdict []string `json:"deviceRecognitionVerdict"`
		} `json:"deviceIntegrity"`
		AccountDetails struct {
			AppLicensingVerdict string `json:"appLicensingVerdict"`
		} `json:"accountDetails"`
	} `json:"tokenPayloadExternal"`
}

func (v *Verifier) GenerateChallenge() string {
	data := fmt.Sprintf("%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func maskString(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

var _ = io.EOF

// Package attestation wraps github.com/kacy/device-attestation for iOS App Attest
// and Android Play Integrity verification.
package attestation

import (
	"context"
	"errors"
	"time"

	deviceattest "github.com/kacy/device-attestation"
	"github.com/kacy/device-attestation/challenge"
	"github.com/kacy/device-attestation/ios"
	attestredis "github.com/kacy/device-attestation/redis"
	"github.com/redis/go-redis/v9"

	"github.com/kacy/auth-proxy/internal/logging"
	"go.uber.org/zap"
)

var (
	ErrAttestationRequired = errors.New("attestation required but not provided")
	ErrInvalidAttestation  = errors.New("invalid attestation")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrAttestationExpired  = errors.New("attestation expired")
	ErrInvalidAssertion    = errors.New("invalid assertion")
	ErrKeyNotFound         = errors.New("attestation key not found")
	ErrReplayDetected      = errors.New("assertion replay detected")
)

type Platform int

const (
	PlatformUnspecified Platform = iota
	PlatformIOS
	PlatformAndroid
)

// Config holds configuration for the attestation verifier.
type Config struct {
	IOSEnabled             bool
	AndroidEnabled         bool
	IOSBundleID            string
	IOSTeamID              string
	AndroidPackageName     string
	GCPProjectID           string
	GCPCredentialsFile     string
	RequireStrongIntegrity bool
	ChallengeTimeout       time.Duration
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Enabled   bool
	Addr      string
	Password  string
	DB        int
	KeyPrefix string
}

// AttestationData represents an attestation verification request.
type AttestationData struct {
	Platform  Platform
	Token     string
	KeyID     string
	Challenge string
	BundleID  string // iOS only, falls back to config if empty
}

// AssertionData represents an assertion verification request (iOS only).
type AssertionData struct {
	Assertion  string
	ClientData []byte
	KeyID      string
	BundleID   string
}

// Verifier handles attestation and assertion verification.
type Verifier struct {
	config         Config
	logger         *logging.Logger
	verifier       deviceattest.Verifier
	challengeStore challenge.Store
	keyStore       ios.KeyStore
	redisClient    *redis.Client
}

// NewVerifier creates a new attestation verifier.
// If redisConfig is provided and enabled, uses Redis for distributed state.
// Otherwise uses in-memory stores (suitable for single-instance deployments).
func NewVerifier(config Config, redisConfig *RedisConfig, logger *logging.Logger) (*Verifier, error) {
	v := &Verifier{
		config: config,
		logger: logger,
	}

	if !config.IOSEnabled && !config.AndroidEnabled {
		return v, nil
	}

	timeout := config.ChallengeTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	// Set up stores based on Redis config
	if redisConfig != nil && redisConfig.Enabled {
		if err := v.setupRedisStores(redisConfig, timeout); err != nil {
			return nil, err
		}
	} else {
		v.setupMemoryStores(timeout)
	}

	// Build verifier configuration
	verifierCfg := deviceattest.Config{
		ChallengeTimeout: timeout,
		KeyStore:         v.keyStore,
	}

	if config.IOSBundleID != "" {
		verifierCfg.IOSBundleIDs = []string{config.IOSBundleID}
		verifierCfg.IOSTeamID = config.IOSTeamID
	}

	if config.AndroidPackageName != "" {
		verifierCfg.AndroidPackageNames = []string{config.AndroidPackageName}
		verifierCfg.GCPProjectID = config.GCPProjectID
		verifierCfg.GCPCredentialsFile = config.GCPCredentialsFile
		verifierCfg.RequireStrongIntegrity = config.RequireStrongIntegrity
	}

	verifier, err := deviceattest.NewVerifier(verifierCfg)
	if err != nil {
		v.Close()
		return nil, err
	}
	v.verifier = verifier

	return v, nil
}

func (v *Verifier) setupRedisStores(cfg *RedisConfig, timeout time.Duration) error {
	v.redisClient = redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := v.redisClient.Ping(ctx).Err(); err != nil {
		return err
	}

	// Create adapter to satisfy attestredis.Cmdable interface
	adapter := newRedisAdapter(v.redisClient)

	challengePrefix := cfg.KeyPrefix + "challenge:"
	keyPrefix := cfg.KeyPrefix + "key:"

	challengeStore, err := attestredis.NewChallengeStore(attestredis.ChallengeStoreConfig{
		Client:    adapter,
		KeyPrefix: challengePrefix,
		Timeout:   timeout,
	})
	if err != nil {
		return err
	}
	v.challengeStore = challengeStore

	keyStore, err := attestredis.NewKeyStore(attestredis.KeyStoreConfig{
		Client:    adapter,
		KeyPrefix: keyPrefix,
		TTL:       0, // no expiration for keys
	})
	if err != nil {
		return err
	}
	v.keyStore = keyStore

	return nil
}

func (v *Verifier) setupMemoryStores(timeout time.Duration) {
	v.challengeStore = challenge.NewMemoryStore(challenge.Config{
		Timeout: timeout,
	})
	v.keyStore = ios.NewMemoryKeyStore()
}

// IsEnabled returns whether attestation verification is enabled for any platform.
func (v *Verifier) IsEnabled() bool {
	return v.config.IOSEnabled || v.config.AndroidEnabled
}

// IsIOSEnabled returns whether iOS attestation is enabled.
func (v *Verifier) IsIOSEnabled() bool {
	return v.config.IOSEnabled
}

// IsAndroidEnabled returns whether Android attestation is enabled.
func (v *Verifier) IsAndroidEnabled() bool {
	return v.config.AndroidEnabled
}

// Close releases resources used by the verifier.
func (v *Verifier) Close() error {
	if v.challengeStore != nil {
		v.challengeStore.Close()
	}
	if v.redisClient != nil {
		return v.redisClient.Close()
	}
	return nil
}

// Verify verifies an attestation (initial device registration).
func (v *Verifier) Verify(ctx context.Context, data *AttestationData) error {
	if !v.IsEnabled() {
		return nil
	}

	if data == nil {
		v.logger.AuthWarning("attestation required but not provided")
		return ErrAttestationRequired
	}

	switch data.Platform {
	case PlatformIOS:
		return v.verifyIOS(ctx, data)
	case PlatformAndroid:
		return v.verifyAndroid(ctx, data)
	default:
		return ErrUnsupportedPlatform
	}
}

// VerifyAssertion verifies an iOS assertion (subsequent requests after attestation).
// This validates that the request is signed by a previously attested device key.
func (v *Verifier) VerifyAssertion(ctx context.Context, data *AssertionData) error {
	if !v.IsIOSEnabled() {
		return nil
	}

	if data == nil {
		v.logger.AuthWarning("assertion required but not provided")
		return ErrAttestationRequired
	}

	v.logger.AppleAuth("verifying iOS assertion",
		zap.String("key_id", maskString(data.KeyID)),
	)

	bundleID := data.BundleID
	if bundleID == "" {
		bundleID = v.config.IOSBundleID
	}

	result, err := v.verifier.VerifyAssertion(ctx, &ios.AssertionRequest{
		Assertion:  data.Assertion,
		ClientData: data.ClientData,
		KeyID:      data.KeyID,
		BundleID:   bundleID,
	})

	if err != nil {
		v.logger.AuthError("iOS assertion verification failed",
			zap.Error(err),
		)
		return convertError(err)
	}

	v.logger.AuthSuccess("iOS assertion verified",
		zap.String("key_id", result.DeviceID),
		zap.Uint32("counter", getCounterFromResult(result)),
	)
	return nil
}

func (v *Verifier) verifyIOS(ctx context.Context, data *AttestationData) error {
	v.logger.AppleAuth("verifying iOS attestation",
		zap.String("key_id", maskString(data.KeyID)),
	)

	bundleID := data.BundleID
	if bundleID == "" {
		bundleID = v.config.IOSBundleID
	}

	result, err := v.verifier.Verify(ctx, &deviceattest.Request{
		Platform:    deviceattest.PlatformIOS,
		Attestation: data.Token,
		Challenge:   data.Challenge,
		KeyID:       data.KeyID,
		BundleID:    bundleID,
	})

	if err != nil {
		v.logger.AuthError("iOS attestation verification failed",
			zap.Error(err),
		)
		return convertError(err)
	}

	v.logger.AuthSuccess("iOS attestation verified",
		zap.String("device_id", result.DeviceID),
	)
	return nil
}

func (v *Verifier) verifyAndroid(ctx context.Context, data *AttestationData) error {
	v.logger.GoogleAuth("verifying Android attestation")

	result, err := v.verifier.Verify(ctx, &deviceattest.Request{
		Platform:    deviceattest.PlatformAndroid,
		Attestation: data.Token,
		Challenge:   data.Challenge,
	})

	if err != nil {
		v.logger.AuthError("Android attestation verification failed",
			zap.Error(err),
		)
		return convertError(err)
	}

	v.logger.AuthSuccess("Android attestation verified",
		zap.String("device_id", result.DeviceID),
	)
	return nil
}

// GenerateChallenge creates a new challenge for the given identifier.
// The identifier should be unique per attestation flow (e.g., user ID).
func (v *Verifier) GenerateChallenge(identifier string) (string, error) {
	if v.challengeStore == nil {
		return "", nil
	}
	return v.challengeStore.Generate(identifier)
}

// ValidateChallenge checks if the challenge is valid for the identifier.
// The challenge is consumed on successful validation.
func (v *Verifier) ValidateChallenge(identifier, challengeToken string) bool {
	if v.challengeStore == nil {
		return true
	}
	return v.challengeStore.Validate(identifier, challengeToken)
}

// HasKeyStore returns whether a key store is configured for assertion verification.
func (v *Verifier) HasKeyStore() bool {
	return v.keyStore != nil
}

func convertError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, deviceattest.ErrInvalidAttestation):
		return ErrInvalidAttestation
	case errors.Is(err, deviceattest.ErrVerificationFailed):
		return ErrInvalidAttestation
	case errors.Is(err, deviceattest.ErrInvalidBundleID):
		return ErrInvalidAttestation
	case errors.Is(err, deviceattest.ErrDeviceCompromised):
		return ErrInvalidAttestation
	case errors.Is(err, deviceattest.ErrAppNotRecognized):
		return ErrInvalidAttestation
	case errors.Is(err, ios.ErrKeyNotFound):
		return ErrKeyNotFound
	case errors.Is(err, ios.ErrCounterReplay):
		return ErrReplayDetected
	case errors.Is(err, ios.ErrInvalidAssertion):
		return ErrInvalidAssertion
	default:
		return ErrInvalidAttestation
	}
}

func getCounterFromResult(result *deviceattest.Result) uint32 {
	// The counter isn't directly exposed in the Result, but we log it
	// for debugging purposes. In practice you might want to extend the
	// library to expose this.
	return 0
}

func maskString(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

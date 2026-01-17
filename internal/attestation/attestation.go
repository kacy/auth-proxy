// Package attestation wraps github.com/kacy/device-attestation for iOS App Attest
// and Android Play Integrity verification.
package attestation

import (
	"context"
	"errors"

	deviceattest "github.com/kacy/device-attestation"
	"github.com/kacy/device-attestation/challenge"

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
	Enabled                bool
	IOSBundleID            string
	IOSTeamID              string
	AndroidPackageName     string
	GCPProjectID           string
	GCPCredentialsFile     string
	RequireStrongIntegrity bool
}

type AttestationData struct {
	Platform  Platform
	Token     string
	KeyID     string
	Challenge string
	BundleID  string // iOS only, falls back to config if empty
}

type Verifier struct {
	config         Config
	logger         *logging.Logger
	server         *deviceattest.Server
	challengeStore challenge.Store
}

func NewVerifier(config Config, logger *logging.Logger) (*Verifier, error) {
	v := &Verifier{
		config: config,
		logger: logger,
	}

	if !config.Enabled {
		return v, nil
	}

	// Build the server configuration
	serverCfg := deviceattest.ServerConfig{}

	if config.IOSBundleID != "" {
		serverCfg.IOS = &deviceattest.IOSConfig{
			BundleIDs: []string{config.IOSBundleID},
			TeamID:    config.IOSTeamID,
		}
	}

	if config.AndroidPackageName != "" {
		serverCfg.Android = &deviceattest.AndroidConfig{
			PackageNames:           []string{config.AndroidPackageName},
			GCPProjectID:           config.GCPProjectID,
			GCPCredentialsFile:     config.GCPCredentialsFile,
			RequireStrongIntegrity: config.RequireStrongIntegrity,
		}
	}

	server, err := deviceattest.NewServer(serverCfg)
	if err != nil {
		return nil, err
	}

	v.server = server
	v.challengeStore = challenge.NewMemoryStore(challenge.Config{})

	return v, nil
}

func (v *Verifier) IsEnabled() bool {
	return v.config.Enabled
}

func (v *Verifier) Close() error {
	if v.server != nil {
		return v.server.Close()
	}
	return nil
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
		return v.verifyIOS(ctx, data)
	case PlatformAndroid:
		return v.verifyAndroid(ctx, data)
	default:
		return ErrUnsupportedPlatform
	}
}

func (v *Verifier) verifyIOS(ctx context.Context, data *AttestationData) error {
	v.logger.AppleAuth("verifying iOS attestation",
		zap.String("key_id", maskString(data.KeyID)),
	)

	bundleID := data.BundleID
	if bundleID == "" {
		bundleID = v.config.IOSBundleID
	}

	result, err := v.server.VerifyAttestation(ctx, data.KeyID, deviceattest.VerifyRequest{
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

	result, err := v.server.VerifyAttestation(ctx, "", deviceattest.VerifyRequest{
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

func (v *Verifier) GenerateChallenge(userID string) (string, error) {
	if v.challengeStore == nil {
		// If attestation is disabled, just return an empty challenge
		return "", nil
	}
	return v.challengeStore.Generate(userID)
}

func (v *Verifier) ValidateChallenge(userID, challengeToken string) bool {
	if v.challengeStore == nil {
		return true
	}
	return v.challengeStore.Validate(userID, challengeToken)
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
	default:
		return ErrInvalidAttestation
	}
}

func maskString(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

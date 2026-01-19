package attestation

import (
	"context"
	"testing"

	"github.com/kacy/auth-proxy/internal/logging"
)

func TestVerifierIsEnabled(t *testing.T) {
	tests := []struct {
		name           string
		iosEnabled     bool
		androidEnabled bool
		want           bool
	}{
		{"both enabled", true, true, true},
		{"only iOS enabled", true, false, true},
		{"only Android enabled", false, true, true},
		{"both disabled", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Verifier{
				config: Config{IOSEnabled: tt.iosEnabled, AndroidEnabled: tt.androidEnabled},
			}
			if got := v.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerifyDisabled(t *testing.T) {
	logger, _ := createTestLogger()
	v, err := NewVerifier(Config{IOSEnabled: false, AndroidEnabled: false}, nil, logger)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	defer v.Close()

	err = v.Verify(context.Background(), nil)
	if err != nil {
		t.Errorf("Verify() with disabled attestation should return nil, got %v", err)
	}
}

func TestVerifyRequiredButMissing(t *testing.T) {
	logger, _ := createTestLogger()
	// Create a verifier with iOS enabled but mock the internal state
	// to avoid needing actual platform config
	v := &Verifier{
		config: Config{
			IOSEnabled:  true,
			IOSBundleID: "com.test.app",
			IOSTeamID:   "TEAM123",
		},
		logger: logger,
	}

	err := v.Verify(context.Background(), nil)
	if err != ErrAttestationRequired {
		t.Errorf("Verify() with nil data should return ErrAttestationRequired, got %v", err)
	}
}

func TestVerifyUnsupportedPlatform(t *testing.T) {
	logger, _ := createTestLogger()
	v := &Verifier{
		config: Config{
			IOSEnabled:  true,
			IOSBundleID: "com.test.app",
			IOSTeamID:   "TEAM123",
		},
		logger: logger,
	}

	data := &AttestationData{
		Platform: PlatformUnspecified,
		Token:    "test-token",
	}

	err := v.Verify(context.Background(), data)
	if err != ErrUnsupportedPlatform {
		t.Errorf("Verify() with unspecified platform should return ErrUnsupportedPlatform, got %v", err)
	}
}

func createTestLogger() (*logging.Logger, error) {
	return logging.New("error", false)
}

func TestGenerateChallenge(t *testing.T) {
	logger, _ := createTestLogger()
	v, err := NewVerifier(Config{IOSEnabled: false, AndroidEnabled: false}, nil, logger)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	defer v.Close()

	// With disabled attestation, challenge generation returns empty string
	challenge, err := v.GenerateChallenge("user-123")
	if err != nil {
		t.Errorf("GenerateChallenge() error = %v", err)
	}
	// When disabled, returns empty string
	if challenge != "" {
		t.Errorf("GenerateChallenge() with disabled verifier should return empty string")
	}
}

func TestMaskString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"short string", "abc", "***"},
		{"exactly 8 chars", "12345678", "***"},
		{"longer string", "1234567890", "1234***7890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskString(tt.input)
			if got != tt.want {
				t.Errorf("maskString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPlatformConversion(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
	}{
		{"unspecified", PlatformUnspecified},
		{"iOS", PlatformIOS},
		{"Android", PlatformAndroid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.platform < 0 || tt.platform > 2 {
				t.Errorf("unexpected platform value: %d", tt.platform)
			}
		})
	}
}

func TestNewVerifierDisabled(t *testing.T) {
	logger, _ := createTestLogger()
	v, err := NewVerifier(Config{IOSEnabled: false, AndroidEnabled: false}, nil, logger)
	if err != nil {
		t.Fatalf("NewVerifier() with disabled config should not error, got %v", err)
	}
	defer v.Close()

	if v.IsEnabled() {
		t.Error("IsEnabled() should return false for disabled verifier")
	}
}

func TestValidateChallengeDisabled(t *testing.T) {
	logger, _ := createTestLogger()
	v, err := NewVerifier(Config{IOSEnabled: false, AndroidEnabled: false}, nil, logger)
	if err != nil {
		t.Fatalf("NewVerifier() error = %v", err)
	}
	defer v.Close()

	// When disabled, validation always returns true
	if !v.ValidateChallenge("user-123", "any-challenge") {
		t.Error("ValidateChallenge() with disabled verifier should return true")
	}
}

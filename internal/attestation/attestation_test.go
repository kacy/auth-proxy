package attestation

import (
	"context"
	"testing"

	"github.com/company/auth-proxy/internal/logging"
)

func TestVerifierIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Verifier{
				config: Config{Enabled: tt.enabled},
			}
			if got := v.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerifyDisabled(t *testing.T) {
	v := &Verifier{
		config: Config{Enabled: false},
	}

	err := v.Verify(context.Background(), nil)
	if err != nil {
		t.Errorf("Verify() with disabled attestation should return nil, got %v", err)
	}
}

func TestVerifyRequiredButMissing(t *testing.T) {
	// Create a no-op logger for testing
	logger, _ := createTestLogger()
	v := &Verifier{
		config: Config{
			Enabled:  true,
			IOSAppID: "com.test.app",
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
			Enabled:  true,
			IOSAppID: "com.test.app",
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

// createTestLogger creates a logger suitable for testing
func createTestLogger() (*logging.Logger, error) {
	return logging.New("error", false) // Use error level to minimize output
}

func TestGenerateChallenge(t *testing.T) {
	v := &Verifier{}

	challenge1 := v.GenerateChallenge()
	if challenge1 == "" {
		t.Error("GenerateChallenge() returned empty string")
	}

	// Challenges should be base64 encoded
	if len(challenge1) < 10 {
		t.Error("GenerateChallenge() returned unexpectedly short challenge")
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
			// Just verify the constants are defined correctly
			if tt.platform < 0 || tt.platform > 2 {
				t.Errorf("unexpected platform value: %d", tt.platform)
			}
		})
	}
}

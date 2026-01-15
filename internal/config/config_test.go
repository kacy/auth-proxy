package config

import (
	"os"
	"testing"
	"time"
)

func TestGetEnvDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{"returns default when not set", "TEST_KEY_1", "default", "", "default"},
		{"returns env value when set", "TEST_KEY_2", "default", "custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvDefault(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvDefault(%q, %q) = %q, want %q", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		want         int
	}{
		{"returns default when not set", "TEST_INT_1", 8080, "", 8080},
		{"returns parsed int when set", "TEST_INT_2", 8080, "9090", 9090},
		{"returns default on invalid int", "TEST_INT_3", 8080, "invalid", 8080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvInt(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvInt(%q, %d) = %d, want %d", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue time.Duration
		envValue     string
		want         time.Duration
	}{
		{"returns default when not set", "TEST_DUR_1", 10 * time.Second, "", 10 * time.Second},
		{"returns parsed duration when set", "TEST_DUR_2", 10 * time.Second, "30s", 30 * time.Second},
		{"returns default on invalid duration", "TEST_DUR_3", 10 * time.Second, "invalid", 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvDuration(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvDuration(%q, %v) = %v, want %v", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				GoTrueURL:     "http://gotrue:9999",
				GoTrueAnonKey: "anon-key",
			},
			wantErr: false,
		},
		{
			name: "missing GoTrueURL",
			config: Config{
				GoTrueAnonKey: "anon-key",
			},
			wantErr: true,
		},
		{
			name: "missing GoTrueAnonKey",
			config: Config{
				GoTrueURL: "http://gotrue:9999",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		want        bool
	}{
		{"production environment", "production", true},
		{"development environment", "development", false},
		{"staging environment", "staging", false},
		{"empty environment", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{Environment: tt.environment}
			if got := c.IsProduction(); got != tt.want {
				t.Errorf("Config.IsProduction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Set required environment variables
	os.Setenv("GOTRUE_URL", "http://gotrue:9999")
	os.Setenv("GOTRUE_ANON_KEY", "test-anon-key")
	defer func() {
		os.Unsetenv("GOTRUE_URL")
		os.Unsetenv("GOTRUE_ANON_KEY")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.GoTrueURL != "http://gotrue:9999" {
		t.Errorf("GoTrueURL = %q, want %q", cfg.GoTrueURL, "http://gotrue:9999")
	}

	if cfg.GoTrueAnonKey != "test-anon-key" {
		t.Errorf("GoTrueAnonKey = %q, want %q", cfg.GoTrueAnonKey, "test-anon-key")
	}

	// Check defaults
	if cfg.GRPCPort != 50051 {
		t.Errorf("GRPCPort = %d, want %d", cfg.GRPCPort, 50051)
	}

	if cfg.MetricsPort != 9090 {
		t.Errorf("MetricsPort = %d, want %d", cfg.MetricsPort, 9090)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	// Ensure required vars are not set
	os.Unsetenv("GOTRUE_URL")
	os.Unsetenv("GOTRUE_ANON_KEY")

	_, err := Load()
	if err == nil {
		t.Error("Load() expected error for missing required vars, got nil")
	}
}

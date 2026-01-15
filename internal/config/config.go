package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the auth-proxy service.
type Config struct {
	// Server settings
	GRPCPort           int
	ServerReadTimeout  time.Duration
	ServerWriteTimeout time.Duration

	// GoTrue settings
	GoTrueURL     string
	GoTrueAnonKey string
	GoTrueTimeout time.Duration

	// OAuth settings
	GoogleClientID     string
	GoogleClientSecret string
	AppleClientID      string
	AppleTeamID        string
	AppleKeyID         string
	ApplePrivateKey    string

	// Metrics settings
	MetricsPort int

	// Runtime settings
	Environment string
	LogLevel    string

	// Attestation settings (optional - for locking down to your apps)
	AttestationEnabled        bool
	AttestationIOSAppID       string // e.g., "TEAMID.com.company.app"
	AttestationIOSEnv         string // "production" or "development"
	AttestationAndroidPackage string // e.g., "com.company.app"
	AttestationAndroidProject string // Google Cloud project ID
	AttestationAndroidKey     string // Service account key JSON

	// TLS settings (optional)
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		GRPCPort:           getEnvInt("GRPC_PORT", 50051),
		ServerReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
		ServerWriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),

		GoTrueURL:     getEnvRequired("GOTRUE_URL"),
		GoTrueAnonKey: getEnvRequired("GOTRUE_ANON_KEY"),
		GoTrueTimeout: getEnvDuration("GOTRUE_TIMEOUT", 30*time.Second),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		AppleClientID:      os.Getenv("APPLE_CLIENT_ID"),
		AppleTeamID:        os.Getenv("APPLE_TEAM_ID"),
		AppleKeyID:         os.Getenv("APPLE_KEY_ID"),
		ApplePrivateKey:    os.Getenv("APPLE_PRIVATE_KEY"),

		MetricsPort: getEnvInt("METRICS_PORT", 9090),

		Environment: getEnvDefault("ENVIRONMENT", "development"),
		LogLevel:    getEnvDefault("LOG_LEVEL", "info"),

		// Attestation - disabled by default
		AttestationEnabled:        getEnvBool("ATTESTATION_ENABLED", false),
		AttestationIOSAppID:       os.Getenv("ATTESTATION_IOS_APP_ID"),
		AttestationIOSEnv:         getEnvDefault("ATTESTATION_IOS_ENV", "production"),
		AttestationAndroidPackage: os.Getenv("ATTESTATION_ANDROID_PACKAGE"),
		AttestationAndroidProject: os.Getenv("ATTESTATION_ANDROID_PROJECT"),
		AttestationAndroidKey:     os.Getenv("ATTESTATION_ANDROID_KEY"),

		// TLS
		TLSEnabled:  getEnvBool("TLS_ENABLED", false),
		TLSCertFile: os.Getenv("TLS_CERT_FILE"),
		TLSKeyFile:  os.Getenv("TLS_KEY_FILE"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that all required configuration is present.
func (c *Config) Validate() error {
	if c.GoTrueURL == "" {
		return fmt.Errorf("GOTRUE_URL is required")
	}
	if c.GoTrueAnonKey == "" {
		return fmt.Errorf("GOTRUE_ANON_KEY is required")
	}

	// Validate attestation config if enabled
	if c.AttestationEnabled {
		// At least one platform must be configured
		hasIOS := c.AttestationIOSAppID != ""
		hasAndroid := c.AttestationAndroidPackage != ""

		if !hasIOS && !hasAndroid {
			return fmt.Errorf("ATTESTATION_ENABLED is true but no platform configured (set ATTESTATION_IOS_APP_ID or ATTESTATION_ANDROID_PACKAGE)")
		}
	}

	// Validate TLS config if enabled
	if c.TLSEnabled {
		if c.TLSCertFile == "" || c.TLSKeyFile == "" {
			return fmt.Errorf("TLS_ENABLED is true but TLS_CERT_FILE or TLS_KEY_FILE not set")
		}
	}

	return nil
}

// IsProduction returns true if running in production environment.
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

func getEnvRequired(key string) string {
	return os.Getenv(key)
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

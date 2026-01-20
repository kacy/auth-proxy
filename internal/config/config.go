package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// HTTP server settings
	HTTPPort           int
	ServerReadTimeout  time.Duration
	ServerWriteTimeout time.Duration
	ServerIdleTimeout  time.Duration

	// Supabase/GoTrue settings
	GoTrueURL     string
	GoTrueAnonKey string
	GoTrueTimeout time.Duration

	// Metrics
	MetricsPort int
	Environment string
	LogLevel    string

	// Logging settings
	LogRequestBodies bool
	MaxLogBodySize   int64

	// API key validation - requires clients to send the Supabase anon key
	RequireAPIKey bool

	// Attestation - leave disabled if you don't need it
	AttestationIOSEnabled           bool
	AttestationAndroidEnabled       bool
	AttestationIOSBundleID          string
	AttestationIOSTeamID            string
	AttestationAndroidPackage       string
	AttestationGCPProjectID         string
	AttestationGCPCredentialsFile   string
	AttestationRequireStrong        bool
	AttestationChallengeTimeout     time.Duration
	AttestationSkipCertVerification bool // WARNING: Development only!

	// Redis for distributed attestation state (challenges + iOS key storage)
	// If not set, uses in-memory stores (single instance only)
	RedisEnabled   bool
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	RedisKeyPrefix string

	// TLS
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPPort:           getEnvInt("HTTP_PORT", 8080),
		ServerReadTimeout:  getEnvDuration("SERVER_READ_TIMEOUT", 10*time.Second),
		ServerWriteTimeout: getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
		ServerIdleTimeout:  getEnvDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),

		GoTrueURL:     getEnvRequired("GOTRUE_URL"),
		GoTrueAnonKey: getEnvRequired("GOTRUE_ANON_KEY"),
		GoTrueTimeout: getEnvDuration("GOTRUE_TIMEOUT", 30*time.Second),

		MetricsPort: getEnvInt("METRICS_PORT", 9090),
		Environment: getEnvDefault("ENVIRONMENT", "development"),
		LogLevel:    getEnvDefault("LOG_LEVEL", "info"),

		LogRequestBodies: getEnvBool("LOG_REQUEST_BODIES", false),
		MaxLogBodySize:   int64(getEnvInt("MAX_LOG_BODY_SIZE", 10240)),

		RequireAPIKey: getEnvBool("REQUIRE_API_KEY", true),

		AttestationIOSEnabled:           getEnvBool("ATTESTATION_IOS_ENABLED", false),
		AttestationAndroidEnabled:       getEnvBool("ATTESTATION_ANDROID_ENABLED", false),
		AttestationIOSBundleID:          os.Getenv("ATTESTATION_IOS_BUNDLE_ID"),
		AttestationIOSTeamID:            os.Getenv("ATTESTATION_IOS_TEAM_ID"),
		AttestationAndroidPackage:       os.Getenv("ATTESTATION_ANDROID_PACKAGE"),
		AttestationGCPProjectID:         os.Getenv("ATTESTATION_GCP_PROJECT_ID"),
		AttestationGCPCredentialsFile:   os.Getenv("ATTESTATION_GCP_CREDENTIALS_FILE"),
		AttestationRequireStrong:        getEnvBool("ATTESTATION_REQUIRE_STRONG_INTEGRITY", false),
		AttestationChallengeTimeout:     getEnvDuration("ATTESTATION_CHALLENGE_TIMEOUT", 5*time.Minute),
		AttestationSkipCertVerification: getEnvBool("ATTESTATION_SKIP_CERT_VERIFICATION", false),

		RedisEnabled:   getEnvBool("REDIS_ENABLED", false),
		RedisAddr:      getEnvDefault("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		RedisKeyPrefix: getEnvDefault("REDIS_KEY_PREFIX", "authproxy:"),

		TLSEnabled:  getEnvBool("TLS_ENABLED", false),
		TLSCertFile: os.Getenv("TLS_CERT_FILE"),
		TLSKeyFile:  os.Getenv("TLS_KEY_FILE"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.GoTrueURL == "" {
		return fmt.Errorf("GOTRUE_URL is required")
	}
	if c.GoTrueAnonKey == "" {
		return fmt.Errorf("GOTRUE_ANON_KEY is required")
	}

	if c.AttestationIOSEnabled {
		if c.AttestationIOSBundleID == "" {
			return fmt.Errorf("ATTESTATION_IOS_ENABLED is true but ATTESTATION_IOS_BUNDLE_ID is not set")
		}
		if c.AttestationIOSTeamID == "" {
			return fmt.Errorf("ATTESTATION_IOS_ENABLED is true but ATTESTATION_IOS_TEAM_ID is not set")
		}
	}

	if c.AttestationAndroidEnabled {
		if c.AttestationAndroidPackage == "" {
			return fmt.Errorf("ATTESTATION_ANDROID_ENABLED is true but ATTESTATION_ANDROID_PACKAGE is not set")
		}
		if c.AttestationGCPProjectID == "" {
			return fmt.Errorf("ATTESTATION_ANDROID_ENABLED is true but ATTESTATION_GCP_PROJECT_ID is not set")
		}
	}

	if c.TLSEnabled {
		if c.TLSCertFile == "" || c.TLSKeyFile == "" {
			return fmt.Errorf("TLS_ENABLED is true but TLS_CERT_FILE or TLS_KEY_FILE not set")
		}
	}

	return nil
}

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

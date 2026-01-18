package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kacy/auth-proxy/internal/attestation"
	"github.com/kacy/auth-proxy/internal/config"
	"github.com/kacy/auth-proxy/internal/logging"
	"github.com/kacy/auth-proxy/internal/middleware"
	"github.com/kacy/auth-proxy/internal/proxy"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logger, err := logging.New(cfg.LogLevel, cfg.IsProduction())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Startup("starting auth-proxy HTTP service")

	// Configure Redis if enabled (for distributed attestation state)
	var redisConfig *attestation.RedisConfig
	if cfg.RedisEnabled {
		redisConfig = &attestation.RedisConfig{
			Enabled:   true,
			Addr:      cfg.RedisAddr,
			Password:  cfg.RedisPassword,
			DB:        cfg.RedisDB,
			KeyPrefix: cfg.RedisKeyPrefix,
		}
		logger.Logger.Info(logging.EmojiDatabase + " redis enabled for attestation state")
	}

	// Initialize attestation verifier
	attestationVerifier, err := attestation.NewVerifier(attestation.Config{
		Enabled:                cfg.AttestationEnabled,
		IOSBundleID:            cfg.AttestationIOSBundleID,
		IOSTeamID:              cfg.AttestationIOSTeamID,
		AndroidPackageName:     cfg.AttestationAndroidPackage,
		GCPProjectID:           cfg.AttestationGCPProjectID,
		GCPCredentialsFile:     cfg.AttestationGCPCredentialsFile,
		RequireStrongIntegrity: cfg.AttestationRequireStrong,
		ChallengeTimeout:       cfg.AttestationChallengeTimeout,
	}, redisConfig, logger)
	if err != nil {
		logger.Logger.Error(logging.EmojiError + " failed to initialize attestation verifier")
		os.Exit(1)
	}
	defer attestationVerifier.Close()

	if cfg.AttestationEnabled {
		logger.Logger.Info(logging.EmojiAuth + " app attestation enabled")
	} else {
		logger.Logger.Info(logging.EmojiAuth + " app attestation disabled")
	}

	// Initialize HTTP metrics
	httpMetrics := middleware.NewHTTPMetrics()
	logger.Logger.Info(logging.EmojiMetrics + " prometheus metrics initialized")

	// Initialize reverse proxy
	authProxy, err := proxy.New(proxy.Config{
		TargetURL: cfg.GoTrueURL,
		AnonKey:   cfg.GoTrueAnonKey,
		Timeout:   cfg.GoTrueTimeout,
	}, logger, nil)
	if err != nil {
		logger.Logger.Error(logging.EmojiError + " failed to initialize proxy")
		os.Exit(1)
	}

	// Build middleware chain
	loggingMiddleware := middleware.NewLoggingMiddleware(logger, middleware.LoggingConfig{
		LogBodies:   cfg.LogRequestBodies,
		MaxBodySize: cfg.MaxLogBodySize,
	})

	attestationMiddleware := middleware.NewAttestationMiddleware(attestationVerifier, logger)

	// Create router/mux
	mux := http.NewServeMux()

	// Health check endpoint (no auth required)
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/healthz", healthHandler)

	// Challenge endpoint for attestation (no attestation required for this)
	mux.HandleFunc("/attestation/challenge", middleware.ChallengeHandler(attestationVerifier, logger))

	// All other requests go to the proxy with attestation middleware
	proxyHandler := attestationMiddleware.Middleware(authProxy)
	mux.Handle("/", proxyHandler)

	// Apply global middleware: metrics -> logging -> handler
	var handler http.Handler = mux
	handler = loggingMiddleware.Middleware(handler)
	handler = httpMetrics.Middleware(handler)

	// Create main HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      handler,
		ReadTimeout:  cfg.ServerReadTimeout,
		WriteTimeout: cfg.ServerWriteTimeout,
		IdleTimeout:  cfg.ServerIdleTimeout,
	}

	// Configure TLS if enabled
	if cfg.TLSEnabled {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			logger.Logger.Error(logging.EmojiError + " failed to load TLS credentials")
			os.Exit(1)
		}
		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		logger.Logger.Info(logging.EmojiAuth + " TLS enabled")
	}

	// Create metrics server
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
		Handler: promhttp.Handler(),
	}

	// Graceful shutdown handling
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start metrics server
	go func() {
		logger.Startup(fmt.Sprintf("metrics server starting on port %d", cfg.MetricsPort))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Logger.Error(logging.EmojiError + " metrics server error")
		}
	}()

	// Start main HTTP server
	go func() {
		logger.Startup(fmt.Sprintf("HTTP proxy server starting on port %d", cfg.HTTPPort))
		var err error
		if cfg.TLSEnabled {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logger.Logger.Error(logging.EmojiError + " HTTP server error")
			shutdown <- syscall.SIGTERM
		}
	}()

	logger.Startup("auth-proxy HTTP service started successfully")

	// Wait for shutdown signal
	<-shutdown

	logger.Shutdown("shutting down...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown main server
	if err := server.Shutdown(ctx); err != nil {
		logger.Logger.Error(logging.EmojiError + " error shutting down HTTP server")
	} else {
		logger.Logger.Info(logging.EmojiSuccess + " HTTP server stopped gracefully")
	}

	// Shutdown metrics server
	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Logger.Error(logging.EmojiError + " error shutting down metrics server")
	}

	logger.Shutdown("done")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

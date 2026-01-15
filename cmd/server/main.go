package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	authv1 "github.com/company/auth-proxy/api/gen/auth/v1"
	"github.com/company/auth-proxy/internal/attestation"
	"github.com/company/auth-proxy/internal/config"
	"github.com/company/auth-proxy/internal/gotrue"
	"github.com/company/auth-proxy/internal/logging"
	"github.com/company/auth-proxy/internal/metrics"
	"github.com/company/auth-proxy/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := logging.New(cfg.LogLevel, cfg.IsProduction())
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Startup("starting auth-proxy gRPC service")

	// Initialize metrics
	m := metrics.New()
	logger.Logger.Info(logging.EmojiMetrics + " prometheus metrics initialized")

	// Initialize GoTrue client
	gotrueClient := gotrue.NewClient(
		cfg.GoTrueURL,
		cfg.GoTrueAnonKey,
		cfg.GoTrueTimeout,
		logger,
		m,
	)
	logger.Logger.Info(logging.EmojiDatabase + " gotrue client initialized")

	// Initialize attestation verifier (optional)
	attestationVerifier := attestation.NewVerifier(attestation.Config{
		Enabled:            cfg.AttestationEnabled,
		IOSAppID:           cfg.AttestationIOSAppID,
		IOSEnv:             cfg.AttestationIOSEnv,
		AndroidPackageName: cfg.AttestationAndroidPackage,
		AndroidProjectID:   cfg.AttestationAndroidProject,
		AndroidServiceKey:  cfg.AttestationAndroidKey,
	}, logger)

	if cfg.AttestationEnabled {
		logger.Logger.Info(logging.EmojiAuth + " 🔒 app attestation enabled")
	} else {
		logger.Logger.Info(logging.EmojiAuth + " app attestation disabled")
	}

	// Build gRPC server options
	serverOpts := buildServerOptions(cfg, logger, attestationVerifier)

	// Create gRPC server
	grpcServer := grpc.NewServer(serverOpts...)

	// Register services
	authService := service.NewAuthService(gotrueClient, logger, m)
	healthService := service.NewHealthService(gotrueClient, logger)

	authv1.RegisterAuthServiceServer(grpcServer, authService)
	authv1.RegisterHealthServiceServer(grpcServer, healthService)

	// Enable reflection for development
	if !cfg.IsProduction() {
		reflection.Register(grpcServer)
		logger.Logger.Info(logging.EmojiConfig + " gRPC reflection enabled (development mode)")
	}

	// Initialize gRPC Prometheus metrics
	grpcprometheus.Register(grpcServer)

	logger.Logger.Info(logging.EmojiNetwork + " gRPC services registered")

	// Create gRPC listener
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		logger.Logger.Error(logging.EmojiError + fmt.Sprintf(" failed to listen on port %d", cfg.GRPCPort))
		os.Exit(1)
	}

	// Create metrics HTTP server
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
		Handler: promhttp.Handler(),
	}

	// Channel to receive shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start metrics server
	go func() {
		logger.Startup(fmt.Sprintf("metrics server starting on port %d", cfg.MetricsPort))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Logger.Error(logging.EmojiError + " metrics server error")
		}
	}()

	// Start gRPC server
	go func() {
		logger.Startup(fmt.Sprintf("gRPC server starting on port %d", cfg.GRPCPort))
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Logger.Error(logging.EmojiError + " gRPC server error")
			shutdown <- syscall.SIGTERM
		}
	}()

	logger.Startup("auth-proxy gRPC service started successfully")

	// Wait for shutdown signal
	<-shutdown

	logger.Shutdown("shutdown signal received, starting graceful shutdown")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Graceful stop gRPC server
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		logger.Logger.Info(logging.EmojiSuccess + " gRPC server stopped gracefully")
	case <-ctx.Done():
		logger.Logger.Warn(logging.EmojiWarning + " gRPC server forced stop (timeout)")
		grpcServer.Stop()
	}

	// Shutdown metrics server
	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Logger.Error(logging.EmojiError + " error shutting down metrics server")
	}

	logger.Shutdown("graceful shutdown completed successfully")
}

func buildServerOptions(cfg *config.Config, logger *logging.Logger, verifier *attestation.Verifier) []grpc.ServerOption {
	opts := []grpc.ServerOption{
		// Keep-alive settings for long-lived connections
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Minute,
			Time:                  5 * time.Minute,
			Timeout:               1 * time.Minute,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             1 * time.Minute,
			PermitWithoutStream: true,
		}),

		// Connection limits
		grpc.MaxConcurrentStreams(1000),

		// Message size limits
		grpc.MaxRecvMsgSize(4 * 1024 * 1024), // 4MB
		grpc.MaxSendMsgSize(4 * 1024 * 1024), // 4MB
	}

	// Build interceptor chain
	unaryInterceptors := []grpc.UnaryServerInterceptor{
		grpcprometheus.UnaryServerInterceptor,
		loggingUnaryInterceptor(logger),
	}

	// Add attestation interceptor if enabled
	if verifier.IsEnabled() {
		unaryInterceptors = append(unaryInterceptors, attestation.UnaryServerInterceptor(verifier, logger))
	}

	opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))

	// Add TLS if enabled
	if cfg.TLSEnabled {
		creds, err := loadTLSCredentials(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			logger.Logger.Error(logging.EmojiError + " failed to load TLS credentials")
			os.Exit(1)
		}
		opts = append(opts, grpc.Creds(creds))
		logger.Logger.Info(logging.EmojiAuth + " 🔐 TLS enabled")
	}

	return opts
}

func loadTLSCredentials(certFile, keyFile string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load key pair: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(config), nil
}

func loggingUnaryInterceptor(logger *logging.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		logger.Request("gRPC request")

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		if err != nil {
			logger.Logger.Error(logging.EmojiError + " gRPC request failed")
		} else {
			logger.Response("gRPC request completed")
		}

		// Suppress unused variable warning
		_ = duration

		return resp, err
	}
}

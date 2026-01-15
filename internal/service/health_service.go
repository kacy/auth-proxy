package service

import (
	"context"

	authv1 "github.com/company/auth-proxy/api/gen/auth/v1"
	"github.com/company/auth-proxy/internal/gotrue"
	"github.com/company/auth-proxy/internal/logging"
)

// HealthService implements the gRPC HealthService.
type HealthService struct {
	authv1.UnimplementedHealthServiceServer
	client *gotrue.Client
	logger *logging.Logger
}

// NewHealthService creates a new HealthService.
func NewHealthService(client *gotrue.Client, logger *logging.Logger) *HealthService {
	return &HealthService{
		client: client,
		logger: logger,
	}
}

// Check performs a health check.
func (s *HealthService) Check(ctx context.Context, req *authv1.HealthCheckRequest) (*authv1.HealthCheckResponse, error) {
	s.logger.Health("health check requested")

	// Check GoTrue connectivity
	if err := s.client.HealthCheck(ctx); err != nil {
		s.logger.Logger.Error(logging.EmojiError + " health check failed: GoTrue is not healthy")
		return &authv1.HealthCheckResponse{
			Status: authv1.ServingStatus_SERVING_STATUS_NOT_SERVING,
		}, nil
	}

	return &authv1.HealthCheckResponse{
		Status: authv1.ServingStatus_SERVING_STATUS_SERVING,
	}, nil
}

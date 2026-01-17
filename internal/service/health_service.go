package service

import (
	"context"

	authv1 "github.com/kacy/auth-proxy/api/gen/auth/v1"
	"github.com/kacy/auth-proxy/internal/gotrue"
	"github.com/kacy/auth-proxy/internal/logging"
)

type HealthService struct {
	authv1.UnimplementedHealthServiceServer
	client *gotrue.Client
	logger *logging.Logger
}

func NewHealthService(client *gotrue.Client, logger *logging.Logger) *HealthService {
	return &HealthService{
		client: client,
		logger: logger,
	}
}

func (s *HealthService) Check(ctx context.Context, req *authv1.HealthCheckRequest) (*authv1.HealthCheckResponse, error) {
	s.logger.Health("health check")

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

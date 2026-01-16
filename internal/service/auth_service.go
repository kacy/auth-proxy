package service

import (
	"context"
	"strings"

	authv1 "github.com/company/auth-proxy/api/gen/auth/v1"
	"github.com/company/auth-proxy/internal/gotrue"
	"github.com/company/auth-proxy/internal/logging"
	"github.com/company/auth-proxy/internal/metrics"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthService struct {
	authv1.UnimplementedAuthServiceServer
	client  *gotrue.Client
	logger  *logging.Logger
	metrics *metrics.Metrics
}

func NewAuthService(client *gotrue.Client, logger *logging.Logger, m *metrics.Metrics) *AuthService {
	return &AuthService{
		client:  client,
		logger:  logger,
		metrics: m,
	}
}

func (s *AuthService) SignUp(ctx context.Context, req *authv1.SignUpRequest) (*authv1.AuthResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	if !isValidEmail(req.Email) {
		return nil, status.Error(codes.InvalidArgument, "invalid email format")
	}

	if len(req.Password) < 8 {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters")
	}

	s.logger.EmailAuth("processing email signup",
		zap.String("email", maskEmail(req.Email)),
	)

	gotrueReq := &gotrue.SignUpRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	resp, err := s.client.SignUp(ctx, gotrueReq)
	if err != nil {
		s.logger.AuthError("signup failed",
			zap.String("email", maskEmail(req.Email)),
			zap.Error(err),
		)
		return nil, status.Error(codes.Internal, "signup failed: "+err.Error())
	}

	s.logger.AuthSuccess("user signed up successfully",
		zap.String("user_id", resp.User.ID),
		zap.String("email", maskEmail(req.Email)),
	)

	return toProtoAuthResponse(resp), nil
}

func (s *AuthService) SignIn(ctx context.Context, req *authv1.SignInRequest) (*authv1.AuthResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	s.logger.EmailAuth("processing email signin",
		zap.String("email", maskEmail(req.Email)),
	)

	gotrueReq := &gotrue.SignInRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	resp, err := s.client.SignIn(ctx, gotrueReq)
	if err != nil {
		s.logger.AuthError("signin failed",
			zap.String("email", maskEmail(req.Email)),
			zap.Error(err),
		)
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	}

	s.logger.AuthSuccess("user signed in successfully",
		zap.String("user_id", resp.User.ID),
		zap.String("email", maskEmail(req.Email)),
	)

	return toProtoAuthResponse(resp), nil
}

func (s *AuthService) SignInWithGoogle(ctx context.Context, req *authv1.OAuthRequest) (*authv1.AuthResponse, error) {
	if req.IdToken == "" {
		return nil, status.Error(codes.InvalidArgument, "id_token is required")
	}

	s.logger.GoogleAuth("processing Google signin")

	resp, err := s.client.SignInWithOAuth(ctx, "google", req.IdToken, req.Nonce)
	if err != nil {
		s.logger.AuthError("Google signin failed",
			zap.Error(err),
		)
		return nil, status.Error(codes.Unauthenticated, "Google authentication failed")
	}

	s.logger.AuthSuccess("Google signin successful",
		zap.String("user_id", resp.User.ID),
	)

	return toProtoAuthResponse(resp), nil
}

func (s *AuthService) SignInWithApple(ctx context.Context, req *authv1.OAuthRequest) (*authv1.AuthResponse, error) {
	if req.IdToken == "" {
		return nil, status.Error(codes.InvalidArgument, "id_token is required")
	}

	s.logger.AppleAuth("processing Apple signin")

	resp, err := s.client.SignInWithOAuth(ctx, "apple", req.IdToken, req.Nonce)
	if err != nil {
		s.logger.AuthError("Apple signin failed",
			zap.Error(err),
		)
		return nil, status.Error(codes.Unauthenticated, "Apple authentication failed")
	}

	s.logger.AuthSuccess("Apple signin successful",
		zap.String("user_id", resp.User.ID),
	)

	return toProtoAuthResponse(resp), nil
}

func (s *AuthService) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.AuthResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	resp, err := s.client.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		s.logger.AuthError("token refresh failed",
			zap.Error(err),
		)
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}

	s.logger.AuthSuccess("token refreshed successfully",
		zap.String("user_id", resp.User.ID),
	)

	return toProtoAuthResponse(resp), nil
}

func (s *AuthService) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if req.AccessToken == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}

	if err := s.client.Logout(ctx, req.AccessToken); err != nil {
		s.logger.AuthError("logout failed",
			zap.Error(err),
		)
		return nil, status.Error(codes.Internal, "logout failed")
	}

	s.logger.AuthSuccess("user logged out successfully")

	return &authv1.LogoutResponse{
		Message: "Logged out successfully",
	}, nil
}

func toProtoAuthResponse(resp *gotrue.AuthResponse) *authv1.AuthResponse {
	protoResp := &authv1.AuthResponse{
		AccessToken:  resp.AccessToken,
		TokenType:    resp.TokenType,
		ExpiresIn:    int32(resp.ExpiresIn),
		RefreshToken: resp.RefreshToken,
	}

	if resp.User != nil {
		protoResp.User = &authv1.User{
			Id:        resp.User.ID,
			Email:     resp.User.Email,
			CreatedAt: resp.User.CreatedAt,
		}
	}

	return protoResp
}

func isValidEmail(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if len(parts[0]) == 0 || len(parts[1]) < 3 {
		return false
	}
	if !strings.Contains(parts[1], ".") {
		return false
	}
	return true
}

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}

	local := parts[0]
	if len(local) <= 2 {
		return local[0:1] + "***@" + parts[1]
	}
	return local[0:2] + "***@" + parts[1]
}

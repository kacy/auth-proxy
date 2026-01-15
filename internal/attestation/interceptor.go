package attestation

import (
	"context"

	authv1 "github.com/company/auth-proxy/api/gen/auth/v1"
	"github.com/company/auth-proxy/internal/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that verifies attestation.
func UnaryServerInterceptor(verifier *Verifier, logger *logging.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip attestation for health checks
		if info.FullMethod == "/auth.v1.HealthService/Check" {
			return handler(ctx, req)
		}

		// Skip if attestation is disabled
		if !verifier.IsEnabled() {
			return handler(ctx, req)
		}

		// Extract attestation data from the request
		attestationData := extractAttestationData(req)

		// Verify attestation
		if err := verifier.Verify(ctx, attestationData); err != nil {
			logger.AuthError("attestation verification failed",
				zap.String("method", info.FullMethod),
				zap.Error(err),
			)

			switch err {
			case ErrAttestationRequired:
				return nil, status.Error(codes.Unauthenticated, "app attestation required")
			case ErrInvalidAttestation:
				return nil, status.Error(codes.PermissionDenied, "invalid app attestation")
			case ErrUnsupportedPlatform:
				return nil, status.Error(codes.InvalidArgument, "unsupported platform")
			default:
				return nil, status.Error(codes.Internal, "attestation verification failed")
			}
		}

		return handler(ctx, req)
	}
}

// extractAttestationData extracts attestation data from various request types.
func extractAttestationData(req interface{}) *AttestationData {
	// Type switch to handle different request types that may contain attestation
	switch r := req.(type) {
	case *authv1.SignUpRequest:
		return protoAttestationToInternal(r.Attestation)
	case *authv1.SignInRequest:
		return protoAttestationToInternal(r.Attestation)
	case *authv1.OAuthRequest:
		return protoAttestationToInternal(r.Attestation)
	case *authv1.RefreshTokenRequest:
		return protoAttestationToInternal(r.Attestation)
	default:
		return nil
	}
}

// protoAttestationToInternal converts proto attestation to internal type.
func protoAttestationToInternal(proto *authv1.AttestationData) *AttestationData {
	if proto == nil {
		return nil
	}

	var platform Platform
	switch proto.Platform {
	case authv1.Platform_PLATFORM_IOS:
		platform = PlatformIOS
	case authv1.Platform_PLATFORM_ANDROID:
		platform = PlatformAndroid
	default:
		platform = PlatformUnspecified
	}

	return &AttestationData{
		Platform:  platform,
		Token:     proto.Token,
		KeyID:     proto.KeyId,
		Challenge: proto.Challenge,
	}
}

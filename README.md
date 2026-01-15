# Auth Proxy (gRPC)

A production-ready gRPC authentication proxy service written in Go that provides a secure layer on top of Supabase/GoTrue for handling user authentication.

## Configuration Checklist

Before deploying, you **must** update the following placeholder values:

### Required Changes

| File | Placeholder | Replace With |
|------|-------------|--------------|
| `go.mod` | `github.com/company/auth-proxy` | Your Go module path (e.g., `github.com/yourorg/auth-proxy`) |
| `infra/kubernetes/secret.yaml` | `your-gotrue-anon-key-here` | Your GoTrue anonymous key |
| `infra/kubernetes/secret.yaml` | `http://gotrue.internal:9999` | Your GoTrue internal URL |
| `infra/kubernetes/ingress.yaml` | `auth.yourdomain.com` | Your actual domain |
| `infra/kubernetes/cert-manager.yaml` | `auth.yourdomain.com` | Your actual domain |
| `infra/kubernetes/cert-manager.yaml` | `your-email@yourdomain.com` | Your email (for Let's Encrypt alerts) |
| `.env` (local dev) | `your-anon-key-here` | Your GoTrue anonymous key |

### Optional Changes (if using these features)

| File | Placeholder | Replace With | Feature |
|------|-------------|--------------|---------|
| `infra/kubernetes/secret.yaml` | `your-google-client-id` | Google OAuth client ID | Google Sign-In |
| `infra/kubernetes/secret.yaml` | `your-google-client-secret` | Google OAuth secret | Google Sign-In |
| `infra/kubernetes/secret.yaml` | `your-apple-client-id` | Apple OAuth client ID | Apple Sign-In |
| `infra/kubernetes/secret.yaml` | `your-apple-team-id` | Apple Team ID | Apple Sign-In |
| `infra/kubernetes/secret.yaml` | `your-apple-key-id` | Apple Key ID | Apple Sign-In |
| `infra/kubernetes/secret.yaml` | `your-apple-private-key` | Apple private key | Apple Sign-In |
| `infra/kubernetes/secret.yaml` | `TEAMID.com.yourcompany.yourapp` | Your iOS App ID | App Attestation |
| `infra/kubernetes/secret.yaml` | `com.yourcompany.yourapp` | Your Android package | App Attestation |
| `infra/kubernetes/secret.yaml` | `your-gcp-project-id` | GCP project ID | App Attestation |

### Quick Replace Commands

```bash
# Replace module name (run from repo root)
find . -type f -name "*.go" -exec sed -i '' 's|github.com/company/auth-proxy|github.com/yourorg/auth-proxy|g' {} +
sed -i '' 's|github.com/company/auth-proxy|github.com/yourorg/auth-proxy|g' go.mod

# Replace domain in Kubernetes files
sed -i '' 's|auth.yourdomain.com|auth.yourrealdomain.com|g' infra/kubernetes/ingress.yaml
sed -i '' 's|auth.yourdomain.com|auth.yourrealdomain.com|g' infra/kubernetes/cert-manager.yaml

# Replace email
sed -i '' 's|your-email@yourdomain.com|you@yourrealdomain.com|g' infra/kubernetes/cert-manager.yaml
```

> **Note**: On Linux, use `sed -i` instead of `sed -i ''`

---

## Features

- **gRPC API**: High-performance Protocol Buffers-based API
- **Email Authentication**: Sign up and sign in with email/password
- **OAuth Support**: Google and Apple ID authentication
- **App Attestation**: Optional iOS App Attest and Android Play Integrity verification to lock down access to your apps only
- **Production Ready**: Graceful shutdown, horizontal scaling, Prometheus metrics
- **Security**: Input validation, TLS support, request validation
- **Observability**: Structured logging with emojis, Prometheus metrics
- **Kubernetes Ready**: Complete K8s manifests including HPA, PDB, and Network Policies

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Mobile App    │────▶│   Auth Proxy    │────▶│    GoTrue       │
│  (iOS/Android)  │gRPC │   (This Svc)    │HTTP │  (Internal)     │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                       │
        │ Attestation           ▼
        │ Verification   ┌─────────────────┐
        └───────────────▶│   Prometheus    │
                         │   (Metrics)     │
                         └─────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.22+
- Protocol Buffers compiler (`protoc`)
- Docker (optional)
- kubectl & Kubernetes cluster (for deployment)

### Local Development

1. **Clone and setup**:
   ```bash
   git clone <repo-url>
   cd auth-proxy
   ```

2. **Install tools**:
   ```bash
   make install-tools
   ```

3. **Configure environment**:
   ```bash
   cp .env.example .env
   # Edit .env with your GoTrue credentials
   ```

4. **Generate protobuf code** (if needed):
   ```bash
   make proto
   ```

5. **Run locally**:
   ```bash
   make run
   ```

6. **Run tests**:
   ```bash
   make test
   ```

### Docker

```bash
# Build image
make docker-build

# Run container
make docker-run
```

## gRPC API

### Services

#### AuthService

| Method | Description |
|--------|-------------|
| `SignUp` | Create a new user with email/password |
| `SignIn` | Authenticate with email/password |
| `SignInWithGoogle` | Authenticate with Google ID token |
| `SignInWithApple` | Authenticate with Apple ID token |
| `RefreshToken` | Refresh access token |
| `Logout` | Invalidate session |

#### HealthService

| Method | Description |
|--------|-------------|
| `Check` | Health check (includes GoTrue connectivity) |

### Proto Definition

See `api/proto/auth.proto` for the complete service definition.

### Example Usage with grpcurl

```bash
# Health check
grpcurl -plaintext localhost:50051 auth.v1.HealthService/Check

# Sign up
grpcurl -plaintext -d '{"email": "user@example.com", "password": "securepassword123"}' \
  localhost:50051 auth.v1.AuthService/SignUp

# Sign in
grpcurl -plaintext -d '{"email": "user@example.com", "password": "securepassword123"}' \
  localhost:50051 auth.v1.AuthService/SignIn

# Google OAuth
grpcurl -plaintext -d '{"id_token": "your-google-id-token"}' \
  localhost:50051 auth.v1.AuthService/SignInWithGoogle

# With attestation (when enabled)
grpcurl -plaintext -d '{
  "email": "user@example.com",
  "password": "securepassword123",
  "attestation": {
    "platform": "PLATFORM_IOS",
    "token": "your-attestation-token",
    "key_id": "your-key-id",
    "challenge": "your-challenge"
  }
}' localhost:50051 auth.v1.AuthService/SignIn
```

## App Attestation (Optional)

The attestation module allows you to lock down access to only your iOS and Android apps. This is useful for preventing unauthorized clients from using your authentication API.

### How It Works

1. **iOS**: Uses Apple's App Attest API to verify the app's integrity
2. **Android**: Uses Google Play Integrity API to verify the app and device

### Enabling Attestation

1. Set `ATTESTATION_ENABLED=true`
2. Configure platform-specific settings:

**iOS**:
```bash
ATTESTATION_IOS_APP_ID=TEAMID.com.yourcompany.yourapp
ATTESTATION_IOS_ENV=production
```

**Android**:
```bash
ATTESTATION_ANDROID_PACKAGE=com.yourcompany.yourapp
ATTESTATION_ANDROID_PROJECT=your-gcp-project-id
ATTESTATION_ANDROID_KEY=service-account-json
```

### Client Implementation

Your mobile apps need to:
1. Generate an attestation token using the platform's API
2. Include the `attestation` field in each auth request

See the proto definition for the `AttestationData` message structure.

### Disabling Attestation

Simply set `ATTESTATION_ENABLED=false` or remove the environment variable. The attestation interceptor will be bypassed, and all clients will be allowed.

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GOTRUE_URL` | Yes | - | GoTrue service URL |
| `GOTRUE_ANON_KEY` | Yes | - | GoTrue anonymous key |
| `GRPC_PORT` | No | 50051 | gRPC server port |
| `METRICS_PORT` | No | 9090 | Prometheus metrics port |
| `LOG_LEVEL` | No | info | Log level (debug/info/warn/error) |
| `ENVIRONMENT` | No | development | Environment (development/production) |
| `TLS_ENABLED` | No | false | Enable TLS for gRPC |
| `TLS_CERT_FILE` | No | - | TLS certificate file |
| `TLS_KEY_FILE` | No | - | TLS key file |
| `ATTESTATION_ENABLED` | No | false | Enable app attestation |
| `ATTESTATION_IOS_APP_ID` | No | - | iOS App ID (TEAMID.bundle.id) |
| `ATTESTATION_IOS_ENV` | No | production | iOS environment |
| `ATTESTATION_ANDROID_PACKAGE` | No | - | Android package name |
| `ATTESTATION_ANDROID_PROJECT` | No | - | GCP project ID |
| `ATTESTATION_ANDROID_KEY` | No | - | Service account key |

## Kubernetes Deployment

### Prerequisites

1. **NGINX Ingress Controller** installed
2. **cert-manager** installed (for automatic TLS certificates)

### Setup Steps

1. **Install cert-manager** (if not already installed):
   ```bash
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml
   
   # Wait for it to be ready
   kubectl wait --for=condition=Ready pods -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=300s
   ```

2. **Update configuration files**:
   ```bash
   # Edit these files and replace placeholder values:
   
   # 1. Set your domain (replace 'auth.yourdomain.com')
   sed -i 's/auth.yourdomain.com/auth.yourrealdomain.com/g' infra/kubernetes/ingress.yaml
   sed -i 's/auth.yourdomain.com/auth.yourrealdomain.com/g' infra/kubernetes/cert-manager.yaml
   
   # 2. Set your email for Let's Encrypt notifications
   sed -i 's/your-email@yourdomain.com/you@yourrealdomain.com/g' infra/kubernetes/cert-manager.yaml
   
   # 3. Edit secrets with your GoTrue credentials
   vi infra/kubernetes/secret.yaml
   ```

3. **Deploy**:
   ```bash
   # Apply all Kubernetes resources
   make k8s-deploy
   ```

4. **Verify TLS certificate**:
   ```bash
   # Check certificate status
   kubectl get certificate -n auth-proxy
   
   # Should show READY=True after a minute or two
   kubectl describe certificate auth-proxy-tls -n auth-proxy
   ```

### TLS Architecture

```
┌─────────────┐      HTTPS/gRPC      ┌─────────────┐      HTTP/gRPC      ┌─────────────┐
│   Mobile    │─────────────────────▶│   Ingress   │───────────────────▶│  auth-proxy │
│    App      │      (encrypted)     │  (TLS term) │    (internal)      │   (pods)    │
└─────────────┘                      └─────────────┘                    └─────────────┘
                                           │
                                           ▼
                                    ┌─────────────┐
                                    │ cert-manager│
                                    │ (auto-renew)│
                                    └─────────────┘
                                           │
                                           ▼
                                    ┌─────────────┐
                                    │Let's Encrypt│
                                    └─────────────┘
```

- **TLS terminates at the Ingress** - the ingress handles encryption/decryption
- **Internal traffic is unencrypted** - pods communicate over the internal network
- **cert-manager auto-renews** - certificates are renewed 30 days before expiration

### Kubernetes Resources

- **Namespace**: Isolated namespace for the service
- **Deployment**: 3 replicas with rolling updates, gRPC health probes
- **Service**: ClusterIP for internal traffic
- **HPA**: Auto-scaling 3-20 replicas based on CPU/memory
- **PDB**: Minimum 2 pods available during disruptions
- **Ingress**: NGINX ingress with gRPC backend protocol + TLS
- **Certificate**: Let's Encrypt certificate (auto-provisioned by cert-manager)
- **NetworkPolicy**: Restricted ingress/egress traffic
- **ServiceMonitor**: Prometheus Operator integration

## Metrics

Prometheus metrics available at `:9090/metrics`:

- `auth_proxy_grpc_requests_total`: Total gRPC requests
- `auth_proxy_grpc_request_duration_seconds`: Request latency histogram
- `auth_proxy_auth_attempts_total`: Authentication attempts by provider
- `auth_proxy_auth_success_total`: Successful authentications
- `auth_proxy_auth_failures_total`: Failed authentications
- `auth_proxy_attestation_attempts_total`: Attestation verification attempts
- `auth_proxy_attestation_success_total`: Successful attestations
- `auth_proxy_attestation_failures_total`: Failed attestations
- `auth_proxy_gotrue_requests_total`: GoTrue API requests
- `auth_proxy_gotrue_request_duration_seconds`: GoTrue request latency

## Logging

Logs use structured JSON format with emoji prefixes for easy filtering:

| Emoji | Category |
|-------|----------|
| 🚀 | Startup |
| 🛑 | Shutdown |
| 📥 | Request |
| 📤 | Response |
| ✅ | Success |
| ❌ | Error |
| ⚠️ | Warning |
| 🔐 | Auth |
| 📧 | Email Auth |
| 🍎 | Apple Auth |
| 🔷 | Google Auth |
| 💚 | Health |
| 🌐 | Network |
| 📊 | Metrics |

## Make Commands

```bash
make help           # Show all available commands
make build          # Build the binary
make run            # Build and run locally
make proto          # Generate protobuf code
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make lint           # Run linters
make clean          # Clean build artifacts
make docker-build   # Build Docker image
make docker-run     # Run Docker container
make k8s-deploy     # Deploy to Kubernetes
make k8s-delete     # Delete from Kubernetes
make grpc-test      # Test gRPC service with grpcurl
make install-tools  # Install development tools
```

## Security Considerations

- Enable TLS in production for gRPC connections
- Use attestation to lock down access to your apps only
- Store secrets using a secrets manager (Vault, AWS Secrets Manager, etc.)
- Keep GoTrue internal and not publicly accessible
- Review and adjust NetworkPolicy for your environment

## Troubleshooting

### Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n auth-proxy

# Check certificate is issued
kubectl get certificate -n auth-proxy
# Should show: READY = True

# Check ingress has an IP
kubectl get ingress -n auth-proxy

# View logs
kubectl logs -l app=auth-proxy -n auth-proxy --tail=100

# Test gRPC endpoint (requires grpcurl)
grpcurl auth.yourdomain.com:443 auth.v1.HealthService/Check
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Certificate not ready | DNS not pointing to ingress | Ensure DNS A record points to ingress IP |
| Certificate stuck pending | cert-manager can't reach Let's Encrypt | Check cert-manager logs: `kubectl logs -n cert-manager -l app=cert-manager` |
| Connection refused | Service not running | Check pod status: `kubectl get pods -n auth-proxy` |
| TLS handshake failure | Wrong domain or expired cert | Verify domain matches certificate: `kubectl describe certificate -n auth-proxy` |
| `transport: authentication handshake failed` | Client not using TLS | Ensure client connects with TLS enabled |
| Attestation failures | Missing or invalid attestation config | Check `ATTESTATION_*` env vars in secret |

### Verify TLS Certificate

```bash
# Check certificate details
echo | openssl s_client -connect auth.yourdomain.com:443 2>/dev/null | openssl x509 -noout -dates -subject

# Should output something like:
# notBefore=Jan 15 00:00:00 2024 GMT
# notAfter=Apr 15 00:00:00 2024 GMT
# subject=CN = auth.yourdomain.com
```

### Local Development without TLS

For local development, you can skip TLS:

```bash
# Run server
make run

# Test with grpcurl (plaintext)
grpcurl -plaintext localhost:50051 auth.v1.HealthService/Check
```

## License

MIT

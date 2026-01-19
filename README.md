# Auth Proxy

HTTP reverse proxy for Supabase Auth (GoTrue). Sits in front of your Supabase project and gives you fine-grained control over authentication requests with optional device attestation.

## What's in the box

- HTTP reverse proxy that forwards requests to Supabase Auth API
- API key validation - clients must prove they have your app's Supabase config
- Optional app attestation (iOS App Attest / Android Play Integrity) to lock things down
- Request/response logging with fine-grained control
- Prometheus metrics, structured logging
- K8s manifests with HPA, PDB, network policies, cert-manager integration

## How it fits together

```
Any Client ──HTTP──▶ Auth Proxy ──HTTP──▶ Supabase Auth
     │                    │
     │                    ├──▶ Prometheus metrics
     │                    └──▶ Request logging
     │
     └── attestation verification (optional)
```

Your clients make the same HTTP requests they would to Supabase directly, but through your proxy. You get full visibility and control.

## Getting started

You'll need Go 1.22+. Docker and kubectl if you're deploying.

```bash
git clone <repo-url>
cd auth-proxy

cp .env.example .env    # fill in your Supabase creds
make run                # fire it up
make test               # run the tests
```

Or with Docker:

```bash
make docker-build
make docker-run
```

## API

The proxy forwards all requests to Supabase Auth's REST API. Just point your client at the proxy instead of `https://your-project.supabase.co`.

### Quick test with curl

```bash
# health check (no API key required)
curl http://localhost:8080/health

# sign up - note the apikey header with your Supabase anon key
curl -X POST http://localhost:8080/auth/v1/signup \
  -H "Content-Type: application/json" \
  -H "apikey: your-supabase-anon-key" \
  -d '{"email": "user@example.com", "password": "securepassword123"}'

# sign in
curl -X POST http://localhost:8080/auth/v1/token?grant_type=password \
  -H "Content-Type: application/json" \
  -H "apikey: your-supabase-anon-key" \
  -d '{"email": "user@example.com", "password": "securepassword123"}'

# OAuth (Google)
curl -X POST http://localhost:8080/auth/v1/token?grant_type=id_token \
  -H "Content-Type: application/json" \
  -H "apikey: your-supabase-anon-key" \
  -d '{"provider": "google", "id_token": "your-google-id-token"}'

# refresh token
curl -X POST http://localhost:8080/auth/v1/token?grant_type=refresh_token \
  -H "Content-Type: application/json" \
  -H "apikey: your-supabase-anon-key" \
  -d '{"refresh_token": "your-refresh-token"}'
```

## API Key Validation

By default, the proxy requires clients to send the Supabase anon key in the `apikey` header (matching Supabase's expected format). This ensures that only clients with your app's configuration can use the proxy.

The proxy compares the provided key against `GOTRUE_ANON_KEY` using constant-time comparison to prevent timing attacks.

To disable (not recommended for production):
```bash
REQUIRE_API_KEY=false
```

## App Attestation

If you want to make sure only your actual apps can hit this API (not some random script), turn on attestation. It uses Apple's App Attest on iOS and Google Play Integrity on Android.

Set `ATTESTATION_ENABLED=true` and configure the platform(s) you care about:

```bash
# iOS
ATTESTATION_IOS_BUNDLE_ID=com.yourcompany.yourapp
ATTESTATION_IOS_TEAM_ID=YOURTEAMID

# Android
ATTESTATION_ANDROID_PACKAGE=com.yourcompany.yourapp
ATTESTATION_GCP_PROJECT_ID=your-gcp-project-id
ATTESTATION_GCP_CREDENTIALS_FILE=/path/to/service-account.json
ATTESTATION_REQUIRE_STRONG_INTEGRITY=false  # optional, require hardware-backed attestation
```

### Attestation Headers

When attestation is enabled, clients must include these headers:

**Initial attestation:**
```
X-Platform: ios (or android)
X-Attestation: <attestation-token>
X-Attestation-Key-ID: <key-id>
X-Attestation-Challenge: <challenge>
```

**iOS assertions (subsequent requests):**
```
X-Attestation-Assertion: <assertion>
X-Attestation-Key-ID: <key-id>
X-Attestation-Client-Data: <client-data>
```

### Getting a Challenge

Before attesting, clients need to get a challenge:

```bash
curl -X POST http://localhost:8080/attestation/challenge \
  -H "Content-Type: application/json" \
  -d '{"identifier": "device-unique-id"}'
```

### Example with attestation

```bash
curl -X POST http://localhost:8080/auth/v1/token?grant_type=password \
  -H "Content-Type: application/json" \
  -H "apikey: your-supabase-anon-key" \
  -H "X-Platform: ios" \
  -H "X-Attestation: <your-attestation-token>" \
  -H "X-Attestation-Key-ID: <your-key-id>" \
  -H "X-Attestation-Challenge: <your-challenge>" \
  -d '{"email": "user@example.com", "password": "securepassword123"}'
```

### iOS Assertions (Ongoing Verification)

After the initial attestation, iOS apps can use **assertions** for subsequent requests. Assertions are signed by the attested device key and include a counter to prevent replay attacks. The server stores the public key after initial attestation and uses it to verify all future assertions.

### Multi-Instance Deployments (Redis)

If you're running multiple server instances with attestation enabled, you need Redis to share attestation state (challenges and iOS key storage):

```bash
REDIS_ENABLED=true
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=your-password
REDIS_DB=0
REDIS_KEY_PREFIX=authproxy:
```

Without Redis, each instance has its own in-memory stores, which will cause attestation failures when requests hit different instances.

Don't need it? Just leave `ATTESTATION_ENABLED` unset or false.

## Config

| Variable | Default | What it does |
|----------|---------|--------------|
| `GOTRUE_URL` | required | Supabase project URL (e.g., https://xxx.supabase.co) |
| `GOTRUE_ANON_KEY` | required | Supabase anon/public key |
| `HTTP_PORT` | 8080 | HTTP server port |
| `METRICS_PORT` | 9090 | Prometheus port |
| `LOG_LEVEL` | info | debug/info/warn/error |
| `LOG_REQUEST_BODIES` | false | Log request/response bodies (careful with sensitive data) |
| `ENVIRONMENT` | development | development or production |
| `REQUIRE_API_KEY` | true | Require `apikey` header matching GOTRUE_ANON_KEY |
| `TLS_ENABLED` | false | Turn on TLS |
| `TLS_CERT_FILE` | - | Cert file path |
| `TLS_KEY_FILE` | - | Key file path |
| `ATTESTATION_ENABLED` | false | Require app attestation |
| `ATTESTATION_IOS_BUNDLE_ID` | - | iOS bundle ID (com.company.app) |
| `ATTESTATION_IOS_TEAM_ID` | - | Apple Developer Team ID |
| `ATTESTATION_ANDROID_PACKAGE` | - | Android package name |
| `ATTESTATION_GCP_PROJECT_ID` | - | GCP project for Play Integrity |
| `ATTESTATION_GCP_CREDENTIALS_FILE` | - | Path to service account JSON |
| `ATTESTATION_REQUIRE_STRONG_INTEGRITY` | false | Require hardware-backed Android attestation |
| `ATTESTATION_CHALLENGE_TIMEOUT` | 5m | How long challenges remain valid |
| `REDIS_ENABLED` | false | Use Redis for distributed attestation state |
| `REDIS_ADDR` | localhost:6379 | Redis server address |
| `REDIS_PASSWORD` | - | Redis password |
| `REDIS_DB` | 0 | Redis database number |
| `REDIS_KEY_PREFIX` | authproxy: | Prefix for Redis keys |

## Deploying to Kubernetes

You'll need NGINX Ingress and cert-manager installed.

```bash
# install cert-manager if you haven't
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml
kubectl wait --for=condition=Ready pods -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=300s

# update the placeholders (domain, email, secrets)
sed -i 's/auth.yourdomain.com/auth.yourrealdomain.com/g' infra/kubernetes/ingress.yaml infra/kubernetes/cert-manager.yaml
sed -i 's/your-email@yourdomain.com/you@yourrealdomain.com/g' infra/kubernetes/cert-manager.yaml
vi infra/kubernetes/secret.yaml  # add your Supabase creds

# deploy
make k8s-deploy

# check that the cert is ready
kubectl get certificate -n auth-proxy  # should show READY=True after a minute
```

TLS terminates at the ingress, so internal pod traffic is plaintext. cert-manager handles renewals automatically.

The manifests include: namespace, deployment (3 replicas), HPA (scales 3-20), PDB (min 2 available), ingress, network policies, and a ServiceMonitor for Prometheus.

## Deploying with Helm (Recommended)

For production deployments, use the Helm chart which provides more flexibility and easier configuration management.

### Prerequisites

- Helm 3.x
- NGINX Ingress Controller
- cert-manager (optional, for automatic TLS)

### Install from OCI Registry (Recommended)

The Helm chart is published to GitHub Container Registry on each release:

```bash
# Install latest version
helm install auth-proxy oci://ghcr.io/kacy/auth-proxy \
  -f /path/to/your/values.yaml \
  --set secrets.gotrueUrl=https://your-project.supabase.co \
  --set secrets.gotrueAnonKey=your-anon-key \
  -n auth-proxy --create-namespace

# Install specific version
helm install auth-proxy oci://ghcr.io/kacy/auth-proxy --version 0.1.0 \
  -f /path/to/your/values.yaml \
  -n auth-proxy --create-namespace

# Upgrade
helm upgrade auth-proxy oci://ghcr.io/kacy/auth-proxy \
  -f /path/to/your/values.yaml \
  -n auth-proxy
```

### Install from Source

If you've cloned the repo or want to customize the chart:

```bash
# Add your values and install
helm install auth-proxy ./infra/helm/auth-proxy \
  --set secrets.gotrueUrl=https://your-project.supabase.co \
  --set secrets.gotrueAnonKey=your-anon-key \
  --set ingress.host=auth.yourdomain.com \
  --set certManager.issuer.email=you@yourdomain.com \
  -n auth-proxy --create-namespace
```

### Production Deployment

Use the production values file as a starting point. Store your values in a separate repo for GitOps workflows:

```bash
# Download the example production values
curl -O https://raw.githubusercontent.com/kacy/auth-proxy/main/infra/helm/auth-proxy/values-production.yaml

# Edit with your settings
vi values-production.yaml

# Deploy from OCI registry with your values
helm install auth-proxy oci://ghcr.io/kacy/auth-proxy \
  -f values-production.yaml \
  --set secrets.gotrueUrl=https://your-project.supabase.co \
  --set secrets.gotrueAnonKey=your-anon-key \
  --set secrets.redisPassword=your-redis-password \
  -n auth-proxy --create-namespace
```

### Using External Secrets

Instead of passing secrets via `--set`, use an external secrets manager:

```bash
# Create secret externally (e.g., with External Secrets Operator, Vault, etc.)
kubectl create secret generic auth-proxy-secrets \
  --from-literal=GOTRUE_URL=https://... \
  --from-literal=GOTRUE_ANON_KEY=... \
  -n auth-proxy

# Install chart using existing secret
helm install auth-proxy oci://ghcr.io/kacy/auth-proxy \
  -f values-production.yaml \
  --set existingSecret=auth-proxy-secrets \
  -n auth-proxy --create-namespace
```

### Upgrading

```bash
helm upgrade auth-proxy oci://ghcr.io/kacy/auth-proxy \
  -f values-production.yaml \
  -n auth-proxy
```

### Uninstalling

```bash
helm uninstall auth-proxy -n auth-proxy
```

### Key Configuration Options

| Parameter | Default | Description |
|-----------|---------|-------------|
| `replicaCount` | 3 | Number of replicas |
| `image.repository` | auth-proxy | Container image |
| `image.tag` | latest | Image tag |
| `ingress.enabled` | true | Enable ingress |
| `ingress.host` | auth.yourdomain.com | Ingress hostname |
| `ingress.tls.enabled` | true | Enable TLS |
| `autoscaling.enabled` | true | Enable HPA |
| `autoscaling.minReplicas` | 3 | Min replicas for HPA |
| `autoscaling.maxReplicas` | 20 | Max replicas for HPA |
| `networkPolicy.enabled` | true | Enable network policies |
| `serviceMonitor.enabled` | true | Enable Prometheus ServiceMonitor |
| `certManager.enabled` | true | Enable cert-manager integration |
| `config.attestation.enabled` | false | Enable device attestation |
| `config.redis.enabled` | false | Enable Redis for distributed state |
| `existingSecret` | "" | Use existing secret instead of creating one |

See `values.yaml` for all available options.

## Metrics

Hit `:9090/metrics` for Prometheus. You get: request counts, latencies, response sizes, auth attempts, attestation stats, and upstream metrics.

## Logging

Structured JSON logs with emoji prefixes so you can grep for specific things:

- startup, shutdown
- request, response  
- success, error, warning
- auth, email, apple, google
- health, network, metrics

## Make targets

`make help` shows everything, but the main ones: `build`, `run`, `test`, `lint`, `docker-build`, `docker-run`, `k8s-deploy`, `k8s-delete`, `http-test`.

## Security notes

- Turn on TLS in prod
- Use attestation if you want to block scripts/bots
- Don't commit secrets - use a secrets manager
- Keep Supabase URL private if possible
- Review the NetworkPolicy for your setup

## Troubleshooting

```bash
kubectl get pods -n auth-proxy           # are pods running?
kubectl get certificate -n auth-proxy    # is cert ready?
kubectl get ingress -n auth-proxy        # does ingress have an IP?
kubectl logs -l app=auth-proxy -n auth-proxy --tail=100
curl https://auth.yourdomain.com/health
```

**Cert not ready?** Make sure DNS points to the ingress IP. Check cert-manager logs if it's stuck.

**Connection refused?** Pods probably aren't running. Check `kubectl get pods`.

**TLS handshake failed?** Either the domain doesn't match the cert, or the client isn't using TLS.

**Attestation failing?** Double-check your `ATTESTATION_*` env vars.

```bash
# check your cert
echo | openssl s_client -connect auth.yourdomain.com:443 2>/dev/null | openssl x509 -noout -dates -subject
```

For local dev, just use `curl http://localhost:8080/health`

## License

MIT

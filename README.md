# Auth Proxy

gRPC auth service that sits in front of GoTrue/Supabase. Handles email/password, Google, and Apple sign-in for your mobile apps.

## Before You Start

You'll need to swap out placeholder values in a few files:

**Must change:**
- `go.mod` - update `github.com/company/auth-proxy` to your module path
- `infra/kubernetes/secret.yaml` - add your GoTrue URL and anon key
- `infra/kubernetes/ingress.yaml` - set your domain
- `infra/kubernetes/cert-manager.yaml` - set your domain and email
- `.env` - add your GoTrue anon key for local dev

**Only if you need them:**
- Google OAuth creds in `secret.yaml`
- Apple OAuth creds in `secret.yaml`  
- App attestation config (iOS App ID, Android package, GCP project)

Quick sed commands to swap the module name and domain:

```bash
# module name
find . -type f -name "*.go" -exec sed -i '' 's|github.com/company/auth-proxy|github.com/yourorg/auth-proxy|g' {} +
sed -i '' 's|github.com/company/auth-proxy|github.com/yourorg/auth-proxy|g' go.mod

# domain
sed -i '' 's|auth.yourdomain.com|auth.yourrealdomain.com|g' infra/kubernetes/ingress.yaml infra/kubernetes/cert-manager.yaml
```

(On Linux, drop the `''` after `-i`)

---

## What's in the box

- gRPC API with email auth, Google, and Apple sign-in
- Optional app attestation (iOS App Attest / Android Play Integrity) if you want to lock things down
- Prometheus metrics, structured logging
- K8s manifests with HPA, PDB, network policies, cert-manager integration

## How it fits together

```
Mobile App ──gRPC──▶ Auth Proxy ──HTTP──▶ GoTrue (internal)
     │                    │
     │                    └──▶ Prometheus
     │
     └── attestation verification (optional)
```

## Getting started

You'll need Go 1.22+ and `protoc`. Docker and kubectl if you're deploying.

```bash
git clone <repo-url>
cd auth-proxy

make install-tools      # grab protoc plugins etc
cp .env.example .env    # fill in your GoTrue creds
make proto              # generate pb code (if needed)
make run                # fire it up
make test               # run the tests
```

Or with Docker:

```bash
make docker-build
make docker-run
```

## API

**AuthService** - `SignUp`, `SignIn`, `SignInWithGoogle`, `SignInWithApple`, `RefreshToken`, `Logout`

**HealthService** - `Check` (pings GoTrue to make sure it's up)

Full proto def is in `api/proto/auth.proto`.

### Quick test with grpcurl

```bash
# health check
grpcurl -plaintext localhost:50051 auth.v1.HealthService/Check

# sign up
grpcurl -plaintext -d '{"email": "user@example.com", "password": "securepassword123"}' \
  localhost:50051 auth.v1.AuthService/SignUp

# sign in
grpcurl -plaintext -d '{"email": "user@example.com", "password": "securepassword123"}' \
  localhost:50051 auth.v1.AuthService/SignIn

# google
grpcurl -plaintext -d '{"id_token": "your-google-id-token"}' \
  localhost:50051 auth.v1.AuthService/SignInWithGoogle

# with attestation
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

Your apps will need to generate attestation tokens and pass them in the `attestation` field on each request. Check the proto for the `AttestationData` structure.

Don't need it? Just leave `ATTESTATION_ENABLED` unset or false.

## Config

| Variable | Default | What it does |
|----------|---------|--------------|
| `GOTRUE_URL` | required | Where GoTrue lives |
| `GOTRUE_ANON_KEY` | required | GoTrue anon key |
| `GRPC_PORT` | 50051 | gRPC port |
| `METRICS_PORT` | 9090 | Prometheus port |
| `LOG_LEVEL` | info | debug/info/warn/error |
| `ENVIRONMENT` | development | development or production |
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

## Deploying to Kubernetes

You'll need NGINX Ingress and cert-manager installed.

```bash
# install cert-manager if you haven't
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml
kubectl wait --for=condition=Ready pods -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=300s

# update the placeholders (domain, email, secrets)
sed -i 's/auth.yourdomain.com/auth.yourrealdomain.com/g' infra/kubernetes/ingress.yaml infra/kubernetes/cert-manager.yaml
sed -i 's/your-email@yourdomain.com/you@yourrealdomain.com/g' infra/kubernetes/cert-manager.yaml
vi infra/kubernetes/secret.yaml  # add your GoTrue creds

# deploy
make k8s-deploy

# check that the cert is ready
kubectl get certificate -n auth-proxy  # should show READY=True after a minute
```

TLS terminates at the ingress, so internal pod traffic is plaintext. cert-manager handles renewals automatically.

The manifests include: namespace, deployment (3 replicas), HPA (scales 3-20), PDB (min 2 available), ingress, network policies, and a ServiceMonitor for Prometheus.

## Metrics

Hit `:9090/metrics` for Prometheus. You get the usual stuff: request counts, latencies, auth attempts/successes/failures, attestation stats, and GoTrue upstream metrics.

## Logging

Structured JSON logs with emoji prefixes so you can grep for specific things:

- 🚀 startup, 🛑 shutdown
- 📥 request, 📤 response  
- ✅ success, ❌ error, ⚠️ warning
- 🔐 auth, 📧 email, 🍎 apple, 🔷 google
- 💚 health, 🌐 network, 📊 metrics

## Make targets

`make help` shows everything, but the main ones: `build`, `run`, `test`, `proto`, `lint`, `docker-build`, `docker-run`, `k8s-deploy`, `k8s-delete`.

## Security notes

- Turn on TLS in prod
- Use attestation if you want to block scripts/bots
- Don't commit secrets - use a secrets manager
- Keep GoTrue internal (not public)
- Review the NetworkPolicy for your setup

## Troubleshooting

```bash
kubectl get pods -n auth-proxy           # are pods running?
kubectl get certificate -n auth-proxy    # is cert ready?
kubectl get ingress -n auth-proxy        # does ingress have an IP?
kubectl logs -l app=auth-proxy -n auth-proxy --tail=100
grpcurl auth.yourdomain.com:443 auth.v1.HealthService/Check
```

**Cert not ready?** Make sure DNS points to the ingress IP. Check cert-manager logs if it's stuck.

**Connection refused?** Pods probably aren't running. Check `kubectl get pods`.

**TLS handshake failed?** Either the domain doesn't match the cert, or the client isn't using TLS.

**Attestation failing?** Double-check your `ATTESTATION_*` env vars.

```bash
# check your cert
echo | openssl s_client -connect auth.yourdomain.com:443 2>/dev/null | openssl x509 -noout -dates -subject
```

For local dev, just skip TLS and use `grpcurl -plaintext localhost:50051 ...`

## License

MIT

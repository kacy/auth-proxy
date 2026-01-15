.PHONY: all build run test lint clean docker docker-build docker-run proto help

# Build variables
BINARY_NAME=auth-proxy
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO=go
GOFLAGS=-ldflags="-w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Docker variables
DOCKER_IMAGE=auth-proxy
DOCKER_TAG=$(VERSION)

# Default target
all: lint test build

## help: Show this help message
help:
	@echo "Auth Proxy (gRPC) - Make Targets"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'

## build: Build the binary
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME) ./cmd/server

## run: Run the application locally
run: build
	@echo "🚀 Starting $(BINARY_NAME)..."
	./bin/$(BINARY_NAME)

## run-dev: Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
run-dev:
	@echo "🔄 Starting $(BINARY_NAME) with hot reload..."
	air

## proto: Generate Go code from proto files
proto:
	@echo "📝 Generating protobuf code..."
	@mkdir -p api/gen/auth/v1
	protoc --go_out=api/gen --go_opt=paths=source_relative \
		--go-grpc_out=api/gen --go-grpc_opt=paths=source_relative \
		-Iapi/proto api/proto/auth.proto
	@echo "✅ Protobuf code generated"

## proto-buf: Generate using buf (alternative to proto target)
proto-buf:
	@echo "📝 Generating protobuf code with buf..."
	buf generate
	@echo "✅ Protobuf code generated"

## test: Run all tests
test:
	@echo "🧪 Running tests..."
	CGO_ENABLED=0 $(GO) test -v ./...

## test-coverage: Run tests and show coverage report
test-coverage:
	@echo "🧪 Running tests with coverage..."
	CGO_ENABLED=0 $(GO) test -coverprofile=coverage.out ./...
	@echo "📊 Generating coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## lint: Run linters
lint:
	@echo "🔍 Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, running go vet only..."; \
		$(GO) vet ./...; \
	fi

## fmt: Format code
fmt:
	@echo "✨ Formatting code..."
	$(GO) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

## tidy: Tidy go modules
tidy:
	@echo "📦 Tidying modules..."
	$(GO) mod tidy

## clean: Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	$(GO) clean

## docker-build: Build Docker image
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

## docker-run: Run Docker container locally
docker-run:
	@echo "🐳 Running Docker container..."
	docker run --rm -p 50051:50051 -p 9090:9090 \
		-e GOTRUE_URL=http://host.docker.internal:9999 \
		-e GOTRUE_ANON_KEY=your-anon-key \
		$(DOCKER_IMAGE):latest

## docker-push: Push Docker image to registry
docker-push:
	@echo "📤 Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

## k8s-deploy: Deploy to Kubernetes
k8s-deploy:
	@echo "☸️  Deploying to Kubernetes..."
	kubectl apply -f infra/kubernetes/

## k8s-delete: Delete from Kubernetes
k8s-delete:
	@echo "☸️  Deleting from Kubernetes..."
	kubectl delete -f infra/kubernetes/

## k8s-logs: View logs from Kubernetes
k8s-logs:
	@echo "📜 Viewing logs..."
	kubectl logs -f -l app=auth-proxy --all-containers=true -n auth-proxy

## grpc-test: Test gRPC service with grpcurl
grpc-test:
	@echo "🧪 Testing gRPC service..."
	grpcurl -plaintext localhost:50051 auth.v1.HealthService/Check

## install-tools: Install development tools
install-tools:
	@echo "🛠️  Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/cosmtrek/air@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

## check: Run all checks (lint, test, build)
check: lint test build
	@echo "✅ All checks passed!"

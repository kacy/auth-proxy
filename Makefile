.PHONY: all build run test lint clean docker docker-build docker-run proto help

BINARY_NAME=auth-proxy
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO=go
GOFLAGS=-ldflags="-w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"
DOCKER_IMAGE=auth-proxy
DOCKER_TAG=$(VERSION)

all: lint test build

help:
	@echo "make [target]"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'

## build: compile binary
build:
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME) ./cmd/server

## run: build and run
run: build
	./bin/$(BINARY_NAME)

## run-dev: hot reload (needs air)
run-dev:
	air

## proto: generate from proto files
proto:
	@mkdir -p api/gen/auth/v1
	protoc --go_out=api/gen --go_opt=paths=source_relative \
		--go-grpc_out=api/gen --go-grpc_opt=paths=source_relative \
		-Iapi/proto api/proto/auth.proto

## proto-buf: generate with buf
proto-buf:
	buf generate

## test: run tests
test:
	CGO_ENABLED=0 $(GO) test -v ./...

## test-coverage: tests + coverage html
test-coverage:
	CGO_ENABLED=0 $(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

## lint: run linters
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		$(GO) vet ./...; \
	fi

## fmt: format code
fmt:
	$(GO) fmt ./...
	@if command -v goimports >/dev/null 2>&1; then goimports -w .; fi

## tidy: go mod tidy
tidy:
	$(GO) mod tidy

## clean: remove build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html
	$(GO) clean

## docker-build: build image
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

## docker-run: run container
docker-run:
	docker run --rm -p 50051:50051 -p 9090:9090 \
		-e GOTRUE_URL=http://host.docker.internal:9999 \
		-e GOTRUE_ANON_KEY=your-anon-key \
		$(DOCKER_IMAGE):latest

## docker-push: push image
docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

## k8s-deploy: apply k8s manifests
k8s-deploy:
	kubectl apply -f infra/kubernetes/

## k8s-delete: remove from k8s
k8s-delete:
	kubectl delete -f infra/kubernetes/

## k8s-logs: tail pod logs
k8s-logs:
	kubectl logs -f -l app=auth-proxy --all-containers=true -n auth-proxy

## grpc-test: health check via grpcurl
grpc-test:
	grpcurl -plaintext localhost:50051 auth.v1.HealthService/Check

## install-tools: install dev dependencies
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/cosmtrek/air@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

## check: lint + test + build
check: lint test build

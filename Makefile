.PHONY: all build run test lint clean docker docker-build docker-run help helm-lint helm-template helm-package helm-push

BINARY_NAME=auth-proxy
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO=go
GOFLAGS=-ldflags="-w -s -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"
DOCKER_IMAGE=auth-proxy
DOCKER_TAG=$(VERSION)

# Helm configuration
HELM_CHART_PATH=infra/helm/auth-proxy
HELM_REGISTRY=ghcr.io/kacy
HELM_CHART_VERSION=$(shell grep '^version:' $(HELM_CHART_PATH)/Chart.yaml | awk '{print $$2}')

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
	docker run --rm -p 8080:8080 -p 9090:9090 \
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

## http-test: health check via curl
http-test:
	curl -s http://localhost:8080/health | jq .

## install-tools: install dev dependencies
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/air-verse/air
	go install golang.org/x/tools/cmd/goimports@latest

## check: lint + test + build
check: lint test build

# ============================================
# Helm
# ============================================

## helm-lint: lint helm chart
helm-lint:
	helm lint $(HELM_CHART_PATH)

## helm-template: render templates locally
helm-template:
	helm template auth-proxy $(HELM_CHART_PATH)

## helm-template-prod: render with production values
helm-template-prod:
	helm template auth-proxy $(HELM_CHART_PATH) -f $(HELM_CHART_PATH)/values-production.yaml

## helm-package: package chart for distribution
helm-package:
	helm package $(HELM_CHART_PATH) --destination .helm-packages

## helm-push: push chart to GHCR (requires: docker login ghcr.io)
helm-push: helm-package
	helm push .helm-packages/auth-proxy-$(HELM_CHART_VERSION).tgz oci://$(HELM_REGISTRY)

## helm-install: install chart locally (for testing)
helm-install:
	helm install auth-proxy $(HELM_CHART_PATH) \
		--set secrets.gotrueUrl=http://localhost:9999 \
		--set secrets.gotrueAnonKey=test-key \
		--set ingress.enabled=false \
		--set certManager.enabled=false \
		-n auth-proxy --create-namespace --dry-run

## helm-install-prod: install from GHCR with production values
helm-install-prod:
	@echo "Usage: make helm-install-prod VALUES_FILE=/path/to/values.yaml"
	@echo ""
	@echo "Example:"
	@echo "  helm install auth-proxy oci://$(HELM_REGISTRY)/auth-proxy \\"
	@echo "    -f /path/to/your/values.yaml \\"
	@echo "    --set secrets.gotrueUrl=https://your-gotrue.example.com \\"
	@echo "    --set secrets.gotrueAnonKey=your-anon-key \\"
	@echo "    -n auth-proxy --create-namespace"

## helm-release: create and push a helm release tag
helm-release:
	@if [ -z "$(V)" ]; then \
		echo "Usage: make helm-release V=0.1.0"; \
		exit 1; \
	fi
	git tag -a helm-v$(V) -m "Helm chart release $(V)"
	git push origin helm-v$(V)
	@echo "Tagged helm-v$(V) - GitHub Actions will publish to GHCR"

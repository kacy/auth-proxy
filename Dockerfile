FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /auth-proxy \
    ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

COPY --from=builder /auth-proxy /usr/local/bin/auth-proxy
RUN chown appuser:appgroup /usr/local/bin/auth-proxy

USER appuser
EXPOSE 50051 9090
ENTRYPOINT ["/usr/local/bin/auth-proxy"]

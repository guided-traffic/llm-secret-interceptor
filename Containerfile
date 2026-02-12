# Build stage
FROM golang:1.26-alpine AS builder

ARG BUILD_NUMBER=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=0

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w -X main.Version=${BUILD_NUMBER} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o llm-secret-interceptor \
    ./cmd/proxy

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="LLM Secret Interceptor"
LABEL org.opencontainers.image.description="HTTPS proxy that intercepts and masks secrets in LLM communications"
LABEL org.opencontainers.image.vendor="LLM Secret Interceptor"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/llm-secret-interceptor /app/llm-secret-interceptor

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Create certs directory
# Note: Certificates should be mounted as volumes
VOLUME ["/app/certs", "/app/config.yaml"]

# Expose ports
# 8080: Proxy server
# 9090: Metrics endpoint
EXPOSE 8080 9090

# Set environment variables
ENV CONFIG_PATH=/app/config.yaml

# Run as non-root user (provided by distroless:nonroot)
USER nonroot:nonroot

ENTRYPOINT ["/app/llm-secret-interceptor"]

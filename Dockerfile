# Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version info
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /build/llm-secret-interceptor \
    ./cmd/proxy

# Final stage - minimal image
FROM alpine:3.19

# Install ca-certificates for HTTPS connections
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S proxy && \
    adduser -u 1000 -S proxy -G proxy

# Create directories
RUN mkdir -p /app/certs /app/configs && \
    chown -R proxy:proxy /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/llm-secret-interceptor /app/

# Copy example config
COPY --chown=proxy:proxy configs/config.example.yaml /app/configs/config.yaml

# Switch to non-root user
USER proxy

# Expose ports
# 8080 - Proxy server
# 9090 - Metrics/Health server
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

# Run the proxy
ENTRYPOINT ["/app/llm-secret-interceptor"]

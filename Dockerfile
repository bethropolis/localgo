# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy module files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# CGO_ENABLED=0 ensures a static binary
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo "docker") -X main.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") -X main.BuildDate=$(date -u +'%Y-%m-%d_%H:%M:%S')" -o localgo-cli ./cmd/localgo-cli

# Runtime Stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (ca-certificates for HTTPS)
RUN apk add --no-cache ca-certificates tzdata

# Create a non-root user
RUN addgroup -S localgo && adduser -S localgo -G localgo

# Copy binary from builder
COPY --from=builder /app/localgo-cli /usr/local/bin/localgo-cli

# Create directory structure and set permissions
RUN mkdir -p /app/downloads /app/.localgo_security && \
    chown -R localgo:localgo /app

# Switch to non-root user
USER localgo

# Expose ports (TCP and UDP for discovery)
EXPOSE 53317/tcp
EXPOSE 53317/udp

# Set environment variables
ENV LOCALSEND_DOWNLOAD_DIR="/app/downloads"
ENV LOCALSEND_SECURITY_DIR="/app/.localgo_security"
ENV LOCALSEND_ALIAS="LocalGo-Docker"

# Command to run
CMD ["localgo-cli", "serve"]

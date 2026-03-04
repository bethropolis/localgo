# Build Stage
FROM golang:1.24-alpine AS builder

# Build arguments for version information
ARG VERSION=docker
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy module files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with version information
# CGO_ENABLED=0 ensures a static binary
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w \
    -X main.Version=${VERSION} \
    -X main.GitCommit=${GIT_COMMIT} \
    -X main.BuildDate=${BUILD_DATE}" \
    -o localgo ./cmd/localgo

# Runtime Stage
FROM alpine:3.21

LABEL org.opencontainers.image.title="LocalGo"
LABEL org.opencontainers.image.description="LocalSend v2.1 Protocol Implementation in Go"
LABEL org.opencontainers.image.url="https://github.com/bethropolis/localgo"
LABEL org.opencontainers.image.source="https://github.com/bethropolis/localgo"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.license="MIT"
LABEL org.opencontainers.image.created="${BUILD_DATE}"

WORKDIR /app

# Install runtime dependencies:
# 1. ca-certificates for HTTPS
# 2. tzdata for timezones
# 3. su-exec for stepping down from root to localgo user
# 4. wget for healthcheck
RUN apk add --no-cache ca-certificates tzdata su-exec wget

# create the user
RUN adduser -D -u 1000 -h /app localgo

# Copy binary from builder
COPY --from=builder /app/localgo /usr/local/bin/localgo

# Copy entrypoint script
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Create directory structure with proper XDG-compliant paths
RUN mkdir -p \
    /app/downloads \
    /app/config/.security

# Expose ports (TCP and UDP for discovery)
EXPOSE 53317/tcp
EXPOSE 53317/udp

# Set environment variables with XDG-compliant paths
ENV LOCALSEND_DOWNLOAD_DIR="/app/downloads" \
    LOCALSEND_SECURITY_DIR="/app/config/.security" \
    LOCALSEND_ALIAS="LocalGo-Docker" \
    LOCALSEND_PORT="53317" \
    LOCALSEND_AUTO_ACCEPT="true"

# Graceful shutdown signal
STOPSIGNAL SIGTERM

# Health check using HTTP endpoint (faster than CLI which loads config)
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:53317/api/localsend/v2/info || exit 1

# Use entrypoint script to fix permissions (runs as root)
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

# Command to run
CMD ["localgo", "serve"]

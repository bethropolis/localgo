# Runtime Stage
FROM alpine:3.21

LABEL org.opencontainers.image.title="LocalGo"
LABEL org.opencontainers.image.description="LocalSend v2.1 Protocol Implementation in Go"
LABEL org.opencontainers.image.url="https://github.com/bethropolis/localgo"
LABEL org.opencontainers.image.source="https://github.com/bethropolis/localgo"
LABEL org.opencontainers.image.license="MIT"
LABEL org.opencontainers.image.vendor="Bethropolis"
LABEL org.opencontainers.image.ref.name="localgo"
LABEL com.centurylinklabs.watchtower.enable="true"

WORKDIR /app

# Install runtime dependencies:
RUN apk add --no-cache ca-certificates tzdata su-exec

# create the user
RUN adduser -D -u 1000 -h /app localgo

# Copy binary for the correct platform compiled by GoReleaser
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/localgo /usr/local/bin/localgo

# Copy entrypoint script (referenced in extra_files)
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

# Health check using the native localgo health check tool
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["localgo", "health"]

# Use entrypoint script to fix permissions (runs as root)
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

# Command to run
CMD ["localgo", "serve"]

# Podman build file for LocalGo
# Usage: podman build -f Containerfile -t localgo .
#
# This file provides the same functionality as Dockerfile but with
# Podman-specific notes. For most use cases, you can also run:
#   podman build -t localgo .
#
# Rootless Podman Notes:
# - Use --userns=keep-id to preserve your user ID mapping
# - Use --network=host for multicast discovery to work on Linux
# - On macOS/Windows, Podman runs in a VM so ports must be mapped explicitly

FROM golang:1.24-alpine AS builder

ARG VERSION=podman
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /app

RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w \
    -X main.Version=${VERSION} \
    -X main.GitCommit=${GIT_COMMIT} \
    -X main.BuildDate=${BUILD_DATE}" \
    -o localgo ./cmd/localgo

FROM alpine:3.21

LABEL org.opencontainers.image.title="LocalGo"
LABEL org.opencontainers.image.description="LocalSend v2.1 Protocol Implementation in Go"
LABEL org.opencontainers.image.url="https://github.com/bethropolis/localgo"
LABEL org.opencontainers.image.source="https://github.com/bethropolis/localgo"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.license="MIT"

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata su-exec

RUN adduser -D -u 1000 -h /app localgo

COPY --from=builder /app/localgo /usr/local/bin/localgo

COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

RUN mkdir -p \
    /app/downloads \
    /app/config/.security

EXPOSE 53317/tcp
ENV LOCALSEND_DOWNLOAD_DIR="/app/downloads" \
    LOCALSEND_SECURITY_DIR="/app/config/.security" \
    LOCALSEND_ALIAS="LocalGo-Podman" \
    LOCALSEND_PORT="53317"

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:53317/api/localsend/v2/info || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["localgo", "serve"]

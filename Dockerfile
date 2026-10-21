# syntax=docker/dockerfile:1.4
# Multi-stage production build for AOSphere BOS

# ----------------- Stage 1: Build -----------------
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Install CA certificates and build essentials
RUN apk add --no-cache ca-certificates tzdata git

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile statically linked binary with stripped debug symbols
# pure Go SQLite (modernc.org/sqlite) builds with CGO_ENABLED=0
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -o /bin/aosphere .

# ----------------- Stage 2: Production Runtime -----------------
FROM alpine:3.21 AS runtime

# Create unprivileged service user and application directories
RUN addgroup -g 10001 -S aosphere && \
    adduser -u 10001 -S aosphere -G aosphere && \
    mkdir -p /var/lib/aosphere/data /etc/aosphere && \
    chown -R aosphere:aosphere /var/lib/aosphere /etc/aosphere

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app

# Copy binary and configuration templates
COPY --from=builder /bin/aosphere /app/aosphere
COPY --from=builder /src/config /etc/aosphere/config

# Run as non-root user
USER 10001:10001

# Volumes and ports
VOLUME ["/var/lib/aosphere/data"]
EXPOSE 8080

# Environment defaults
ENV AOSPHERE_ENV=production \
    AOSPHERE_PORT=8080 \
    AOSPHERE_DB_PATH=/var/lib/aosphere/data/aosphere.db \
    AOSPHERE_SEED=false \
    AOSPHERE_LOG_LEVEL=info

# Health check probe against readyz endpoint
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/readyz || exit 1

ENTRYPOINT ["/app/aosphere"]

# syntax=docker/dockerfile:1.4
# Multi-stage minimal distroless production build for AO Sphere BOS (< 25MB total footprint)

# ----------------- Stage 1: Build -----------------
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Install build essentials
RUN apk add --no-cache ca-certificates tzdata git

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and assets
COPY . .

# Compile statically linked binary with stripped debug symbols (-s -w)
# pure Go SQLite (modernc.org/sqlite) builds with CGO_ENABLED=0
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -o /bin/aosphere .

# ----------------- Stage 2: Distroless Non-Root Runtime -----------------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy binary, web dashboard assets, and configs
COPY --from=builder /bin/aosphere /app/aosphere
COPY --from=builder /src/web /app/web
COPY --from=builder /src/config /etc/aosphere/config

# Run as nonroot user (UID 65532:65532)
USER nonroot:nonroot

# Persistent data volumes and network port
VOLUME ["/var/lib/aosphere/data"]
EXPOSE 8080

# Environment defaults
ENV AOSPHERE_ENV=production \
    AOSPHERE_PORT=8080 \
    AOSPHERE_DB_PATH=/var/lib/aosphere/data/aosphere.db \
    AOSPHERE_SEED=false \
    AOSPHERE_LOG_LEVEL=info

ENTRYPOINT ["/app/aosphere"]

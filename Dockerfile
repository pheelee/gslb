# Multi-stage Dockerfile for GSLB (Go backend + Vue frontend)
# Build: docker build -t gslb:latest .
# Run:   docker run -p 8090:8090 -v gslb-data:/data gslb:latest

# -----------------------------------------------------------------------------
# Stage 1: Build Frontend
# -----------------------------------------------------------------------------
FROM node:22-alpine AS frontend-builder

WORKDIR /build/web

# Copy package files first for better caching
COPY web/package.json web/package-lock.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY web/ ./

# Build production bundle
RUN npm run build

# -----------------------------------------------------------------------------
# Stage 2: Build Backend
# -----------------------------------------------------------------------------
FROM golang:1.26-alpine AS backend-builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go module files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend from previous stage
COPY --from=frontend-builder /build/web/dist ./web/dist

# Build the binary (statically linked for Alpine)
# -ldflags="-s -w" strips debug info for smaller binary
# CGO_ENABLED=0 creates a static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo 'docker')" \
    -o gslb \
    ./cmd/gslb

# -----------------------------------------------------------------------------
# Stage 3: Production Image
# -----------------------------------------------------------------------------
FROM alpine:3.21

# Import CA certificates and timezone data from builder
COPY --from=backend-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend-builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary (owned by root, not writable by the runtime user)
COPY --from=backend-builder /build/gslb /gslb

# Create a non-root user, set up data dir, and embed cap_net_raw on the binary.
#
# cap_net_raw+ep lets the unprivileged gslb user open raw ICMP sockets (needed
# for ICMP health checks).  The capability is stored as an xattr on the binary
# itself; libcap is only needed at image-build time to write that xattr.
#
# NOTE: in Docker/Podman deployments that enforce no-new-privileges the kernel
# ignores file capabilities.  Grant the capability at the container level with
# --cap-add=NET_RAW (or cap_add in docker-compose) if ICMP health checks are used.
RUN apk add --no-cache libcap && \
    setcap cap_net_raw+ep /gslb && \
    apk del libcap && \
    adduser -D -u 1000 -s /sbin/nologin gslb && \
    mkdir -p /data && \
    chown gslb:gslb /data

USER gslb

# Expose the default port
EXPOSE 8090

# Volume for persistent data (SQLite database)
VOLUME ["/data"]

# Set the working directory for data persistence
WORKDIR /data

# Health check — verifies the HTTP server is actually responding
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8090/health || exit 1

# Run the application
# Config path defaults to /data/gslb.yaml, can be overridden
ENTRYPOINT ["/gslb"]
CMD ["-config", "/data/gslb.yaml"]

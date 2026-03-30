# Multi-stage build for HSM Service
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    softhsm \
    openssl \
    build-base \
    git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build main service with version from VERSION file
RUN HSM_VERSION="$(cat VERSION)" && \
    CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-X main.version=${HSM_VERSION}" \
    -o hsm-service .

# Build admin CLI
RUN cd cmd/hsm-admin && CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o ../../hsm-admin .


# Runtime image
FROM alpine:3.23

# Install runtime dependencies
RUN apk add --no-cache \
    softhsm \
    openssl \
    opensc \
    ca-certificates

# Create necessary directories
RUN mkdir -p /var/lib/softhsm/tokens \
    && mkdir -p /app/pki \
    && mkdir -p /app/data/tokens

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/hsm-service .
COPY --from=builder /app/hsm-admin .

# Copy configuration files
COPY config.yaml .
COPY softhsm2.conf /etc/softhsm/softhsm2.conf

# Copy init script
COPY scripts/init-hsm.sh .
RUN chmod +x init-hsm.sh

# Expose HTTPS port
EXPOSE 8443

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider https://localhost:8443/health || exit 1

# Run init script which will start the service
ENTRYPOINT ["/app/init-hsm.sh"]

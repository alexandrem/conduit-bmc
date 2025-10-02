# Manager Service Dockerfile
# Multi-stage build with development and production targets

# Base stage with common dependencies
FROM golang:1.25-alpine AS base
RUN apk add --no-cache \
    git \
    curl \
    build-base

# Development build stage with hot reloading tools
FROM base AS build
# Install Go development tools
RUN go install github.com/air-verse/air@latest && \
    go install github.com/bufbuild/buf/cmd/buf@latest && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

# Set working directory to workspace root
WORKDIR /workspace

# Copy the entire workspace first (needed for local module dependencies)
COPY . .

# Download dependencies from manager directory
WORKDIR /workspace/manager
RUN go mod download

# Create tmp directory for Air
RUN mkdir -p tmp

# Set environment variables
ENV CGO_ENABLED=0
ENV GOOS=linux

# Expose manager port
EXPOSE 8080

# Copy entrypoint script
COPY docker/scripts/manager-entrypoint.sh /usr/local/bin/manager-entrypoint.sh
RUN chmod +x /usr/local/bin/manager-entrypoint.sh

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=5 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command with Air hot reloading via entrypoint
CMD ["/usr/local/bin/manager-entrypoint.sh"]

# Production stage with minimal runtime
FROM base AS production
WORKDIR /workspace

# Copy the entire workspace for module dependencies
COPY . .

# Build the manager binary
WORKDIR /workspace/manager
RUN go mod download && \
    go build -ldflags="-w -s" -o /usr/local/bin/manager ./cmd/manager

# Create non-root user
RUN adduser -D -s /bin/sh manager
USER manager

# Expose manager port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=5 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the binary
CMD ["/usr/local/bin/manager"]
# Local Agent Service Dockerfile
# Multi-stage build with development and production targets

# Base stage with common dependencies
FROM golang:1.25-alpine AS base
# Install system dependencies:
# - git, curl, build-base: Build toolchain
# - freeipmi: IPMI Serial-over-LAN (SOL) console support via ipmiconsole subprocess
# - ipmitool: IPMI power management via ipmitool subprocess (more reliable than go-ipmi library)
RUN apk add --no-cache \
    git \
    curl \
    build-base \
    freeipmi \
    ipmitool

# Development build stage with hot reloading tools
FROM base AS build
# Install Go development tools
RUN go install github.com/air-verse/air@latest

# Set working directory to workspace root
WORKDIR /workspace

# Copy the entire workspace first (needed for local module dependencies)
COPY . .

# Download dependencies from local-agent directory
WORKDIR /workspace/local-agent
RUN go mod download

# Create tmp directory for Air
RUN mkdir -p tmp

# Set environment variables
ENV CGO_ENABLED=0
ENV GOOS=linux

# Expose local agent port
EXPOSE 8082

# Copy entrypoint script
COPY docker/scripts/local-agent-entrypoint.sh /usr/local/bin/local-agent-entrypoint.sh
RUN chmod +x /usr/local/bin/local-agent-entrypoint.sh

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=5 \
    CMD curl -f http://localhost:8082/health || exit 1

# Default command with Air hot reloading via entrypoint
CMD ["/usr/local/bin/local-agent-entrypoint.sh"]

# Production stage with minimal runtime
FROM base AS production
WORKDIR /workspace

# Copy the entire workspace for module dependencies
COPY . .

# Build the local-agent binary
WORKDIR /workspace/local-agent
RUN go mod download && \
    go build -ldflags="-w -s" -o /usr/local/bin/local-agent ./cmd/agent

# Create non-root user
RUN adduser -D -s /bin/sh agent
USER agent

# Expose local agent port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=5 \
    CMD curl -f http://localhost:8082/health || exit 1

# Run the binary
CMD ["/usr/local/bin/local-agent"]
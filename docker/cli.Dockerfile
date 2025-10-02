# CLI Dockerfile
# Multi-stage build with development and production targets

# Base stage with common dependencies
FROM golang:1.25-alpine AS base
RUN apk add --no-cache \
    git \
    curl \
    build-base \
    bash \
    ipmitool \
    make

# Development build stage for interactive development
FROM base AS build
# Set working directory to workspace root
WORKDIR /workspace

# Copy the entire workspace first (needed for local module dependencies)
COPY . .

# Download dependencies from cli directory
WORKDIR /workspace/cli
RUN go mod download

# Create tmp directory
RUN mkdir -p tmp

# Set environment variables
ENV CGO_ENABLED=0
ENV GOOS=linux

# Set up shell environment
RUN echo 'alias ll="ls -la"' >> /root/.bashrc && \
    echo 'alias bmc="go run ."' >> /root/.bashrc && \
    echo 'echo "🚀 CLI development container ready. Use: go run . server list"' >> /root/.bashrc

# Default command to keep container running for exec access
CMD ["tail", "-f", "/dev/null"]

# Production stage with minimal runtime
FROM base AS production
WORKDIR /workspace

# Copy the entire workspace for module dependencies
COPY . .

# Build the CLI binary
WORKDIR /workspace/cli
RUN go mod download && \
    go build -ldflags="-w -s" -o /usr/local/bin/bmc-cli .

# Create non-root user
RUN adduser -D -s /bin/sh bmcuser
USER bmcuser

# Set up shell environment for production user
USER root
RUN echo 'alias ll="ls -la"' >> /home/bmcuser/.bashrc && \
    echo 'alias bmc="bmc-cli"' >> /home/bmcuser/.bashrc && \
    echo 'echo "🚀 BMC CLI ready. Use: bmc-cli server list"' >> /home/bmcuser/.bashrc
USER bmcuser

# Default command provides interactive shell
CMD ["bash"]
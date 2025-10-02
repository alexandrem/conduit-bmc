# ----------------------
# Base stage with common dependencies
# ----------------------
FROM golang:1.25-alpine AS base
RUN apk add --no-cache git curl build-base

# ----------------------
# Dev stage (hot reload, no source copied)
# ----------------------
FROM base AS dev

# Install dev tools
RUN go install github.com/air-verse/air@latest \
    && go install github.com/bufbuild/buf/cmd/buf@latest \
    && go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

WORKDIR /workspace

# Copy only go.mod/go.sum for caching modules
COPY gateway/go.mod gateway/go.sum ./gateway/
RUN cd gateway

# Create Air tmp directory (will be mounted as a volume)
RUN mkdir -p /workspace/gateway/tmp

# Set env for static builds if needed
ENV CGO_ENABLED=0
ENV GOOS=linux

# Copy entrypoint script
COPY docker/scripts/gateway-entrypoint.sh /usr/local/bin/gateway-entrypoint.sh
RUN chmod +x /usr/local/bin/gateway-entrypoint.sh

# Expose port
EXPOSE 8081

# Default command runs Air via entrypoint
CMD ["/usr/local/bin/gateway-entrypoint.sh"]

# ----------------------
# Build stage (static binary)
# ----------------------
FROM base AS build

WORKDIR /workspace

# Copy full source
COPY . .

# Build static binary
WORKDIR /workspace/gateway
RUN go mod download && \
    go build -ldflags="-w -s" -o /usr/local/bin/gateway ./cmd/gateway

# Minimal runtime
FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates curl

# Non-root user
RUN adduser -D -s /bin/sh gateway
USER gateway

# Copy binary from build stage
COPY --from=build /usr/local/bin/gateway /usr/local/bin/gateway

EXPOSE 8081
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=5 \
    CMD curl -f http://localhost:8081/health || exit 1

CMD ["/usr/local/bin/gateway"]

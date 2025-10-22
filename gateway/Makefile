.PHONY: gen build run test clean deps lint fmt

# Generate protobuf code
gen:
	buf generate

# Build binary
build: gen
	go build -o bin/gateway ./cmd/gateway

# Run the gateway
run: build
	./bin/gateway

# Test
test:
	go test ./...

# Clean
clean:
	rm -rf bin/ gen/

# Install dependencies
deps:
	go mod download
	go mod tidy

# Lint
lint:
	golangci-lint run

# Format code
fmt:
	goimports -w .

# Build for different platforms
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/regional-gateway-linux-amd64 ./cmd/gateway

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o bin/regional-gateway-darwin-amd64 ./cmd/gateway

build-all: build-linux build-darwin

# Docker build
docker-build:
	docker build -t regional-gateway:latest .

# Create bin directory
bin:
	mkdir -p bin

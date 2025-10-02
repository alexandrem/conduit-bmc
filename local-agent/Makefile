.PHONY: build run test clean deps lint fmt

# Build binary
build:
	go build -o bin/local-agent ./cmd/agent

# Run the local agent
run: build
	./bin/local-agent

# Test
test:
	go test ./... -v

# Clean
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod download
	go mod tidy

# Lint
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Create bin directory
bin:
	mkdir -p bin

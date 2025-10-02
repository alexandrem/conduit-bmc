.PHONY: build run test clean deps fmt

# Build CLI binary
build:
	go build -o bin/bmc-cli .

# Run CLI (show help)
run: build
	./bin/bmc-cli

# Test
test:
	go test ./...

# Clean
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Install CLI globally
install: build
	cp bin/bmc-cli $(GOPATH)/bin/

# Example commands for development
examples:
	@echo "Example CLI commands:"
	@echo "  ./bin/bmc-cli server list"
	@echo "  ./bin/bmc-cli server show server-123"
	@echo "  ./bin/bmc-cli server power on server-123"
	@echo "  ./bin/bmc-cli server power status server-123"
	@echo "  ./bin/bmc-cli server console server-123"
	@echo "  ./bin/bmc-cli server proxy server-123"
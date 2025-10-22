.PHONY: gen build run test clean proto

# Generate protobuf code
gen:
	buf generate

# Build all binaries
build: gen
	go build -o bin/manager ./cmd/manager

# Run BMC Manager
run: build
	./bin/manager

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

# Install protobuf tools
install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
	go install github.com/bufbuild/buf/cmd/buf@latest

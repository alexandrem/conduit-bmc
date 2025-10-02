.PHONY: build build-all run-all test clean deps help

ROOT := $(realpath $(dir $(lastword $(MAKEFILE_LIST))))

# Include modular makefiles
include tooling/make/Makefile.dev
include tooling/make/Makefile.bmc
include tooling/make/Makefile.e2e

help:
	@echo "BMC Management System - Monorepo Build"
	@echo ""
	@echo "Available targets:"
	@echo "  build-all           Build all components"
	@echo "  build-manager       Build Manager (Gateway + BMC Manager)"
	@echo "  build-gateway       Build Gateway"
	@echo "  build-local-agent   Build Local Agent"
	@echo "  build-cli           Build CLI"
	@echo "  clean-all           Clean all build artifacts"
	@echo "  deps-all            Update dependencies for all components"
	@echo "  lint-all            Lint all components"
	@echo ""
	@echo "Development Environments:"
	@echo "  dev-help            Show Docker development environment commands"
	@echo "  bmc-help            Show BMC development and testing commands"
	@echo ""
	@echo "Tests:"
	@echo "  test-all            Run unit tests for all components"
	@echo "  test-smoke          Run smoke tests (quick confidence checks)"
	@echo "  test-e2e-help       Show E2E test commands"


# Build all components
build-all: build-manager build-gateway build-local-agent build-cli

# Build individual components
build-manager:
	@echo "Building Manager (Gateway + BMC Manager)..."
	cd manager && $(MAKE) build

build-gateway:
	@echo "Building Gateway..."
	cd gateway && $(MAKE) build

build-local-agent:
	@echo "Building Local Agent..."
	cd local-agent && $(MAKE) build

build-cli:
	@echo "Building CLI..."
	cd cli && $(MAKE) build

# Test all components
test-all:
	@echo "Testing Manager..."
	cd manager && $(MAKE) test
	@echo "Testing Gateway..."
	cd gateway && $(MAKE) test
	@echo "Testing Local Agent..."
	cd local-agent && $(MAKE) test
	@echo "Testing CLI..."
	cd cli && $(MAKE) test
	#@echo "Running smoke tests..."
	#$(MAKE) test-smoke

# Run smoke tests only
test-smoke:
	@echo "Running smoke tests (quick confidence checks)..."
	cd tests && go test -v ./smoke/...

# Legacy alias for smoke tests
test-functional: test-smoke

# Clean all components
clean-all:
	@echo "Cleaning Manager..."
	cd manager && $(MAKE) clean
	@echo "Cleaning Gateway..."
	cd gateway && $(MAKE) clean
	@echo "Cleaning Local Agent..."
	cd local-agent && $(MAKE) clean
	@echo "Cleaning CLI..."
	cd cli && $(MAKE) clean

# Update dependencies for all components
deps-all:
	@echo "Updating Manager dependencies..."
	cd manager && $(MAKE) deps
	@echo "Updating Gateway dependencies..."
	cd gateway && $(MAKE) deps
	@echo "Updating Local Agent dependencies..."
	cd local-agent && $(MAKE) deps
	@echo "Updating CLI dependencies..."
	cd cli && $(MAKE) deps

# Lint all components
lint-all:
	@echo "Linting Manager..."
	cd manager && $(MAKE) lint
	@echo "Linting Gateway..."
	cd gateway && $(MAKE) lint
	@echo "Linting Local Agent..."
	cd local-agent && $(MAKE) lint
	@echo "Linting CLI..."
	cd cli && $(MAKE) lint

# Development quick start
dev-setup: deps-all build-all
	@echo ""
	@echo "Development environment set up successfully!"
	@echo ""
	@echo "Quick start commands:"
	@echo "  make run-all     # Start all services"
	@echo "  make test-all    # Run all tests"
	@echo "  make clean-all   # Clean everything"

# Production build
prod-build: clean-all deps-all lint-all test-all build-all
	@echo ""
	@echo "Production build completed successfully!"
	@echo ""
	@echo "Binaries location:"
	@echo "  Manager:         manager/bin/gateway + manager/bin/bmc-manager"
	@echo "  Gateway:         gateway/bin/gateway"
	@echo "  Local Agent:     local-agent/bin/local-agent"
	@echo "  CLI:             cli/bin/bmc-cli"


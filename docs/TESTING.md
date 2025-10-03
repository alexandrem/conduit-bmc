# Testing Overview

This document provides a comprehensive guide to the BMC Management System's
testing strategy and organization.

## Testing Taxonomy

Our testing is organized into three distinct tiers, each with clear
responsibilities and scope:

```
tests/
├── smoke/         # Fast confidence checks (< 5 seconds)
└── e2e/          # Full system verification against real/simulated BMCs

# Reserved for future expansion:
└── integration/   # Planned service-to-service validation tier
└── contract/      # Planned API/schema compatibility tests
```

## Test Tier Responsibilities

### Smoke Tests (`tests/smoke/`)

- **Purpose**: Ultra-fast checks developers can run before pushing
- **Scope**: Configuration validation, basic service initialization, unit-level
  checks
- **Constraints**: No network calls, no long-lived services, no external
  dependencies
- **Runtime**: < 5 seconds total
- **When to run**: Before every commit, in pre-push hooks

**Examples**:

- Environment configuration validation
- Service startup and health checks
- Basic API response format validation

### Integration Tests (`tests/integration/` - Planned)

- **Purpose**: Validate cross-service behavior with controlled dependencies
- **Status**: Reserved for future work. Today, these scenarios are covered in
  the end-to-end suite while the dedicated integration tier is being designed.

### End-to-End Tests (`tests/e2e/`)

- **Purpose**: Represent complete user journeys against real or high-fidelity
  simulated systems
- **Scope**: Full stack through real network paths, actual BMC interactions
- **Constraints**: Real or simulated BMC backends, complete environment setup
- **Runtime**: 5-15 minutes
- **When to run**: Nightly builds, release validation

**Examples**:

- Complete power management workflows
- Console access scenarios
- Multi-tenant isolation verification
- Performance benchmarks

## Quick Start

### Running Tests by Tier

```bash
# Smoke tests – run before every commit
make test-smoke

# E2E tests – orchestrated VirtualBMC run (Docker required)
make test-e2e

# Component unit tests – run per service (example for manager)
cd manager && make test
```

### Running Specific Test Suites

Use standard Go tooling when you need to focus on specific suites:

```bash
# Run smoke tests in a single file
go test ./tests/smoke -run TestEnvironmentConfiguration

# Run a specific E2E suite
cd tests && E2E_TEST_CONFIG=configs/default.yaml go test ./e2e/suites/power -run TestPowerOperations
```

## Test Organization

### Directory Structure

- `tests/smoke/`: Fast tests
- `tests/e2e/framework/`: Shared testing harness and utilities
- `tests/e2e/suites/`: Organized by scenario (auth, power, console, performance)
- `tests/e2e/backends/`: Backend-specific setup and simulation helpers
- `tests/integration/`: Reserved for upcoming mid-tier tests

### Naming Conventions

- Test files: `<scenario>_test.go`
- Suite directories: `tests/e2e/suites/<domain>/`
- Shared code: `tests/e2e/framework/` or `tests/e2e/backends/<backend>/`

## Backend Support

### Available Backends

- **Synthetic**: HTTP-based Redfish simulation for development (low fidelity)
- **VirtualBMC**: Docker-based IPMI simulation (default for E2E runs)
- **Real Hardware**: Actual BMC endpoints (optional)

### Backend Selection

By default, `make test-e2e` runs against the containerized VirtualBMC backend.

> NOTE: This will be replaced by containerized OpenBMC.

## Adding New Tests

### Where Should My Test Go?

1. **Smoke**: Configuration validation, basic service checks
2. **Integration**: Service-to-service interaction, authentication flows
3. **E2E**: Complete user workflows, BMC interactions

### Test Development Workflow

1. Identify the appropriate tier based on scope and dependencies
2. Create test in `tests/<tier>/suites/<domain>/`
3. Use existing framework and fixtures where possible
4. Update documentation and CI configuration

## Configuration

### Test Configurations

- Default configs in `tests/e2e/configs/`
- Backend-specific configs in `tests/e2e/backends/<backend>/configs/`
- Environment overrides via environment variables

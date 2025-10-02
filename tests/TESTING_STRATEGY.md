# BMC Management System - Testing Strategy

## Overview

This document outlines the comprehensive testing strategy for the BMC
Management System.

## Test Pyramid Structure

```
                    E2E Tests (Few)
                  /                \
        (Planned) Integration Tests (Some)
          /                          \
    Unit Tests (Many)          Contract Tests (Planned)
```

## Test Categories

### 1. Unit Tests ✅ (Implemented)

**Location**: `*/internal/*/` directories
**Purpose**: Test individual components in isolation
**Coverage**: Authentication, server management, BMC operations

**Examples**:

- `manager/internal/manager/email_auth_test.go` - Email-based authentication
- `manager/internal/manager/manager_handlers_test.go` - Connect RPC handlers
- Individual service logic validation

### 2. Integration Tests 🔄 (Planned)

**Location**: `tests/integration/` (reserved)
**Purpose**: Validate cross-service behavior with controlled dependencies
**Status**: In design. Current end-to-end coverage lives in `tests/e2e/` while
the team builds a leaner integration tier.

### 3. Smoke Tests ✅

**Location**: `tests/smoke/`
**Purpose**: Ultra-fast confidence checks with no external dependencies
**Coverage**: Configuration validation, service bootstrapping, basic helpers

### 4. Contract Tests 📋 (Planned)

**Location**: `tests/contract/` (reserved)
**Purpose**: Verify API compatibility between services once schemas stabilize

## 🆕 **Test Infrastructure**

1. **TestEnvironment Framework**
	- Automated service startup/shutdown
	- Synthetic BMC server management
	- Cross-service health checking
	- Failure simulation capabilities

2. **Real Service Integration**
	- Tests use actual Go binaries, not mocks
	- Real database interactions
	- Actual network communication
	- True multi-service orchestration

## Test Execution Strategy

### Development Testing

```bash
# Smoke tests (quick feedback)
make test-smoke

# Full E2E run (Docker VirtualBMC)
make test-e2e

# Full component suite (manager, gateway, agent, cli + smoke)
make test-all
```

### CI/CD Pipeline (Recommended)

```yaml
stages:
	- unit-tests      # Service-level unit tests via make test
	- smoke           # make test-smoke (every PR)
	- end-to-end      # make test-e2e (nightly / release)
	- performance     # make test-e2e-load (scheduled)
```

### Test Categories by Speed

- **Fast** (< 5s): Smoke tests (`tests/smoke`)
- **Medium** (minutes): Service unit tests executed via each component's
  `make test-all`
- **Slow** (5–15 min): End-to-end suites (`tests/e2e/suites`)

## Test Data Management

### Test Isolation

- Each test gets fresh database
- Unique ports for parallel execution
- Temporary directories for test artifacts
- Complete service restart between test suites

### Synthetic Test Services

- HTTP Redfish mock (`tests/e2e/backends/synthetic`) with configurable responses
- Config-driven power state simulation
- Lightweight auth hooks for negative testing

## Quality Gates

### Success Criteria

- ✅ All tests pass consistently
- ✅ No flaky tests (< 1% failure rate)
- ✅ Tests run in < 5 minutes total
- ✅ Clear failure diagnostics

## Monitoring & Reporting (planned)

### Test Metrics

- Test execution time trends
- Test failure rates by category
- Code coverage evolution
- Performance regression detection

### Test Reports

- JUnit XML for CI integration
- Coverage reports in HTML format
- Performance trend charts
- Failure analysis with logs

## Future Enhancements

### 1. Performance Testing

- Load testing with multiple concurrent users
- Stress testing under high BMC operation volume
- Memory and CPU usage profiling

### 2. Chaos Engineering

- Random service failures during operation
- Network partition simulation
- Database corruption recovery

### 3. Security Testing

- Authentication bypass attempts
- Authorization boundary testing
- Token manipulation validation

### 4. Browser-based E2E Tests

- Web console interface testing (when implemented)
- Cross-browser compatibility
- User interaction flows

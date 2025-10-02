# Smoke Tests

Fast confidence checks that can be run before every commit.

## Purpose

Smoke tests provide ultra-fast validation of basic system functionality without external dependencies. These tests act as a safety net during development and should complete in under 5 seconds total.

## Scope

✅ **Include:**
- Configuration validation
- Basic service initialization
- Environment setup verification
- Unit-level API response format checks
- Object creation and validation

❌ **Exclude:**
- Network calls to external services
- Long-lived service dependencies
- Database interactions
- BMC endpoint connections
- Multi-service integration

## Running Smoke Tests

```bash
# Run all smoke tests from repo root
make test-smoke

# Run a specific test directly
cd tests && go test ./smoke -run TestEnvironmentConfiguration
```

## Test Files

- `environment_config_test.go` - Environment and configuration validation
- `local_agent_test.go` - Local agent service initialization
- `integration_test.go` - Basic integration setup verification

## Performance Target

All smoke tests combined should complete in **under 5 seconds**. If tests are taking longer, consider:

1. Moving to the future `tests/integration/` tier if they require cross-service interaction
2. Moving to `tests/e2e/` if they require full system validation
3. Optimizing test setup/teardown
4. Removing external dependencies

## Writing Smoke Tests

### Good Smoke Test Example
```go
func TestConfigurationValidation(t *testing.T) {
    config, err := LoadConfig()
    require.NoError(t, err)

    assert.NotEmpty(t, config.ManagerEndpoint)
    assert.True(t, config.Timeout > 0)
}
```

### Avoid in Smoke Tests
```go
// DON'T - Network call
func TestManagerConnection(t *testing.T) {
    client := NewManagerClient(endpoint)
    _, err := client.HealthCheck() // Network call!
    assert.NoError(t, err)
}

// DON'T - Long-running operation
func TestFullAuthFlow(t *testing.T) {
    // Complex multi-step process taking 10+ seconds
}
```

## When to Add Smoke Tests

Add smoke tests when you want to:
- Validate configuration loading
- Check service initialization
- Verify basic object creation
- Test helper functions
- Validate input parsing

For more complex scenarios, see:
- `tests/e2e/` - Complete user workflows (VirtualBMC / synthetic backends)
- Upcoming `tests/integration/` - Planned service-to-service validation tier

## CI Integration

Smoke tests run on:
- Every commit (pre-push hooks)
- All pull requests
- Before integration and E2E test suites

They serve as a fast feedback mechanism to catch basic issues early in the development cycle.

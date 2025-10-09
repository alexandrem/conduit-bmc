# Testing (AI Reference)

## Tiers

**Smoke** (`tests/smoke/`): < 5s, no network, config validation, pre-commit
**Integration** (`tests/integration/`): Planned, cross-service validation
**E2E** (`tests/e2e/`): 5-15min, full stack, real/simulated BMCs, nightly/release

## Commands

```bash
make test-smoke                    # Fast checks
make test-e2e                      # Full E2E (Docker required)
cd manager && make test            # Service unit tests

# Specific suites
go test ./tests/smoke -run TestEnvironmentConfiguration
cd tests && E2E_TEST_CONFIG=configs/default.yaml go test ./e2e/suites/power -run TestPowerOperations
```

## Structure

- `tests/smoke/`: Fast checks
- `tests/e2e/framework/`: Shared harness
- `tests/e2e/suites/<domain>/`: Organized scenarios (auth, power, console)
- `tests/e2e/backends/`: Backend-specific setup (VirtualBMC, synthetic, real)

## Backends

**Synthetic**: HTTP Redfish sim (low fidelity, dev)
**VirtualBMC**: Docker IPMI sim (default E2E)
**Real Hardware**: Actual BMCs (optional)

Default: `make test-e2e` uses VirtualBMC (will migrate to OpenBMC)

## Test Placement

**Smoke**: Config validation, service health
**Integration**: Service-to-service, auth flows
**E2E**: Complete workflows, BMC interactions

## Config

- Default: `tests/e2e/configs/`
- Backend-specific: `tests/e2e/backends/<backend>/configs/`
- Env overrides: Environment variables

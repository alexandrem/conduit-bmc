# End-to-End Test Suite

This directory contains the orchestrated end-to-end (E2E) tests for the BMC Management System. The suite exercises real control-plane workflows against simulated BMC endpoints (VirtualBMC/IPMI and synthetic Redfish) to ensure the full stack behaves like production hardware.

## Related Documentation

- `tests/e2e/OVERVIEW.md` – high-level context for the E2E architecture
- `tests/e2e/backends/ipmi/docs/framework-design.md` – detailed VirtualBMC framework design
- `tests/e2e/backends/ipmi/docs/ipmi-setup-guide.md` – optional real hardware setup
- `tests/docs/environments.md` – backend configuration and troubleshooting

## Layout

```
tests/e2e/
├── framework/                  # Shared harness (clients, config, environment helpers)
├── suites/                     # Scenario-based suites (auth, power, console, performance)
├── backends/
│   ├── ipmi/                   # VirtualBMC implementation + docs
│   └── synthetic/              # Pure-Go Redfish simulator
└── configs/
    └── default.yaml            # Default VirtualBMC test configuration
```

Each suite imports shared helpers from `framework/` so tests stay focused on behaviour rather than setup.

## Configuration

The default configuration (`configs/default.yaml`) targets the Docker-based VirtualBMC environment:

```yaml
manager_endpoint: "http://localhost:8080"
gateway_endpoint: "http://localhost:8081"
agent_endpoint:   "http://localhost:8082"
ipmi_endpoints:
  - id: "ipmi-server-01"
    address: "localhost:7230"
    username: "ipmiusr"
    password: "test"
```

Set `E2E_TEST_CONFIG` to point at an alternate file as needed:

```bash
export E2E_TEST_CONFIG=configs/default.yaml
```

## Running Tests

### Orchestrated Workflow (recommended)

These targets start the development stack, launch VirtualBMC containers (`docker-compose.e2e.yml`), run the suites, and tear everything down.

```bash
# Standard run (uses existing dev environment if available)
make test-e2e

# Completely fresh run (stops everything first)
make test-e2e-clean
```

### Manual / Focused Runs

```bash
# Start dev services and E2E machines manually
make dev-up
make test-e2e-machines-up

# Run a focused suite
cd tests
E2E_TEST_CONFIG=configs/default.yaml go test ./e2e/suites/power -run TestPowerOperations

# Tear down machines when finished
make test-e2e-machines-down
```

### Backend Selection

Use Go build tags to target a specific backend:

```bash
# Synthetic Redfish backend
E2E_TEST_CONFIG=configs/default.yaml go test -tags "e2e backend_synthetic" ./e2e/suites/auth

# VirtualBMC (default) backend
E2E_TEST_CONFIG=configs/default.yaml go test -tags "e2e backend_ipmi" ./e2e/suites/...
```

## Suite Summary

| Suite            | File(s)                            | What it covers                          |
|------------------|------------------------------------|-----------------------------------------|
| Auth + Tokens    | `suites/auth/*.go`                 | Manager auth, token issuance & TTL      |
| Power Management | `suites/power/*.go`                | Power on/off/cycle/reset, concurrency   |
| Console Access   | `suites/console/*.go`              | SOL sessions, WebSocket flows           |
| Performance      | `suites/performance/*.go`          | Load/stress scenarios                   |

## Prerequisites

- Docker & Docker Compose (for VirtualBMC backend)
- `ipmitool` (optional, useful for debugging)
- Development stack (Manager, Gateway, Local Agent) built locally or via `make dev-up`

## Tips

- Keep suites deterministic—prefer the shared helpers in `framework/` for setup/teardown.
- Use build tags to isolate backend-specific scenarios.
- Inspect logs with `make test-e2e-logs` while the orchestrated run is active.
- For new backends, document setup steps in `tests/docs/environments.md` and add a README under `tests/e2e/backends/<backend>/`.
- If you need a reusable test binary, compile with `go test -c -o tmp/<suite>.test` so artifacts stay in `tests/tmp/` (gitignored).

## Troubleshooting

- `make test-e2e-machines-logs` – tail Docker logs for the VirtualBMC stack
- `docker-compose -f docker-compose.e2e.yml ps` – verify container health
- `docker exec e2e-ipmi-01 ipmitool ...` – sanity-check IPMI commands manually

Feel free to extend the suite with new scenarios—just remember to update the documentation and CI workflows when adding a new backend or orchestration step.

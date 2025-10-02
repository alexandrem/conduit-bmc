# E2E Testing Overview

The end-to-end (E2E) suite validates real user workflows by exercising the full BMC Management System—Manager, Gateway, Local Agent, and simulated BMCs. It lives alongside `tests/docs` and `tests/smoke` as part of the reorganized testing layout.

## Goals

- Cover authentication, power control, console access, and performance scenarios against VirtualBMC IPMI endpoints.
- Provide realistic BMC behaviour without physical hardware.
- Support backend selection (VirtualBMC, synthetic Redfish, optional hardware) via build tags.
- Integrate seamlessly with local development (`make test-e2e`) and CI pipelines.

## Directory Map

```
tests/
├── docs/                  # Testing documentation (overview, environments, contributing)
├── smoke/                 # Fast confidence tests
└── e2e/
    ├── framework/         # Shared harness (config, clients, environment helpers)
    ├── suites/            # Scenario-focused tests (auth, power, console, performance)
    └── backends/
        ├── ipmi/          # VirtualBMC implementation + docs
        └── synthetic/     # HTTP Redfish simulator
```

## Backends

| Backend     | Description                                | Fidelity |
|-------------|--------------------------------------------|----------|
| `backend_ipmi` (default) | Docker VirtualBMC with IPMI LAN + SOL | High     |
| `backend_synthetic`     | In-process Redfish simulation         | Medium   |
| `backend_hardware`      | Real BMC endpoints (optional)         | Highest  |

Select a backend with build tags, e.g. `go test -tags "e2e backend_ipmi" ./tests/e2e/suites/power`.

## Orchestration Flow

`make test-e2e` performs the following steps:

1. Ensure the development stack (Manager/Gateway/Agent) is running via
   `tooling/make/Makefile.dev`.
2. Launch VirtualBMC containers using `docker-compose.e2e.yml` (IPMI ports 7230–7232).
3. Override the local agent configuration so it points at the temporary BMCs.
4. Run all suites under `tests/e2e/suites` with `E2E_TEST_CONFIG=configs/default.yaml`.
5. Restore the development environment and tear down the VirtualBMC containers.

Use `make test-e2e-clean` for a fully fresh run, or `make test-e2e-machines-up` / `make test-e2e-machines-down` when debugging.

## Key Components

- **framework/config.go** – Loads test configuration (`E2E_TEST_CONFIG`).
- **framework/framework.go** – Sets up context, clients, and shared lifecycle hooks.
- **framework/clients.go** – ConnectRPC clients for Manager and Gateway.
- **framework/test_environment.go** – Utility for orchestrating backend-specific state.
- **suites/** – Individual scenarios (auth, power, console, performance) built on the shared framework.
- **backends/ipmi/** – Docker+VirtualBMC implementation and documentation.
- **backends/synthetic/** – Lightweight HTTP Redfish mock used for fast iterations.

## Running Focused Suites

```bash
# Power scenarios only
cd tests
E2E_TEST_CONFIG=configs/default.yaml go test ./e2e/suites/power -run TestPowerOperations

# Authentication scenarios against synthetic backend
E2E_TEST_CONFIG=configs/default.yaml go test -tags "backend_synthetic" ./e2e/suites/auth
```

## Adding New Scenarios

1. Create a new file under `tests/e2e/suites/<domain>/` following the existing naming pattern.
2. Reuse helpers from `tests/e2e/framework/` for setup and teardown.
3. Document backend requirements in the file header and, if needed, update `tests/docs/environments.md`.
4. If a new backend is required, create `tests/e2e/backends/<backend>/` and add documentation.

## Troubleshooting

- `docker-compose -f docker-compose.e2e.yml ps` – verify container health.
- `docker-compose -f docker-compose.e2e.yml logs -f e2e-virtualbmc-01` – inspect VirtualBMC logs.
- `docker exec e2e-ipmi-01 ipmitool ...` – manually test IPMI commands.
- Check `tests/docs/environments.md` for more detailed backend tips.

For deeper architectural context and real hardware guidance, refer to:
- `tests/e2e/backends/ipmi/docs/framework-design.md`
- `tests/e2e/backends/ipmi/docs/ipmi-setup-guide.md`

# Development (AI Reference)

## Quick Start

**Prerequisites**: Go 1.25+, Docker, make, buf

**Primary (fastest)**: `make local-env-up` - Air hot reload, no Docker, ~10s startup

**Docker**: `make dev-up` - Containerized, ~45s startup

**BMC Simulation**: `make bmc-up` - VirtualBMC IPMI + Redfish + VNC, ~30s

**Full Stack**: `make bmc-full-up` - Core + BMC simulation

## Commands

```bash
# Status/Logs
make local-env-status|logs|down
make dev-status|logs|down
make bmc-logs|test-ipmi|test-redfish

# Service-specific logs
make local-env-logs-manager|gateway|agent
make dev-logs-manager|gateway|agent

# Cleanup
make local-env-clean
make dev-down  # stops all (core + BMC)
make bmc-down  # stops BMC only
```

## Service Ports

**Local/Docker**:
- Manager: 8080 (auth, server mgmt)
- Gateway: 8081 (proxy, web UI)
- Agent: 8082 (BMC discovery)
- Test BMCs (local): 9001-9003

**BMC Simulation**:
- IPMI: 6230-6232 (VirtualBMC)
- Redfish: 8000-8002 (Sushy)
- VNC: 6080-6082 (noVNC)

## CLI Testing

```bash
cd cli
go run . server list|show|power <id>
go run . server console <id>              # Web UI (default)
go run . server console <id> --terminal   # Direct SOL
go run . server vnc <id>                  # VNC web
```

## Hot Reload

Each service has `.air.toml`:
- Watches Go + proto files
- Auto-rebuild on change
- Gateway/Manager include protobuf gen

## Environment Comparison

| Feature      | Local Air | Docker Dev | BMC Sim |
|--------------|-----------|------------|---------|
| Startup      | ~10s      | ~45s       | ~30s    |
| Resources    | Low       | Moderate   | Moderate|
| Hot Reload   | Yes       | Yes        | N/A     |
| Isolation    | No        | Yes        | Yes     |
| IPMI/Redfish | Mock      | No         | Real    |

**Use Local**: Rapid dev, debugging, low resources
**Use Docker**: Integration testing, containers
**Use BMC Sim**: Protocol testing (IPMI/Redfish)
**Use Full**: E2E with real protocols

## Dependencies

Auto-installed (local env):
- Air (hot reload)
- Buf (protobuf)
- protoc-gen-go, protoc-gen-connect-go

**IPMI SOL requires** `ipmiconsole` (FreeIPMI):
```bash
brew install freeipmi              # macOS
apt-get install freeipmi-tools     # Debian/Ubuntu
```

## Troubleshooting

**Local env**:
```bash
lsof -i :8080 -i :8081 -i :8082
make local-env-status
make local-env-clean && make local-env-up
pkill -f "air.*manager|gateway|local-agent"
```

**Docker**: `make dev-rebuild`, check `docker-compose.core.yml` for port conflicts

## Files

**Local**: `./manager/manager.db`, `./tmp/logs/*.log`, `./tmp/local-env.pids`
**Docker**: Container volumes, logs via `make dev-logs`

# 🛠️ Development Setup

This guide covers setting up the BMC Management System for local development
with hot reloading.

## Quick Start

### Prerequisites

- Go 1.25+
- Docker & Docker Compose (for containerized dev workflow)
- `make`
- [buf](https://buf.build/)

### Option 1: Local Air Environment (Fastest)

Start all services with Air locally (no Docker):

```bash
make local-env-up
```

This automatically:

- ✅ Installs required dependencies (Air, Buf, protobuf tools)
- ✅ Creates necessary directories and files
- ✅ Starts test BMC servers on ports 9001-9003
- ✅ Starts Manager (8080), Gateway (8081), Agent (8082) with hot reloading

### Option 2: Docker Development Environment

Start all services with VirtualBMC IPMI simulation + hot reloading:

```bash
make dev-up
```

This provides:

- 🔌 **VirtualBMC IPMI Simulation** - Real IPMI protocol using VirtualBMC
- 🖥️ **Docker Container Control** - IPMI commands control Docker containers
- 🌐 **Redfish API Emulation** - Complete REST-based BMC management with Sushy
- 📟 **VNC Console Access** - Web-based console access via noVNC
- 🐳 **Container Orchestration** - Full Docker Compose environment
- ⚡ **Hot Reloading** - Instant code reload with Air

### Option 3: BMC Simulation Environment (Persistent Development)

Start BMC simulation environment with VirtualBMC, Redfish, and VNC:

```bash
make bmc-up
```

This provides a persistent BMC simulation environment that stays running for development:

- 🖥️ **VirtualBMC IPMI Simulation** - Real IPMI v2.0 RMCP+ protocol using VirtualBMC
- 🌐 **Redfish API Emulation** - Full DMTF Redfish standard implementation with Sushy
- 📟 **VNC Console Access** - Web-based graphical console (noVNC) on ports 6080-6082
- 🔌 **IPMI Protocol Testing** - Authentic IPMI commands via ipmitool (ports 6230-6232)
- 🌐 **Redfish REST API** - Full REST-based BMC management (ports 8000-8002)

**When to use:**
- Testing IPMI protocol compatibility
- Validating Redfish API integrations
- Testing VNC console functionality
- Development without requiring full core services
- Isolated BMC protocol testing

### Management Commands

```bash
# Check status
make local-env-status               # Local services
make dev-status                     # Docker services

# View logs
make local-env-logs                 # All local logs
make local-env-logs-manager         # Specific service
make dev-logs                       # All Docker logs
make bmc-logs                       # BMC simulation logs

# Stop services
make local-env-down                 # Local environment
make dev-down                       # Docker environment (stops all)
make bmc-down                       # BMC simulation only (keeps core running)
```

### Combined Development Setup

Start full environment (core services + BMC simulation):

```bash
make bmc-full-up    # Starts dev-up + bmc-up together
```

## Testing the System

### Standard Testing (Local/Docker)

Once services are running, test with the CLI:

```bash
cd cli

# List servers
go run . server list

## Show server details
go run . server show server-001

# Check server power status
go run . server power status server-001

# Power operations
go run . server power on server-001
go run . server power off server-001

# Console access
go run . server console server-001             # Web console (default)
go run . server console server-001 --terminal  # Terminal streaming (advanced)
go run . server vnc server-001                 # VNC console
```

### BMC Simulation Environment Testing

Test IPMI and Redfish protocols with the BMC simulation environment:

```bash
# Quick connectivity tests
make bmc-test-ipmi      # Test all IPMI servers
make bmc-test-redfish   # Test all Redfish APIs

# Manual IPMI testing with VirtualBMC
ipmitool -I lanplus -H localhost -p 6230 -U ipmiusr -P test power status
ipmitool -I lanplus -H localhost -p 6231 -U ipmiusr -P test power status
ipmitool -I lanplus -H localhost -p 6232 -U ipmiusr -P test power status

# Manual Redfish testing (curl)
curl http://localhost:8000/redfish/v1/Systems/server-01
curl http://localhost:8001/redfish/v1/Systems/server-02
curl http://localhost:8002/redfish/v1/Systems/server-03

# Test VNC console access via web
open http://localhost:6080    # Server 01 VNC console
open http://localhost:6081    # Server 02 VNC console
open http://localhost:6082    # Server 03 VNC console

# Interactive testing shells
make bmc-shell-ipmi     # Interactive IPMI testing
make bmc-shell-redfish  # Interactive Redfish testing
```

## Architecture

The development setup includes:

- **Manager** (port 8080): BMC management service with protobuf generation
- **Gateway** (port 8081): API gateway, routing service, and web UI server
- **Local Agent** (port 8082): Agent for BMC discovery and communication
- **CLI**: Pure command-line interface optimized for scripting and automation
- **Database**: Shared SQLite database volume

## Hot Reloading Configuration

Each service has an `.air.toml` configuration file that:

- Watches Go source files and proto files
- Excludes test files and temporary directories
- Automatically rebuilds and restarts on changes
- Includes protobuf generation for Gateway and Manager

### Air Configuration Files

- `gateway/.air.toml` - Includes protobuf generation
- `manager/.air.toml` - Includes protobuf generation
- `local-agent/.air.toml` - Standard Go hot reloading
- `cli/.air.toml` - Standard Go hot reloading

## Development Commands

### Docker Development Commands

```bash
# Start development environment
make dev-up

# View all service logs
make dev-logs

# View specific service logs
make dev-logs-gateway
make dev-logs-manager
make dev-logs-agent

# Stop development environment
make dev-down

# Check status
make dev-status

# Access container shells for debugging
docker exec -it manager sh
docker exec -it gateway sh
docker exec -it local-agent sh
```

### Local Air Commands

```bash
# Install Air locally
make dev-install-air

# Run individual services locally
make dev-gateway-local
make dev-manager-local
make dev-agent-local
make dev-cli-local
```

## Testing CLI Commands

Once the development environment is running, test CLI commands:

```bash
# Access CLI container
make dev-shell-cli

# Inside the container, test CLI commands:
go run . server list
go run . server show server-123
go run . server power status server-123
go run . server console server-123             # Web console (default, opens browser)
go run . server console server-123 --terminal  # Terminal streaming (advanced, IPMI SOL)
go run . server vnc server-123                 # VNC console (opens browser)
```

## Service URLs

### Local Air Environment

- **🔧 Manager**: http://localhost:8080 (Authentication and server management)
- **🌐 Gateway**: http://localhost:8081 (BMC operations proxy + web UI)
- **🤖 Local Agent**: http://localhost:8082 (BMC discovery)
- **🖥️ Test BMC Servers**: localhost:9001, localhost:9002, localhost:9003

### Docker Development Environment

- **Manager API**: http://localhost:8080 (Authentication and server management)
- **Gateway API**: http://localhost:8081 (BMC operations proxy)
- **Local Agent**: http://localhost:8082 (BMC discovery and management)

### BMC Simulation Environment

- **VirtualBMC IPMI**: localhost:6230, localhost:6231, localhost:6232 (IPMI v2.0 RMCP+)
- **Redfish APIs**: localhost:8000, localhost:8001, localhost:8002 (DMTF Redfish)
- **VNC Consoles**: localhost:6080, localhost:6081, localhost:6082 (noVNC web access)

### Web Console Access

- VNC Console: http://localhost:8081/vnc/{session-id}
- Serial Console: http://localhost:8081/console/{session-id}
- Health Check: http://localhost:8081/health

## Volumes and Persistence

The setup uses Docker volumes for:

- **Go module cache**: Shared across all services for faster builds
- **Temporary build artifacts**: Separate for each service
- **Database data**: Persistent SQLite database
- **Source code**: Mounted from host for hot reloading

## Environment Comparison

| Feature            | Local Air                  | Docker Dev (Core)            | BMC Simulation              |
|--------------------|----------------------------|------------------------------|-----------------------------|
| **Startup Time**   | ⚡ Fast (~10s)              | 🐌 Moderate (~45s)           | 🐌 Moderate (~30s)          |
| **Resource Usage** | 💚 Low                     | 🟡 Moderate                  | 🟡 Moderate                 |
| **Hot Reloading**  | ✅ Yes                      | ✅ Yes                        | ✅ N/A (simulation)         |
| **Isolation**      | ❌ No                       | ✅ Yes                        | ✅ Yes                       |
| **Dependencies**   | 📋 Go tools required       | 🐳 Docker only               | 🐳 Docker only              |
| **Debugging**      | 🔍 Easy (direct processes) | 🔍 Moderate (container logs) | 🔍 Moderate (logs)          |
| **IPMI Protocol**  | ❌ Mock HTTP                | ❌ Not included              | ✅ VirtualBMC (real IPMI)   |
| **Redfish API**    | ❌ Not available            | ❌ Not included              | ✅ Sushy emulator           |
| **VNC Console**    | ❌ Not available            | ❌ Not included              | ✅ Web-based noVNC          |
| **Core Services**  | ✅ Manager/Gateway/Agent    | ✅ Manager/Gateway/Agent     | ❌ BMC simulation only      |

**Use Local Air when:** Rapid development, debugging, lower resource usage

**Use Docker Development when:** Full stack testing with containers, integration testing

**Use BMC Simulation when:** IPMI/Redfish protocol testing, BMC feature validation

**Use Combined (bmc-full-up) when:** Complete end-to-end testing with real protocols

## File Locations

### Local Air Environment

- **📁 Database**: `./manager/manager.db`
- **📊 Logs**: `./tmp/logs/` (manager.log, gateway.log, local-agent.log,
  test-servers.log)
- **🆔 PIDs**: `./tmp/local-env.pids`

### Docker Environment

- **Database**: Docker volumes
- **Logs**: Container logs (view with `make dev-logs`)

## Troubleshooting

### Local Air Environment

**Services Not Starting:**

```bash
# Check ports
lsof -i :8080 -i :8081 -i :8082 -i :9001 -i :9002 -i :9003

# Check status
make local-env-status

# Clean restart
make local-env-clean
make local-env-up
```

**Process Cleanup:**

```bash
# Automatic cleanup
make local-env-down

# Manual cleanup if needed
pkill -f "air.*manager"
pkill -f "air.*gateway"
pkill -f "air.*local-agent"
rm -rf tmp/
```

### Docker Environment

**Port Conflicts:** Modify port mappings in `docker-compose.core.yml`
**Slow Startup:** First run downloads modules, subsequent runs are faster
**Container Issues:** Use `make dev-rebuild` to rebuild containers

### Dependencies

**Missing Tools:** The local environment auto-installs:

- Air (hot reloading)
- Buf (protobuf management)
- protoc-gen-go & protoc-gen-connect-go (code generation)

**IPMI SOL Console Support:**

For IPMI Serial-over-LAN (SOL) console functionality, the local-agent requires FreeIPMI's `ipmiconsole`:

```bash
# macOS
brew install freeipmi

# Ubuntu/Debian
sudo apt-get install freeipmi-tools

# RHEL/CentOS/Fedora
sudo dnf install freeipmi

# Arch Linux
sudo pacman -S freeipmi
```

**Note:** The agent will fail to start if `ipmiconsole` is not found and either:
- IPMI discovery is enabled (`agent.bmc_discovery.enable_ipmi_detection: true`), or
- Static IPMI servers are configured

Agents with only Redfish servers and disabled IPMI discovery do not require this dependency.

## Complete Command Reference

### Local Air Environment Commands

| Command                       | Description                            |
|-------------------------------|----------------------------------------|
| `make local-env-up`           | Start all services with Air locally    |
| `make local-env-down`         | Stop all local services                |
| `make local-env-status`       | Show status of local services          |
| `make local-env-logs`         | View logs from all services            |
| `make local-env-logs-manager` | View Manager service logs (follow)     |
| `make local-env-logs-gateway` | View Gateway service logs (follow)     |
| `make local-env-logs-agent`   | View Local Agent service logs (follow) |
| `make local-env-logs-test`    | View test servers logs (follow)        |
| `make local-env-clean`        | Clean all environment data and logs    |
| `make local-env-restart`      | Restart the entire environment         |

### Docker Environment Commands

| Command                 | Description                   |
|-------------------------|-------------------------------|
| `make dev-up`           | Start development environment |
| `make dev-down`         | Stop all services             |
| `make dev-logs`         | View all service logs         |
| `make dev-logs-gateway` | View Gateway logs             |
| `make dev-logs-manager` | View Manager logs             |
| `make dev-logs-agent`   | View Agent logs               |
| `make dev-status`       | Check status                  |

### BMC Simulation Commands

| Command                 | Description                                    |
|-------------------------|------------------------------------------------|
| `make bmc-up`           | Start BMC simulation (VirtualBMC + Redfish)    |
| `make bmc-down`         | Stop BMC simulation (keep core running)        |
| `make bmc-full-up`      | Start core services + BMC simulation           |
| `make bmc-logs`         | View BMC simulation logs                       |
| `make bmc-rebuild`      | Rebuild BMC simulation containers              |
| `make bmc-test-ipmi`    | Test IPMI connectivity (all servers)           |
| `make bmc-test-redfish` | Test Redfish API endpoints (all servers)       |
| `make bmc-shell-ipmi`   | Interactive IPMI testing shell                 |
| `make bmc-shell-redfish`| Interactive Redfish testing shell              |
| `make bmc-help`         | Show all BMC commands                          |

## Example Development Workflow

### Quick Development (No BMC Protocols)
```bash
# 1. Start local environment (fastest)
make local-env-up

# 2. Make code changes (Air will auto-reload)
# Edit files in manager/, gateway/, local-agent/

# 3. Test with CLI
cd cli && go run . server list

# 4. Check logs if needed
make local-env-logs-manager

# 5. Stop when done
make local-env-down
```

### Full Protocol Testing (With BMC Simulation)
```bash
# 1. Start full environment (core + BMC simulation)
make bmc-full-up

# 2. Verify BMC simulation is working
make bmc-test-ipmi
make bmc-test-redfish

# 3. Test with CLI against real protocols
cd cli
go run . server list
go run . server power status server-001

# 4. Check logs if needed
make dev-logs-gateway    # Core service logs
make bmc-logs            # BMC simulation logs

# 5. Stop when done
make dev-down            # Stops everything (core + BMC)
```

### BMC Protocol Development Only
```bash
# 1. Start only BMC simulation (no core services)
make bmc-up

# 2. Test IPMI/Redfish directly
make bmc-shell-ipmi      # Interactive IPMI testing
make bmc-shell-redfish   # Interactive Redfish testing

# 3. Stop BMC simulation
make bmc-down
```

## BMC Simulation Benefits

When using `make bmc-up` or `make bmc-full-up`, you get:

- **🔌 Authentic Protocol Testing**: Real IPMI v2.0 RMCP+ and Redfish DMTF standard
- **⚡ Power Management**: Test actual BMC power operations via VirtualBMC
- **📟 Console Validation**: VNC console with web-based noVNC access
- **🔒 Multi-Protocol Support**: Both IPMI and Redfish on same simulated servers
- **📊 Development Isolation**: Run BMC simulation independently or with core services
- **🐛 Error Scenarios**: Test network failures, timeouts, and protocol edge cases
- **🧪 Interactive Testing**: Built-in shells for manual IPMI and Redfish testing

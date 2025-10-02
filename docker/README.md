# Docker BMC Simulation Environment

This directory contains Docker configurations for running the BMC Management
System with containerized BMC simulators. This provides a fully containerized
environment for development, testing, and CI/CD integration.

## 🎯 Architecture

```
Docker Host
├── BMC Management System Containers
│   ├── bmc-manager     (port 8080) - Authentication & server mapping
│   ├── bmc-gateway     (port 8081) - BMC operations proxy & web UI
│   └── bmc-local-agent (port 8082) - BMC discovery & access
├── VirtualBMC IPMI Simulators (Advanced IPMI)
│   ├── dev-virtualbmc-01 (6230/udp) - VirtualBMC with libvirt/QEMU
│   ├── dev-virtualbmc-02 (6231/udp) - VirtualBMC with libvirt/QEMU
│   └── dev-virtualbmc-03 (6232/udp) - VirtualBMC with libvirt/QEMU
├── Redfish API Emulators (Modern BMC)
│   ├── dev-redfish-01 (port 8000) - Sushy Redfish emulator
│   ├── dev-redfish-02 (port 8001) - Sushy Redfish emulator
│   └── dev-redfish-03 (port 8002) - Sushy Redfish emulator
├── Server Containers (VNC targets)
│   ├── dev-server-01 - Ubuntu server with Xvfb + x11vnc
│   ├── dev-server-02 - Ubuntu server with Xvfb + x11vnc
│   └── dev-server-03 - Ubuntu server with Xvfb + x11vnc
└── noVNC Proxy
    └── dev-novnc (ports 6080-6082) - Web-based VNC viewer
```

## 📁 Directory Structure

```
docker/
├── README.md                          # This file
├── virtualbmc.Dockerfile              # VirtualBMC IPMI simulator
├── redfish.Dockerfile                 # Redfish API emulator
├── server.Dockerfile                  # Server with VNC support
├── novnc.Dockerfile                   # noVNC web viewer
├── scripts/                           # Container startup scripts
│   ├── virtualbmc-startup.sh          # VirtualBMC initialization
│   ├── server-vnc-startup.sh          # Server VNC setup
│   └── novnc-startup.sh               # noVNC proxy setup
└── supervisor/                        # Process management configs
```

## 🚀 Quick Start

### Prerequisites

```bash
# Docker and Docker Compose
docker --version          # 20.10+
docker-compose --version  # 1.28+
```

### Launch Complete Environment

```bash
# Start all services (from project root)
docker-compose -f docker-compose.core.yml up -d     # Core services
docker-compose -f docker-compose.virtualbmc.yml up -d     # IPMI/Redfish simulators

# Check status
docker ps --format "table {{.Names}}\t{{.Status}}"

# View logs
docker logs bmc-mgmt-gateway-1
docker logs dev-virtualbmc-01
```

### Quick Test

```bash
# Test IPMI connectivity (from inside Docker network)
docker run --rm --network bmc-network pnnlmiscscripts/ipmitool \
  -I lanplus -H dev-virtualbmc-02 -p 623 -U ipmiusr -P test power status

# Test Redfish API
curl http://localhost:8000/redfish/v1/Systems

# Test VNC console via web browser
# Open: http://localhost:8081/vnc/<session-id>
```

## 🧪 BMC Simulators

### VirtualBMC (IPMI Simulation)

**Technology**: Python VirtualBMC + libvirt + QEMU

**Features**:

- ✅ **IPMI Power Control**: on, off, cycle, reset, status
- ✅ **IPMI Authentication**: RAKP+ (IPMI v2.0)
- ⚠️ **SOL Console**: Defined but not fully functional yet
- ✅ **Standard IPMI Port**: 623/UDP

**Access**:

```bash
# From inside Docker network (agent container)
ipmitool -I lanplus -H dev-virtualbmc-01 -p 623 -U ipmiusr -P test power status
ipmitool -I lanplus -H dev-virtualbmc-01 -p 623 -U ipmiusr -P test chassis status

# From host (UDP port forwarding doesn't work reliably on Docker for Mac)
# Use the agent container or other containers on bmc-network
```

**Configuration**: See `local-agent/config/docker-agent.yaml`:

```yaml
-   id: "ipmi-server-01"
control_endpoint:
	endpoint: "dev-virtualbmc-01:623"
	type: "ipmi"
	username: "ipmiusr"
	password: "test"
sol_endpoint:
	endpoint: "dev-virtualbmc-01:623"
	type: "ipmi"
```

**Requirements**:

- Runs in **privileged mode** (required for libvirt/QEMU)
- Uses libvirt with `security_driver = "none"`
- No cgroup management (disabled for container environment)
- VMs defined with minimal devices (disk only, no network, file-based serial)

**Limitations**:

- UDP port forwarding to host doesn't work on Docker for Mac (use container
  network)
- SOL console needs completion of IPMI transport implementation
- Requires privileged containers (may not work in all CI/CD environments)

### Redfish API Emulators

**Technology**: Sushy-emulator (OpenStack project)

**Features**:

- ✅ **Redfish Power Control**: Systems, Chassis, Managers
- ✅ **RESTful API**: Standard Redfish v1 protocol
- ⚠️ **Serial Console**: Not supported by sushy-emulator
- ✅ **VNC Integration**: Via noVNC proxy

**Access**:

```bash
# Redfish API endpoints
curl http://localhost:8000/redfish/v1                      # Service root
curl http://localhost:8000/redfish/v1/Systems              # Systems collection
curl http://localhost:8000/redfish/v1/Systems/server-01   # Specific system

# Power control
curl -X POST http://localhost:8000/redfish/v1/Systems/server-01/Actions/ComputerSystem.Reset \
  -H "Content-Type: application/json" \
  -d '{"ResetType": "On"}'
```

**Configuration**: See `local-agent/config/docker-agent.yaml`:

```yaml
-   id: "redfish-server-01"
control_endpoint:
	endpoint: "http://dev-redfish-01:8000"
	type: "redfish"
sol_endpoint:
	endpoint: "http://dev-redfish-01:8000"
	type: "redfish_serial"
```

**Limitations**:

- sushy-emulator doesn't implement SerialConsole WebSocket endpoint
- SOL console not available for Redfish servers

### Server Containers (VNC Targets)

**Technology**: Ubuntu + Xvfb + x11vnc + Fluxbox

**Features**:

- Headless X11 display (Xvfb)
- VNC server exposing the display
- Fluxbox window manager for realistic desktop
- xterm terminal for interaction

**Access**:

```bash
# Via noVNC web viewer (through gateway)
# http://localhost:8081/vnc/<session-id>

# Direct VNC connection (from inside Docker network)
# Connect to dev-server-01:5901, dev-server-02:5901, etc.
```

### noVNC Proxy

**Technology**: noVNC WebSocket proxy

**Features**:

- WebSocket-to-TCP proxy for VNC connections
- Multiple instances for parallel sessions
- Integrated with gateway web UI

**Ports**:

- 6080: dev-server-01 VNC proxy
- 6081: dev-server-02 VNC proxy
- 6082: dev-server-03 VNC proxy

## 🌐 Service Endpoints

| Service          | Container              | Host Port | Internal Port | Protocol  |
|------------------|------------------------|-----------|---------------|-----------|
| **Manager**      | bmc-mgmt-manager-1     | 8080      | 8080          | HTTP      |
| **Gateway**      | bmc-mgmt-gateway-1     | 8081      | 8081          | HTTP      |
| **Agent**        | bmc-mgmt-local-agent-1 | 8082      | 8082          | HTTP      |
| **VirtualBMC 1** | dev-virtualbmc-01      | 6230/udp  | 623/udp       | IPMI      |
| **VirtualBMC 2** | dev-virtualbmc-02      | 6231/udp  | 623/udp       | IPMI      |
| **VirtualBMC 3** | dev-virtualbmc-03      | 6232/udp  | 623/udp       | IPMI      |
| **Redfish 1**    | dev-redfish-01         | 8000      | 8000          | HTTP      |
| **Redfish 2**    | dev-redfish-02         | 8001      | 8000          | HTTP      |
| **Redfish 3**    | dev-redfish-03         | 8002      | 8000          | HTTP      |
| **noVNC 1**      | dev-novnc              | 6080      | 6080          | WebSocket |
| **noVNC 2**      | dev-novnc              | 6081      | 6081          | WebSocket |
| **noVNC 3**      | dev-novnc              | 6082      | 6082          | WebSocket |

## 🎮 Web UI Access

The gateway provides a centralized web interface for all console access:

### VNC Console

```
http://localhost:8081/vnc/<session-id>
```

- Web-based graphical console
- Full keyboard and mouse support
- Session switching to serial console

### Serial Console (Future)

```
http://localhost:8081/console/<session-id>
```

- Web-based terminal (xterm.js)
- Interactive command-line access
- Session switching to VNC

### Creating Sessions

Use the CLI to create sessions:

```bash
# VNC session
BMC_MANAGER_ENDPOINT=http://localhost:8080 ./bin/bmc-cli server vnc <server-id>

# Console session (when implemented)
BMC_MANAGER_ENDPOINT=http://localhost:8080 ./bin/bmc-cli server console <server-id> --web
```

## 🛠️ Management Commands

### Service Management

```bash
# Start services
docker-compose -f docker-compose.core.yml up -d
docker-compose -f docker-compose.virtualbmc.yml up -d

# Stop services
docker-compose -f docker-compose.core.yml down
docker-compose -f docker-compose.virtualbmc.yml down

# View logs
docker logs -f bmc-mgmt-gateway-1
docker logs -f dev-virtualbmc-01
docker logs -f dev-redfish-01

# Restart specific service
docker restart bmc-mgmt-local-agent-1
docker restart dev-virtualbmc-02

# View status
docker ps --format "table {{.Names}}\t{{.Status}}"
```

### Development Access

```bash
# Shell access to containers
docker exec -it bmc-mgmt-manager-1 sh
docker exec -it bmc-mgmt-gateway-1 sh
docker exec -it bmc-mgmt-local-agent-1 sh
docker exec -it dev-virtualbmc-01 bash

# Check VirtualBMC status
docker exec dev-virtualbmc-01 vbmc list

# Check libvirt VMs
docker exec dev-virtualbmc-01 virsh -c qemu:///system list --all

# Check Redfish API
docker exec dev-redfish-01 curl http://localhost:8000/redfish/v1/Systems
```

## 🔧 Configuration Files

### Agent Configuration

`local-agent/config/docker-agent.yaml` - Defines static BMC hosts for the agent
to manage

### Docker Compose Files

- `docker-compose.core.yml` - Core services (manager, gateway, agent)
- `docker-compose.virtualbmc.yml` - BMC simulators (VirtualBMC, Redfish,
  servers,
  noVNC)

## 🚨 Troubleshooting

### VirtualBMC Issues

**Container fails to start:**

```bash
# Check logs
docker logs dev-virtualbmc-01

# Common issues:
# - libvirt failed to start: Check privileged mode is enabled
# - Network errors: Check if default network exists
# - VM definition failed: Check XML syntax in startup script
```

**IPMI commands timeout:**

```bash
# Test from inside Docker network (not from host due to UDP limitations)
docker exec bmc-mgmt-local-agent-1 ipmitool -I lanplus -H dev-virtualbmc-01 -p 623 -U ipmiusr -P test power status

# Check VirtualBMC is running
docker exec dev-virtualbmc-01 vbmc list
# Should show "running" status
```

**Power commands fail:**

```bash
# Check libvirt status
docker exec dev-virtualbmc-01 virsh -c qemu:///system list --all

# Check VM definition
docker exec dev-virtualbmc-01 virsh -c qemu:///system dumpxml dev-server-01

# Check libvirt logs
docker exec dev-virtualbmc-01 cat /var/log/libvirt/qemu/dev-server-01.log
```

### Redfish Issues

**API not responding:**

```bash
# Test from host
curl http://localhost:8000/redfish/v1

# Test from inside Docker network
docker exec bmc-mgmt-local-agent-1 curl http://dev-redfish-01:8000/redfish/v1

# Check container health
docker inspect dev-redfish-01 --format='{{.State.Health.Status}}'
```

### VNC Issues

**Black screen in VNC viewer:**

```bash
# Check if server container is running
docker ps | grep dev-server-01

# Check VNC server process
docker exec dev-server-01 pgrep Xvfb
docker exec dev-server-01 pgrep x11vnc

# Check noVNC proxy
docker logs dev-novnc
```

### Network Issues

```bash
# Check Docker network
docker network inspect bmc-network

# Check connectivity between containers
docker exec bmc-mgmt-local-agent-1 ping dev-virtualbmc-01
docker exec bmc-mgmt-local-agent-1 ping dev-redfish-01

# Check DNS resolution
docker exec bmc-mgmt-local-agent-1 nslookup dev-virtualbmc-01
```

## 🔒 Security Notes

⚠️ **Development Only**: This setup uses default credentials and is intended for
development/testing only.

- Default IPMI credentials: `ipmiusr` / `test`
- Default Redfish: No authentication
- VirtualBMC runs in privileged mode (full host access)
- Container networking isolates services from external access
- No production security hardening applied

## 🚀 CI/CD Considerations

### Limitations for CI/CD

1. **VirtualBMC requires privileged mode**
	- May not be available in GitHub Actions or other CI platforms
	- Requires self-hosted runners or special permissions

2. **UDP port forwarding issues on Docker for Mac**
	- IPMI testing must be done from inside Docker network
	- Host-to-container UDP doesn't work reliably

### CI/CD Strategy

**Option 1: Redfish-only testing**

```yaml
# .github/workflows/test.yml
-   name: Run Redfish tests
run: |
	docker-compose -f docker-compose.core.yml up -d
	docker-compose -f docker-compose.virtualbmc.yml up -d dev-redfish-01 dev-redfish-02 dev-redfish-03
	# Run tests against Redfish endpoints only
```

**Option 2: Mock IPMI server**

- Create lightweight IPMI mock without libvirt/QEMU
- No privileged mode required
- Fast and reliable for CI/CD

**Option 3: Self-hosted runners**

- Configure runners to support privileged containers
- Full testing including VirtualBMC

## 📊 Feature Support Matrix

| Feature              | VirtualBMC (IPMI)       | Redfish Emulator |
|----------------------|-------------------------|------------------|
| Power Control        | ✅ Working               | ✅ Working        |
| Power Status         | ✅ Working               | ✅ Working        |
| VNC Console          | ✅ Working               | ✅ Working        |
| Serial Console (SOL) | ⚠️ Needs implementation | ❌ Not supported  |
| Sensor Data          | ⚠️ Limited              | ❌ Not supported  |
| System Event Log     | ⚠️ Limited              | ❌ Not supported  |
| CI/CD Friendly       | ❌ Needs privileged mode | ✅ Yes            |
| Docker for Mac       | ⚠️ Network limitations  | ✅ Full support   |

## 🎯 Current Status

### ✅ Fully Working

- Core services (manager, gateway, agent)
- VNC console access (web-based graphical console)
- IPMI power commands via VirtualBMC
- Redfish power commands via emulator
- Session switching (VNC ↔ Console UI)

### ⚠️ Partially Working

- IPMI SOL console (serial defined but transport needs completion)
- Redfish serial console (emulator doesn't support it)

## 📚 Additional Resources

- **IPMI Specification
  **: https://www.intel.com/content/www/us/en/products/docs/servers/ipmi/ipmi-home.html
- **Redfish API**: https://www.dmtf.org/standards/redfish
- **VirtualBMC**: https://docs.openstack.org/virtualbmc/
- **Sushy Emulator**: https://docs.openstack.org/sushy-tools/
- **noVNC**: https://novnc.com/

This containerized BMC environment provides **realistic BMC testing** with the
convenience and portability of Docker containers, making it ideal for
development teams and CI/CD pipelines.

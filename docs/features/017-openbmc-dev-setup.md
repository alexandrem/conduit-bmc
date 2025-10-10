---
rfd: "017"
title: "OpenBMC Development Environment"
state: "draft"
breaking_changes: false
testing_required: true
database_changes: false
api_changes: false
dependencies: [ "005" ]
database_migrations: [ ]
areas: [ "docker", "tests", "local-agent" ]
---

# RFD 017 - OpenBMC Development Environment

**Status:** 🚧 Draft

## Summary

Enhance the containerized development environment (RFD 005) by providing real
OpenBMC firmware emulation using QEMU. This delivers production-grade IPMI and
Redfish implementations with complete SOL, sensor data, and event logs,
replacing the limited VirtualBMC and Sushy simulators while remaining CI/CD
friendly.

## Problem

**Current RFD 005 simulator limitations:**

- **VirtualBMC (IPMI)**:
    - IPMI SOL console not working (transport layer incomplete)
    - Requires privileged containers (libvirt/QEMU dependency)
    - Incompatible with most CI/CD platforms (GitHub Actions, GitLab CI)
    - Limited IPMI command coverage
    - No realistic sensor data or System Event Log (SEL)

- **Sushy Emulator (Redfish)**:
    - Missing SerialConsole WebSocket endpoint
    - No SOL console support via Redfish
    - Limited sensor and event log simulation
    - Doesn't match real BMC behavior

**Why this matters:**

- **Testing gaps**: SOL/console features cannot be validated in development
- **Production differences**: Behavior mismatches between simulators and real
  hardware
- **CI/CD blockers**: VirtualBMC's privileged mode requirement prevents
  automated testing
- **Developer experience**: Debugging console-related issues requires physical
  hardware

## Solution

Provide QEMU-based OpenBMC containers that run complete BMC firmware in
**system-mode QEMU (TCG)**, exposing identical interfaces to production hardware while
remaining suitable for CI/CD environments.

**Key Design Decisions:**

- **Use pre-built images**: Distribute OpenBMC firmware as container images in a
  registry (Docker Hub/GHCR) to avoid complex Yocto builds
- **QEMU system-mode (TCG)**: Run BMC ARM firmware in QEMU without privileged
  containers (KVM not required)
- **Platform choice**: Start with QEMU ARM machines (virt, romulus, witherspoon)
  supported by OpenBMC
- **Parallel operation**: Keep existing simulators for fast smoke tests, use
  OpenBMC for comprehensive validation

**Benefits:**

- ✅ **Complete IPMI implementation**: Full IPMItool command support including
  SOL
- ✅ **Full Redfish API**: WebSocket-based SOL console, complete sensor data
- ✅ **Production parity**: Exact same phosphor services as production BMCs
- ✅ **No privileged mode**: QEMU system-mode compatible with standard CI runners
- ✅ **Realistic behavior**: Sensor readings, FRU data, SEL entries match
  hardware

**Trade-offs:**

- ⚠️ **Startup latency**: 30-60 seconds boot time vs instant simulators
- ⚠️ **Resource usage**: ~512MB RAM per instance vs ~50MB for simulators
- ⚠️ **Image size**: ~500MB container images vs ~100MB for simulators

**Architecture Overview:**

```
┌─────────────────────────────────────────────────────────────┐
│ Docker Compose Stack (docker-compose.openbmc.yml)           │
│                                                             │
│  ┌────────────────────┐  ┌────────────────────┐             │
│  │ openbmc-server-01  │  │ openbmc-server-02  │  ...        │
│  │                    │  │                    │             │
│  │  ┌──────────────┐  │  │  ┌──────────────┐  │             │
│  │  │ QEMU ARM     │  │  │  │ QEMU ARM     │  │             │
│  │  │              │  │  │  │              │  │             │
│  │  │ OpenBMC      │  │  │  │ OpenBMC      │  │             │
│  │  │ ├─ phosphor  │  │  │  │ ├─ phosphor  │  │             │
│  │  │ ├─ ipmid     │  │  │  │ ├─ ipmid     │  │             │
│  │  │ ├─ bmcweb    │  │  │  │ ├─ bmcweb    │  │             │
│  │  │ └─ obmc-cons │  │  │  │ └─ obmc-cons │  │             │
│  │  └──────────────┘  │  │  └──────────────┘  │             │
│  │                    │  │                    │             │
│  │  Ports:            │  │  Ports:            │             │
│  │  - 6230/udp (IPMI) │  │  - 6231/udp (IPMI) │             │
│  │  - 8000 (Redfish)  │  │  - 8001 (Redfish)  │             │
│  │  - 6080 (noVNC)    │  │  - 6081 (noVNC)    │             │
│  └────────────────────┘  └────────────────────┘             │
│                                                             │
│  ┌──────────────────────────────────────────────┐           │
│  │ dev-novnc (shared noVNC proxy)               │           │
│  │ Proxies WebSocket console to all servers     │           │
│  └──────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────┘

CLI/Browser ─→ Manager ─→ Gateway ─→ Agent ─→ OpenBMC (IPMI/Redfish/SOL)
```

### Component Changes

1. **Docker Infrastructure**:
    - New `docker/openbmc.Dockerfile` - QEMU + OpenBMC firmware setup
    - New `docker/scripts/openbmc-startup.sh` - Firmware boot and health checks
    - New `docker-compose.openbmc.yml` - OpenBMC server stack (3 instances)
    - Updated `docker/README.md` - OpenBMC usage documentation

2. **Local Agent**:
    - New `local-agent/config/openbmc-agent.yaml` - OpenBMC host configuration
    - No code changes - existing IPMI/Redfish handlers work unchanged

3. **E2E Tests**:
    - New test fixtures using OpenBMC servers
    - Extend SOL console tests (previously skipped)
    - Validate WebSocket Redfish console endpoint

4. **CI/CD Integration**:
    - GitHub Actions workflow using pre-built OpenBMC images
    - Selective test runs: simulators for fast checks, OpenBMC for full
      validation

**Configuration Example:**

```yaml
# docker-compose.openbmc.yml (simplified)
services:
    openbmc-server-01:
        image: ghcr.io/bmc-mgmt/openbmc-qemu:latest
        container_name: openbmc-server-01
        environment:
            BMC_MACHINE: romulus
            BMC_USERNAME: root
            BMC_PASSWORD: 0penBmc
        ports:
            - "6230:623/udp"    # IPMI
            - "8000:8000"        # Redfish (HTTPS)
            - "5900:5900"        # VNC console
        healthcheck:
            test: [ "CMD", "curl", "-kf", "https://localhost:8000/redfish/v1" ]
            interval: 10s
            timeout: 5s
            retries: 30
```

```yaml
# local-agent/config/docker-agent.yaml (updated)
bmc_hosts:
    -   id: "openbmc-server-01"
        control_endpoint:
            endpoint: "openbmc-server-01:623"
            type: "ipmi"
            username: "root"
            password: "0penBmc"

    -   id: "openbmc-redfish-01"
        control_endpoint:
            endpoint: "https://openbmc-server-01:8000"
            type: "redfish"
            username: "root"
            password: "0penBmc"
```

## Implementation Plan

### Phase 1: Image Build and Distribution

- [ ] Create `docker/openbmc.Dockerfile` with QEMU and OpenBMC dependencies
- [ ] Write `docker/scripts/openbmc-startup.sh` boot script
- [ ] Build OpenBMC firmware for target platforms (romulus, witherspoon)
- [ ] Test local image build process
- [ ] Push images to container registry (GHCR)

### Phase 2: Compose Stack Integration

- [ ] Create `docker-compose.openbmc.yml` with 3 server instances
- [ ] Configure port mappings (IPMI 6230-6232, Redfish 8000-8002)
- [ ] Add noVNC proxy for console access
- [ ] Implement health checks for OpenBMC boot completion
- [ ] Validate IPMI, Redfish, and VNC access from host

### Phase 3: Agent Configuration and Testing

- [ ] Create `local-agent/config/openbmc-agent.yaml`
- [ ] Test agent discovery of OpenBMC servers
- [ ] Validate power operations via IPMI
- [ ] Validate power operations via Redfish
- [ ] Test SOL console activation (IPMI and Redfish)

### Phase 4: Documentation and CI

- [ ] Update `docker/README.md` with OpenBMC setup instructions
- [ ] Document OpenBMC vs simulator trade-offs
- [ ] Create GitHub Actions workflow for OpenBMC tests
- [ ] Add E2E tests for SOL console (previously untested)
- [ ] Performance benchmarking (startup time, resource usage)

## Configuration Changes

**New files:**

- `docker/openbmc.Dockerfile` - OpenBMC container definition
- `docker/scripts/openbmc-startup.sh` - Boot and initialization script
- `docker-compose.openbmc.yml` - OpenBMC development stack
- `local-agent/config/openbmc-agent.yaml` - Agent configuration for OpenBMC

**Modified files:**

- `docker/README.md` - Add OpenBMC usage section
- `.github/workflows/` - New CI workflow for OpenBMC tests

**No changes required:**

- Manager, Gateway, CLI - work unchanged with OpenBMC
- Existing simulator configurations remain for fast smoke tests

## Testing Strategy

### Manual Testing

```bash
# Start OpenBMC stack
docker compose -f docker-compose.openbmc.yml up -d

# Wait for boot (30-60s)
docker compose -f docker-compose.openbmc.yml logs -f openbmc-server-01

# Test IPMI access
ipmitool -I lanplus -H localhost -p 6230 -U root -P 0penBmc power status

# Test Redfish access
curl -k -u root:0penBmc https://localhost:8000/redfish/v1/Systems

# Test SOL console (IPMI)
ipmitool -I lanplus -H localhost -p 6230 -U root -P 0penBmc sol activate

# Test VNC framebuffer (optional)
open vnc://localhost:6080

# Test SOL via Redfish WebSocket
wscat -n -c "wss://localhost:8000/redfish/v1/Managers/bmc/SerialInterfaces/1/Actions/Manager.Connect" \
  -H "Authorization: Basic $(echo -n root:0penBmc | base64)"
```

### Integration Tests

- Power operations against OpenBMC (on/off/reset/status)
- Sensor data retrieval via IPMI and Redfish
- System Event Log (SEL) queries
- FRU inventory data validation

### E2E Tests

- Full SOL console session (previously untested with simulators)
- Redfish WebSocket console connection
- Multi-user concurrent console access
- Console session persistence across agent restarts

## Security Considerations

- **Credentials**: Use development-only credentials (`root:0penBmc`)
- **Network isolation**: OpenBMC containers on isolated Docker network
- **TLS**: OpenBMC bmcweb provides HTTPS for Redfish (self-signed certs in dev)

## Future Enhancements

### Additional OpenBMC Platforms

- Support more hardware platforms (AST2500, AST2600 BMC chips)
- Add specific vendor BMC configurations (Supermicro, Dell, HPE)

### Performance Optimizations

- Pre-booted OpenBMC snapshots for faster startup
- Resource tuning (CPU/memory limits per platform)
- Parallel boot orchestration

### Advanced Features

- Firmware update simulation
- BIOS configuration via IPMI
- Hardware sensor simulation (temperature, voltage, fan speed)
- Network boot (PXE) integration

### CI/CD Enhancements

- Automated image builds on OpenBMC upstream releases
- Multi-architecture support (x86_64, ARM64 CI runners)
- Test result caching to skip redundant OpenBMC tests

## Appendix

### OpenBMC Architecture

OpenBMC runs a Linux-based BMC firmware with D-Bus as the core IPC mechanism:

```
┌────────────────────────────────────────────┐
│ OpenBMC Firmware (Yocto Linux)             │
│                                            │
│  ┌─────────────────────────────────────┐   │
│  │ bmcweb (Redfish REST API + WS)      │   │
│  └────────────┬────────────────────────┘   │
│               │                            │
│  ┌────────────┴─────────────┐              │
│  │ D-Bus (System Bus)       │              │
│  └─┬──────────┬──────────┬──┘              │
│    │          │          │                 │
│  ┌─┴────┐  ┌─┴──────┐ ┌─┴───────────┐      │
│  │ipmid │  │phosphor│ │obmc-console │      │
│  │(IPMI)│  │(sensors│ │(SOL)        │      │
│  └──────┘  │/events)│ └─────────────┘      │
│            └────────┘                      │
└────────────────────────────────────────────┘
```

**Key Services:**

- **ipmid**: IPMI daemon handling IPMI LAN protocol
- **bmcweb**: HTTP/HTTPS server for Redfish API and WebSocket console
- **phosphor-***: D-Bus services for sensors, inventory, logging, state management
- **obmc-console**: Serial-over-LAN console multiplexer

### QEMU Machine Types

OpenBMC supports multiple ARM platforms in QEMU:

| Machine       | SoC     | RAM   | Notes                    |
|---------------|---------|-------|--------------------------|
| `romulus`     | AST2500 | 512MB | Default; used in CI      |
| `witherspoon` | AST2500 | 512MB | IBM Power9 systems       |
| `palmetto`    | AST2400 | 256MB | Legacy OpenPOWER         |
| `ast2600-evb` | AST2600 | 1GB   | Modern Aspeed eval board |

**Recommendation**: Start with `romulus` for stability and moderate resource usage.

### Pre-built Image Repository

**Image naming:**

```
ghcr.io/bmc-mgmt/openbmc-qemu:latest
ghcr.io/bmc-mgmt/openbmc-qemu:romulus-v2.15.0
ghcr.io/bmc-mgmt/openbmc-qemu:witherspoon-v2.15.0
```

**Image contents:**

- QEMU ARM system emulator (system-mode / TCG)
- OpenBMC kernel and rootfs
- Pre-configured BMC services (ipmid, bmcweb, obmc-console)
- Startup scripts and health checks

### Dockerfile & Entrypoint Example

#### `docker/openbmc.Dockerfile`

```dockerfile
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y \
    qemu-system-arm \
    curl \
    net-tools \
    iproute2 \
    socat \
    sudo \
    && rm -rf /var/lib/apt/lists/*

ENV MACHINE=romulus
ENV BMC_USER=root
ENV BMC_PASSWORD=0penBmc
ENV QEMU_RAM=512M
ENV QEMU_CPUS=1

COPY firmware/${MACHINE}/kernel /opt/openbmc/kernel
COPY firmware/${MACHINE}/rootfs.ext4 /opt/openbmc/rootfs.ext4
COPY scripts/openbmc-startup.sh /usr/local/bin/openbmc-startup.sh
RUN chmod +x /usr/local/bin/openbmc-startup.sh

EXPOSE 623/udp
EXPOSE 8000
EXPOSE 5900

ENTRYPOINT ["/usr/local/bin/openbmc-startup.sh"]
```

#### `docker/scripts/openbmc-startup.sh`

```bash
#!/bin/bash
set -e

FIRMWARE_DIR="/opt/openbmc"

echo "[INFO] Starting OpenBMC QEMU container..."
echo "[INFO] Machine: $MACHINE"
echo "[INFO] RAM: $QEMU_RAM, CPUs: $QEMU_CPUS"

qemu-system-arm \
    -M virt \
    -cpu cortex-a15 \
    -m $QEMU_RAM \
    -smp $QEMU_CPUS \
    -kernel $FIRMWARE_DIR/kernel \
    -drive if=none,file=$FIRMWARE_DIR/rootfs.ext4,format=raw,id=hd0 \
    -device virtio-blk-device,drive=hd0 \
    -netdev user,id=net0,hostfwd=tcp::8000-:8000,hostfwd=tcp::5900-:5900,hostfwd=udp::623-:623 \
    -device virtio-net-device,netdev=net0 \
    -nographic \
    -serial mon:stdio &

QEMU_PID=$!

echo "[INFO] Waiting for OpenBMC services..."
while ! curl -k --silent --fail https://localhost:8000/redfish/v1 >/dev/null 2>&1; do
    sleep 2
done

echo "[INFO] OpenBMC is ready!"
wait $QEMU_PID
```

### Service Endpoints Reference

| Service     | Container         | Host Port | Protocol  | Credentials   |
|-------------|-------------------|-----------|-----------|---------------|
| OpenBMC 01  | openbmc-server-01 |           |           |               |
| - IPMI      |                   | 6230/udp  | IPMI LAN  | root:0penBmc  |
| - Redfish   |                   | 8000      | HTTPS     | root:0penBmc  |
| - VNC       |                   | 5900      | VNC/RFB   | (no password) |
| OpenBMC 02  | openbmc-server-02 |           |           |               |
| - IPMI      |                   | 6231/udp  | IPMI LAN  | root:0penBmc  |
| - Redfish   |                   | 8001      | HTTPS     | root:0penBmc  |
| - VNC       |                   | 5901      | VNC/RFB   | (no password) |
| OpenBMC 03  | openbmc-server-03 |           |           |               |
| - IPMI      |                   | 6232/udp  | IPMI LAN  | root:0penBmc  |
| - Redfish   |                   | 8002      | HTTPS     | root:0penBmc  |
| - VNC       |                   | 5902      | VNC/RFB   | (no password) |
| noVNC Proxy | dev-novnc         | 5900-5902 | WebSocket | (no auth)     |

### Development Commands

```bash
# Start OpenBMC environment
docker compose -f docker-compose.openbmc.yml up -d

# View boot logs
docker compose -f docker-compose.openbmc.yml logs -f openbmc-server-01

# Check OpenBMC service status inside container
docker exec openbmc-server-01 systemctl status bmcweb
docker exec openbmc-server-01 systemctl status phosphor-ipmi-net

# Access OpenBMC shell
docker exec -it openbmc-server-01 /bin/sh

# Stop environment
docker compose -f docker-compose.openbmc.yml down

# Clean up volumes
docker compose -f docker-compose.openbmc.yml down -v
```

### Reference Implementations

- **OpenBMC Project**: https://github.com/openbmc/openbmc
- **QEMU OpenBMC Machines**: https://github.com/openbmc/qemu
- **Phosphor D-Bus Interfaces
  **: https://github.com/openbmc/phosphor-dbus-interfaces
- **bmcweb Redfish Server**: https://github.com/openbmc/bmcweb

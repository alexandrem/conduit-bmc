---
rfd: "005"
title: "Containerized IPMI/Redfish Development Environment"
state: "implemented"
priority: "medium"
status: "implemented"
breaking_changes: false
testing_required: true
database_changes: false
api_changes: false
dependencies: [ ]
database_migrations: [ ]
areas: [ "docker", "tests", "local-agent" ]
---

# RFD 005 - Containerized IPMI/Redfish Development Environment

**Status:** 🎉 Implemented

## Summary

Provide a fast, reproducible Docker-based environment that simulates BMC
functionality (IPMI, Redfish, SOL, VNC) for developers and CI pipelines. This
replaces heavy UTM/QEMU setups with lightweight containers so end-to-end
workflows can run consistently across macOS, Linux, and CI.

## Problem

- Real BMC hardware is not available for most contributors; existing UTM/QEMU
  labs are slow, require large OS images, and are hard to automate.
- Tests need access to both legacy IPMI LAN commands and modern Redfish REST
  APIs alongside console access (SOL/VNC) to validate the full control plane.
- We currently lack a standardized, documented stack that developers can spin up
  quickly and that CI can rely on.

## Solution

Build a Docker Compose stack that exposes the same surfaces as production
hardware:

1. **Simulated server containers** running X11 + x11vnc with a serial console (
   `/dev/ttyS0`) so SOL/VNC flows work.
2. **VirtualBMC** daemons per server to emulate IPMI LAN 1.5/2.0 and map
   power/reset to the container lifecycle.
3. **Redfish emulator** (Sushy) per server for REST operations, aligned with
   VirtualBMC power state.
4. **noVNC gateway** to present web consoles for desktop sessions.

The stack should start in seconds, expose predictable ports (623X for IPMI, 800X
for Redfish, 608X for VNC), and integrate with the local agent for discovery or
static registration.

## Implementation Status

### Phase 1 – Compose topology ✅ COMPLETE

**Implemented:**

- ✅ All Dockerfiles created:
    - `docker/server.Dockerfile` - Ubuntu servers with X11/VNC
    - `docker/virtualbmc.Dockerfile` - VirtualBMC with libvirt/QEMU
    - `docker/redfish.Dockerfile` - Sushy Redfish emulator
    - `docker/novnc.Dockerfile` - noVNC web proxy
- ✅ Docker Compose files:
    - `docker-compose.virtualbmc.yml` - Persistent dev environment (3 servers)
    - `docker-compose.e2e.yml` - E2E test environment (separate instances)
    - `docker-compose.core.yml` - Core services (manager, gateway, agent)

**Note:** Direct docker-compose commands are used instead of Makefile targets.
See `docker/README.md` for all commands.

### Phase 2 – Feature enablement ✅ COMPLETE

**Implemented:**

- ✅ Serial console configured (`/dev/ttyS0` device in server containers)
- ✅ Xvfb + x11vnc running in server containers with Fluxbox window manager
- ✅ VirtualBMC power operations control Docker container lifecycle via socket
  mount
- ✅ Sushy emulator configured per server with Docker backend
- ✅ noVNC proxy serving multiple ports (6080-6082) for parallel sessions
- ✅ Health checks for all services (Xvfb, VirtualBMC, Redfish, noVNC)

**Startup Scripts:**

- `docker/scripts/virtualbmc-startup.sh` - VirtualBMC initialization
- `docker/scripts/server-vnc-startup.sh` - Server VNC setup
- `docker/scripts/novnc-startup.sh` - noVNC proxy configuration

### Phase 3 – Integration with platform ✅ COMPLETE

**Implemented:**

- ✅ Agent configuration: `local-agent/config/docker-agent.yaml`
- ✅ E2E test config: `tests/e2e/configs/e2e-test-config.yaml`
- ✅ Comprehensive documentation:
    - `docker/README.md` (500+ lines) - Full setup, usage, troubleshooting
    - Architecture diagrams and service endpoints table
    - Management commands and development access instructions

**Configuration Example:**

```yaml
# local-agent/config/docker-agent.yaml
bmc_hosts:
    -   id: "dev-server-01"
        control_endpoint:
            endpoint: "dev-virtualbmc-01:623"
            type: "ipmi"
            username: "ipmiusr"
            password: "test"
    -   id: "dev-redfish-01"
        control_endpoint:
            endpoint: "http://dev-redfish-01:8000"
            type: "redfish"
```

### Phase 4 – CI adoption ⚠️ PARTIAL

**Status:**

- ✅ Redfish emulators work in all environments (no special permissions)
- ⚠️ VirtualBMC requires privileged containers (libvirt/QEMU dependency)
- ⚠️ GitHub Actions and many CI platforms don't support privileged mode

**CI Strategy:**

- **Redfish-only testing**: Fully supported for CI/CD
- **VirtualBMC testing**: Requires self-hosted runners with privileged mode

**Next Steps for Full CI Adoption:**

1. Create lightweight IPMI mock server (no libvirt/QEMU)
2. No privileged mode requirement
3. Fast startup and cleanup for CI environments

## API Changes

None. All changes are confined to development tooling.

## Testing Strategy

- Manual smoke tests using `ipmitool` and Redfish `curl` against all containers.
- Automated e2e suites covering power operations,
  SOL activation, auth scenarios.
- Health checks embedded in Docker Compose (Xvfb, VirtualBMC, Sushy, noVNC).

## Service Endpoints Reference

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

## Known Limitations

### VirtualBMC (IPMI)

- Requires privileged containers (libvirt/QEMU dependency)
- UDP port forwarding doesn't work reliably on Docker for Mac (use container
  network)
- SOL console needs completion of IPMI transport implementation
- Not suitable for all CI/CD platforms

### Redfish Emulator

- Sushy-emulator doesn't implement SerialConsole WebSocket endpoint
- No SOL console support
- Limited sensor and event log simulation

### General

- Development credentials only (not for production)
- Container networking isolates services from external access
- No production security hardening applied

## Future Enhancements

### OpenBMC Real Firmware Emulation (In Progress)

**Benefits over current simulators:**

- ✅ **Real BMC firmware**: Full OpenBMC stack with actual phosphor services
- ✅ **Complete IPMI SOL**: Native IPMI Serial-over-LAN support (works out of the box)
- ✅ **Full Redfish API**: Complete implementation with WebSocket support for SOL
- ✅ **Realistic behavior**: Matches production BMC behavior exactly
- ✅ **Sensor data**: Real sensor simulation from OpenBMC
- ✅ **Event logs**: Complete System Event Log (SEL) implementation
- ✅ **No privileged mode**: QEMU runs in user mode (CI/CD friendly)

**What this solves:**

- ❌ VirtualBMC limitations: SOL not working, requires privileged mode
- ❌ Sushy emulator limitations: No serial console, limited features
- ✅ All features work as in production BMC hardware

**Next steps:**

1. Build OpenBMC firmware images (one-time, done by maintainers)
2. Publish pre-built images to container registry (Docker Hub or GHCR)
3. Update docker-compose to use pre-built images
4. Integrate with local-agent configuration
5. Add to E2E test suite
6. Migrate from VirtualBMC/Sushy to OpenBMC as default

**Trade-offs:**

- **Startup time**: OpenBMC takes ~30-60s to boot (vs instant simulators)
- **Resource usage**: Higher memory (512MB per instance vs ~50MB for simulators)

**Note on build complexity:** Pre-built OpenBMC Docker images will be available in
a container registry, eliminating the need to build firmware locally. Developers can
simply pull and run the images.

### Lightweight IPMI Mock (Alternative for CI/CD)

Create a minimal IPMI responder:

- Simple Python IPMI server without QEMU
- Instant startup (<1 second)
- No privileged mode
- Basic power commands only (no SOL, no sensors)
- Use for fast smoke tests, OpenBMC for comprehensive testing

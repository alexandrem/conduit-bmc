# 🔐 Conduit BMC Proxy

**Secure, multi-tenant BMC proxy for hosting providers**

> ⚠️ **Experimental**: APIs and features are evolving; not production-ready.

Provides secure, multi-tenant access to server BMCs (IPMI / Redfish) without
exposing BMC ports to the public internet. Ideal for bare metal hosting
providers who need to give customers secure access to their server consoles.

## ✨ Features

- 🔐 Zero BMC exposure (no public ports)
- 🌐 Multi-datacenter, scales with local agents
- 👥 Multi-tenant isolation
- 💻 CLI, web console, and API proxy
- 🔄 Auto-discovery or static BMC config
- 🚀 NAT-friendly outbound connections
- 🔌 Dual APIs: REST + gRPC

## 🖥️ Supported BMC Protocols

**Control Protocols:**
- ✅ **IPMI v2.0 (RMCP+) / v1.5 (lan)** — Power management with auto-fallback for compatibility
- ✅ **Redfish** — standardized REST API for server management (basic operations tested)

**Console Access:**
- ✅ **Serial Console (SOL)**
  - **IPMI SOL (Serial-over-LAN)** over **RCMP+ / lan** (auto-fallback)
  - **Redfish Serial Console** over **WebSocket**
- ✅ **Graphical Console (KVM / VNC)**
  - **Native VNC (RFB protocol)** — direct TCP port 5900 access (where supported)
  - **WebSocket-wrapped VNC** — RFB over WebSocket (tested with `websockify` + noVNC client)

**Verified Implementations:**
- ✅ **OpenBMC** — IPMI v2.0, Redfish, SOL, and VNC access tested (including WebSocket VNC)
- ✅ **VirtualBMC** — Basic IPMI commands verified; console via QEMU VNC
- ✅ **QEMU** — Native VNC server (RFB TCP) for guest framebuffer

### ⚠️ Additional / Theoretical Support

- The architecture *can* support WebSocket VNC on BMCs that implement it
- Vendor-specific endpoints, authentication, and session handling may vary
- See [Detailed BMC Theoretical Support](docs/guides/BMC-SUPPORT.md#theoretical-support-untested) for full notes


## 🏗️ Architecture

The system is split into four services:

| Component       | Responsibilities                                         | Key Interfaces          |
|-----------------|----------------------------------------------------------|-------------------------|
| **Manager**     | AuthN/Z, token issuance, server-to-gateway mapping       | REST + gRPC (Connect)   |
| **Gateway**     | Web console, API proxy, per-region routing               | REST + gRPC + WebSocket |
| **Local Agent** | BMC discovery, IPMI/Redfish operations, outbound tunnels | gRPC -> Gateway         |
| **CLI**         | User automation and scripting surface                    | gRPC -> Gateway/Manager |

For more details:
- See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for diagrams and data flows
- See [docs/DESIGN.md](docs/DESIGN.md) for deeper design considerations

## 🔒 Security Highlights

- JWT tokens scoped per customer/server
- All BMC traffic stays inside datacenter (outbound only)
- Role-based isolation for multi-tenancy
- Audit logging (designed for SIEM integration)

See [Security Overview](docs/security/overview.md) for broader
security consideration
notes.

## 🛠️ Quick Start (Development)

```bash
make dev-full-up
```

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for full instructions.

### Development Workflow

1. **Start services**: `make local-env-up` (or `make dev-up` for Docker).
2. **Iterate**: edit Go files; Air handles hot reloads for Manager, Gateway, Agent, CLI.
3. **Tests**:
   ```bash
   make test-all         # Unit tests
   make test-e2e         # Full E2E suite
   ```
4. **Stop services**: `make local-env-down` (or `make dev-down`).

## 📊 Monitoring

Every service exposes:
- `GET /health` – readiness/liveness indicator.
- `GET /metrics` – Prometheus metrics (latency, error rates, queue depth, etc.). - TODO


## 📚 Documentation

### Core Documentation
- **[Architecture Overview](docs/ARCHITECTURE.md)** - High-level system topology and component interactions
- **[System Design](docs/DESIGN.md)** - Design decisions and architectural rationale
- **[Development Guide](docs/DEVELOPMENT.md)** - Setup instructions and development workflow
- **[Testing Guide](docs/TESTING.md)** - Testing strategy and test execution

### Technical Documentation
In-depth technical specifications and protocol implementations:
- **[Authentication Flow](docs/technical/auth-flow.md)** - JWT tokens, session management, and authorization
- **[VNC Protocol Flow](docs/technical/vnc-protocol-flow.md)** - RFB proxy architecture and VNC implementation
- **[SOL Protocol Flow](docs/technical/sol-protocol-flow.md)** - SOL streaming architecture (web & terminal)
- **[IPMI Implementation](docs/technical/ipmi-implementation.md)** - IPMI SOL and power control details
- **[Web Architecture](docs/technical/web-architecture.md)** - Web console and UI implementation
- **[Protocol Overview](docs/technical/protocols.md)** - BMC protocol comparison and usage

### Guides
User-facing guides and compatibility information:
- **[BMC Support](docs/guides/BMC-SUPPORT.md)** - Supported BMC types and compatibility matrix
- **[Building OpenBMC](docs/guides/build-openbmc.md)** - OpenBMC compilation instructions
- **[VirtualBMC Setup](docs/guides/dev-virtualbmc.md)** - VirtualBMC development environment

### Security
Security considerations and threat modeling:
- **[Security Overview](docs/security/overview.md)** - Security design and best practices
<!--
- **[Threat Model](docs/security/threats-model.md)** - Attack vectors and mitigations
- **[BMC Risks](docs/security/bmc-risks.md)** - BMC-specific security concerns
-->

### Features (RFDs)
Request for Discussion documents for upcoming features:
- **[Feature Proposals](docs/features/)** - RFDs for planned features and enhancements

## 🔮 Future Work

Contributions welcome for:
- Vendor-specific session authentication (iDRAC, Supermicro, etc.)
- Protocol adapters for non-standard RFB implementations
- Hardware KVM-over-IP device support

## 🤝 Contributing

**TBD**

## 📄 License

MIT License.

See [LICENSE](LICENSE) for details.

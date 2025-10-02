# 🔐 Conduit BMC Proxy

**Secure BMC access proxy for hosting providers**

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
- ✅ **IPMI v2.0 (RMCP+)** — Power management, sensor monitoring, event logs
- ✅ **Redfish** — standardized REST API for server management (basic operations tested)

**Console Access:**
- ✅ **Serial Console (SOL)**
  - **IPMI SOL (Serial-over-LAN)** over **RCMP+**
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
- See [Detailed BMC Theoretical Support](docs/BMC-SUPPORT.md#theoretical-support-untested) for full notes


## 🏗️ Architecture

The system is split into four services:

| Component       | Responsibilities                                         | Key Interfaces          |
|-----------------|----------------------------------------------------------|-------------------------|
| **Manager**     | AuthN/Z, token issuance, server-to-gateway mapping       | REST + gRPC (Connect)   |
| **Gateway**     | Web console, API proxy, per-region routing               | REST + gRPC + WebSocket |
| **Local Agent** | BMC discovery, IPMI/Redfish operations, outbound tunnels | gRPC -> Gateway         |
| **CLI**         | User automation and scripting surface                    | gRPC -> Gateway/Manager |

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for diagrams and deeper design notes.

## 🔒 Security Highlights

- JWT tokens scoped per customer/server
- All BMC traffic stays inside datacenter (outbound only)
- Role-based isolation for multi-tenancy
- Audit logging (designed for SIEM integration)

See [docs/SECURITY.md](docs/ARCHITECTURE.md) for broader security consideration notes.

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

- **[System Design](docs/DESIGN.md)** - Complete architecture and design
  decisions
- **[Architecture Overview](docs/ARCHITECTURE.md)** - High-level system topology
- **[Development Guide](docs/DEVELOPMENT.md)** - Complete setup and development
  workflow

<!--
- **[Features (RFDs)](docs/features/)** - Upcoming initiatives & design documents
-->

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

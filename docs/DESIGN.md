# BMC Access System Design

## Overview

This document describes the design for a secure, multi-tenant **BMC (Baseboard
Management Controller) access solution** for a hosting provider. The system
allows customers to access their server's management interface (IPMI or Redfish)
without exposing raw BMC ports publicly.

It combines a **local agent per datacenter**, **regional gateways**, a
central **manager**, and a **command-line tool**.

### Control Plane vs Data Plane Separation

The system separates **control plane** (management operations) from
**data plane** (console/proxy traffic):

- **Control Plane**: Authentication, server discovery, session management via
  HTTP/JSON and gRPC APIs
- **Data Plane**: Console streams, proxy tunnels, and BMC protocol traffic via
  WebSocket and direct protocols

The system is composed of three main components:

1. **Local Agent** – Runs inside the OOB network in each datacenter, registers
   available BMCs with a Regional Gateway
2. **Regional Gateway** – Serves web console interfaces, routes traffic between
   customers and local agents, and validates delegated tokens from BMC Manager
3. **BMC Manager** – Central coordination service, responsible for
   authentication, authorization, and token delegation

Customers interact with the system through:

- **CLI Tool**: Pure command-line interface optimized for scripting and
  automation
- **Web Console**: Browser-based interfaces served directly by Regional Gateway

## Components

### Local Agent

- Runs **inside the out-of-band (OOB) management network** of a datacenter.
- Discovers and registers the BMCs it can reach with its assigned **Regional
  Gateway**
- Maintains an **outbound connection** to the Regional Gateway (
  firewall/NAT-friendly)
- Provides the actual connectivity to BMCs (IPMI, Redfish, or virtual console).
- **Never exposes the OOB network directly** to customers — only relays via the
  gateway

### Regional Gateway

- **Control Plane**: HTTP/JSON and gRPC APIs for session management, server
  operations
- **Data Plane**: Serves web-based console interfaces (noVNC viewer, serial
  console)
- Receives registrations from multiple Local Agents
- Maintains in-memory mapping of:
	- Local Agent registrations
	- BMC server-to-agent routing
	- Active console sessions (VNC, SOL)
- Stateless aside from mapping; on restart, Local Agents automatically
  re-register
- Authorizes connections using delegated tokens issued by the BMC Manager
- Forwards traffic between customers and Local Agents, supporting three session
  types:
	- **Serial Console**: Terminal-based console access (IPMI SOL/Redfish
	  serial)
	- **VNC Console**: Web-based graphical console access (VNC/KVM)
	- **API Proxy**: Direct IPMI/Redfish API tunneling for external tools

### BMC Manager

- **Control Plane**: HTTP/JSON and gRPC APIs for authentication, server
  management
- Central service that coordinates all Regional Gateways
- Maintains global mapping of `server_id -> regional_gateway`
- Handles authentication (via OIDC or provider IAM)
- Issues **delegated tokens** to clients for accessing specific servers through
  Regional Gateways
- Delegated tokens are time-bound; refresh tokens allow long-lived sessions

## Token Delegation & Refresh

- Clients authenticate with the **BMC Manager** (OIDC or IAM)
- BMC Manager issues a **delegated token** for a specific server
- Delegated token is presented to the **Regional Gateway** to authorize access
- To prevent token expiry mid-session, clients periodically **refresh tokens**
  using a DHCP-like lease refresh model
- Refresh can occur:
	- Over gRPC for CLI and terminal console sessions
	- Over WebSocket for VNC console viewer sessions

## API Architecture

### Control Plane APIs

Both BMC Manager and Regional Gateway expose dual API interfaces:

**BMC Manager APIs:**

- **HTTP/JSON REST API**: Web dashboards, external integrations
- **gRPC API**: High-performance CLI and service-to-service communication
- Endpoints: Authentication, server management, token delegation

**Regional Gateway APIs:**

- **HTTP/JSON REST API**: Web console management, session creation
- **gRPC API**: CLI operations, power management, proxy session setup
- **WebSocket API**: Real-time console streams (VNC, serial)

### Data Plane Protocols

**Console Streaming:**

- **WebSocket**: Browser-based VNC and serial console viewers
- **Raw TCP/UDP**: Direct proxy tunnels for external BMC tools
- **IPMI/Redfish**: Native BMC protocol forwarding

**VNC/KVM Transport:**

The system supports dual VNC transport modes with automatic detection:

1. **Native TCP VNC** (QEMU, VirtualBMC, BMC VNC servers)
	- Direct TCP connection to VNC port (typically 5900)
	- RFB (Remote Framebuffer) protocol over raw TCP
	- Auto-detected from endpoint: `host:5900` or `vnc://host:5900`
	- Used by: QEMU, VirtualBMC simulators, BMCs with native VNC

2. **WebSocket VNC** (OpenBMC verified, enterprise BMCs theoretical)
	- WebSocket-wrapped RFB protocol
	- Binary framing (WebSocket opcode 0x2)
	- Auto-detected from endpoint: `ws://...` or `wss://...`
	- **Verified:** OpenBMC bmcweb
	- **Untested:** Dell iDRAC, Supermicro, Lenovo XCC (may need session auth,
	  protocol adapters)
	- **Not supported:** HPE iLO (proprietary HTML5/.NET clients)

**Terminology Note:** "KVM" in BMC/server context means "Keyboard, Video,
Mouse" (graphical console access to the server's local display). This is **not**
the same as external "KVM-over-IP" hardware switches (Raritan, APC, ATEN). This
system supports BMC-integrated graphical console, not external KVM switches.

**Transport Auto-Detection:**

- No manual configuration required
- URL scheme determines transport: `ws://`, `wss://` → WebSocket; `vnc://`,
  `host:port` → Native TCP
- Agent automatically selects appropriate transport based on BMC endpoint

**Tested Implementations:**

- ✅ OpenBMC: `wss://bmc-host/kvm/0` (verified working)
- ✅ QEMU/VirtualBMC: `192.168.1.100:5900` (verified working)

## Web Console Architecture

The Regional Gateway serves web-based console interfaces using a template
system:

**Template System:**

- Located in `gateway/internal/webui/templates/`
- Uses Go embed filesystem for deployment
- Shared base template with component-specific extensions
- Real-time status updates and WebSocket management

**Access Patterns:**

1. CLI creates session via gRPC: `bmc-cli server vnc <id>`
2. Gateway returns session URL: `https://gateway:8081/vnc/{session-id}`
3. CLI launches browser to session URL
4. Gateway serves HTML interface with WebSocket connection
5. Agent connects to BMC VNC using auto-detected transport (native TCP or
   WebSocket)
6. Bidirectional data flow: Browser ↔ Gateway (WS) ↔ Agent (gRPC) ↔ BMC (TCP or
   WS)

## CLI Commands

- `server show <server_id>`
  Show server and BMC information (detected automatically: IPMI or Redfish)

- `server power <op> <server_id>`
  Control server power operations

- `server console <server_id>`
  Open Gateway's web-based serial console in browser

- `server console <server_id> --terminal`
  Open a terminal-based serial console directly in the CLI (IPMI SOL)

- `server vnc <server_id>`
  Open Gateway's web-based graphical console viewer (BMC virtual console/remote
  KVM access)

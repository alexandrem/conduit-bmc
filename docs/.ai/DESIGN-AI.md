# Design (AI Reference)

## Architecture

**Flow**: `CLI/Browser → Manager (auth/tokens) → Gateway (routing/webUI/proxy) → Agent (outbound-only) → BMC`

**Control vs Data Plane**:
- Control: Auth, discovery, session mgmt (HTTP/JSON, gRPC)
- Data: Console streams, proxy tunnels (WebSocket, direct protocols)

## Components

**Manager**: Central auth/coordination, OIDC/IAM, issues delegated tokens, maintains `server_id → gateway` mapping

**Gateway**: Validates Manager tokens, routes to agents, serves web UIs (`internal/webui/`), manages sessions (VNC/serial/API proxy), stateless (agents re-register on restart)

**Agent**: Per-datacenter, OOB network, outbound connection to Gateway, discovers/registers BMCs, handles IPMI/Redfish/VNC protocols

**CLI**: Pure Unix tool, no web servers, delegates UI to Gateway

## Session Types

1. **Serial Console**: IPMI SOL/Redfish serial, XTerm.js web UI (default) or `--terminal` direct streaming
2. **VNC**: Graphical KVM, web-based, auto-detects transport:
   - Native TCP VNC: `host:5900`, `vnc://...` (QEMU, VirtualBMC)
   - WebSocket VNC: `ws://...`, `wss://...` (OpenBMC verified, Dell/Supermicro/Lenovo theoretical)
   - HPE iLO NOT supported (proprietary)
3. **API Proxy**: Direct IPMI/Redfish tunneling

## Token Flow

1. CLI auth with Manager (OIDC/IAM) → get JWT + Gateway endpoint
2. CLI requests session from Gateway with Manager JWT
3. Gateway validates token (signature check, no Manager roundtrip)
4. Gateway routes to Agent over Agent's persistent connection
5. DHCP-style refresh: gRPC (CLI/terminal) or WebSocket (VNC)

## API Types

**Manager**: HTTP/JSON REST (dashboards), gRPC (CLI/services)
**Gateway**: HTTP/JSON REST (web console), gRPC (CLI ops), WebSocket (console streams)
**Data Plane**: WebSocket (browser consoles), raw TCP/UDP (direct proxy)

## Web Console Flow

1. CLI: `bmc-cli server vnc <id>` → gRPC CreateSession
2. Gateway returns URL: `https://gateway:8081/vnc/{session-id}`
3. CLI launches browser
4. Browser ↔ Gateway (WS) ↔ Agent (gRPC) ↔ BMC (TCP/WS)

## Connection Model

- CLI/Browser → Manager: Client-initiated (auth)
- CLI/Browser → Gateway: Client-initiated (sessions)
- Gateway → Manager: Outbound (registration, BMC reporting)
- Agent → Gateway: Outbound persistent (NAT-friendly)
- Gateway → Agent: Bidirectional over Agent's connection
- Manager NEVER initiates to Gateway/Agent
- Gateway NEVER initiates to Agent

## Key Properties

- No public BMC exposure
- All connections outbound-initiated (NAT/firewall friendly)
- Stateless Gateway (agents re-register)
- JWT signature validation (no Manager roundtrip)
- Multi-tenant with strict separation

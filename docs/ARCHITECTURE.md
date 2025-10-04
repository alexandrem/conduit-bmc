# Architecture

## Overview

```console
                    ┌──────────────┐
                    │   Manager    │
                    │  (Auth/JWT)  │
                    └──────────────┘
                           ▲
                           │ 1. Authenticate
                           │    (email/password)
                           │ 2. Receive JWT
┌─────────────┐            │
│     CLI     │────────────┘
│             │
│             │────────────┐
└─────────────┘            │ 3. CreateSession
       │                   │    (with JWT)
       │                   │ 4. Receive session URL
       │                   ▼
       │            ┌──────────────┐
       │            │   Gateway    │ ───────────┐
       │            │  (Session +  │            │ 4a. Validate JWT
       │            │   Proxy)     │ ←──────────┘     (outbound to Manager)
       │            └──────────────┘
       │                   │
       │ 5. Open URL       │
       │    in browser     │
       │                   │
       └──────────►┌─────────────┐
                   │   Browser   │
                   │ (Web Console)│
                   └─────────────┘
                           │
                           │ 6. Use cookie
                           │    for all ops
                           ▼
                    ┌──────────────┐
                    │   Gateway    │
                    │   (Proxy)    │
                    └──────────────┘
                           │
                           │ gRPC / HTTP/2 ↔ (outbound init)
                           ▼
                    ┌──────────────┐
                    │ Local Agent  │
                    │(per datacenter)
                    └──────────────┘
                           │
                           │ IPMI / Redfish / VNC
                           ▼
                    ┌──────────────┐
                    │   BMC Server │
                    │ IPMI-SOL /   │
                    │ Redfish /    │
                    │ VNC Console  │
                    └──────────────┘
```

## Flows

1. Console / Command Flow
   - CLI clients authenticate with Manager to obtain tokens
   - Manager provides Gateway endpoint information for the appropriate datacenter based on requested server operation
   - CLI coordinates with Gateway for server operations and web UI access
   - For web console access, CLI launches browser to Gateway's web interface
   - Gateway validates authentication tokens from Manager
   - Gateway routes requests to the correct Local Agent based on datacenter in server ID
   - Agents maintain persistent outbound connections to Gateway for NAT/firewall traversal
   - Agent communicates with BMC using:
     - **Power operations**: IPMI/Redfish REST APIs
     - **Serial console**: IPMI SOL or Redfish serial console streaming
     - **Graphical console**: Native VNC TCP (QEMU, VirtualBMC) or WebSocket VNC (Redfish GraphicalConsole, OpenBMC, Dell, Supermicro, Lenovo)
       - Note: This is BMC-integrated remote console, not external KVM-over-IP hardware
   - VNC transport auto-detected from endpoint URL scheme
   - Multiplexed heartbeat and control messages keep sessions alive and monitored
2. Authentication & Authorization
   - **Manager Authentication**: Manager handles user authentication and issues JWT tokens
   - **Gateway Endpoint Info**: Manager provides Gateway endpoint URL and authentication token to CLI
   - **CLI to Gateway**: CLI includes Manager-issued authentication token when requesting sessions from Gateway
   - **Token Validation**: Gateway validates authentication tokens from Manager before session setup
   - **Server Mapping**: Manager maps customer server IDs to actual BMC endpoints
   - **Authorization**: Manager enforces RBAC and server ownership verification
   - **Session Flow**:
     1. CLI authenticates with Manager (email/password or API key)
     2. Manager returns JWT token + Gateway endpoint URL
     3. CLI requests session from Gateway with Manager's token
     4. Gateway validates token with Manager
     5. Gateway establishes session with appropriate Agent
   - Refresh flow ensures long-lived sessions remain active safely

## Topology

```console
                                 ┌────────────────────┐
                                 │   BMC Manager      │
                                 │ (Central Control)  │
                                 │ Auth / Tokens /    │
                                 │ Server Mapping     │
                                 └────────────────────┘
                                          ▲
                                          │
                  ┌───────────────────────┼───────────────────────┐
                  │                       │                       │
                  │ (1) CLI → Manager     │ (2) Gateway → Manager │
                  │     Auth requests     │     Token validation  │
                  │     (client-init)     │     (outbound-init)   │
                  │                       │                       │
     ┌────────────┴────────┐    ┌────────┴─────────┐    ┌────────┴─────────┐
     │  Regional Gateway   │    │ Regional Gateway │    │ Regional Gateway │
     │      DC-A           │    │      DC-B        │    │      DC-C        │
     │ (session + proxy)   │    │ (session + proxy)│    │ (session + proxy)│
     └──────────┬──────────┘    └──────────┬───────┘    └──────────┬───────┘
                ▲                           ▲                       ▲
                │                           │                       │
        ┌───────┴────────┐          ┌───────┴────────┐     ┌───────┴────────┐
        │                │          │                │     │                │
     ┌──┴──┐          ┌──┴──┐    ┌──┴──┐          ┌──┴──┐ ┌──┴──┐        ┌──┴──┐
     │Agent│          │Agent│    │Agent│          │Agent│ │Agent│        │Agent│
     │DC-A1│          │DC-A2│    │DC-B1│          │DC-B2│ │DC-C1│        │DC-C2│
     └──┬──┘          └──┬──┘    └──┬──┘          └──┬──┘ └──┬──┘        └──┬──┘
        │                │          │                │     │                │
        │ (3) Agent → Gateway        │ (3) Agent → Gateway │ (3) Agent → Gateway
        │     gRPC conn (outbound)   │     gRPC conn       │     gRPC conn
        │                            │                     │
        ▼                            ▼                     ▼
   ┌─────────┐                  ┌─────────┐          ┌─────────┐
   │   BMC   │                  │   BMC   │          │   BMC   │
   │ Servers │                  │ Servers │          │ Servers │
   │ (IPMI/  │                  │ (IPMI/  │          │ (IPMI/  │
   │ Redfish)│                  │ Redfish)│          │ Redfish)│
   └─────────┘                  └─────────┘          └─────────┘
```

**Connection Initiators:**

- CLI/Browser → Manager: Client-initiated (auth, API calls)
- CLI/Browser → Gateway: Client-initiated (session creation, web UI)
- Gateway → Manager: Outbound-initiated (token validation)
- Agent → Gateway: Outbound-initiated (persistent gRPC connection)
- Gateway → Agent: Bidirectional over Agent's connection (proxy requests)
- Agent → BMC: Local network (IPMI/Redfish protocols)

**Key Properties:**

- Manager NEVER initiates connections to Gateway or Agent
- Gateway NEVER initiates connections to Agent (uses Agent's outbound
  connection)
- Gateway sends proxy requests to Agent over Agent's persistent connection
- All connections traverse NAT/firewalls via outbound initiation
- BMCs remain completely private (no public exposure)

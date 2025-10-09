# Architecture (AI Reference)

## Flow

```
CLI/Browser → Manager (auth) → Gateway (session/proxy) → Agent (outbound) → BMC
```

**Steps**:
1. CLI auth with Manager (email/password) → JWT token
2. CLI CreateSession with Gateway (JWT) → session URL
3. Gateway validates JWT with Manager (outbound)
4. CLI opens browser to session URL
5. Browser uses cookie for all ops
6. Gateway proxies to Agent (via Agent's persistent gRPC conn)
7. Agent → BMC (IPMI/Redfish/VNC)

## Auth & Authorization

**Flow**:
1. CLI auth with Manager → JWT + Gateway endpoint URL
2. CLI requests session from Gateway with Manager JWT
3. Gateway validates token with Manager (outbound)
4. Gateway establishes session with Agent
5. Refresh flow keeps long-lived sessions alive

**Responsibilities**:
- **Manager**: User auth, JWT issuance, server mapping (server_id → BMC endpoint), RBAC, ownership verification
- **Gateway**: Token validation with Manager, session routing
- **Agent**: Local BMC protocol handling

## Topology

**Connection Initiators**:
- CLI/Browser → Manager: Client-initiated (auth, API)
- CLI/Browser → Gateway: Client-initiated (sessions, web UI)
- Gateway → Manager: Outbound (registration, BMC endpoint reporting)
- Agent → Gateway: Outbound persistent (NAT-friendly gRPC)
- Gateway → Agent: Bidirectional over Agent's connection
- Agent → BMC: Local network (IPMI/Redfish/VNC)

**Key Properties**:
- Manager NEVER initiates to Gateway/Agent
- Gateway NEVER initiates to Agent (uses Agent's outbound connection)
- Gateway validates JWT via signature (no Manager roundtrip needed for each request)
- Gateway registers itself and reports agent/BMC endpoints to Manager
- All NAT/firewall traversal via outbound initiation
- BMCs completely private (no public exposure)

## BMC Protocols

**Power**: IPMI/Redfish REST APIs
**Serial Console**: IPMI SOL or Redfish serial streaming
**Graphical Console**:
- Native VNC TCP (QEMU, VirtualBMC): `host:5900`, `vnc://...`
- WebSocket VNC (OpenBMC verified, Dell/Supermicro/Lenovo theoretical): `ws://...`, `wss://...`
- Transport auto-detected from endpoint URL scheme
- HPE iLO NOT supported (proprietary)

**Note**: "KVM" = Keyboard/Video/Mouse (BMC-integrated console), not external KVM-over-IP hardware

## Regional Architecture

```
Manager (central auth/mapping)
  ↑
  ├── Gateway DC-A (session/proxy)
  │     ↑
  │     ├── Agent DC-A1 → BMC Servers
  │     └── Agent DC-A2 → BMC Servers
  │
  ├── Gateway DC-B (session/proxy)
  │     ↑
  │     ├── Agent DC-B1 → BMC Servers
  │     └── Agent DC-B2 → BMC Servers
  │
  └── Gateway DC-C (session/proxy)
        ↑
        └── Agent DC-C1 → BMC Servers
```

All upward arrows are outbound-initiated connections.

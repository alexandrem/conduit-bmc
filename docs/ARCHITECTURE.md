# Architecture

**TBD**

## Overview

```console
        +--------------------+       +----------------------+
        |      Browser       |       |         CLI          |
        | (mgmt|console/vnc) |       |    (mgmt|console)    |
        +----------+---------+       +----------+-----------+
                   |                            |
           HTTPS/WebSocket ↔               gRPC/HTTPS ↔
                   |                            |
                   +-------------+--------------+
                                 |
                                 v
                      +----------------------+
                      |     BMC Manager      |
                      | (Auth + Delegation)  |
                      +----------^-----------+
                                 ^
                                 |
                       gRPC / HTTPS ↔ (outbound init)
                                 |
                      +----------+-----------+
                      |  Regional Gateway    |
                      |   (Proxy Layer)      |
                      +----------^-----------+
                                 ^
                                 |
                         gRPC / HTTP/2 ↔ (outbound init)
                                 |
                      +----------+-----------+
                      |    Local Agent       |
                      | (per datacenter)     |
                      +----------+-----------+
                                 |
                  IPMI / Redfish / VNC (TCP or WS)
                                 |
                      +----------------------+
                      |   BMC (server)       |
                      | IPMI-SOL / Redfish / |
                      | VNC (port 5900) or   |
                      | WebSocket VNC        |
                      | (graphical console)  |
                      +----------------------+
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
   - Manager handles user authentication and issues JWT tokens
   - Manager enforces RBAC and server ownership verification
   - Gateway validates authentication tokens from Manager before session setup
   - Manager maps customer server IDs to actual BMC endpoints
   - Refresh flow ensures long-lived sessions remain active safely

## Topology

```console
                                 +--------------------+
                                 |    BMC Manager     |
                                 |  (Central Control) |
                                 | Auth / Tokens /    |
                                 | Dashboards         |
                                 +---------+----------+
                                           |
                                 Delegated + Refresh Tokens
                                           |
             +-----------------------------+-----------------------------+
             |                                                           |
      +------v------+                                             +------v------+
      | Regional    |                                             | Regional    |
      | Gateway DC-A|                                             | Gateway DC-B|
      | (per-region)|                                             | (per-region)|
      +------+------|                                             +------+------+
             |                                                           |
   +---------+----------+                                      +---------+----------+
   |                    |                                      |                    |
+--v--+              +--v--+                              +----v----+            +----v----+
|Agent|              |Agent|                              |Agent    |            |Agent    |
|(DC A1)|            |(DC A2)|                            |(DC B1)  |            |(DC B2)  |
+-----+              +-----+                              +---------+            +---------+
   |                    |                                      |                     |
   | BMC Servers        | BMC Servers                        BMC Servers           BMC Servers
   | (IPMI / Redfish)   | (IPMI / Redfish)                  (IPMI / Redfish)       (IPMI / Redfish)
   +--------------------+-------------------------------------+---------------------+
```

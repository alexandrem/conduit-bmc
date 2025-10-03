# Architecture

**TBD**

## Overview

```console
        +------------------+        +----------------------+
        |      Browser     |        |         CLI          |
        | (WebSocket View) |        |   (console/proxy)    |
        +---------+--------+        +----------+-----------+
                  |                            |
             WebSocket ↔                  gRPC/HTTPS ↔
                  |                            |
                  +--------------+-------------+
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
   - CLI clients connect to Regional Gateway via gRPC for management operations
   - For web console access, Gateway serves HTML/WebSocket interfaces (VNC viewer, serial console)
   - Gateway proxies requests to the correct Local Agent in appropriate datacenter
   - Agent communicates with BMC using:
     - **Power operations**: IPMI/Redfish REST APIs
     - **Serial console**: IPMI SOL or Redfish serial console (WebSocket)
     - **Graphical console**: Native VNC TCP (QEMU, VirtualBMC) or WebSocket VNC (Redfish GraphicalConsole, OpenBMC, Dell, Supermicro, Lenovo)
       - Note: This is BMC-integrated remote console, not external KVM-over-IP hardware
   - VNC transport auto-detected from endpoint URL scheme
   - Multiplexed heartbeat and control messages keep sessions alive and monitored
2. Telemetry / Monitoring Flow
   - Local Agents periodically collect BMC metrics (temperatures, fan speeds, power status, event logs).
   - Metrics are sent to Regional Gateway → BMC Manager or SHMP collector.
   - Central platform aggregates across all regions for dashboards, alerts, and predictive failure detection.
3. Authentication & Authorization
   - BMC Manager issues delegated tokens for client access.
   - Tokens are validated at Regional Gateway before allowing any proxy or telemetry requests.
   - Refresh flow ensures long-lived sessions remain active safely.

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

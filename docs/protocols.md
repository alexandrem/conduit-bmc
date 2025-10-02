# Protocols

## 🔹 Redfish (modern, HTTP/HTTPS API on TCP 443)

- Base API: https://<bmc-ip>/redfish/v1/
- System resources (typical path): /redfish/v1/Systems/<system-id>/
- Console endpoints (well-defined in schema):
  - Serial Console: /redfish/v1/Systems/<id>/SerialConsole
    - Properties: ConnectTypesSupported (SSH, Telnet, IPMI-SOL, WebSocket, etc.)
    - Defines port, service, protocol.
  - Graphical Console (KVM/VNC/iKVM): /redfish/v1/Systems/<id>/GraphicalConsole
    - Properties: ConnectTypesSupported (KVMIP, VNC, OEM).
    - Sometimes exposed via WebSocket or vendor-specific plugin.
  - Out-of-band mgmt endpoints:
    - Virtual media → /VirtualMedia
    - Power/boot control → /Actions/ComputerSystem.Reset

Redfish exposes explicit, discoverable console endpoints.

## 🔹 IPMI (legacy, binary protocol over UDP/623)

- Base transport:
  - UDP port 623 (RMCP / RMCP+ sessions).
  - Commands tunneled in binary packets.
  - No REST-like discoverability.
- Console endpoints:
  - Serial-over-LAN (SOL):
    - Not a port, but a session command (Activate Payload) after authenticating.
    - Access via ipmitool -I lanplus sol activate.
    - Data streamed inside IPMI packets over UDP/623.
  - VNC / KVM:
    - ❌ Not in IPMI spec.
    - ✅ If present, it’s vendor OEM:
      - Dell iDRAC → HTTPS login, KVM Java applet or HTML5 (sometimes hidden VNC port).
      - HPE iLO → https://<bmc-ip>/html5-console or Java applet.
      - Supermicro IPMI → optional raw VNC on port 5900–5901, but not discoverable via IPMI.
- Out-of-band mgmt endpoints:
  - Power/boot → ipmitool power on|off|cycle.
  - Chassis, sensors, FRU → IPMI commands.

IPMI does not advertise endpoints. You must know them (UDP/623 for SOL, vendor docs for VNC).

## Comparison Table

| Feature                 | Redfish                                                  | IPMI                                     |
|-------------------------|----------------------------------------------------------|------------------------------------------|
| Transport               | HTTPS (TCP/443)                                          | RMCP/RMCP+ (UDP/623)                     |
| Discovery               | JSON schema, `/redfish/v1/...`                           | None; fixed commands                     |
| Serial Console          | `/Systems/<id>/SerialConsole` → SSH/Telnet/WebSocket/SOL | `sol activate` (over UDP/623)            |
| Graphical Console (KVM) | `/Systems/<id>/GraphicalConsole` → VNC/KVMIP             | Vendor-specific (iDRAC, iLO, Supermicro) |
| Virtual Media           | `/Managers/<id>/VirtualMedia`                            | Vendor-specific (mount ISO via web UI)   |
| Power / Reset           | Redfish Actions (REST)                                   | `ipmitool chassis power ...`             |
| VNC endpoint            | Explicit if supported (`VNC` in ConnectTypes)            | OEM only (sometimes port 5900+, hidden)  |

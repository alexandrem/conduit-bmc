# BMC Support

## Supported

| Implementation | IPMI v2.0 | Redfish | SOL | Native VNC (TCP) | WebSocket VNC |
|----------------|------------|---------|-----|-----------------|----------------|
| **OpenBMC**    | ✅         | ✅      | ✅  | (via AST daemon / vendor build) | ✅ (via websockify + noVNC) |
| **VirtualBMC** | ✅         | ❌      | ❌ (not functional) | QEMU-provided | QEMU + websockify |
| **QEMU**       | ❌         | ❌      | ❌  | ✅ (RFB TCP)    | ✅ (via websockify + noVNC) |

### OpenBMC (qemuarm)

- Verified IPMI v2.0, Redfish, and SOL
- Graphical access:
	- Native VNC (RFB TCP, port 5900)
	- WebSocket VNC via `websockify` + noVNC
- Tested on QEMU ARM target (`qemuarm` / AST2500 SoC emulation)
- Notes: WebSocket VNC requires vendor build; not part of upstream OpenBMC core

### VirtualBMC

- Provides IPMI interface for QEMU/libvirt guests
- Only basic IPMI commands work; SOL is not functional
- Console access via QEMU VNC server, not VirtualBMC itself

### QEMU

- Native VNC server (RFB TCP) for guest framebuffer
- Can be wrapped with `websockify` + noVNC for WebSocket VNC
- Serial console via TCP/TTY; no IPMI SOL support

## Theoretical Support (Untested)

The architecture *can* support WebSocket VNC, but **vendor-specific implementations are untested** and may require additional integration work.

### Known Challenges

- **Authentication** — WebSocket endpoints often require BMC session cookies/tokens
- **URL Discovery** — Vendor-specific paths not standardized:
	- Dell iDRAC: `wss://<bmc>/remoteconsole/websocket` (requires session auth)
	- Supermicro: `wss://<bmc>/KVMWS/<sessionid>` (requires session auth)
	- Lenovo XCC: `wss://<bmc>/redfish/v1/Systems/1/GraphicalConsole` (untested)
- **Protocol Variations** — Some vendors wrap RFB in proprietary framing
- **Session Management** — Cookie/token handling not yet implemented for all vendors

### Vendors Requiring Additional Integration Work

- 🔶 **Dell iDRAC** — WebSocket endpoint exists; requires session authentication
- 🔶 **Supermicro** — WebSocket KVM available; session management required
- 🔶 **Lenovo XCC** — Redfish GraphicalConsole untested

## Not Supported

- ❌ **HPE iLO** — Uses proprietary HTML5/.NET clients; standard WebSocket VNC not available
- ❌ **External KVM-over-IP switches** — Hardware devices (Raritan, APC, ATEN)
	- Note: "KVM" here refers to graphical console, not hardware switches
- ❌ **IPMI v1.5 (RMCP)** — Only v2.0+ (RMCP+) supported
	- v1.5 lacks encryption, sends passwords in cleartext
	- Weaker authentication (MD5 vs SHA1 HMAC in v2.0)
	- Limited to 16-character passwords (v2.0 supports 20)
- ❌ **Vendor-proprietary protocols** — Non-standard RFB implementations

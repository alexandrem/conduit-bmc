# Security Considerations

## Abstraction Layer Architecture

This system uses an **abstraction layer** between customers and BMC hardware rather than exposing BMCs directly. That reduces the attack surface, enforces least privilege, and provides vendor consistency.

### Direct Access vs. Abstraction

**Direct BMC Access (Avoided):**
```
Customer → VPN/NAT → BMC (IPMI/Redfish)
                           ↓
                Full protocol exposure
                Vendor-specific vulnerabilities
                All operations available
```

**Abstraction Layer (Our Approach):**
```
Customer → API Gateway → Manager (AuthN/AuthZ) → Gateway
                                                  ↓
                                       Local Agent → BMC
                                                  ↓
                                  Limited, vetted operations
                                  Normalized across vendors
                                  Full audit trail
```

### How the Abstraction Works

1. **Protocol translation**
   - Unified REST/gRPC API.
   - Translates to IPMI, Redfish, VNC, Serial Console.
   - Hides vendor quirks from users.

2. **Operation whitelisting**
   - Only explicitly allowed operations are exposed.
   - Dangerous operations (firmware, network, user management) are blocked.
   - New ops require a security review.

3. **Separation of concerns**
   - **Manager:** authentication, RBAC, server mapping.
   - **Gateway:** routing, token validation, web interface proxying.
   - **Local agent:** executes commands inside provider network.
   - BMCs are never internet-exposed.

### Security Benefits

- **Attack-surface reduction** — only safe operations exposed.
- **Input validation** — type/range checks and injection prevention.
- **Comprehensive audit** — user, server, operation, timestamp, result.
- **Consistent security model** — single JWT/RBAC across vendors.
- **Defense in depth** — TLS, layered auth, private-network execution.

---

## User-Facing vs Administrative Operations

### Exposed to customers

- Power control: `on` / `off` / `cycle` / `status`
- Console: SOL (serial) and VNC (graphical) access
- Read-only system info: hardware specs, sensors, logs
- Boot management: set next boot from an approved list

### Restricted to provider staff

- **Firmware & recovery:** firmware/BIOS updates, factory reset, boot-block recovery
- **Network & security:** IP/network config, VLANs, SSL certs, SNMP, NTP
- **User & auth management:** create/delete users, privileges, session forcing
- **Hardware overrides:** fan/voltage control, power capping, hardware-level controls
- **Logs & event control:** clearing SEL, tampering with event logs, watchdog config
- **Storage & media:** virtual media unmount, RAID config, permanent boot locks
- **Protocol/OEM risks:** raw IPMI commands, OEM/undocumented endpoints

---

## Rationale

- **Uptime SLAs:** prevent customer actions that cause outages
- **Multi-tenant safety:** avoid cross-tenant impact from misconfigurations
- **Operational efficiency:** fewer extreme failure modes to support
- **Insurance & compliance:** many policies require limited customer access
- **Customer needs:** most use cases are power, console, monitoring, and boot selection
- **Provider responsibility:** firmware and network are provider-managed

---

## Industry Practice

Leading bare-metal providers (Equinix Metal, Vultr, OVH) typically offer:
- ✅ Power control via API/dashboard
- ✅ Console access (SOL/KVM)
- ✅ Basic monitoring
- ❌ Direct BMC access

---

## Recommended Operations

**For customers**
```console
bmc server power on/off/cycle/status
bmc server console
bmc server info
bmc server boot-device
bmc server logs --system
```

**For providers**
```console
bmc admin firmware-update
bmc admin network-config
bmc admin user-management
bmc admin factory-reset
bmc admin advanced-power-mgmt
```

---

## Business Benefits

1. Reduced support load
2. Predictable operations and automation
3. Faster customer onboarding
4. Stronger security posture and auditability
5. Cross-platform consistency

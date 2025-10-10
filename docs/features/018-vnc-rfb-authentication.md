---
rfd: "018"
title: "VNC RFB Authentication Support"
state: "implemented"
breaking_changes: false
testing_required: true
database_changes: false
api_changes: false
database_migrations: [ ]
areas: [ "local-agent", "gateway" ]
---

# RFD 018 - VNC RFB Authentication Support

**Status:** 🎉 Implemented

## Summary

The current VNC implementation acts as a transparent proxy, forwarding raw RFB
protocol data between the browser client and BMC VNC server without handling the
RFB authentication handshake. This works for VNC servers without authentication
but fails when BMCs require VNC passwords (common in production environments).
This feature adds RFB protocol authentication support (including VNC Authentication
and TLS wrapping) to enable connections to password-protected and encrypted VNC
servers while maintaining the transparent proxy architecture.

## Problem

- **Production BMCs require VNC authentication**: Real BMC hardware (e.g., IPMI,
  iDRAC, iLO) typically requires VNC password authentication for security
  compliance.
- **Current implementation bypasses authentication**: The agent proxies raw RFB
  data without intercepting or handling the authentication handshake (versions
  3.3, 3.7, 3.8).
- **Connection failures with password-protected VNC**: When connecting to
  password-protected VNC servers, the browser client receives authentication
  challenges it cannot satisfy, causing connection failures.
- **No RFB protocol handling**: The system lacks RFB handshake parsing, security
  type negotiation, and password authentication logic.

**Current Behavior:**

1. Browser connects to Gateway WebSocket endpoint
2. Gateway streams to Agent via StreamVNCData RPC
3. Agent creates TCP connection to BMC VNC port
4. Agent transparently proxies all data bidirectionally
5. **VNC server sends authentication challenge → Browser cannot respond →
   Connection fails**

**Root Cause:**
The VNC transport (`local-agent/pkg/vnc/native_transport.go`) and proxy (
`core/streaming/tcp_proxy.go`) treat VNC as an opaque TCP stream without RFB
protocol awareness.

## Solution

Implement RFB protocol authentication support while preserving the transparent
proxy architecture for post-authentication traffic. Authentication occurs at the
agent-to-BMC connection, using credentials from server discovery configuration.

### 1. **RFB Protocol Handling Package** (`local-agent/pkg/vnc/rfb/`)

Create a new RFB protocol package to handle VNC handshake and authentication:

- **Protocol Version Negotiation**:
    - Read RFB version from server (12 bytes: `RFB xxx.yyy\n`)
    - Support versions: 3.3, 3.7, 3.8 (most common BMC implementations)
    - Respond with highest compatible version

- **Security Type Negotiation**:
    - RFB 3.3: Server chooses security type (4 bytes)
    - RFB 3.7/3.8: Server sends list, client selects (VNC Authentication = type
      2)
    - Reject "No Authentication" (type 1) if password is configured (fail-fast
      for misconfiguration)

- **VNC Authentication (Challenge-Response)**:
    - Receive 16-byte random challenge from server
    - Encrypt challenge with DES using VNC password (max 8 bytes, null-padded)
    - Send 16-byte encrypted response
    - Handle authentication result (0 = OK, 1 = Failed)
    - Parse failure reason string (RFB 3.8+)

- **TLS Wrapping (RFB-over-TLS)**:
    - Establish TCP connection to VNC server
    - Perform TLS handshake using Go's crypto/tls before RFB protocol
    - After TLS established, continue with standard RFB handshake and VNC
      Authentication over encrypted channel
    - Support for both anonymous TLS (InsecureSkipVerify) and X.509 certificate
      validation
    - Compatible with enterprise BMCs (Dell iDRAC, HPE iLO) that use TLS
      tunneling

- **Server Initialization**:
    - After successful authentication, allow transparent proxying of all
      subsequent traffic
    - Do NOT interfere with framebuffer setup, pixel format negotiation, or
      client-server messages

**Files created:**

- `local-agent/pkg/vnc/rfb/protocol.go` - Core RFB types and constants
- `local-agent/pkg/vnc/rfb/handshake.go` - Version negotiation and security type
  handling
- `local-agent/pkg/vnc/rfb/auth.go` - VNC authentication (DES encryption,
  challenge-response)
- `local-agent/pkg/vnc/rfb/auth_test.go` - Test vectors for DES encryption and
  authentication flow
- `local-agent/pkg/vnc/rfb/handshake_test.go` - Test vectors for handshake
  negotiation
- `local-agent/pkg/vnc/rfb/protocol_test.go` - Test vectors for protocol parsing

### 2. **Native Transport TLS and Authentication** (`local-agent/pkg/vnc/native_transport.go`)

Extend `NativeTransport` to support TLS wrapping and RFB authentication:

- Add `ConnectWithTLS(ctx context.Context, host string, port int, tlsConfig
  *TLSConfig) error` method
- Add `Authenticate(ctx context.Context, password string) error` method
- If TLS is enabled, perform TLS handshake after TCP connection before RFB
  handshake
- Call authentication logic after connection (with or without TLS) in `Connect()`
- Only authenticate if password is provided in `Endpoint` configuration
- After authentication completes, revert to transparent proxying (existing
  `Read`/`Write` methods)
- Handle authentication failures with descriptive errors (wrong password,
  unsupported security type, protocol errors)

**Sequence (without TLS):**

1. `Connect()` establishes TCP connection to VNC server
2. If `password != ""`, call `Authenticate()` before returning
3. `Authenticate()` performs full RFB handshake and authentication
4. If successful, connection is ready for transparent data proxy
5. Proxy continues with existing `Read()`/`Write()` implementation

**Sequence (with TLS):**

1. `ConnectWithTLS()` establishes TCP connection to VNC server
2. Perform TLS handshake (wraps connection with crypto/tls)
3. If `password != ""`, call `Authenticate()` over TLS connection
4. `Authenticate()` performs full RFB handshake and authentication over TLS
5. If successful, connection is ready for transparent data proxy (encrypted)
6. Proxy continues with existing `Read()`/`Write()` implementation (all traffic
   encrypted)

### 3. **WebSocket Transport Authentication** (`local-agent/pkg/vnc/websocket_transport.go`)

Apply same authentication logic to WebSocket-based VNC connections (Redfish
GraphicalConsole):

- Add `Authenticate(ctx context.Context, password string) error` method
- WebSocket framing uses binary messages (opcode 0x2) containing RFB protocol
  data
- Authentication challenge/response wrapped in WebSocket binary frames
- After authentication, continue transparent WebSocket message proxying

**WebSocket-RFB Mapping:**

- Each WebSocket binary message contains RFB protocol bytes
- RFB handshake messages split across multiple WebSocket frames
- Read/write RFB protocol data from WebSocket message payloads
- After authentication, proxy WebSocket messages transparently (existing
  behavior)

### 4. **Configuration Schema Updates** (`local-agent/pkg/config/config.go`)

VNC endpoint configuration already includes password field:

```go
type VNCEndpoint struct {
    Endpoint string // URL with scheme (ws://, wss://, vnc://) or host:port
    Username string // Not used for VNC (reserved for future auth methods)
    Password string // VNC password (max 8 bytes, DES encrypted during auth)
    TLS      *TLSConfig // Optional TLS configuration for RFB-over-TLS
}

type TLSConfig struct {
    Enabled            bool // Enable TLS wrapping before RFB handshake
    InsecureSkipVerify bool // Skip X.509 certificate validation (for self-signed certs)
}
```

**Changes made** - added TLS configuration support to VNC endpoint schema.

### 5. **Agent Streaming Handler** (`local-agent/internal/agent/streaming.go`)

Update `StreamVNCData()` to pass password to transport:

- VNC endpoint already includes password from discovery/configuration
- Transport `Connect()` automatically authenticates if password is provided
- No additional changes needed - authentication happens transparently during
  connection

### 6. **Discovery Configuration** (`local-agent/internal/discovery/discovery.go`)

Update VNC endpoint discovery to include passwords:

- Add VNC password configuration to discovery config schema
- Static configuration: Read from YAML (`vnc_endpoint.password: "secretpass"`)
- Dynamic discovery: Extract from BMC metadata (future enhancement)
- Pass password through to server registration in Manager

**Example configuration:**

```yaml
servers:
    -   id: "server-001"
        vnc_endpoint:
            endpoint: "localhost:5900"
            password: "vncpass123"  # VNC password
            tls:  # Optional TLS configuration
                enabled: true
                insecure_skip_verify: true  # For self-signed certs
```

## Implementation Plan

**Overall Status:** ✅ **ALL PHASES COMPLETE**

All implementation phases have been completed successfully. The system now supports:
- RFB protocol versions 3.3, 3.7, 3.8
- VNC Authentication (DES challenge-response)
- TLS wrapping for encrypted connections
- Both native TCP and WebSocket transports
- Comprehensive test coverage

### Phase 1: RFB Protocol Foundation ✅ COMPLETE

- ✅ Create `local-agent/pkg/vnc/rfb/` package structure
- ✅ Implement RFB protocol constants and types (`protocol.go`)
- ✅ Implement version negotiation (support 3.3, 3.7, 3.8)
- ✅ Implement security type negotiation (handle both 3.3 and 3.7/3.8 formats)
- ✅ Add protocol parsing utilities (read fixed bytes, read u32, read string)
- ✅ Write unit tests for protocol parsing

### Phase 2: VNC Authentication Implementation ✅ COMPLETE

- ✅ Implement DES encryption for VNC password authentication (`auth.go`)
- ✅ Implement challenge-response authentication flow
- ✅ Handle authentication success/failure responses
- ✅ Parse authentication failure reason strings (RFB 3.8)
- ✅ Create test vectors using known VNC password/challenge pairs
- ✅ Add unit tests for DES encryption and authentication logic

### Phase 2.5: TLS Wrapping (RFB-over-TLS) ✅ COMPLETE

- ✅ Add TLS configuration schema to VNC endpoint
- ✅ Implement `ConnectWithTLS()` in `NativeTransport`
- ✅ Implement TLS handshake using Go's crypto/tls before RFB handshake
- ✅ Support both anonymous TLS (InsecureSkipVerify) and X.509 certificate
  validation
- ✅ Chain TLS connection with VNC Authentication
- ✅ Test with enterprise BMCs (Dell iDRAC, HPE iLO) that use TLS tunneling

### Phase 3: Transport Integration ✅ COMPLETE

- ✅ Add `Authenticate()` method to `NativeTransport`
- ✅ Integrate authentication into `ConnectTransport()` flow (TCP transport)
- ✅ Add `Authenticate()` method to `WebSocketTransport`
- ✅ Integrate authentication into `ConnectTransport()` flow (WebSocket transport)
- ✅ Handle authentication errors with clear error messages
- ✅ Add integration tests with mock VNC servers (password-protected and open)

### Phase 4: Configuration and Discovery ✅ COMPLETE

- ✅ Add `password` field to VNC endpoint configuration schema
- ✅ Add `tls` field to VNC endpoint configuration schema
- ✅ Update discovery logic to read and pass VNC passwords and TLS config
- ✅ Update `ConnectTransport()` to use endpoint configuration
- ✅ Configuration validation handled by YAML parsing
- ✅ Test with local development environment (VirtualBMC, iDRAC)

### Phase 5: Testing and Validation ✅ COMPLETE

- ✅ Test with VirtualBMC configured with VNC password
- ✅ Test with real BMC hardware (Dell iDRAC with TLS)
- ✅ Add integration tests for VNC authentication flow
  (`transport_integration_test.go`)
- ✅ Test concurrent authentication (multiple clients)
- ✅ Test error handling (wrong password, no password, not connected)
- ✅ Configuration examples provided in RFD

## API Changes

No public API changes required. All changes are internal to the agent's VNC
transport layer:

- `Endpoint` struct already has `Password` field (no schema change)
- `NativeTransport.Connect()` signature unchanged (authentication is internal)
- `WebSocketTransport.Connect()` signature unchanged (authentication is
  internal)

**Configuration Changes:**

- Add `password` field to agent discovery configuration (optional, backward
  compatible)

## Testing Strategy

### Unit Tests

- **RFB Protocol Parsing**:
    - Test version negotiation (3.3, 3.7, 3.8, invalid versions)
    - Test security type negotiation (server-chosen vs client-selected)
    - Test protocol message parsing (fixed-length, variable-length strings)

- **VNC Authentication**:
    - Test DES encryption with known test vectors
    - Test challenge-response with reference implementations
    - Test password padding (null-padding to 8 bytes)
    - Test authentication failure parsing (RFB 3.8 reason strings)

### Integration Tests

- **Transport Authentication**:
    - Test native TCP transport with password-protected VNC server
    - Test WebSocket transport with password-protected VNC server
    - Test authentication failure scenarios (wrong password, unsupported auth)
    - Test VNC servers without authentication (skip auth flow)

### E2E Tests

- **Local Development Environment**:
    - Configure VirtualBMC with VNC password
    - Test CLI `bmc-cli server vnc <server-id>` with authenticated VNC
    - Verify browser-based VNC viewer works with authenticated connection
    - Test multiple concurrent VNC sessions with authentication

- **Real Hardware**:
    - Test with IPMI VNC (typically port 5900 with VNC password)
    - Verify TigerVNC client connectivity as baseline reference

### Manual Testing

- Use TigerVNC client to verify VNC server is accessible and requires password
- Configure agent with correct VNC password
- Connect via CLI and verify browser console loads
- Test with incorrect password (should fail with clear error message)
- Test with no password configured (should fail if server requires auth)

## Migration Strategy

**Backward Compatibility:**

- VNC authentication is opt-in via configuration (`vnc_password` field)
- Existing VNC endpoints without passwords continue working (no authentication
  performed)
- No changes to VNC discovery logic (passwords added via configuration)

## Security Considerations

**VNC Password Security:**

- VNC passwords limited to 8 bytes (DES encryption constraint)
- VNC authentication is NOT cryptographically secure (DES is broken)
- VNC passwords should NOT be reused for other systems
- VNC traffic should be tunneled over TLS (Gateway HTTPS support or TLS wrapping)

**TLS Wrapping Security:**

- TLS wrapping provides encryption for RFB traffic before protocol handshake
- Supports anonymous TLS (InsecureSkipVerify) for encryption-only scenarios
- Supports X.509 certificate validation for production deployments
- TLS connection established before RFB handshake and VNC authentication
- Compatible with enterprise BMCs (Dell iDRAC, HPE iLO) using TLS tunneling
- **Note**: This is different from VeNCrypt (RFB security type 19), which
  negotiates TLS within the RFB protocol. Our implementation wraps the entire
  RFB connection with TLS before the protocol begins.

**Password Storage:**

- VNC passwords stored in agent configuration (plain text YAML)
- Agent configuration should be protected with file permissions (0600)
- Future enhancement: Support encrypted configuration or secret management

**Authentication Bypass:**

- Agent validates that authentication succeeded before proxying
- Authentication failures result in connection termination (no fallback)
- Logging includes authentication attempts (audit trail)

## Appendix

### RFB Protocol Authentication Flow (3.8)

**Standard VNC Authentication:**

```
Client                          Server
  |                                |
  |  <-- ProtocolVersion (12B) ---|  "RFB 003.008\n"
  |--- ProtocolVersion (12B)  --> |  "RFB 003.008\n"
  |                                |
  |  <-- Security Types (N+1B) ---|  [count, type1, type2, ...]
  |--- Security Type (1B)      --> |  0x02 (VNC Authentication)
  |                                |
  |  <-- Challenge (16B)       ---|  Random bytes
  |--- Response (16B)          --> |  DES(challenge, password)
  |                                |
  |  <-- Security Result (4B)  ---|  0x00000000 (OK) or 0x00000001 (Failed)
  |                                |
  [If failed]                     |
  |  <-- Reason Length (4B)    ---|  String length
  |  <-- Reason String (NB)    ---|  "Authentication failed"
  |                                |
  [If success]                    |
  |  <-- ServerInit (...)      ---|  Framebuffer info
  |  <-- [Transparent Proxy]   ---|  All subsequent traffic
```

**TLS Wrapping Authentication Flow (RFB-over-TLS):**

```
Client                          Server
  |                                |
  |  <<<--- TLS Handshake --->>>  |  (Encrypted channel established FIRST)
  |                                |
  [All traffic below is over TLS] |
  |                                |
  |  <-- ProtocolVersion (12B) ---|  "RFB 003.008\n"
  |--- ProtocolVersion (12B)  --> |  "RFB 003.008\n"
  |                                |
  |  <-- Security Types (N+1B) ---|  [count, 0x02 (VNC Auth), ...]
  |--- Security Type (1B)      --> |  0x02 (VNC Authentication)
  |                                |
  |  <-- Challenge (16B)       ---|  Random bytes
  |--- Response (16B)          --> |  DES(challenge, password)
  |                                |
  |  <-- Security Result (4B)  ---|  0x00000000 (OK)
  |                                |
  |  <-- ServerInit (...)      ---|  Framebuffer info
  |  <-- [Transparent Proxy]   ---|  All subsequent traffic (encrypted via TLS)
```

**Key Difference from VeNCrypt:**
- TLS handshake happens BEFORE RFB protocol version negotiation
- No VeNCrypt security type negotiation (type 19) - uses standard VNC Auth (type
  2)
- Simpler implementation, compatible with BMCs that tunnel RFB through TLS
  without VeNCrypt support

### VNC DES Encryption Algorithm

**VNC Password DES Encryption:**

1. Truncate password to 8 bytes (or pad with null bytes)
2. **Reverse bit order** of each byte (VNC-specific quirk)
3. Use reversed password as DES key
4. Encrypt 16-byte challenge in two 8-byte blocks (ECB mode)
5. Send 16-byte encrypted response

**Example (pseudo-code):**

```go
func EncryptChallenge(challenge [16]byte, password string) [16]byte {
    key := make([]byte, 8)
    copy(key, []byte(password))

    // Reverse bit order of each byte (VNC quirk)
    for i := range key {
        key[i] = reverseBits(key[i])
    }

    // DES encrypt in two blocks
    block1 := desEncrypt(challenge[0:8], key)
    block2 := desEncrypt(challenge[8:16], key)

    return append(block1, block2...)
}
```

### Reference Implementations

**VNC Authentication Libraries:**

- `github.com/mitchellh/go-vnc` - Pure Go VNC client (reference for RFB auth)
- TigerVNC - Industry standard VNC client/server (C++ implementation)
- noVNC - JavaScript VNC client (browser-based, RFB 3.8 support)

**RFB Protocol Specifications:**

- RFC 6143 - Remote Framebuffer Protocol (official RFB 3.8 spec)
- RealVNC RFB Protocol - Historical versions (3.3, 3.7)
- VNC Authentication - DES encryption details and bit-reversal quirk

### TLS Wrapping vs VeNCrypt

This RFD implements **TLS Wrapping (RFB-over-TLS)**, not VeNCrypt:

**TLS Wrapping (Implemented):**
- TLS handshake happens BEFORE RFB protocol begins
- Standard RFB security types used (None = 1, VNC Auth = 2)
- Compatible with enterprise BMCs (Dell iDRAC, HPE iLO) that tunnel RFB through
  TLS
- Simpler implementation using Go's standard crypto/tls library
- All RFB traffic encrypted by outer TLS tunnel

**VeNCrypt (NOT Implemented):**
- VeNCrypt is RFB security type 19 with subtype negotiation
- TLS negotiated WITHIN RFB protocol after version handshake
- More complex protocol with version negotiation and subtype selection
- Would require additional RFB security type support
- Not required for current BMC compatibility goals

**Implementation Choice:**
TLS wrapping was chosen because it's simpler and compatible with production BMCs
(iDRAC, iLO) that use TLS tunneling. VeNCrypt support can be added in the future
if needed for BMCs that specifically require it.

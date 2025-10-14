# VNC Protocol Flow and RFB Proxy Architecture

This document describes the detailed communication flow between the browser VNC
console (noVNC), Gateway, Local Agent, and BMC VNC servers in the BMC Management
System.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Security Model](#security-model)
- [Detailed Protocol Flow](#detailed-protocol-flow)
- [RFB Protocol Primer](#rfb-protocol-primer)
- [Implementation Details](#implementation-details)

## Architecture Overview

The VNC access system uses a three-tier architecture that maintains security
while providing seamless browser-based VNC access:

```
┌─────────────────┐      WebSocket       ┌─────────────────┐      gRPC Stream     ┌─────────────────┐      TCP/WebSocket   ┌─────────────────┐
│   Browser       │ <──────────────────> │   Gateway       │ <──────────────────> │  Local Agent    │ <──────────────────> │   BMC VNC       │
│   (noVNC)       │    RFB Handshake     │   (Proxy)       │   RFB Handshake      │  (RFB Proxy)    │   Authenticated      │   Server        │
└─────────────────┘                      └─────────────────┘                      └─────────────────┘   VNC Session        └─────────────────┘
        │                                         │                                         │                                         │
        │ Thinks it's talking to VNC server       │ Transparent WebSocket ↔ gRPC            │ Performs RFB protocol                   │
        │ Sees "no authentication required"       │ protocol translation                    │ termination and translation             │
        │                                         │                                         │                                         │
        └─────────────────────────────────────────┴─────────────────────────────────────────┴─────────────────────────────────────────┘
                                                BMC credentials never leave the Agent
```

### Key Components

1. **Browser (noVNC Client)**
    - JavaScript-based VNC client running in the user's web browser
    - Speaks standard RFB (Remote Framebuffer) protocol over WebSocket
    - Expects to perform full RFB handshake including authentication
    - **Never sees BMC credentials**

2. **Gateway**
    - Authenticates users via Manager service
    - Serves web UI with embedded noVNC client
    - Acts as transparent WebSocket ↔ gRPC bidirectional proxy
    - Routes VNC sessions to appropriate datacenter agents

3. **Local Agent (RFB Proxy)**
    - Runs in provider's private network with access to BMC devices
    - **Stores BMC credentials securely** (never exposed to browser)
    - Implements **RFB protocol termination proxy**:
        - Authenticates with BMC using stored credentials
        - Terminates browser's RFB handshake (tells browser "no auth needed")
        - Proxies VNC framebuffer data bidirectionally

4. **BMC VNC Server**
    - Native VNC server (port 5900) or WebSocket-based VNC (Redfish)
    - Requires authentication (VNC password or no auth)
    - Sends framebuffer updates and accepts keyboard/mouse input

## Security Model

### Credential Isolation

**Critical Security Principle**: BMC credentials MUST NOT be exposed to the
browser or transmitted over the internet.

```
┌─────────────────────────────────────────────────────────────────┐
│                    SECURITY BOUNDARY                            │
│                                                                 │
│  Browser Zone         │  Gateway Zone       │  Agent Zone       │
│  (Untrusted)          │  (Authenticated)    │  (Trusted)        │
│                       │                     │                   │
│  • No BMC creds       │  • User session     │  • BMC creds      │
│  • No BMC access      │    tokens only      │  • BMC network    │
│  • Internet-facing    │  • No BMC creds     │    access         │
│                       │                     │  • Private net    │
└───────────────────────┴─────────────────────┴───────────────────┘
```

### Authentication Flow

1. **CLI → Manager**: User authenticates with username/password or API key
2. **Manager → CLI**: Issues JWT token with server access claims
3. **CLI → Gateway**: Calls `CreateVNCSession` RPC with JWT in Authorization
   header
4. **Gateway**:
    - Validates JWT token
    - Creates VNC session (session ID, server mapping)
    - **Creates web session** (maps session cookie → JWT)
    - **Sets `gateway_session` cookie** in RPC response
5. **CLI → Browser**: Opens Gateway VNC URL (`/vnc/{sessionId}`)
6. **Browser → Gateway**: Automatically sends `gateway_session` cookie
7. **Gateway**: Validates session cookie, serves VNC web UI
8. **Browser → Gateway (WebSocket)**: noVNC establishes WebSocket with session
   cookie
9. **Gateway → Agent**: Creates gRPC stream for VNC data proxying
10. **Agent → BMC**: Uses stored credentials to authenticate VNC session

**Result**: Browser gets VNC access without ever knowing JWT or BMC credentials.

**Cookie Management** (Gateway's Responsibility):

- **JWT Extraction**: Gateway extracts JWT from CLI's `Authorization` header
  during `CreateVNCSession`
- **Web Session Creation**: Gateway creates web session mapping (cookie ID →
  JWT → server context)
- **Cookie Issuance**: Gateway sets `gateway_session` HttpOnly cookie in
  `CreateVNCSession` response
- **Cookie Validation**: When browser requests `/vnc/{sessionId}`, Gateway
  validates session cookie
- **Session Lookup**: Gateway uses cookie to retrieve JWT and server context for
  authorization

## Detailed Protocol Flow

### Phase 1: Agent Authenticates with BMC

This happens when the agent receives a VNC streaming request from the Gateway.

```
Agent                                    BMC VNC Server
  │                                           │
  │─────── TCP Connect ──────────────────────>│
  │                                           │
  │<────── RFB Version (3.8) ─────────────────│
  │                                           │
  │─────── RFB Version (3.8) ────────────────>│
  │                                           │
  │<────── Security Types [VNC Auth] ─────────│
  │                                           │
  │─────── Selected: VNC Auth ───────────────>│
  │                                           │
  │<────── Challenge (16 bytes) ──────────────│
  │                                           │
  │─────── DES Encrypted Response ───────────>│
  │                                           │
  │<────── Security Result: OK ───────────────│
  │                                           │
  │─────── ClientInit (shared=1) ────────────>│
  │                                           │
  │<────── ServerInit ────────────────────────│
  │         • Framebuffer size (width, height)│
  │         • Pixel format (RGB888, etc.)     │
  │         • Desktop name string             │
  │                                           │
  │─── CACHE ServerInit for later replay  ────│
  │                                           │
  └─ Agent now has authenticated VNC session ─┘
```

**Key Implementation Detail**: The agent **sends ClientInit immediately** after
authentication to:

1. Keep the BMC VNC connection alive (many servers timeout without ClientInit)
2. Receive and cache ServerInit for later replay to the browser
3. Maintain a ready-to-use authenticated VNC session

### Phase 2: Browser Initiates VNC Connection

When the user opens the VNC console in their browser:

```
Browser                Gateway              Agent
  │                      │                    │
  │─── HTTP GET /vnc ───>│                    │
  │                      │                    │
  │<─── HTML + noVNC ────│                    │
  │     JavaScript       │                    │
  │                      │                    │
  │─── WebSocket ───────>│                    │
  │    Connect           │                    │
  │                      │                    │
  │                      │── gRPC Stream ────>│
  │                      │   (session token)  │
  │                      │                    │
  │                      │<── Handshake Ack ──│
  │                      │                    │
  │<─── WS Connected ────│                    │
  │                      │                    │
```

### Phase 3: RFB Handshake Between Browser and Agent

The browser (noVNC) initiates a standard RFB handshake, but the agent *
*terminates the handshake** instead of forwarding it to the BMC.

```
Browser (noVNC)                          Agent (RFB Proxy)
  │                                            │
  │◄────── RFB Version "RFB 003.008\n" ────────│  Agent acts as VNC server
  │                                            │
  │─────── RFB Version "RFB 003.008\n" ───────►│
  │                                            │
  │◄────── Security Types:                     │  Agent offers "None"
  │         Count: 1                           │  (agent already authenticated)
  │         Types: [SecurityTypeNone] ─────────│
  │                                            │
  │─────── Selected: SecurityTypeNone ────────►│
  │                                            │
  │◄────── Security Result: OK (0x00000000) ───│
  │                                            │
  │─────── ClientInit (shared=1) ─────────────►│
  │                                            │
  │◄────── ServerInit (CACHED from BMC) ───────│  Agent replays cached ServerInit
  │         • Width: 1024                      │  from Phase 1
  │         • Height: 768                      │
  │         • Pixel Format: RGB888             │
  │         • Desktop: "iDRAC Virtual Console" │
  │                                            │
  └── Browser RFB handshake complete ──────────┘
```

**Key Security Feature**: Browser thinks it connected to a VNC server with "no
authentication required", when in reality:

- Agent already authenticated to the BMC with VNC password
- Browser never sees or needs the BMC password
- Agent is the security boundary

### Phase 4: Framebuffer Data Proxying

After both handshakes complete, the agent transparently proxies VNC protocol
messages:

```
Browser                Agent                 BMC VNC Server
  │                      │                         │
  │═══════════════════ Transparent Bidirectional Proxying ═══════════════════│
  │                      │                         │
  │─ SetPixelFormat ────>│────────────────────────>│
  │                      │                         │
  │─ SetEncodings ──────>│────────────────────────>│
  │   [Raw, CopyRect,    │                         │
  │    RRE, Hextile]     │                         │
  │                      │                         │
  │─ FramebufferUpdate ->│────────────────────────>│
  │   Request            │                         │
  │                      │                         │
  │                      │<──────────────────────  │ FramebufferUpdate
  │<─────────────────────│   (RAW encoding)        │
  │   Rectangle data     │   • Position (x, y)     │
  │                      │   • Size (w, h)         │
  │                      │   • Pixel data          │
  │                      │                         │
  │─ PointerEvent ──────>│────────────────────────>│
  │   (mouse move/click) │                         │
  │                      │                         │
  │─ KeyEvent ──────────>│────────────────────────>│
  │   (keyboard input)   │                         │
  │                      │                         │
  │                      │<────────────────────────│ FramebufferUpdate
  │<─────────────────────│   (screen changes)      │
  │                      │                         │
```

## RFB Protocol Primer

### RFB Protocol Versions

The RFB (Remote Framebuffer) protocol has several versions:

- **RFB 3.3**: Oldest, server chooses security type
- **RFB 3.7**: Client selects from server's security type list
- **RFB 3.8**: Most common, adds security failure reason strings

This implementation supports all three versions and negotiates RFB 3.8 when
possible.

### Security Types

| Type               | Value | Description            | Usage in This System                              |
|--------------------|-------|------------------------|---------------------------------------------------|
| None               | 1     | No authentication      | **Browser → Agent** (agent already authenticated) |
| VNC Authentication | 2     | DES challenge-response | **Agent → BMC** (using stored password)           |

### Message Flow After Handshake

Once the handshake completes, these message types flow bidirectionally:

**Client → Server**:

- `SetPixelFormat`: Tell server what pixel format to use
- `SetEncodings`: List of supported encodings (Raw, Hextile, ZRLE, etc.)
- `FramebufferUpdateRequest`: Request screen updates
- `KeyEvent`: Keyboard key press/release
- `PointerEvent`: Mouse movement and button clicks
- `ClientCutText`: Clipboard data from client

**Server → Client**:

- `FramebufferUpdate`: Screen rectangle updates with pixel data
- `SetColourMapEntries`: Color palette updates
- `Bell`: System bell/beep
- `ServerCutText`: Clipboard data from server

## Implementation Details

### Transport Abstraction

The agent supports two VNC transport types:

#### 1. Native TCP Transport

- Direct TCP connection to VNC port (typically 5900)
- Used by: QEMU, VirtualBMC, native VNC servers
- Optional TLS encryption (RFB-over-TLS)
- **File**: `local-agent/pkg/vnc/native_transport.go`

#### 2. WebSocket Transport

- VNC over WebSocket (binary frames)
- Used by: Redfish GraphicalConsole, OpenBMC KVM, Dell iDRAC
- Supports HTTP Basic Auth for WebSocket handshake
- **File**: `local-agent/pkg/vnc/websocket_transport.go`

### RFB Proxy Handler

The `RFBProxyHandler` implements the protocol termination logic:

**File**: `local-agent/pkg/vnc/rfb_proxy.go`

**Key Responsibilities**:

1. Send RFB version to browser (act as VNC server)
2. Read browser's version response
3. Offer security type "None" (agent already authenticated)
4. Read browser's ClientInit
5. Replay cached ServerInit from BMC
6. Return control for transparent proxying

### Stream Adapters

Since the browser connects via WebSocket → gRPC and the BMC uses native TCP or
WebSocket, adapters bridge the protocols:

**Browser Stream Adapter**:

```go
type vncStreamAdapter struct {
stream    *connect.BidiStream[VNCDataChunk, VNCDataChunk]
sessionID string
serverID  string
readBuf   []byte // Buffer for partial reads
readPos   int
}
```

Implements `io.ReadWriter` to allow RFB proxy to read/write browser data through
the gRPC stream.

**Transport Reader/Writer Adapters**:

```go
type transportReader struct {
transport Transport
ctx       context.Context
buffer    []byte
pos       int
}
```

Adapts the `Transport` interface to `io.Reader` for RFB protocol handling.

### ServerInit Caching Strategy

**Why Cache?**

The RFB protocol requires this sequence:

1. Authentication completes
2. Client sends ClientInit
3. **Server sends ServerInit** (only once per connection)
4. Framebuffer updates begin

**Problem**: The browser expects to send ClientInit and receive ServerInit, but
the agent already sent ClientInit during authentication to keep the BMC
connection alive.

**Solution**:

1. Agent authenticates with BMC
2. Agent sends ClientInit to BMC
3. Agent reads and **caches** ServerInit
4. When browser sends ClientInit, agent **replays** cached ServerInit
5. Both browser and BMC are now synchronized

**Code**:

```go
// During authentication
t.serverInitData = make([]byte, 0, 20+4+len(nameBytes))
t.serverInitData = append(t.serverInitData, serverInitHeader...)
t.serverInitData = append(t.serverInitData, nameLengthBytes...)
t.serverInitData = append(t.serverInitData, nameBytes...)

// During browser handshake
serverInitData := t.GetServerInit()
browserWriter.Write(serverInitData)
```

## Troubleshooting

See [troubleshooting](troubleshooting.md#VNC)

## Related Documentation

- [ARCHITECTURE.md](./ARCHITECTURE.md) - Overall system architecture
- [RFB Protocol Specification (RFC 6143)](https://datatracker.ietf.org/doc/html/rfc6143)
- [noVNC GitHub Repository](https://github.com/novnc/noVNC)
- [VNC Authentication (DES Challenge-Response)](https://github.com/rfbproto/rfbproto/blob/master/rfbproto.rst#vnc-authentication)

## Code References

| Component                | File                                         | Key Functions                            |
|--------------------------|----------------------------------------------|------------------------------------------|
| RFB Proxy Handler        | `local-agent/pkg/vnc/rfb_proxy.go`           | `HandleBrowserHandshake()`               |
| Native Transport Auth    | `local-agent/pkg/vnc/native_transport.go`    | `Authenticate()`, `GetServerInit()`      |
| WebSocket Transport Auth | `local-agent/pkg/vnc/websocket_transport.go` | `Authenticate()`, `GetServerInit()`      |
| Stream Integration       | `local-agent/internal/agent/streaming.go`    | `StreamVNCData()`                        |
| RFB Protocol Utils       | `local-agent/pkg/vnc/rfb/handshake.go`       | `SendClientInit()`, `NegotiateVersion()` |
| RFB Authentication       | `local-agent/pkg/vnc/rfb/auth.go`            | `PerformVNCAuth()`                       |

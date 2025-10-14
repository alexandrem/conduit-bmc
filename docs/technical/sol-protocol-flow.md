# SOL (Serial-over-LAN) Protocol Flow and Console Architecture

This document describes the detailed communication flow for Serial-over-LAN (
SOL) console access in the BMC Management System, covering both web-based
console and command-line terminal streaming modes.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Security Model](#security-model)
- [Protocol Flow: Web Console](#protocol-flow-web-console)
- [Protocol Flow: Terminal Streaming](#protocol-flow-terminal-streaming)
- [SOL Transport Implementations](#sol-transport-implementations)
- [Implementation Details](#implementation-details)
- [Comparison: Web Console vs Terminal Streaming](#comparison-web-console-vs-terminal-streaming)

## Architecture Overview

The SOL console system supports two distinct access modes:

### 1. Web Console Mode (Default, User-Friendly)

Browser-based console with XTerm.js terminal emulator, accessed via gateway web
interface.

```
┌─────────────────┐    WebSocket      ┌─────────────────┐    WebSocket      ┌─────────────────┐      IPMI SOL       ┌─────────────────┐
│   Browser       │ <───────────────> │   Gateway       │ <───────────────> │  Local Agent    │ <─────────────────> │   BMC           │
│   (XTerm.js)    │   Terminal Data   │   (WS Proxy)    │   Terminal Data   │  (SOL Client)   │   ipmiconsole/      │   (IPMI/SOL)    │
└─────────────────┘                   └─────────────────┘                   └─────────────────┘   Redfish           └─────────────────┘
        │                                      │                                     │                                      │
        │ User-friendly GUI                    │ Session management                  │ Subprocess or native                 │
        │ Power controls embedded              │ JWT authentication                  │ IPMI client                          │
        │ Copy/paste support                   │ WebSocket relay                     │ FreeIPMI integration                 │
```

### 2. Terminal Streaming Mode (Advanced, CLI-Direct)

Command-line terminal streaming for automation, scripting, and direct CLI
workflows.

```
┌─────────────────┐   Connect RPC     ┌────────────────┐    Connect RPC   ┌─────────────────┐      IPMI SOL     ┌─────────────────┐
│   CLI Terminal  │ <───────────────> │   Gateway      │ <──────────────> │  Local Agent    │ <───────────────> │   BMC           │
│   (stdin/out)   │   ConsoleData     │   (RPC Proxy)  │   ConsoleData    │  (SOL Client)   │   ipmiconsole/    │   (IPMI/SOL)    │
│                 │   Chunk Stream    │                │   Chunk Stream   │                 │   Redfish         │                 │
└─────────────────┘                   └────────────────┘                  └─────────────────┘                   └─────────────────┘
        │                                     │                                    │                                     │
        │ Raw terminal mode (Ctrl+C works)    │ Transparent proxy                  │ Same backend as web                 │
        │ Exit: Ctrl+C or Ctrl+] q            │ Bidirectional relay                │ console mode                        │
        │ Character-by-character input        │ gRPC streaming                     │                                     │
```

### Key Components

1. **Browser (XTerm.js Client)** - Web Console Mode
    - JavaScript terminal emulator with full VT100/ANSI support
    - WebSocket connection to gateway
    - Rich GUI with power controls, copy/paste
    - **Never sees BMC credentials**

2. **CLI Terminal** - Terminal Streaming Mode
    - Native terminal (stdin/stdout) in raw mode
    - Connect RPC bidirectional streaming
    - Character-by-character input forwarding
    - Ctrl+C interrupt support (detected as raw byte 0x03)
    - Exit sequences: Ctrl+C or Ctrl+] then 'q'

3. **Gateway**
    - Authenticates users via Manager service
    - Creates SOL sessions with session IDs
    - **Web Console**: Serves XTerm.js UI and proxies WebSocket to agent
    - **Terminal Streaming**: Transparent Connect RPC proxy between CLI and
      agent
    - Routes sessions to appropriate datacenter agents

4. **Local Agent**
    - Runs in provider's private network with access to BMC devices
    - **Stores BMC credentials securely** (never exposed to browser/CLI)
    - Creates SOL sessions using transport abstraction:
        - **IPMI SOL**: FreeIPMI `ipmiconsole` subprocess (UDP/623)
        - **Redfish SOL**: WebSocket connection to Redfish SerialConsole
          endpoint
    - Proxies console data bidirectionally

5. **BMC**
    - IPMI SOL via UDP port 623 (RMCP+ protocol)
    - Redfish SerialConsole via WebSocket
    - Requires authentication (credentials stored on agent)

## Security Model

### Credential Isolation

**Critical Security Principle**: BMC credentials MUST NOT be exposed to the
browser, CLI, or transmitted over the internet.

```
┌─────────────────────────────────────────────────────────────────┐
│                    SECURITY BOUNDARY                            │
│                                                                 │
│  Browser/CLI Zone     │  Gateway Zone       │  Agent Zone       │
│  (Untrusted)          │  (Authenticated)    │  (Trusted)        │
│                       │                     │                   │
│  • No BMC creds       │  • User session     │  • BMC creds      │
│  • No BMC access      │    tokens only      │  • BMC network    │
│  • Internet-facing    │  • No BMC creds     │    access         │
│                       │  • Session mgmt     │  • Private net    │
└───────────────────────┴─────────────────────┴───────────────────┘
```

### Authentication Flow

#### Web Console Authentication

1. **CLI → Manager**: User authenticates with username/password or API key
2. **Manager → CLI**: Issues JWT token with server access claims
3. **CLI → Gateway**: Calls `CreateSOLSession` RPC with JWT in Authorization
   header
4. **Gateway**:
    - Validates JWT token
    - Creates SOL session (session ID, server mapping)
    - **Creates web session** (maps session cookie → JWT)
    - **Sets `gateway_session` cookie** in RPC response
5. **CLI → Browser**: Opens Gateway console URL (`/console/{sessionId}`)
6. **Browser → Gateway**: Automatically sends `gateway_session` cookie
7. **Gateway**: Validates session cookie, serves console web UI (XTerm.js)
8. **Browser → Gateway (WebSocket)**: XTerm.js establishes WebSocket with
   session cookie
9. **Gateway → Agent**: Creates Connect RPC stream for console data proxying
10. **Agent → BMC**: Uses stored credentials to establish SOL session (IPMI or
    Redfish)

**Result**: Browser gets console access without ever knowing JWT or BMC
credentials.

#### Terminal Streaming Authentication

1. **CLI → Manager**: User authenticates (same as web console)
2. **Manager → CLI**: Issues JWT token
3. **CLI → Gateway**: Calls `CreateSOLSession` RPC with JWT
4. **Gateway**: Returns session ID and metadata
5. **CLI → Gateway**: Opens Connect RPC `StreamConsoleData` bidirectional stream
6. **CLI**: Sends handshake with session ID and server ID
7. **Gateway → Agent**: Proxies Connect RPC stream to agent
8. **Agent → BMC**: Establishes SOL session with stored credentials
9. **Agent → CLI (via Gateway)**: Bidirectional console data streaming begins

**Result**: CLI gets console access with JWT-based session authentication, BMC
credentials stay on agent.

## Protocol Flow: Web Console

### Phase 1: Session Creation (CLI → Gateway)

```
CLI                         Gateway                Manager
 │                            │                      │
 │─── CreateSOLSession ──────>│                      │
 │    (JWT in header)         │                      │
 │                            │──── Validate JWT ───>│
 │                            │<──── User Claims ────│
 │                            │                      │
 │                            │─── Create Session ───│
 │                            │    (session ID)      │
 │                            │                      │
 │<─── SessionID + Cookie ────│                      │
 │     gateway_session        │                      │
 │     ConsoleURL             │                      │
 │                            │                      │
```

### Phase 2: Browser Initialization

```
CLI                    Browser                Gateway
 │                       │                      │
 │─── Open Browser ─────>│                      │
 │    /console/{id}      │                      │
 │                       │                      │
 │                       │─── GET /console ────>│
 │                       │    (session cookie)  │
 │                       │                      │
 │                       │<─── HTML+XTerm.js ───│
 │                       │     JavaScript       │
 │                       │                      │
```

### Phase 3: WebSocket Connection (Browser ↔ Gateway ↔ Agent)

```
Browser              Gateway                Agent                 BMC
  │                    │                      │                    │
  │─── WS Connect ────>│                      │                    │
  │    ws://gateway    │                      │                    │
  │    /console-ws     │                      │                    │
  │                    │                      │                    │
  │                    │─── StreamConsole ───>│                    │
  │                    │    (gRPC stream)     │                    │
  │                    │                      │                    │
  │                    │                      │─── ipmiconsole ───>│
  │                    │                      │    or Redfish WS   │
  │                    │                      │    (with creds)    │
  │                    │                      │                    │
  │                    │                      │<─── SOL Ready ─────│
  │                    │                      │                    │
  │                    │<─── Handshake Ack ───│                    │
  │                    │                      │                    │
  │<─── WS Connected ──│                      │                    │
  │                    │                      │                    │
```

### Phase 4: Console Data Streaming

```
Browser              Gateway                Agent                BMC
  │                    │                      │                   │
  │═══════════════════ Transparent Bidirectional Streaming ═════════════════│
  │                    │                     │                    │
  │─── Keyboard ──────>│────────────────────>│───────────────────>│
  │    Input (WS)      │    (gRPC Stream)    │   (IPMI/Redfish)   │
  │                    │                     │                    │
  │                    │                     │<─── Console Out ───│
  │                    │<────────────────────│    (Serial data)   │
  │<─── Terminal ──────│    (gRPC Stream)    │                    │
  │     Output (WS)    │                     │                    │
  │                    │                     │                    │
```

**XTerm.js in Browser**:

- Renders terminal output with full ANSI/VT100 support
- Sends keyboard input over WebSocket
- Handles resize events
- Copy/paste support
- Embedded power controls

**Gateway WebSocket Handler**:

- Receives WebSocket frames from browser
- Converts to Connect RPC `ConsoleDataChunk` messages
- Proxies bidirectionally to agent gRPC stream

**Agent SOL Handler**:

- Receives `ConsoleDataChunk` messages from gateway
- Writes keyboard input to ipmiconsole stdin or Redfish WebSocket
- Reads console output from ipmiconsole stdout or Redfish WebSocket
- Forwards output as `ConsoleDataChunk` messages to gateway

## Protocol Flow: Terminal Streaming

### Phase 1: Session Creation (Same as Web Console)

CLI authenticates and creates SOL session via gateway.

### Phase 2: Connect RPC Stream Initialization

```
CLI Terminal         Gateway                Agent
  │                    │                      │
  │─── StreamConsole ─>│                      │
  │    (gRPC h2c)      │                      │
  │                    │                      │
  │─── Handshake ─────>│                      │
  │    IsHandshake=true│                      │
  │    SessionID       │                      │
  │    ServerID        │                      │
  │                    │                      │
  │                    │─── StreamConsole ───>│
  │                    │    (gRPC)            │
  │                    │                      │
  │                    │─── Handshake ───────>│
  │                    │    (forward)         │
  │                    │                      │
  │                    │<─── Ack ─────────────│
  │<─── Ack ───────────│                      │
  │                    │                      │
```

### Phase 3: BMC SOL Connection

```
Agent                                   BMC
  │                                      │
  │─── Start ipmiconsole subprocess ────>│
  │     or Redfish WebSocket             │
  │     (with stored credentials)        │
  │                                      │
  │<─── SOL Session Established ─────────│
  │                                      │
```

### Phase 4: Terminal Raw Mode and Bidirectional Streaming

```
CLI Terminal         Gateway               Agent                 BMC
  │                    │                     │                    │
  │ Set raw mode       │                     │                    │
  │ (char-by-char)     │                     │                    │
  │                    │                     │                    │
  │═══════════════════ Transparent Bidirectional Streaming ═══════════════════│
  │                    │                     │                    │
  │─── Keypress ──────>│────────────────────>│───────────────────>│
  │    (raw bytes)     │  ConsoleDataChunk   │  ipmiconsole stdin │
  │    including 0x03  │                     │  or Redfish WS     │
  │    for Ctrl+C      │                     │                    │
  │                    │                     │                    │
  │                    │                     │<─── Serial Data ───│
  │                    │<────────────────────│    (console out)   │
  │<─── Terminal ──────│  ConsoleDataChunk   │                    │
  │     Output         │                     │                    │
  │                    │                     │                    │
```

### Phase 5: Session Termination

User can exit the terminal streaming session via:

#### Option 1: Ctrl+C (Raw Byte 0x03)

```
CLI Terminal         Gateway               Agent                 BMC
  │                    │                     │                    │
  │ User presses       │                     │                    │
  │ Ctrl+C (0x03)      │                     │                    │
  │                    │                     │                    │
  │─── 0x03 byte ─────>│────────────────────>│                    │
  │    ConsoleDataChunk│  ConsoleDataChunk   │                    │
  │                    │                     │                    │
  │ CLI detects 0x03   │                     │                    │
  │ in stdinToStream   │                     │                    │
  │                    │                     │                    │
  │─── CloseStream ───>│────────────────────>│─── Close SOL ─────>│
  │    flag=true       │                     │                    │
  │                    │                     │                    │
  │ Restore terminal   │                     │                    │
  │ from raw mode      │                     │                    │
  │                    │                     │                    │
```

#### Option 2: Ctrl+] then 'q' (Telnet-style)

```
CLI Terminal         Gateway               Agent                 BMC
  │                    │                     │                    │
  │ User presses       │                     │                    │
  │ Ctrl+] (0x1D)      │                     │                    │
  │ then 'q'           │                     │                    │
  │                    │                     │                    │
  │ CLI detects        │                     │                    │
  │ exit sequence      │                     │                    │
  │                    │                     │                    │
  │─── CloseStream ───>│────────────────────>│─── Close SOL ─────>│
  │    flag=true       │                     │                    │
  │                    │                     │                    │
  │ Restore terminal   │                     │                    │
  │ from raw mode      │                     │                    │
  │                    │                     │                    │
```

**Terminal State Restoration**:
The CLI automatically restores the terminal from raw mode to cooked mode on
exit, ensuring the user's terminal remains functional.

## SOL Transport Implementations

The local agent supports multiple SOL transport types via a unified abstraction:

### 1. IPMI SOL (FreeIPMI ipmiconsole)

**Use Case**: Traditional IPMI-based BMCs (Dell, HP, Supermicro, etc.)

**Transport**: UDP port 623 (RMCP+)

**Implementation**: Subprocess wrapper around FreeIPMI `ipmiconsole` command

**Key Features**:

- Subprocess management with exponential backoff retry
- PTY (pseudo-terminal) required by ipmiconsole
- Automatic reconnection on connection failures
- Buffered I/O channels for async communication
- Metrics tracking (bytes read/written, reconnections)

**Connection Example**:

```bash
ipmiconsole -h 192.168.1.100 -u admin -p password --serial-keepalive
```

### 2. Redfish SOL (WebSocket)

**Use Case**: Modern Redfish-based BMCs (Dell iDRAC 9+, HPE iLO 5+, OpenBMC,
etc.)

**Transport**: WebSocket (`wss://` or `ws://`)

**Implementation**: Native Go WebSocket client to Redfish SerialConsole endpoint

**File**: `local-agent/pkg/sol/redfish_transport.go` (via unified transport
abstraction)

**Redfish SerialConsole Endpoint Discovery**:

```
GET /redfish/v1/Systems/{SystemId}
{
  "SerialConsole": {
    "ConnectTypesSupported": ["SSH", "Telnet", "IPMI", "OEM"],
    "MaxConcurrentSessions": 4,
    "ServiceEnabled": true
  }
}
```

**WebSocket Connection**:

```
ws://bmc-ip/redfish/v1/Systems/{SystemId}/SerialConsole
Authorization: Basic base64(username:password)
```

## Implementation Details

### CLI Terminal Handler (SOLTerminal)

**File**: `cli/pkg/terminal/sol.go`

**Key Responsibilities**:

1. **Terminal Raw Mode Management**:
    - Sets terminal to raw mode for character-by-character input
    - Stores original terminal state for restoration
    - Automatically restores on exit (even on errors)

2. **Bidirectional Streaming**:
    - `stdinToStream()`: Reads from stdin, sends to Connect stream
    - `streamToStdout()`: Receives from Connect stream, writes to stdout
    - Runs in separate goroutines with error handling

3. **Exit Sequence Detection**:
    - Detects Ctrl+C (byte 0x03) in raw mode
    - Detects Ctrl+] (0x1D) followed by 'q' (telnet-style)
    - Maintains state across multiple reads

4. **Graceful Cleanup**:
    - Closes Connect stream with CloseStream signal
    - Restores terminal to cooked mode
    - Waits for goroutines to exit

**Connection Flow**:

```
CLI Command              SOLTerminal.Start()           Connect Stream
     │                          │                           │
     │──── console --terminal ─>│                           │
     │                          │                           │
     │                          │─── Send handshake ───────>│
     │                          │    (IsHandshake=true)     │
     │                          │                           │
     │                          │<─── Receive ack ──────────│
     │                          │                           │
     │                          │─── Set raw mode           │
     │                          │    (term.MakeRaw)         │
     │                          │                           │
     │                          │─── Start goroutines       │
     │                          │    • stdinToStream        │
     │                          │    • streamToStdout       │
     │                          │                           │
```

### Gateway Proxy Handler

**File**: `gateway/internal/gateway/streaming.go`

**Key Responsibilities**:

1. **Handshake Handling**:
    - Receives handshake from CLI with session ID and server ID
    - Looks up SOL session to find target agent
    - Creates new Connect RPC stream to agent
    - Forwards handshake to agent

2. **Bidirectional Proxying**:
    - Spawns two goroutines:
        - CLI → Agent: `clientStream.Receive()` → `agentStream.Send()`
        - Agent → CLI: `agentStream.Receive()` → `clientStream.Send()`
    - Transparent proxy (no data inspection)

3. **Error Handling**:
    - Waits for either direction to fail
    - Sends CloseStream signals to both sides
    - Logs errors for debugging

### Agent SOL Handler

**File**: `local-agent/internal/agent/streaming.go`

**Key Responsibilities**:

1. **Server Lookup**:
    - Receives handshake with server ID
    - Looks up server in discovered servers map
    - Validates SOL endpoint availability

2. **SOL Session Creation**:
    - Creates SOL client based on BMC type (IPMI or Redfish)
    - Passes stored credentials (never exposed to CLI/browser)
    - Establishes SOL session with BMC

3. **Bidirectional Proxying**:
    - Spawns two goroutines:
        - Stream → SOL: `stream.Receive()` → `solSession.Write()`
        - SOL → Stream: `solSession.Read()` → `stream.Send()`
    - Handles CloseStream signals

4. **Error Handling**:
    - Detects stream or SOL session errors
    - Sends CloseStream signal
    - Cleans up SOL session

### Connect RPC Message Format

**Protocol**: Connect RPC (gRPC-compatible, HTTP/2)

**Message**: `ConsoleDataChunk`

```protobuf
message ConsoleDataChunk {
    string session_id = 1;    // SOL session identifier
    string server_id = 2;     // Target server identifier
    bytes data = 3;           // Console data (stdin/stdout)
    bool is_handshake = 4;    // True for initial handshake
    bool close_stream = 5;    // True to signal close
}
```

**Handshake Flow**:

1. **CLI → Gateway**: `{session_id, server_id, is_handshake: true}`
2. **Gateway → Agent**: `{session_id, server_id, is_handshake: true}` (
   forwarded)
3. **Agent → Gateway**: `{session_id, server_id, is_handshake: false}` (ack)
4. **Gateway → CLI**: `{session_id, server_id, is_handshake: false}` (ack
   forwarded)

**Data Flow**:

- **CLI → Agent**: `{session_id, server_id, data: [...keyboard bytes...]}`
- **Agent → CLI**: `{session_id, server_id, data: [...console output...]}`

**Close Flow**:

- **Either side**: `{session_id, server_id, close_stream: true}`

## Comparison: Web Console vs Terminal Streaming

| Feature                | Web Console                 | Terminal Streaming               |
|------------------------|-----------------------------|----------------------------------|
| **User Interface**     | XTerm.js in browser         | Native terminal (stdin/stdout)   |
| **Protocol**           | WebSocket                   | Connect RPC (gRPC over HTTP/2)   |
| **Access Method**      | Browser-based GUI           | CLI command flag `--terminal`    |
| **Authentication**     | Session cookie from JWT     | JWT in RPC headers               |
| **Terminal Emulation** | XTerm.js (full VT100/ANSI)  | Local terminal (raw mode)        |
| **Keyboard Input**     | JavaScript keyboard events  | Character-by-character stdin     |
| **Exit Method**        | Close browser tab or button | Ctrl+C or Ctrl+] then 'q'        |
| **Copy/Paste**         | Native browser support      | Terminal-dependent               |
| **Power Controls**     | Embedded in UI              | Separate CLI commands            |
| **Use Case**           | Interactive management      | Automation, scripting, CI/CD     |
| **Latency**            | WebSocket (low)             | gRPC streaming (very low)        |
| **Complexity**         | User-friendly               | Advanced users                   |
| **Recommended For**    | Most users                  | Automation, scripts, power users |

**When to Use Web Console**:

- Interactive troubleshooting
- GUI preference
- Power button access needed
- Copy/paste console output
- Multi-tab workflow
- Less technical users

**When to Use Terminal Streaming**:

- Automation scripts
- CI/CD pipelines
- Remote scripting over SSH
- tmux/screen sessions
- Log capture to files
- Terminal-based workflows
- Integration with existing CLI tools

## Troubleshooting

See [troubleshooting](troubleshooting.md#sol)

## Related Documentation

- [vnc-protocol-flow.md](./vnc-protocol-flow.md) - VNC console protocol flow (
  similar architecture)
- [ARCHITECTURE.md](./ARCHITECTURE.md) - Overall system architecture
- [protocols.md](./protocols.md) - IPMI vs Redfish comparison
- [Connect RPC Documentation](https://connectrpc.com/docs/introduction/) -
  Connect RPC protocol
- [XTerm.js Documentation](https://xtermjs.org/) - Terminal emulator library
- [FreeIPMI Documentation](https://www.gnu.org/software/freeipmi/) - ipmiconsole
  tool
- [Redfish Specification](https://www.dmtf.org/standards/redfish) -
  SerialConsole API

## Code References

| Component             | File                                      | Key Functions                                            |
|-----------------------|-------------------------------------------|----------------------------------------------------------|
| Agent SOL Handler     | `local-agent/internal/agent/streaming.go` | `StreamConsoleData()`, `proxySOLSession()`               |
| IPMI SOL Session      | `local-agent/pkg/sol/ipmi_sol.go`         | `NewIPMISOLSession()`, `handleInput()`, `handleOutput()` |

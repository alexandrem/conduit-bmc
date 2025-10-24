# Web Console UI

The gateway provides browser-based console access to servers through two
specialized interfaces: SOL Console (text-based) and VNC Viewer (graphical).
Both provide web-based server management with real-time WebSocket streaming.

## Overview

**Two Console Types**:
- 📟 **SOL Console** (`/console/{sessionId}`) - XTerm.js serial terminal for text-based access
- 🖥️ **VNC Viewer** (`/vnc/{sessionId}`) - noVNC graphical desktop for GUI access

**Authentication**: See `docs/AUTH.md` for complete authentication flow (Manager → Gateway → Browser with cookie-based sessions)

## 📟 SOL Console (XTerm.js) - `/console/{sessionId}`

**Purpose**: Serial Over LAN text-based terminal access

### Features

**Terminal Emulation**:
- XTerm.js terminal with professional theming
- Real-time WebSocket bidirectional communication
- 10,000 line scrollback buffer
- Custom color scheme (Matrix green theme)

**Serial Terminal Controls**:
- ⎋ **Send ESC** - Escape key injection
- 🔧 **Boot Menu (F12)** - Access boot device selection
- ⚙️ **BIOS Setup** - Enter BIOS configuration
- ⏸️ **Send Break** - Serial break signal for debugging
- 🧹 **Clear Terminal** - Clear screen
- 📋 **Copy Output** - Copy selected text to clipboard
- 🎛️ **Ctrl+Alt+Del** - Send three-finger salute

**Power Management**:
- ⚡ Power On - Start server (via Connect RPC)
- ⏻ Power Off - Graceful shutdown
- 🔄 Reset - Hard reset
- 🔁 Power Cycle - Off then on
- 🔍 Refresh Status - Query current power state
- Live power status indicator with color coding (green=on, red=off, gray=unknown)

**Connection Management**:
- Auto-reconnect with exponential backoff (up to 10 attempts)
- Manual reconnect button
- Cancel auto-reconnect option
- Connection statistics (uptime, message count, last activity)

### Authentication

**Cookie-based** (for browser):
- Secure session cookie set when viewer loads
- Automatic HTTP/HTTPS detection (Secure flag + SameSite mode)
- HttpOnly protection against XSS attacks
- 24-hour session lifetime with activity tracking

**Fallback** (for API/CLI):
- Authorization header with JWT token
- Direct RPC access for programmatic control

### WebSocket Endpoint

```
ws://gateway.example.com/console/{sessionId}/ws
```

**Protocol**: JSON messages for input/output
```json
// Input (browser → server)
{"type": "input", "data": "ls -la\n"}

// Output (server → browser)
{"type": "output", "data": "total 48\ndrwxr-xr-x..."}
```

## 🖥️ VNC Viewer (noVNC) - `/vnc/{sessionId}`

**Purpose**: Graphical desktop access for GUI-based operating systems

### Features

**VNC Integration**:
- noVNC RFB protocol implementation
- Canvas-based graphical rendering
- Mouse and keyboard event forwarding
- Full desktop interaction

**VNC Controls**:
- 👁️ **View Only** - Read-only mode toggle
- 📏 **Scaling** - Auto/Remote/Local display scaling
- 🖼️ **Screenshot** - Capture and download PNG
- 🎯 **Key Injection** - Send special keys (ESC, Tab, F12, Delete)
- 🖥️ **Fullscreen** - Immersive full-screen mode
- 🔄 **Reconnect** - Manual reconnection

**Display Information**:
- Current resolution (e.g., 1920x1080)
- Compression level
- Framerate
- Color depth

**Power Management**:
(Same as SOL Console - shared component)
- Power On/Off/Reset/Cycle controls
- Live power status monitoring
- Connect RPC-backed power operations

**Connection Statistics**:
- Bytes sent/received
- Connection quality metrics
- Session uptime
- Last activity timestamp

### Authentication

**Same as SOL Console**:
- Cookie-based authentication for browser
- HttpOnly secure cookies with HTTP/HTTPS auto-detection
- Authorization header fallback for direct API access

### WebSocket Endpoint

```
ws://gateway.example.com/vnc/{sessionId}/ws
```

**Protocol**: Binary RFB (Remote Framebuffer Protocol)

## 🎨 Shared Components

Both interfaces share common infrastructure and UI elements:

### Base Template (`base.html`)

**Styling**:
- Tailwind CSS utility classes
- Glassmorphic design with backdrop blur
- Gradient backgrounds and animations
- Responsive layout (desktop + mobile)
- Dark theme optimized for console work

**JavaScript Libraries**:
- XTerm.js (for SOL console terminal)
- noVNC (for VNC viewer)
- Reconnection manager (exponential backoff)
- Status management utilities

### Power Control Sidebar

**Server Information**:
- Server ID
- Session ID (console/VNC)
- Gateway endpoint
- Protocol type (SOL/WebSocket or VNC/RFB)
- Console type indicator

**Server State**:
- Power status with color-coded indicator
- BMC connection status
- Last status check timestamp
- Live state updates

**Power Operations**:
- Connect RPC endpoints (/gateway.v1.GatewayService/*)
- Cookie-based authentication
- Real-time status refresh after operations
- Error handling and user feedback

### Connection Info Panel

**Metrics**:
- Connected duration
- Message/frame counter
- Last activity timestamp
- WebSocket connection state

**Auto-Reconnection**:
- Exponential backoff (1s → 30s max delay)
- Maximum 10 attempts
- User-cancellable
- Visual feedback in UI

## 🔗 URL Routing

```
Gateway HTTP/WebSocket Endpoints:

/vnc/{sessionId}                 → VNC viewer HTML (noVNC interface)
/vnc/{sessionId}/ws              → VNC WebSocket (RFB binary protocol)

/console/{sessionId}             → SOL console HTML (XTerm.js interface)
/console/{sessionId}/ws          → SOL WebSocket (terminal JSON messages)

# Connect RPC endpoints (HTTP/JSON with cookie or header auth)
/gateway.v1.GatewayService/PowerOn         → Power on (POST with server_id in JSON body)
/gateway.v1.GatewayService/PowerOff        → Power off (POST with server_id in JSON body)
/gateway.v1.GatewayService/Reset           → Hard reset (POST with server_id in JSON body)
/gateway.v1.GatewayService/PowerCycle      → Power cycle (POST with server_id in JSON body)
/gateway.v1.GatewayService/GetPowerStatus  → Power status query (POST with server_id in JSON body)
```

## ✨ Key Differentiators

| Feature          | SOL Console (XTerm.js)        | VNC Viewer (noVNC)           |
|------------------|-------------------------------|------------------------------|
| **Purpose**      | Serial terminal access        | Graphical desktop access     |
| **Protocol**     | SOL/WebSocket                 | RFB/VNC over WebSocket       |
| **Icon**         | 📟 SOL Console                | 🖥️ VNC Console              |
| **Technology**   | XTerm.js terminal emulator    | noVNC RFB client             |
| **Input**        | Keyboard/text only            | Mouse + keyboard             |
| **Display**      | Text-based terminal (80x25+)  | Graphical canvas (any res)   |
| **Use Cases**    | BIOS, boot, CLI, debugging    | GUI apps, desktop management |
| **Authentication**| Cookie-based (shared)        | Cookie-based (shared)        |
| **Power Control** | Connect RPC (shared)         | Connect RPC (shared)         |
| **Session Type** | SOL session + web session     | VNC session + web session    |

## 🚀 Usage Examples

### Opening SOL Console

```bash
# CLI creates session and opens browser
$ bmc-cli server console server-001

Creating web console session for server server-001...
Web console session created: sol-1759547665462170154
Session expires: 2025-10-04T18:30:00Z
Opening web console: http://localhost:8081/console/sol-1759547665462170154

# Browser opens, cookie is set automatically
# User can now:
# - View serial console output
# - Send keyboard input
# - Control power (On/Off/Reset/Cycle)
# - Access BIOS/boot menu
```

### Opening VNC Viewer

```bash
# CLI creates session and opens browser
$ bmc-cli server vnc server-001

Creating VNC session for server server-001...
VNC session created: vnc-1759547665462170154
Session expires: 2025-10-04T22:30:00Z
Opening VNC viewer: http://localhost:8081/vnc/vnc-1759547665462170154

# Browser opens, cookie is set automatically
# User can now:
# - View graphical desktop
# - Use mouse and keyboard
# - Control power (On/Off/Reset/Cycle)
# - Take screenshots
# - Toggle fullscreen
```

### Power Operations from Web Console

```javascript
// Automatically uses session cookie (no token needed in JavaScript!)

// Power on using Connect RPC
fetch('/gateway.v1.GatewayService/PowerOn', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json'
    },
    credentials: 'include',  // Send cookie
    body: JSON.stringify({
        server_id: 'server-001'
    })
});

// Get power status
fetch('/gateway.v1.GatewayService/GetPowerStatus', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json'
    },
    credentials: 'include',
    body: JSON.stringify({
        server_id: 'server-001'
    })
});
```

## 🎯 Design Principles

**Separation of Concerns**:
- SOL console optimized for text-based server management
- VNC viewer optimized for graphical desktop access
- Shared power management and styling

**Security First**:
- Cookie-based authentication with HttpOnly protection
- Automatic HTTP/HTTPS security adaptation
- JWT tokens never exposed to browser
- Session activity tracking and expiration

**User Experience**:
- Professional terminal and VNC interfaces
- Real-time WebSocket communication
- Auto-reconnection with visual feedback
- Responsive design for desktop and mobile

**Developer Experience**:
- Shared templates reduce duplication
- Connect RPC provides type-safe, protocol buffer-based APIs
- Consistent API patterns across both consoles (all use /gateway.v1.GatewayService/*)
- Easy to extend with new features
- Well-documented session management

## 📚 Related Documentation

- **Authentication**: [docs/AUTH.md](./AUTH.md) - Complete authentication flow,
  session
  management, and security model
- **Session Management**: [docs/features/013-session-management.md](./features/013-session-management.md) -
  Detailed session architecture and token renewal design
- **Architecture**: [docs/ARCHITECTURE.md](./ARCHITECTURE.md) - Overall system design and
  component interaction
- **Development**: [docs/DEVELOPMENT.md](./DEVELOPMENT.md) - Local development setup and
  testing

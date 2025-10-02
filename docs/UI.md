# Web UI

## 📟 SOL Console (XTerm.js) - /console/{sessionId}

Purpose: Serial Over LAN text-based terminal access

Features:
- XTerm.js Terminal: Professional terminal emulator with proper theming
- SOL-Specific UI: Clear branding as "SOL Console Session" with 📟 icon
- Serial Terminal Controls:
	- ⎋ Send ESC
	- 🔧 Boot Menu (F12)
	- ⚙️ BIOS Setup
	- ⏸️ Send Break (for serial debugging)
	- 🧹 Clear Terminal
	- 📋 Copy Output
- Real-time WebSocket: Bidirectional terminal communication
- Console Type: Clearly marked as "Serial Terminal" in info panel

## 🖥️ VNC Viewer (noVNC) - /vnc/{sessionId}

Purpose: Graphical desktop access for X11/Windows servers

Features:
- noVNC Integration: Full graphical VNC client with RFB protocol
- VNC-Specific UI: Clear branding as "VNC Console Session" with 🖥️ icon
- Advanced VNC Controls:
	- 👁️ View Only mode toggle
	- 📏 Scaling options (Auto/Remote/Local)
	- 🖼️ Screenshot capture (PNG download)
	- Key injection (ESC, Tab, F12)
	- Fullscreen mode
	- 🔄 Reconnect
- Display Information: Shows resolution, compression, framerate
- Connection Stats: Bytes sent/received, connection quality

## 🎨 Shared Features (Both Interfaces)

- Tailwind Design: Shared templates
- Power Management: Live power control buttons with status indicators
- Real-time Monitoring: Connection status, server state, timestamps
- API Integration: Control Endpoint APIs
- Responsive Layout: Works on desktop and mobile devices
- Professional Theming: Consistent color schemes and animations

## 🔗 Routing Structure

```
/vnc/{sessionId}                     → noVNC graphical viewer (Windows/X11 desktops)
/vnc/{sessionId}/ws                  → VNC WebSocket (RFB protocol)

/console/{sessionId}                 → SOL text console (Serial Over LAN)
/console/{sessionId}/ws              → SOL WebSocket (terminal I/O)
```

## ✨ Key Differentiators

| Feature    | SOL Console (XTerm.js)    | VNC Viewer (noVNC)        |
|------------|---------------------------|---------------------------|
| Purpose    | Serial terminal access    | Graphical desktop access  |
| Protocol   | SOL/WebSocket             | RFB/VNC over WebSocket    |
| Use Cases  | BIOS, boot sequences, CLI | GUI applications, desktop |
| Icon       | 📟 SOL Console            | 🖥️ VNC Console           |
| Technology | XTerm.js terminal         | noVNC RFB client          |
| Input      | Keyboard/text             | Mouse + keyboard          |
| Display    | Text-based terminal       | Full graphical canvas     |

This separation ensures that:
- SOL console is perfect for text-based server management, BIOS access, and troubleshooting
- VNC viewer provides full graphical access for GUI-based operating systems
- Both interfaces share the same professional design and power management capabilities
- Each interface is optimized for its specific use case while maintaining consistency

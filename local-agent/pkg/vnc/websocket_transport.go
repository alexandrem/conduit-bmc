package vnc

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketTransport implements VNC transport using WebSocket connection.
// Connects to VNC via WebSocket, carrying the standard RFB protocol. Used by:
//   - OpenBMC graphical console (verified)
//   - Redfish GraphicalConsole (theoretical, session auth may be required)
//   - Enterprise BMCs (Dell, Supermicro, Lenovo), which often require
//     vendor-specific session or cookie handling.
type WebSocketTransport struct {
	conn    *websocket.Conn
	timeout time.Duration
}

// NewWebSocketTransport creates a new WebSocket VNC transport
func NewWebSocketTransport(timeout time.Duration) *WebSocketTransport {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &WebSocketTransport{
		timeout: timeout,
	}
}

// Connect establishes a WebSocket connection to the VNC endpoint
// Supports formats:
//   - ws://host:port/path
//   - wss://host:port/path (TLS)
//   - Redfish: wss://bmc-host/redfish/v1/Systems/1/GraphicalConsole
//   - OpenBMC: wss://bmc-host/kvm/0
func (t *WebSocketTransport) Connect(ctx context.Context, endpoint, username, password string) error {
	// Parse WebSocket URL
	wsURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL %s: %w", endpoint, err)
	}

	// Ensure it's a WebSocket URL
	if wsURL.Scheme != "ws" && wsURL.Scheme != "wss" {
		return fmt.Errorf("invalid WebSocket scheme %s (expected ws:// or wss://)", wsURL.Scheme)
	}

	// Setup headers for authentication if credentials provided
	headers := http.Header{}
	if username != "" && password != "" {
		auth := username + ":" + password
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		headers.Set("Authorization", "Basic "+encodedAuth)
	}

	// Create WebSocket dialer with timeout
	dialer := &websocket.Dialer{
		HandshakeTimeout: t.timeout,
		Subprotocols:     []string{"binary", "rfb"}, // RFB is the VNC protocol
	}

	// Dial WebSocket connection
	conn, _, err := dialer.DialContext(ctx, wsURL.String(), headers)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket VNC at %s: %w", wsURL.String(), err)
	}

	t.conn = conn
	return nil
}

// ConnectSimple is a helper for connecting without credentials
func (t *WebSocketTransport) ConnectSimple(ctx context.Context, endpoint string) error {
	return t.Connect(ctx, endpoint, "", "")
}

// Read reads VNC data from the WebSocket connection
func (t *WebSocketTransport) Read(ctx context.Context) ([]byte, error) {
	if t.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// For streaming connections, only set deadline if context has one
	// This allows VNC to handle idle periods (no framebuffer updates)
	if deadline, ok := ctx.Deadline(); ok {
		t.conn.SetReadDeadline(deadline)
	} else {
		// Clear any existing deadline for long-lived streaming connections
		t.conn.SetReadDeadline(time.Time{})
	}

	// Read message from WebSocket
	messageType, data, err := t.conn.ReadMessage()
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			return nil, fmt.Errorf("WebSocket VNC connection closed: %w", err)
		}
		return nil, fmt.Errorf("WebSocket VNC read error: %w", err)
	}

	// Only handle binary messages for VNC/RFB protocol
	if messageType != websocket.BinaryMessage {
		// Ignore text/control messages, read again
		return t.Read(ctx)
	}

	return data, nil
}

// Write writes VNC data to the WebSocket connection
func (t *WebSocketTransport) Write(ctx context.Context, data []byte) error {
	if t.conn == nil {
		return fmt.Errorf("not connected")
	}

	// For streaming connections, only set deadline if context has one
	if deadline, ok := ctx.Deadline(); ok {
		t.conn.SetWriteDeadline(deadline)
	} else {
		// Clear any existing deadline for long-lived streaming connections
		t.conn.SetWriteDeadline(time.Time{})
	}

	// Write binary message to WebSocket (VNC/RFB uses binary)
	err := t.conn.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return fmt.Errorf("WebSocket VNC write error: %w", err)
	}

	return nil
}

// Close closes the WebSocket VNC connection
func (t *WebSocketTransport) Close() error {
	if t.conn == nil {
		return nil
	}

	// Send close message
	err := t.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		// Ignore close errors, just close the connection
	}

	closeErr := t.conn.Close()
	t.conn = nil
	return closeErr
}

// IsConnected returns true if the transport is connected
func (t *WebSocketTransport) IsConnected() bool {
	return t.conn != nil
}

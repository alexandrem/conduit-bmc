package vnc

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"local-agent/pkg/vnc/rfb"
)

// WebSocketTransport implements VNC transport using WebSocket connection.
// Connects to VNC via WebSocket, carrying the standard RFB protocol. Used by:
//   - OpenBMC graphical console (verified)
//   - Redfish GraphicalConsole (theoretical, session auth may be required)
//   - Enterprise BMCs (Dell, Supermicro, Lenovo), which often require
//     vendor-specific session or cookie handling.
type WebSocketTransport struct {
	conn           *websocket.Conn
	timeout        time.Duration
	serverInitData []byte // Cached ServerInit message for RFB proxy mode
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

// Authenticate performs RFB handshake and authentication over WebSocket
//
// This method should be called after Connect() and before starting data proxying.
// It performs the same RFB handshake as NativeTransport, but over WebSocket frames.
//
// WebSocket framing:
// - RFB protocol data is carried in binary WebSocket messages (opcode 0x2)
// - Each Read/Write operation maps to a WebSocket message
// - The RFB protocol itself is identical to native TCP VNC
//
// Authentication flow:
// 1. RFB protocol version negotiation (3.3, 3.7, 3.8)
// 2. Security type negotiation
// 3. VNC authentication (if password provided and required)
// 4. Security result verification
func (t *WebSocketTransport) Authenticate(ctx context.Context, password string) error {
	if t.conn == nil {
		return fmt.Errorf("not connected - call Connect() first")
	}

	log.Debug().
		Str("transport", "websocket").
		Str("remote_addr", t.conn.RemoteAddr().String()).
		Msg("Starting VNC authentication handshake over WebSocket")

	// Create a wrapper that adapts WebSocket Read/Write to io.ReadWriter
	// RFB handshake expects io.ReadWriter, but we have WebSocket-specific methods
	wsAdapter := &webSocketAdapter{
		transport: t,
		ctx:       ctx,
	}

	// Create handshake handler with WebSocket adapter
	handshake := rfb.NewHandshake(wsAdapter)

	// Step 1: Negotiate protocol version
	version, err := handshake.NegotiateVersion()
	if err != nil {
		log.Error().
			Err(err).
			Str("transport", "websocket").
			Msg("VNC version negotiation failed")
		return fmt.Errorf("version negotiation failed: %w", err)
	}

	log.Debug().
		Str("transport", "websocket").
		Str("rfb_version", version.String()).
		Msg("VNC protocol version negotiated")

	// Step 2: Negotiate security type
	// Prefer VNC auth if password is provided
	preferVNCAuth := password != ""
	securityType, err := handshake.NegotiateSecurityType(preferVNCAuth)
	if err != nil {
		log.Error().
			Err(err).
			Str("transport", "websocket").
			Str("rfb_version", version.String()).
			Msg("VNC security type negotiation failed")
		return fmt.Errorf("security type negotiation failed: %w", err)
	}

	log.Info().
		Str("transport", "websocket").
		Str("security_type", securityType.String()).
		Str("rfb_version", version.String()).
		Msg("VNC security type negotiated")

	// Step 3: Perform authentication based on negotiated security type
	switch securityType {
	case rfb.SecurityTypeNone:
		log.Debug().
			Str("transport", "websocket").
			Msg("VNC authentication not required (SecurityTypeNone)")

		// No authentication required
		// Still need to read security result for RFB 3.8+
		if version.Minor >= 8 {
			if err := handshake.ReadSecurityResult(); err != nil {
				log.Error().
					Err(err).
					Str("transport", "websocket").
					Msg("VNC security result check failed")
				return fmt.Errorf("security result check failed: %w", err)
			}
		}

		log.Info().
			Str("transport", "websocket").
			Str("security_type", "None").
			Msg("VNC authentication completed successfully (no auth required)")

	case rfb.SecurityTypeVNCAuth:
		// VNC authentication required
		if password == "" {
			log.Error().
				Str("transport", "websocket").
				Msg("VNC authentication required but no password provided")
			return fmt.Errorf("VNC authentication required but no password provided")
		}

		log.Debug().
			Str("transport", "websocket").
			Msg("Performing VNC challenge-response authentication over WebSocket")

		// Perform VNC challenge-response authentication
		authenticator := rfb.NewAuthenticator(wsAdapter)
		if err := authenticator.PerformVNCAuth(password); err != nil {
			log.Error().
				Err(err).
				Str("transport", "websocket").
				Msg("VNC challenge-response authentication failed")
			return fmt.Errorf("VNC authentication failed: %w", err)
		}

		// Read security result
		if err := handshake.ReadSecurityResult(); err != nil {
			log.Error().
				Err(err).
				Str("transport", "websocket").
				Msg("VNC authentication failed (server rejected credentials)")
			return fmt.Errorf("authentication failed: %w", err)
		}

		log.Info().
			Str("transport", "websocket").
			Str("security_type", "VNC Authentication").
			Msg("VNC authentication completed successfully")

	default:
		log.Error().
			Str("transport", "websocket").
			Str("security_type", securityType.String()).
			Msg("Unsupported VNC security type")
		return fmt.Errorf("unsupported security type: %s", securityType)
	}

	// Step 4: Send ClientInit and cache ServerInit for RFB proxy mode
	// See native_transport.go for detailed explanation

	log.Debug().
		Str("transport", "websocket").
		Msg("Sending ClientInit to BMC and caching ServerInit for proxy mode")

	if err := handshake.SendClientInit(true); err != nil {
		log.Error().
			Err(err).
			Str("transport", "websocket").
			Msg("Failed to send ClientInit")
		return fmt.Errorf("ClientInit failed: %w", err)
	}

	// Read and cache ServerInit for later replay to browser
	// Use a protocol reader directly on the WebSocket adapter
	reader := rfb.NewProtocolReader(wsAdapter)

	serverInitHeader, err := reader.ReadBytes(20)
	if err != nil {
		log.Error().
			Err(err).
			Str("transport", "websocket").
			Msg("Failed to read ServerInit header")
		return fmt.Errorf("failed to read ServerInit header: %w", err)
	}

	nameLength, err := reader.ReadU32()
	if err != nil {
		log.Error().
			Err(err).
			Str("transport", "websocket").
			Msg("Failed to read ServerInit name length")
		return fmt.Errorf("failed to read ServerInit name length: %w", err)
	}

	nameBytes := []byte{}
	if nameLength > 0 {
		nameBytes, err = reader.ReadBytes(int(nameLength))
		if err != nil {
			log.Error().
				Err(err).
				Str("transport", "websocket").
				Int("name_length", int(nameLength)).
				Msg("Failed to read ServerInit name")
			return fmt.Errorf("failed to read ServerInit name: %w", err)
		}
	}

	// Cache the complete ServerInit message
	t.serverInitData = make([]byte, 0, 20+4+len(nameBytes))
	t.serverInitData = append(t.serverInitData, serverInitHeader...)

	nameLengthBytes := []byte{
		byte(nameLength >> 24),
		byte(nameLength >> 16),
		byte(nameLength >> 8),
		byte(nameLength),
	}
	t.serverInitData = append(t.serverInitData, nameLengthBytes...)
	t.serverInitData = append(t.serverInitData, nameBytes...)

	log.Debug().
		Str("transport", "websocket").
		Int("server_init_size", len(t.serverInitData)).
		Str("desktop_name", string(nameBytes)).
		Msg("VNC authentication completed and ServerInit cached - ready for RFB proxy mode")

	return nil
}

// GetServerInit returns the cached ServerInit message
func (t *WebSocketTransport) GetServerInit() []byte {
	return t.serverInitData
}

// webSocketAdapter adapts WebSocketTransport to io.ReadWriter for RFB handshake
type webSocketAdapter struct {
	transport *WebSocketTransport
	ctx       context.Context
	readBuf   []byte // Buffer for partial reads
	readPos   int    // Current position in read buffer
}

// Read implements io.Reader for WebSocket transport
// Buffers WebSocket messages and returns requested bytes
func (w *webSocketAdapter) Read(p []byte) (int, error) {
	// If we have buffered data, return it first
	if w.readPos < len(w.readBuf) {
		n := copy(p, w.readBuf[w.readPos:])
		w.readPos += n

		// Clear buffer if fully consumed
		if w.readPos >= len(w.readBuf) {
			w.readBuf = nil
			w.readPos = 0
		}

		return n, nil
	}

	// Need to read new data from WebSocket
	data, err := w.transport.Read(w.ctx)
	if err != nil {
		return 0, err
	}

	// Copy what we can to the output buffer
	n := copy(p, data)

	// If there's leftover data, buffer it
	if n < len(data) {
		w.readBuf = data
		w.readPos = n
	}

	return n, nil
}

// Write implements io.Writer for WebSocket transport
func (w *webSocketAdapter) Write(p []byte) (int, error) {
	if err := w.transport.Write(w.ctx, p); err != nil {
		return 0, err
	}
	return len(p), nil
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

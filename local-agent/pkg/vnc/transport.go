package vnc

import (
	"context"
	"fmt"
	"strings"
)

// Transport defines the interface for VNC transport implementations.
//
// Both native TCP and WebSocket transports implement this interface.
// Both transports carry the RFB protocol (versions 3.3, 3.7, 3.8).
// WebSocket framing uses opcode 0x2; TCP uses a raw stream.
//
// For specification details, see RFC 6143 (RFB) and
// draft-realvnc-websocket-02 (WebSocket RFB transport).
type Transport interface {
	// Read reads VNC protocol data from the connection
	Read(ctx context.Context) ([]byte, error)

	// Write writes VNC protocol data to the connection
	Write(ctx context.Context, data []byte) error

	// Close closes the VNC connection
	Close() error

	// IsConnected returns true if the transport is currently connected
	IsConnected() bool
}

// EndpointType represents the type of VNC endpoint
type EndpointType int

const (
	// TypeUnknown - Unknown or unspecified transport type
	TypeUnknown EndpointType = iota

	// TypeNative - Native VNC TCP connection (port 5900)
	// Used by: QEMU, VirtualBMC, some standalone KVM devices
	TypeNative

	// TypeWebSocket - WebSocket-based VNC/RFB connection
	// Used by: Redfish GraphicalConsole, OpenBMC KVM, Dell iDRAC, HPE iLO, Supermicro
	TypeWebSocket
)

// String returns the string representation of EndpointType
func (t EndpointType) String() string {
	switch t {
	case TypeNative:
		return "native"
	case TypeWebSocket:
		return "websocket"
	default:
		return "unknown"
	}
}

// ParseEndpointType parses a string into an EndpointType
func ParseEndpointType(s string) EndpointType {
	switch s {
	case "native":
		return TypeNative
	case "websocket":
		return TypeWebSocket
	default:
		return TypeUnknown
	}
}

// Endpoint represents a VNC connection endpoint configuration
type Endpoint struct {
	Endpoint string // URL with scheme (ws://, wss://, vnc://) or host:port (defaults to native TCP)
	Username string
	Password string
	TLS      *TLSConfig // Optional TLS configuration for encrypted VNC connections
}

// TLSConfig represents TLS/SSL configuration for VNC connections
// Used for VeNCrypt, RFB-over-TLS, and enterprise BMC VNC (Dell iDRAC, HPE iLO)
type TLSConfig struct {
	Enabled            bool // Enable TLS wrapping of VNC connection
	InsecureSkipVerify bool // Skip certificate verification (for self-signed certs)
}

// NewTransport creates the appropriate VNC transport based on endpoint URL scheme
// Auto-detects transport type from endpoint:
//   - ws://... or wss://... → WebSocket transport
//   - vnc://host:port or host:port → Native TCP transport
func NewTransport(endpoint *Endpoint) (Transport, error) {
	if endpoint == nil {
		return nil, fmt.Errorf("VNC endpoint configuration is nil")
	}

	if endpoint.Endpoint == "" {
		return nil, fmt.Errorf("VNC endpoint is empty")
	}

	// Detect transport type from URL scheme
	transportType := detectTransportType(endpoint.Endpoint)

	switch transportType {
	case TypeNative:
		// Native TCP VNC connection
		return NewNativeTransport(0), nil

	case TypeWebSocket:
		// WebSocket-based VNC connection
		return NewWebSocketTransport(0), nil

	default:
		return nil, fmt.Errorf("unable to detect transport type from endpoint: %s", endpoint.Endpoint)
	}
}

// detectTransportType determines the transport type from the endpoint URL scheme
func detectTransportType(endpoint string) EndpointType {
	// WebSocket schemes
	if strings.HasPrefix(endpoint, "ws://") || strings.HasPrefix(endpoint, "wss://") {
		return TypeWebSocket
	}

	// Native VNC schemes or host:port
	if strings.HasPrefix(endpoint, "vnc://") || !strings.Contains(endpoint, "://") {
		return TypeNative
	}

	// Unknown scheme
	return TypeUnknown
}

// ConnectTransport connects the transport to the VNC endpoint and performs authentication
//
// This is a convenience function that:
// 1. Connects to the VNC server
// 2. Performs RFB handshake and authentication (if password provided)
//
// After this function returns successfully, the transport is ready for data proxying.
func ConnectTransport(ctx context.Context, transport Transport, endpoint *Endpoint) error {
	if endpoint == nil {
		return fmt.Errorf("VNC endpoint configuration is nil")
	}

	// Determine connection method based on transport type
	switch t := transport.(type) {
	case *NativeTransport:
		// Parse host:port from endpoint
		host, port, err := parseEndpoint(endpoint.Endpoint)
		if err != nil {
			return fmt.Errorf("invalid native VNC endpoint %s: %w", endpoint.Endpoint, err)
		}

		// Connect to VNC server with optional TLS
		if err := t.ConnectWithTLS(ctx, host, port, endpoint.TLS); err != nil {
			return err
		}

		// Perform RFB handshake and authentication
		if err := t.Authenticate(ctx, endpoint.Password); err != nil {
			t.Close() // Clean up connection on auth failure
			return err
		}

		return nil

	case *WebSocketTransport:
		// Connect using WebSocket URL (may include HTTP Basic Auth credentials)
		if err := t.Connect(ctx, endpoint.Endpoint, endpoint.Username, endpoint.Password); err != nil {
			return err
		}

		// Perform RFB handshake and authentication over WebSocket
		// Note: endpoint.Password is used for VNC password, not HTTP auth
		if err := t.Authenticate(ctx, endpoint.Password); err != nil {
			t.Close() // Clean up connection on auth failure
			return err
		}

		return nil

	default:
		return fmt.Errorf("unknown transport type: %T", transport)
	}
}

// parseEndpoint parses a VNC endpoint string to extract host and port
// Supports formats: "host:port", "vnc://host:port", "host" (defaults to port 5900)
func parseEndpoint(endpoint string) (string, int, error) {
	// If it looks like a WebSocket URL, it's probably misconfigured
	if strings.HasPrefix(endpoint, "ws://") || strings.HasPrefix(endpoint, "wss://") {
		return "", 0, fmt.Errorf("WebSocket URL provided for native VNC transport - use websocket transport type instead")
	}

	// Try parsing as URL first (vnc://host:port or http://host:port)
	if strings.Contains(endpoint, "://") {
		return parseEndpointURL(endpoint)
	}

	// Try parsing as host:port
	if strings.Contains(endpoint, ":") {
		return parseHostPort(endpoint)
	}

	// Just a hostname/IP - use default VNC port
	return endpoint, 5900, nil
}

// parseEndpointURL parses a URL-formatted VNC endpoint
func parseEndpointURL(endpoint string) (string, int, error) {
	u, err := parseURL(endpoint)
	if err != nil {
		return "", 0, fmt.Errorf("invalid URL format: %w", err)
	}
	host := u.Hostname()
	portStr := u.Port()
	if portStr == "" {
		return host, 5900, nil // Default VNC port
	}
	port, err := parseInt(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %s: %w", portStr, err)
	}
	return host, port, nil
}

// parseHostPort parses a host:port formatted endpoint
func parseHostPort(endpoint string) (string, int, error) {
	parts := strings.Split(endpoint, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid host:port format")
	}
	host := parts[0]
	port, err := parseInt(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %s: %w", parts[1], err)
	}
	return host, port, nil
}

// Helper functions to avoid import conflicts
func parseURL(s string) (*simpleURL, error) {
	u := &simpleURL{}
	// Simple URL parsing - extract scheme, host, port
	if idx := strings.Index(s, "://"); idx > 0 {
		u.scheme = s[:idx]
		rest := s[idx+3:]

		// Extract path if present
		if pathIdx := strings.Index(rest, "/"); pathIdx > 0 {
			u.host = rest[:pathIdx]
			u.path = rest[pathIdx:]
		} else {
			u.host = rest
		}

		return u, nil
	}
	return nil, fmt.Errorf("invalid URL: missing scheme")
}

func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
		result = result*10 + int(c-'0')
	}
	return result, nil
}

type simpleURL struct {
	scheme string
	host   string
	path   string
}

func (u *simpleURL) Hostname() string {
	if idx := strings.Index(u.host, ":"); idx > 0 {
		return u.host[:idx]
	}
	return u.host
}

func (u *simpleURL) Port() string {
	if idx := strings.Index(u.host, ":"); idx > 0 {
		return u.host[idx+1:]
	}
	return ""
}

package vnc

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/rs/zerolog/log"

	"local-agent/pkg/vnc/rfb"
)

const (
	keepaliveTime = 30 * time.Second
	lingerSec     = 5
)

// NativeTransport implements VNC transport using native TCP connection to VNC port.
// Direct TCP connection to the VNC port (typically 5900). Commonly used with:
//   - QEMU VNC servers
//   - VirtualBMC (test environments)
//   - BMCs that expose native VNC on port 5900
type NativeTransport struct {
	conn           net.Conn
	timeout        time.Duration
	serverInitData []byte // Cached ServerInit message for RFB proxy mode
}

// NewNativeTransport creates a new native VNC transport
func NewNativeTransport(timeout time.Duration) *NativeTransport {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &NativeTransport{
		timeout: timeout,
	}
}

// Connect establishes a TCP connection to the VNC server
func (t *NativeTransport) Connect(ctx context.Context, host string, port int) error {
	return t.ConnectWithTLS(ctx, host, port, nil)
}

// ConnectWithTLS establishes a TCP connection to the VNC server with optional TLS encryption
// If tlsConfig is provided and Enabled=true, performs TLS handshake after TCP connection
// This supports VeNCrypt, RFB-over-TLS, and enterprise BMCs (Dell iDRAC, HPE iLO)
func (t *NativeTransport) ConnectWithTLS(ctx context.Context, host string, port int, tlsConfig *TLSConfig) error {
	if port == 0 {
		port = 5900 // Default VNC port
	}

	address := fmt.Sprintf("%s:%d", host, port)

	log.Debug().
		Str("host", host).
		Int("port", port).
		Bool("tls_config_present", tlsConfig != nil).
		Bool("tls_enabled", tlsConfig != nil && tlsConfig.Enabled).
		Msg("Connecting to VNC server")

	dialer := &net.Dialer{
		Timeout:   t.timeout,
		KeepAlive: keepaliveTime, // Enable TCP keepaliveTime for long-lived VNC connections
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to VNC server at %s: %w", address, err)
	}

	// Enable TCP keepaliveTime explicitly (dialer sets it, but be explicit)
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(keepaliveTime)

		// Optionally enable SO_LINGER to ensure graceful shutdown
		// Linger for 5 seconds to allow data to flush on close
		tcpConn.SetLinger(lingerSec)
	}

	// If TLS is enabled, wrap the connection with TLS
	if tlsConfig != nil && tlsConfig.Enabled {
		log.Debug().
			Str("host", host).
			Int("port", port).
			Bool("insecure_skip_verify", tlsConfig.InsecureSkipVerify).
			Msg("Performing TLS handshake for VNC connection")

		tlsConn := tls.Client(conn, &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
			MinVersion:         tls.VersionTLS12,
			MaxVersion:         tls.VersionTLS13,
		})

		// Perform TLS handshake
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return fmt.Errorf("TLS handshake failed for VNC at %s: %w", address, err)
		}

		log.Info().
			Str("host", host).
			Int("port", port).
			Str("protocol", tlsConn.ConnectionState().NegotiatedProtocol).
			Str("cipher_suite", tls.CipherSuiteName(tlsConn.ConnectionState().CipherSuite)).
			Msg("TLS handshake successful for VNC connection")

		// Dell iDRAC sends RFB version after TLS, but we should read immediately
		// to avoid the server closing an idle connection
		log.Debug().Msg("TLS connection established, proceeding to RFB handshake immediately")

		conn = tlsConn
	}

	t.conn = conn
	return nil
}

// bufferedConn wraps a connection with a buffer for pre-read data
type bufferedConn struct {
	net.Conn
	buffer []byte
	pos    int
}

func (b *bufferedConn) Read(p []byte) (int, error) {
	// First, return buffered data if available
	if b.pos < len(b.buffer) {
		n := copy(p, b.buffer[b.pos:])
		b.pos += n
		return n, nil
	}
	// Then read from underlying connection
	return b.Conn.Read(p)
}

// Authenticate performs RFB handshake and authentication
//
// This method should be called after Connect() and before starting data proxying.
// It performs:
// 1. RFB protocol version negotiation (3.3, 3.7, 3.8)
// 2. Security type negotiation
// 3. VNC authentication (if password provided and required)
// 4. Security result verification
//
// If password is empty and server requires no authentication, this succeeds.
// If password is provided, VNC Authentication is preferred.
func (t *NativeTransport) Authenticate(ctx context.Context, password string) error {
	if t.conn == nil {
		return fmt.Errorf("not connected - call Connect() first")
	}

	log.Debug().
		Str("transport", "native-tcp").
		Str("remote_addr", t.conn.RemoteAddr().String()).
		Bool("has_password", password != "").
		Msg("Starting VNC authentication handshake")

	// Set a very long read deadline to handle BMCs (like Dell iDRAC) that have
	// a significant delay after TLS handshake before sending RFB version string
	// Dell iDRAC can take up to 5+ seconds in some cases
	t.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	defer t.conn.SetReadDeadline(time.Time{}) // Clear deadline after handshake

	// Create handshake handler
	handshake := rfb.NewHandshake(t.conn)

	// Step 1: Negotiate protocol version
	version, err := handshake.NegotiateVersion()
	if err != nil {
		log.Error().
			Err(err).
			Str("transport", "native-tcp").
			Msg("VNC version negotiation failed")
		return fmt.Errorf("version negotiation failed: %w", err)
	}

	log.Debug().
		Str("transport", "native-tcp").
		Str("rfb_version", version.String()).
		Msg("VNC protocol version negotiated")

	// Step 2: Negotiate security type
	// Prefer VNC auth if password is provided
	preferVNCAuth := password != ""
	securityType, err := handshake.NegotiateSecurityType(preferVNCAuth)
	if err != nil {
		log.Error().
			Err(err).
			Str("transport", "native-tcp").
			Str("rfb_version", version.String()).
			Msg("VNC security type negotiation failed")
		return fmt.Errorf("security type negotiation failed: %w", err)
	}

	log.Info().
		Str("transport", "native-tcp").
		Str("security_type", securityType.String()).
		Str("rfb_version", version.String()).
		Msg("VNC security type negotiated")

	// Step 3: Perform authentication based on negotiated security type
	switch securityType {
	case rfb.SecurityTypeNone:
		log.Debug().
			Str("transport", "native-tcp").
			Msg("VNC authentication not required (SecurityTypeNone)")

		// No authentication required
		// Still need to read security result for RFB 3.8+
		if version.Minor >= 8 {
			if err := handshake.ReadSecurityResult(); err != nil {
				log.Error().
					Err(err).
					Str("transport", "native-tcp").
					Msg("VNC security result check failed")
				return fmt.Errorf("security result check failed: %w", err)
			}
		}

		log.Info().
			Str("transport", "native-tcp").
			Str("security_type", "None").
			Msg("VNC authentication completed successfully (no auth required)")

	case rfb.SecurityTypeVNCAuth:
		// VNC authentication required
		if password == "" {
			log.Error().
				Str("transport", "native-tcp").
				Msg("VNC authentication required but no password provided")
			return fmt.Errorf("VNC authentication required but no password provided")
		}

		log.Debug().
			Str("transport", "native-tcp").
			Msg("Performing VNC challenge-response authentication")

		// Perform VNC challenge-response authentication
		authenticator := rfb.NewAuthenticator(t.conn)
		if err := authenticator.PerformVNCAuth(password); err != nil {
			log.Error().
				Err(err).
				Str("transport", "native-tcp").
				Msg("VNC challenge-response authentication failed")
			return fmt.Errorf("VNC authentication failed: %w", err)
		}

		// Read security result
		if err := handshake.ReadSecurityResult(); err != nil {
			log.Error().
				Err(err).
				Str("transport", "native-tcp").
				Msg("VNC authentication failed (server rejected credentials)")
			return fmt.Errorf("authentication failed: %w", err)
		}

		log.Info().
			Str("transport", "native-tcp").
			Str("security_type", "VNC Authentication").
			Msg("VNC authentication completed successfully")

	default:
		log.Error().
			Str("transport", "native-tcp").
			Str("security_type", securityType.String()).
			Msg("Unsupported VNC security type")
		return fmt.Errorf("unsupported security type: %s", securityType)
	}

	// Step 4: Send ClientInit and cache ServerInit for RFB proxy mode
	//
	// For VNC proxying (browser client), we need to:
	// 1. Send ClientInit NOW to keep the BMC connection alive
	// 2. Read and cache the ServerInit response
	// 3. Later, replay ServerInit to the browser when it sends its ClientInit
	//
	// If we don't send ClientInit, many VNC servers will timeout and close the connection.

	log.Debug().
		Str("transport", "native-tcp").
		Msg("Sending ClientInit to BMC and caching ServerInit for proxy mode")

	if err := handshake.SendClientInit(true); err != nil {
		log.Error().
			Err(err).
			Str("transport", "native-tcp").
			Msg("Failed to send ClientInit")
		return fmt.Errorf("ClientInit failed: %w", err)
	}

	// Read and cache ServerInit for later replay to browser
	// ServerInit format: width(2) + height(2) + pixel_format(16) + name_length(4) + name(N)
	// Use a protocol reader directly on the connection
	reader := rfb.NewProtocolReader(t.conn)

	serverInitHeader, err := reader.ReadBytes(20) // width + height + pixel_format
	if err != nil {
		log.Error().
			Err(err).
			Str("transport", "native-tcp").
			Msg("Failed to read ServerInit header")
		return fmt.Errorf("failed to read ServerInit header: %w", err)
	}

	nameLength, err := reader.ReadU32()
	if err != nil {
		log.Error().
			Err(err).
			Str("transport", "native-tcp").
			Msg("Failed to read ServerInit name length")
		return fmt.Errorf("failed to read ServerInit name length: %w", err)
	}

	nameBytes := []byte{}
	if nameLength > 0 {
		nameBytes, err = reader.ReadBytes(int(nameLength))
		if err != nil {
			log.Error().
				Err(err).
				Str("transport", "native-tcp").
				Int("name_length", int(nameLength)).
				Msg("Failed to read ServerInit name")
			return fmt.Errorf("failed to read ServerInit name: %w", err)
		}
	}

	// Cache the complete ServerInit message (we'll replay it to the browser)
	t.serverInitData = make([]byte, 0, 20+4+len(nameBytes))
	t.serverInitData = append(t.serverInitData, serverInitHeader...)

	// Append name length as big-endian u32
	nameLengthBytes := []byte{
		byte(nameLength >> 24),
		byte(nameLength >> 16),
		byte(nameLength >> 8),
		byte(nameLength),
	}
	t.serverInitData = append(t.serverInitData, nameLengthBytes...)
	t.serverInitData = append(t.serverInitData, nameBytes...)

	log.Debug().
		Str("transport", "native-tcp").
		Int("server_init_size", len(t.serverInitData)).
		Str("desktop_name", string(nameBytes)).
		Msg("VNC authentication completed and ServerInit cached - ready for RFB proxy mode")

	return nil
}

// GetServerInit returns the cached ServerInit message
// This is used by the RFB proxy to replay ServerInit to the browser client
func (t *NativeTransport) GetServerInit() []byte {
	return t.serverInitData
}

// Read reads data from the VNC connection
func (t *NativeTransport) Read(ctx context.Context) ([]byte, error) {
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

	buf := make([]byte, 8192) // VNC typically uses larger buffers for framebuffer updates
	n, err := t.conn.Read(buf)
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("VNC connection closed: %w", err)
		}
		// Check for timeout
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, fmt.Errorf("VNC read timeout: %w", err)
		}
		return nil, fmt.Errorf("VNC read error: %w", err)
	}

	return buf[:n], nil
}

// Write writes data to the VNC connection
func (t *NativeTransport) Write(ctx context.Context, data []byte) error {
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

	_, err := t.conn.Write(data)
	if err != nil {
		return fmt.Errorf("VNC write error: %w", err)
	}

	return nil
}

// Close closes the VNC connection
func (t *NativeTransport) Close() error {
	if t.conn == nil {
		return nil
	}

	err := t.conn.Close()
	t.conn = nil
	return err
}

// IsConnected returns true if the transport is connected
func (t *NativeTransport) IsConnected() bool {
	return t.conn != nil
}

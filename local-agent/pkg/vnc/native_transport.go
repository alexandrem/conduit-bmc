package vnc

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"
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
	conn    net.Conn
	timeout time.Duration
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
	if port == 0 {
		port = 5900 // Default VNC port
	}

	address := fmt.Sprintf("%s:%d", host, port)

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

	t.conn = conn
	return nil
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

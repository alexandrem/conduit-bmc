package vnc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"local-agent/pkg/vnc/rfb"
)

// mockVNCServer simulates a VNC server for testing
type mockVNCServer struct {
	listener      net.Listener
	version       string
	securityTypes []rfb.SecurityType
	password      string
	challenge     []byte
	authResult    uint32 // 0 = success, 1 = failure
	failReason    string
	acceptConn    bool
}

// newMockVNCServer creates a new mock VNC server
func newMockVNCServer() *mockVNCServer {
	return &mockVNCServer{
		version:       rfb.ProtocolVersion38,
		securityTypes: []rfb.SecurityType{rfb.SecurityTypeVNCAuth},
		password:      "testpass",
		challenge:     make([]byte, rfb.VNCAuthChallengeLength),
		authResult:    0, // Success by default
		acceptConn:    true,
	}
}

// start starts the mock VNC server
func (m *mockVNCServer) start(t *testing.T) string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start mock VNC server: %v", err)
	}

	m.listener = listener

	// Fill challenge with test data
	for i := range m.challenge {
		m.challenge[i] = byte(i)
	}

	// Start accepting connections
	go m.acceptConnections(t)

	return listener.Addr().String()
}

// acceptConnections accepts and handles client connections
func (m *mockVNCServer) acceptConnections(t *testing.T) {
	for m.acceptConn {
		conn, err := m.listener.Accept()
		if err != nil {
			if m.acceptConn {
				t.Logf("Accept error: %v", err)
			}
			return
		}

		go m.handleConnection(t, conn)
	}
}

// handleConnection handles a single client connection
func (m *mockVNCServer) handleConnection(t *testing.T, conn net.Conn) {
	defer conn.Close()

	// Step 1: Send protocol version
	if _, err := conn.Write([]byte(m.version)); err != nil {
		t.Logf("Failed to send version: %v", err)
		return
	}

	// Step 2: Read client version
	versionBuf := make([]byte, rfb.ProtocolVersionLength)
	if _, err := io.ReadFull(conn, versionBuf); err != nil {
		t.Logf("Failed to read client version: %v", err)
		return
	}

	clientVersion, err := rfb.ParseProtocolVersion(versionBuf)
	if err != nil {
		t.Logf("Invalid client version: %v", err)
		return
	}

	// Step 3: Send security types (RFB 3.8 format)
	if clientVersion.Minor >= 7 {
		// Send count + list of security types
		secTypes := make([]byte, 1+len(m.securityTypes))
		secTypes[0] = byte(len(m.securityTypes))
		for i, st := range m.securityTypes {
			secTypes[1+i] = byte(st)
		}
		if _, err := conn.Write(secTypes); err != nil {
			t.Logf("Failed to send security types: %v", err)
			return
		}

		// Read client's selected security type
		selectedType := make([]byte, 1)
		if _, err := io.ReadFull(conn, selectedType); err != nil {
			t.Logf("Failed to read selected security type: %v", err)
			return
		}
	} else {
		// RFB 3.3: Server chooses security type
		secType := make([]byte, 4)
		secType[3] = byte(m.securityTypes[0])
		if _, err := conn.Write(secType); err != nil {
			t.Logf("Failed to send security type: %v", err)
			return
		}
	}

	// Step 4: Perform VNC authentication
	if len(m.securityTypes) > 0 && m.securityTypes[0] == rfb.SecurityTypeVNCAuth {
		// Send challenge
		if _, err := conn.Write(m.challenge); err != nil {
			t.Logf("Failed to send challenge: %v", err)
			return
		}

		// Read response
		response := make([]byte, rfb.VNCAuthChallengeLength)
		if _, err := io.ReadFull(conn, response); err != nil {
			t.Logf("Failed to read auth response: %v", err)
			return
		}

		// Verify response by decrypting it and comparing with challenge
		decrypted, err := rfb.DecryptVNCResponse(response, m.password)
		if err != nil {
			t.Logf("Failed to decrypt response: %v", err)
			m.authResult = 1
		} else if !bytes.Equal(decrypted, m.challenge) {
			t.Logf("Authentication failed: wrong password")
			m.authResult = 1
		}
	}

	// Step 5: Send security result
	result := make([]byte, 4)
	result[3] = byte(m.authResult)
	if _, err := conn.Write(result); err != nil {
		t.Logf("Failed to send security result: %v", err)
		return
	}

	// If auth failed and RFB 3.8, send failure reason
	if m.authResult != 0 && clientVersion.Minor >= 8 && m.failReason != "" {
		// Send reason length + string
		reasonLen := make([]byte, 4)
		reasonLen[3] = byte(len(m.failReason))
		conn.Write(reasonLen)
		conn.Write([]byte(m.failReason))
		return // Close connection after auth failure
	}

	// If auth failed, close connection
	if m.authResult != 0 {
		return
	}

	// Step 6: Read ClientInit
	clientInitBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, clientInitBuf); err != nil {
		t.Logf("Failed to read ClientInit: %v", err)
		return
	}

	// Step 7: Send ServerInit
	// ServerInit format: width(2) + height(2) + pixel_format(16) + name_length(4) + name(N)
	serverInit := make([]byte, 0, 256)

	// Framebuffer width (640) - big endian u16
	serverInit = append(serverInit, 0x02, 0x80)
	// Framebuffer height (480) - big endian u16
	serverInit = append(serverInit, 0x01, 0xE0)

	// Pixel format (16 bytes) - standard RGB888
	pixelFormat := []byte{
		32,     // bits-per-pixel
		24,     // depth
		0,      // big-endian-flag
		1,      // true-color-flag
		0, 255, // red-max (255)
		0, 255, // green-max (255)
		0, 255, // blue-max (255)
		16,      // red-shift
		8,       // green-shift
		0,       // blue-shift
		0, 0, 0, // padding
	}
	serverInit = append(serverInit, pixelFormat...)

	// Desktop name
	desktopName := "Test VNC Server"
	nameLen := make([]byte, 4)
	nameLen[3] = byte(len(desktopName))
	serverInit = append(serverInit, nameLen...)
	serverInit = append(serverInit, []byte(desktopName)...)

	if _, err := conn.Write(serverInit); err != nil {
		t.Logf("Failed to send ServerInit: %v", err)
		return
	}

	// Keep connection open for a bit
	time.Sleep(100 * time.Millisecond)
}

// stop stops the mock VNC server
func (m *mockVNCServer) stop() {
	m.acceptConn = false
	if m.listener != nil {
		m.listener.Close()
	}
}

// TestNativeTransportAuthentication tests native VNC authentication
func TestNativeTransportAuthentication(t *testing.T) {
	tests := []struct {
		name          string
		password      string
		serverPass    string
		expectSuccess bool
	}{
		{
			name:          "Correct password",
			password:      "testpass",
			serverPass:    "testpass",
			expectSuccess: true,
		},
		{
			name:          "Wrong password",
			password:      "wrongpass",
			serverPass:    "testpass",
			expectSuccess: false,
		},
		{
			name:          "Empty password (server expects password)",
			password:      "",
			serverPass:    "testpass",
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start mock server
			server := newMockVNCServer()
			server.password = tt.serverPass
			addr := server.start(t)
			defer server.stop()

			// Parse address
			host, port := parseServerAddr(t, addr)

			// Create transport
			transport := NewNativeTransport(5 * time.Second)

			// Connect
			ctx := context.Background()
			err := transport.Connect(ctx, host, port)
			if err != nil {
				t.Fatalf("Connect failed: %v", err)
			}
			defer transport.Close()

			// Authenticate
			err = transport.Authenticate(ctx, tt.password)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Authentication failed: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Authentication should have failed but succeeded")
				}
			}
		})
	}
}

// TestConnectTransportWithAuth tests the ConnectTransport convenience function
func TestConnectTransportWithAuth(t *testing.T) {
	// Start mock server
	server := newMockVNCServer()
	server.password = "secret"
	addr := server.start(t)
	defer server.stop()

	// Create endpoint
	endpoint := &Endpoint{
		Endpoint: addr,
		Password: "secret",
	}

	// Create transport
	transport, err := NewTransport(endpoint)
	if err != nil {
		t.Fatalf("NewTransport failed: %v", err)
	}

	// Connect and authenticate in one call
	ctx := context.Background()
	err = ConnectTransport(ctx, transport, endpoint)
	defer transport.Close()

	if err != nil {
		t.Fatalf("ConnectTransport failed: %v", err)
	}

	// Verify transport is connected
	if !transport.IsConnected() {
		t.Error("Transport should be connected after ConnectTransport")
	}
}

// TestNativeTransportNoAuth tests VNC servers without authentication
func TestNativeTransportNoAuth(t *testing.T) {
	// Start mock server with no authentication
	server := newMockVNCServer()
	server.securityTypes = []rfb.SecurityType{rfb.SecurityTypeNone}
	addr := server.start(t)
	defer server.stop()

	host, port := parseServerAddr(t, addr)

	// Create transport
	transport := NewNativeTransport(5 * time.Second)

	// Connect
	ctx := context.Background()
	err := transport.Connect(ctx, host, port)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Authenticate (no password required)
	err = transport.Authenticate(ctx, "")
	if err != nil {
		t.Errorf("Authentication failed: %v", err)
	}
}

// TestNativeTransportAuthenticationConcurrent tests concurrent authentications
func TestNativeTransportAuthenticationConcurrent(t *testing.T) {
	// Start mock server
	server := newMockVNCServer()
	server.password = "testpass"
	addr := server.start(t)
	defer server.stop()

	host, port := parseServerAddr(t, addr)

	// Run multiple concurrent authentications
	const numClients = 5
	var wg sync.WaitGroup
	wg.Add(numClients)

	errors := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			defer wg.Done()

			transport := NewNativeTransport(5 * time.Second)
			ctx := context.Background()

			if err := transport.Connect(ctx, host, port); err != nil {
				errors <- fmt.Errorf("client %d connect failed: %w", clientID, err)
				return
			}
			defer transport.Close()

			if err := transport.Authenticate(ctx, "testpass"); err != nil {
				errors <- fmt.Errorf("client %d auth failed: %w", clientID, err)
				return
			}

			errors <- nil
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			t.Error(err)
		}
	}
}

// TestAuthenticateWithoutConnect tests error when authenticating without connecting
func TestAuthenticateWithoutConnect(t *testing.T) {
	transport := NewNativeTransport(5 * time.Second)
	ctx := context.Background()

	err := transport.Authenticate(ctx, "password")
	if err == nil {
		t.Error("Authenticate should fail when not connected")
	}

	if !contains(err.Error(), "not connected") {
		t.Errorf("Error should mention 'not connected', got: %v", err)
	}
}

// Helper functions

func parseServerAddr(t *testing.T, addr string) (string, int) {
	host, port, err := parseEndpoint(addr)
	if err != nil {
		t.Fatalf("Failed to parse server address %s: %v", addr, err)
	}
	return host, port
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

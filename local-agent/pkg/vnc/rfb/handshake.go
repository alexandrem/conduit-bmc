package rfb

import (
	"fmt"
	"io"
)

// Handshake manages the RFB protocol handshake process
type Handshake struct {
	reader  *ProtocolReader
	writer  *ProtocolWriter
	version *ProtocolVersion
}

// NewHandshake creates a new RFB handshake handler
func NewHandshake(rw io.ReadWriter) *Handshake {
	return &Handshake{
		reader: NewProtocolReader(rw),
		writer: NewProtocolWriter(rw),
	}
}

// NegotiateVersion performs RFB protocol version negotiation
//
// Protocol flow:
// 1. Server sends its protocol version (12 bytes: "RFB xxx.yyy\n")
// 2. Client responds with highest compatible version
//
// This implementation supports RFB 3.3, 3.7, and 3.8
func (h *Handshake) NegotiateVersion() (*ProtocolVersion, error) {
	// Read server's protocol version
	// NOTE: Server should send version string immediately after connection
	// For TLS connections, this happens after TLS handshake completes
	versionBytes, err := h.reader.ReadBytes(ProtocolVersionLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read server version: %w", err)
	}

	serverVersion, err := ParseProtocolVersion(versionBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid server version: %w", err)
	}

	// Check if server version is supported
	if !serverVersion.IsSupported() {
		return nil, fmt.Errorf("unsupported server version: %s (we support 3.3, 3.7, 3.8)", serverVersion)
	}

	// Select client version (match server version for maximum compatibility)
	clientVersion := serverVersion

	// Send client version
	if err := h.writer.WriteString(clientVersion.ToWireFormat()); err != nil {
		return nil, fmt.Errorf("failed to send client version: %w", err)
	}

	h.version = clientVersion
	return clientVersion, nil
}

// NegotiateSecurityType performs RFB security type negotiation
//
// Protocol flow varies by version:
//
// RFB 3.3:
//
//	Server sends security type (4 bytes, u32)
//	If type = 0, connection failed (read failure reason)
//	Client uses server-chosen type (no negotiation)
//
// RFB 3.7/3.8:
//
//	Server sends list of security types (1 byte count + N type bytes)
//	If count = 0, connection failed (read failure reason)
//	Client selects preferred type from list
//	Client sends selected type (1 byte)
//
// Returns the negotiated security type
func (h *Handshake) NegotiateSecurityType(preferVNCAuth bool) (SecurityType, error) {
	if h.version == nil {
		return SecurityTypeInvalid, fmt.Errorf("version negotiation must be completed first")
	}

	// RFB 3.3: Server chooses security type
	if h.version.Minor == 3 {
		return h.negotiateSecurityType33()
	}

	// RFB 3.7/3.8: Client selects from server's list
	return h.negotiateSecurityType37(preferVNCAuth)
}

// negotiateSecurityType33 handles RFB 3.3 security type negotiation
// Server sends single u32 security type (server-chosen, no client selection)
func (h *Handshake) negotiateSecurityType33() (SecurityType, error) {
	// Read server's security type (4 bytes, u32)
	securityType32, err := h.reader.ReadU32()
	if err != nil {
		return SecurityTypeInvalid, fmt.Errorf("failed to read security type: %w", err)
	}

	// Check for connection failure (type = 0)
	if securityType32 == 0 {
		// Read failure reason string (RFB 3.3 format: length + string)
		reason, err := h.reader.ReadString()
		if err != nil {
			return SecurityTypeInvalid, fmt.Errorf("connection failed (no reason provided): %w", err)
		}
		return SecurityTypeInvalid, fmt.Errorf("connection failed: %s", reason)
	}

	securityType := SecurityType(securityType32)

	// Validate security type
	if !securityType.IsSupported() {
		return SecurityTypeInvalid, fmt.Errorf("unsupported security type: %s", securityType)
	}

	return securityType, nil
}

// negotiateSecurityType37 handles RFB 3.7/3.8 security type negotiation
// Server sends list of types, client selects preferred type
func (h *Handshake) negotiateSecurityType37(preferVNCAuth bool) (SecurityType, error) {
	// Read number of security types
	count, err := h.reader.ReadU8()
	if err != nil {
		return SecurityTypeInvalid, fmt.Errorf("failed to read security type count: %w", err)
	}

	// Check for connection failure (count = 0)
	if count == 0 {
		// Read failure reason string (RFB 3.7/3.8 format: u32 length + string)
		reason, err := h.reader.ReadString()
		if err != nil {
			return SecurityTypeInvalid, fmt.Errorf("connection failed (no reason provided): %w", err)
		}
		return SecurityTypeInvalid, fmt.Errorf("connection failed: %s", reason)
	}

	// Read list of security types
	types, err := h.reader.ReadBytes(int(count))
	if err != nil {
		return SecurityTypeInvalid, fmt.Errorf("failed to read security types: %w", err)
	}

	// Select preferred security type
	selectedType := h.selectSecurityType(types, preferVNCAuth)
	if selectedType == SecurityTypeInvalid {
		return SecurityTypeInvalid, fmt.Errorf("no supported security type offered by server (available: %v)", formatSecurityTypes(types))
	}

	// Send selected security type to server
	if err := h.writer.WriteU8(uint8(selectedType)); err != nil {
		return SecurityTypeInvalid, fmt.Errorf("failed to send security type: %w", err)
	}

	return selectedType, nil
}

// selectSecurityType selects the best security type from the server's list
// Priority: VNC Authentication > None (if password not required)
func (h *Handshake) selectSecurityType(types []byte, preferVNCAuth bool) SecurityType {
	hasNone := false
	hasVNCAuth := false

	// Scan available types
	for _, t := range types {
		if SecurityType(t) == SecurityTypeNone {
			hasNone = true
		}
		if SecurityType(t) == SecurityTypeVNCAuth {
			hasVNCAuth = true
		}
	}

	// Prefer VNC Authentication if requested and available
	if preferVNCAuth && hasVNCAuth {
		return SecurityTypeVNCAuth
	}

	// Fall back to None if available and VNC auth not required
	if !preferVNCAuth && hasNone {
		return SecurityTypeNone
	}

	// If VNC auth is available but not required, use it
	if hasVNCAuth {
		return SecurityTypeVNCAuth
	}

	// If None is available, use it
	if hasNone {
		return SecurityTypeNone
	}

	// No supported security type found
	return SecurityTypeInvalid
}

// ReadSecurityResult reads the security result from the server
// Called after authentication completes (or for SecurityTypeNone)
//
// RFB 3.3/3.7:
//
//	Server sends u32 result (0 = OK, 1 = Failed)
//	If failed, server may close connection (no reason string)
//
// RFB 3.8:
//
//	Server sends u32 result (0 = OK, 1 = Failed)
//	If failed, server sends reason string (u32 length + string)
func (h *Handshake) ReadSecurityResult() error {
	if h.version == nil {
		return fmt.Errorf("version negotiation must be completed first")
	}

	// Read security result (4 bytes, u32)
	result, err := h.reader.ReadU32()
	if err != nil {
		return fmt.Errorf("failed to read security result: %w", err)
	}

	// Check for success
	if result == SecurityResultOK {
		return nil
	}

	// Authentication failed
	// RFB 3.8 includes failure reason string
	if h.version.Minor == 8 {
		reason, err := h.reader.ReadString()
		if err != nil {
			return fmt.Errorf("authentication failed (no reason provided): %w", err)
		}
		return fmt.Errorf("authentication failed: %s", reason)
	}

	// RFB 3.3/3.7: No reason string provided
	return fmt.Errorf("authentication failed (server did not provide reason)")
}

// SendClientInit sends the ClientInit message to the server
//
// Protocol flow (after authentication):
// 1. Client sends ClientInit (1 byte):
//   - 0 = Exclusive access (kick other clients)
//   - 1 = Shared access (allow multiple clients)
//
// 2. Server responds with ServerInit (framebuffer parameters)
//
// This must be called after authentication completes, before data proxying.
// The shared parameter controls whether multiple clients can connect simultaneously.
func (h *Handshake) SendClientInit(shared bool) error {
	if h.version == nil {
		return fmt.Errorf("version negotiation must be completed first")
	}

	// Send ClientInit message (1 byte)
	var sharedFlag uint8
	if shared {
		sharedFlag = 1
	}

	if err := h.writer.WriteU8(sharedFlag); err != nil {
		return fmt.Errorf("failed to send ClientInit: %w", err)
	}

	return nil
}

// GetNegotiatedVersion returns the negotiated protocol version
func (h *Handshake) GetNegotiatedVersion() *ProtocolVersion {
	return h.version
}

// formatSecurityTypes formats a list of security type bytes for error messages
func formatSecurityTypes(types []byte) string {
	if len(types) == 0 {
		return "none"
	}

	result := ""
	for i, t := range types {
		if i > 0 {
			result += ", "
		}
		result += SecurityType(t).String()
	}
	return result
}

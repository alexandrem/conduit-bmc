package rfb

import (
	"encoding/binary"
	"fmt"
	"io"
)

// RFB Protocol Version constants
// RFB (Remote Framebuffer) protocol versions supported by this implementation
const (
	// ProtocolVersion33 - RFB 3.3 (oldest, server-chosen security type)
	ProtocolVersion33 = "RFB 003.003\n"

	// ProtocolVersion37 - RFB 3.7 (client-selected security type)
	ProtocolVersion37 = "RFB 003.007\n"

	// ProtocolVersion38 - RFB 3.8 (most common, adds failure reason strings)
	ProtocolVersion38 = "RFB 003.008\n"

	// ProtocolVersionLength - All RFB version strings are exactly 12 bytes
	ProtocolVersionLength = 12
)

// SecurityType represents an RFB security type
type SecurityType uint8

// Security Type constants
// RFB security types define authentication methods
const (
	// SecurityTypeInvalid - Invalid security type (0 = connection failed in RFB 3.3)
	SecurityTypeInvalid SecurityType = 0

	// SecurityTypeNone - No authentication required
	SecurityTypeNone SecurityType = 1

	// SecurityTypeVNCAuth - VNC Authentication (DES challenge-response)
	SecurityTypeVNCAuth SecurityType = 2

	// VNC Authentication uses 16-byte challenge/response
	VNCAuthChallengeLength = 16
)

// Security Result constants
// Server response after authentication attempt
const (
	// SecurityResultOK - Authentication succeeded
	SecurityResultOK uint32 = 0

	// SecurityResultFailed - Authentication failed
	SecurityResultFailed uint32 = 1
)

// ProtocolVersion represents a parsed RFB protocol version
type ProtocolVersion struct {
	Major int
	Minor int
	Raw   string // Original 12-byte version string
}

// String returns the version as "RFB x.y"
func (v ProtocolVersion) String() string {
	return fmt.Sprintf("RFB %d.%d", v.Major, v.Minor)
}

// IsSupported returns true if this version is supported by our implementation
func (v ProtocolVersion) IsSupported() bool {
	// Support RFB 3.3, 3.7, 3.8
	return v.Major == 3 && (v.Minor == 3 || v.Minor == 7 || v.Minor == 8)
}

// ToWireFormat returns the 12-byte wire format version string
func (v ProtocolVersion) ToWireFormat() string {
	return fmt.Sprintf("RFB %03d.%03d\n", v.Major, v.Minor)
}

// String returns the security type name
func (s SecurityType) String() string {
	switch s {
	case SecurityTypeNone:
		return "None"
	case SecurityTypeVNCAuth:
		return "VNC Authentication"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// IsSupported returns true if this security type is supported
func (s SecurityType) IsSupported() bool {
	return s == SecurityTypeNone || s == SecurityTypeVNCAuth
}

// ProtocolReader provides utility methods for reading RFB protocol data
type ProtocolReader struct {
	r io.Reader
}

// NewProtocolReader creates a new protocol reader
func NewProtocolReader(r io.Reader) *ProtocolReader {
	return &ProtocolReader{r: r}
}

// ReadFull reads exactly len(buf) bytes from the reader
func (pr *ProtocolReader) ReadFull(buf []byte) error {
	// Try reading with detailed error reporting
	totalRead := 0
	for totalRead < len(buf) {
		n, err := pr.r.Read(buf[totalRead:])
		totalRead += n

		if err != nil {
			if err == io.EOF && totalRead == 0 {
				return fmt.Errorf("failed to read %d bytes: immediate EOF (server closed connection)", len(buf))
			}
			if err == io.EOF && totalRead > 0 {
				return fmt.Errorf("failed to read %d bytes (got %d before EOF): %w", len(buf), totalRead, err)
			}
			if totalRead > 0 {
				return fmt.Errorf("failed to read %d bytes (got %d before error): %w", len(buf), totalRead, err)
			}
			return fmt.Errorf("failed to read %d bytes: %w", len(buf), err)
		}

		if totalRead >= len(buf) {
			break
		}
	}
	return nil
}

// ReadBytes reads exactly n bytes from the reader
func (pr *ProtocolReader) ReadBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	if err := pr.ReadFull(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// ReadU8 reads a single unsigned 8-bit integer
func (pr *ProtocolReader) ReadU8() (uint8, error) {
	buf, err := pr.ReadBytes(1)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

// ReadU32 reads an unsigned 32-bit integer (big-endian)
func (pr *ProtocolReader) ReadU32() (uint32, error) {
	buf, err := pr.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf), nil
}

// ReadString reads a length-prefixed string (u32 length + data)
func (pr *ProtocolReader) ReadString() (string, error) {
	length, err := pr.ReadU32()
	if err != nil {
		return "", fmt.Errorf("failed to read string length: %w", err)
	}

	if length == 0 {
		return "", nil
	}

	if length > 4096 { // Sanity check to prevent excessive memory allocation
		return "", fmt.Errorf("string length too large: %d bytes", length)
	}

	buf, err := pr.ReadBytes(int(length))
	if err != nil {
		return "", fmt.Errorf("failed to read string data: %w", err)
	}

	return string(buf), nil
}

// ProtocolWriter provides utility methods for writing RFB protocol data
type ProtocolWriter struct {
	w io.Writer
}

// NewProtocolWriter creates a new protocol writer
func NewProtocolWriter(w io.Writer) *ProtocolWriter {
	return &ProtocolWriter{w: w}
}

// Write writes data to the underlying writer
func (pw *ProtocolWriter) Write(data []byte) error {
	n, err := pw.w.Write(data)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	if n != len(data) {
		return fmt.Errorf("incomplete write: wrote %d of %d bytes", n, len(data))
	}
	return nil
}

// WriteU8 writes a single unsigned 8-bit integer
func (pw *ProtocolWriter) WriteU8(value uint8) error {
	return pw.Write([]byte{value})
}

// WriteU32 writes an unsigned 32-bit integer (big-endian)
func (pw *ProtocolWriter) WriteU32(value uint32) error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, value)
	return pw.Write(buf)
}

// WriteString writes a raw string (no length prefix)
func (pw *ProtocolWriter) WriteString(s string) error {
	return pw.Write([]byte(s))
}

// ParseProtocolVersion parses a 12-byte RFB version string
// Format: "RFB xxx.yyy\n" (e.g., "RFB 003.008\n")
func ParseProtocolVersion(data []byte) (*ProtocolVersion, error) {
	if len(data) != ProtocolVersionLength {
		return nil, fmt.Errorf("invalid version length: got %d bytes, expected %d", len(data), ProtocolVersionLength)
	}

	// Verify "RFB " prefix
	if string(data[0:4]) != "RFB " {
		return nil, fmt.Errorf("invalid version format: expected 'RFB ' prefix, got %q", string(data[0:4]))
	}

	// Verify newline suffix
	if data[11] != '\n' {
		return nil, fmt.Errorf("invalid version format: expected newline suffix, got %q", data[11])
	}

	// Parse major version (3 digits)
	major := 0
	for i := 4; i < 7; i++ {
		if data[i] < '0' || data[i] > '9' {
			return nil, fmt.Errorf("invalid major version: non-digit character %q", data[i])
		}
		major = major*10 + int(data[i]-'0')
	}

	// Verify dot separator
	if data[7] != '.' {
		return nil, fmt.Errorf("invalid version format: expected '.' separator, got %q", data[7])
	}

	// Parse minor version (3 digits)
	minor := 0
	for i := 8; i < 11; i++ {
		if data[i] < '0' || data[i] > '9' {
			return nil, fmt.Errorf("invalid minor version: non-digit character %q", data[i])
		}
		minor = minor*10 + int(data[i]-'0')
	}

	return &ProtocolVersion{
		Major: major,
		Minor: minor,
		Raw:   string(data),
	}, nil
}

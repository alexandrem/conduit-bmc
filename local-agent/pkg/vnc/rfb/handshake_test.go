package rfb

import (
	"bytes"
	"io"
	"testing"
)

// mockReadWriter is a mock io.ReadWriter for testing
type mockReadWriter struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
}

func newMockReadWriter() *mockReadWriter {
	return &mockReadWriter{
		readBuf:  &bytes.Buffer{},
		writeBuf: &bytes.Buffer{},
	}
}

func (m *mockReadWriter) Read(p []byte) (int, error) {
	return m.readBuf.Read(p)
}

func (m *mockReadWriter) Write(p []byte) (int, error) {
	return m.writeBuf.Write(p)
}

// TestNegotiateVersion tests protocol version negotiation
func TestNegotiateVersion(t *testing.T) {
	tests := []struct {
		name          string
		serverVersion string
		wantMajor     int
		wantMinor     int
		wantErr       bool
		errContains   string
	}{
		{
			name:          "RFB 3.3",
			serverVersion: "RFB 003.003\n",
			wantMajor:     3,
			wantMinor:     3,
			wantErr:       false,
		},
		{
			name:          "RFB 3.7",
			serverVersion: "RFB 003.007\n",
			wantMajor:     3,
			wantMinor:     7,
			wantErr:       false,
		},
		{
			name:          "RFB 3.8",
			serverVersion: "RFB 003.008\n",
			wantMajor:     3,
			wantMinor:     8,
			wantErr:       false,
		},
		{
			name:          "Unsupported version 4.0",
			serverVersion: "RFB 004.000\n",
			wantErr:       true,
			errContains:   "unsupported server version",
		},
		{
			name:          "Invalid version format",
			serverVersion: "INVALID DATA",
			wantErr:       true,
			errContains:   "invalid server version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()
			mock.readBuf.WriteString(tt.serverVersion)

			h := NewHandshake(mock)
			version, err := h.NegotiateVersion()

			if tt.wantErr {
				if err == nil {
					t.Errorf("NegotiateVersion() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NegotiateVersion() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NegotiateVersion() unexpected error = %v", err)
				return
			}

			if version.Major != tt.wantMajor || version.Minor != tt.wantMinor {
				t.Errorf("NegotiateVersion() version = %d.%d, want %d.%d",
					version.Major, version.Minor, tt.wantMajor, tt.wantMinor)
			}

			// Verify client sent correct version back
			clientVersion := mock.writeBuf.String()
			expectedVersion := version.ToWireFormat()
			if clientVersion != expectedVersion {
				t.Errorf("Client sent version %q, want %q", clientVersion, expectedVersion)
			}
		})
	}
}

// TestNegotiateSecurityType33 tests RFB 3.3 security type negotiation
func TestNegotiateSecurityType33(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse []byte
		preferVNCAuth  bool
		wantType       SecurityType
		wantErr        bool
		errContains    string
	}{
		{
			name:           "VNC Authentication",
			serverResponse: []byte{0x00, 0x00, 0x00, 0x02}, // u32: SecurityTypeVNCAuth
			preferVNCAuth:  true,
			wantType:       SecurityTypeVNCAuth,
			wantErr:        false,
		},
		{
			name:           "No Authentication",
			serverResponse: []byte{0x00, 0x00, 0x00, 0x01}, // u32: SecurityTypeNone
			preferVNCAuth:  false,
			wantType:       SecurityTypeNone,
			wantErr:        false,
		},
		{
			name: "Connection failed",
			serverResponse: append(
				[]byte{0x00, 0x00, 0x00, 0x00}, // u32: 0 = failed
				encodeString("Access denied")...,
			),
			wantErr:     true,
			errContains: "connection failed: Access denied",
		},
		{
			name:           "Unsupported security type",
			serverResponse: []byte{0x00, 0x00, 0x00, 0x10}, // u32: 16 (unsupported)
			wantErr:        true,
			errContains:    "unsupported security type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()

			// Setup version (3.3)
			mock.readBuf.WriteString("RFB 003.003\n")

			h := NewHandshake(mock)
			_, err := h.NegotiateVersion()
			if err != nil {
				t.Fatalf("NegotiateVersion() error = %v", err)
			}

			// Clear write buffer
			mock.writeBuf.Reset()

			// Setup security type response
			mock.readBuf.Write(tt.serverResponse)

			secType, err := h.NegotiateSecurityType(tt.preferVNCAuth)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NegotiateSecurityType() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NegotiateSecurityType() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NegotiateSecurityType() unexpected error = %v", err)
				return
			}

			if secType != tt.wantType {
				t.Errorf("NegotiateSecurityType() = %s, want %s", secType, tt.wantType)
			}

			// RFB 3.3: Client doesn't send anything back
			if mock.writeBuf.Len() != 0 {
				t.Errorf("Client sent data in RFB 3.3, expected no response (got %d bytes)", mock.writeBuf.Len())
			}
		})
	}
}

// TestNegotiateSecurityType37 tests RFB 3.7/3.8 security type negotiation
func TestNegotiateSecurityType37(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse []byte
		preferVNCAuth  bool
		wantType       SecurityType
		wantClientSent uint8
		wantErr        bool
		errContains    string
	}{
		{
			name: "Select VNC Auth (preferred)",
			serverResponse: []byte{
				0x02,                       // count = 2
				uint8(SecurityTypeNone),    // type 1
				uint8(SecurityTypeVNCAuth), // type 2
			},
			preferVNCAuth:  true,
			wantType:       SecurityTypeVNCAuth,
			wantClientSent: uint8(SecurityTypeVNCAuth),
			wantErr:        false,
		},
		{
			name: "Select None (preferred)",
			serverResponse: []byte{
				0x02,                       // count = 2
				uint8(SecurityTypeNone),    // type 1
				uint8(SecurityTypeVNCAuth), // type 2
			},
			preferVNCAuth:  false,
			wantType:       SecurityTypeNone,
			wantClientSent: uint8(SecurityTypeNone),
			wantErr:        false,
		},
		{
			name: "Only VNC Auth available",
			serverResponse: []byte{
				0x01,                       // count = 1
				uint8(SecurityTypeVNCAuth), // type 2
			},
			preferVNCAuth:  false, // Prefer None, but not available
			wantType:       SecurityTypeVNCAuth,
			wantClientSent: uint8(SecurityTypeVNCAuth),
			wantErr:        false,
		},
		{
			name: "Only None available",
			serverResponse: []byte{
				0x01,                    // count = 1
				uint8(SecurityTypeNone), // type 1
			},
			preferVNCAuth:  true, // Prefer VNC, but not available
			wantType:       SecurityTypeNone,
			wantClientSent: uint8(SecurityTypeNone),
			wantErr:        false,
		},
		{
			name: "No supported types",
			serverResponse: []byte{
				0x02, // count = 2
				0x10, // unknown type
				0x11, // unknown type
			},
			preferVNCAuth: true,
			wantErr:       true,
			errContains:   "no supported security type",
		},
		{
			name: "Connection failed (count = 0)",
			serverResponse: append(
				[]byte{0x00}, // count = 0
				encodeString("Server is busy")...,
			),
			wantErr:     true,
			errContains: "connection failed: Server is busy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()

			// Setup version (3.8)
			mock.readBuf.WriteString("RFB 003.008\n")

			h := NewHandshake(mock)
			_, err := h.NegotiateVersion()
			if err != nil {
				t.Fatalf("NegotiateVersion() error = %v", err)
			}

			// Clear write buffer
			mock.writeBuf.Reset()

			// Setup security type response
			mock.readBuf.Write(tt.serverResponse)

			secType, err := h.NegotiateSecurityType(tt.preferVNCAuth)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NegotiateSecurityType() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NegotiateSecurityType() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NegotiateSecurityType() unexpected error = %v", err)
				return
			}

			if secType != tt.wantType {
				t.Errorf("NegotiateSecurityType() = %s, want %s", secType, tt.wantType)
			}

			// Verify client sent selected type
			clientSent := mock.writeBuf.Bytes()
			if len(clientSent) != 1 {
				t.Errorf("Client sent %d bytes, want 1", len(clientSent))
				return
			}
			if clientSent[0] != tt.wantClientSent {
				t.Errorf("Client sent type %d, want %d", clientSent[0], tt.wantClientSent)
			}
		})
	}
}

// TestReadSecurityResult tests reading security result
func TestReadSecurityResult(t *testing.T) {
	tests := []struct {
		name           string
		rfbVersion     string
		serverResponse []byte
		wantErr        bool
		errContains    string
	}{
		{
			name:           "Success (3.3)",
			rfbVersion:     "RFB 003.003\n",
			serverResponse: []byte{0x00, 0x00, 0x00, 0x00}, // OK
			wantErr:        false,
		},
		{
			name:           "Success (3.8)",
			rfbVersion:     "RFB 003.008\n",
			serverResponse: []byte{0x00, 0x00, 0x00, 0x00}, // OK
			wantErr:        false,
		},
		{
			name:           "Failure (3.3 - no reason)",
			rfbVersion:     "RFB 003.003\n",
			serverResponse: []byte{0x00, 0x00, 0x00, 0x01}, // Failed
			wantErr:        true,
			errContains:    "authentication failed (server did not provide reason)",
		},
		{
			name:           "Failure (3.7 - no reason)",
			rfbVersion:     "RFB 003.007\n",
			serverResponse: []byte{0x00, 0x00, 0x00, 0x01}, // Failed
			wantErr:        true,
			errContains:    "authentication failed (server did not provide reason)",
		},
		{
			name:       "Failure (3.8 - with reason)",
			rfbVersion: "RFB 003.008\n",
			serverResponse: append(
				[]byte{0x00, 0x00, 0x00, 0x01}, // Failed
				encodeString("Invalid password")...,
			),
			wantErr:     true,
			errContains: "authentication failed: Invalid password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockReadWriter()

			// Setup version
			mock.readBuf.WriteString(tt.rfbVersion)

			h := NewHandshake(mock)
			_, err := h.NegotiateVersion()
			if err != nil {
				t.Fatalf("NegotiateVersion() error = %v", err)
			}

			// Setup security result
			mock.readBuf.Write(tt.serverResponse)

			err = h.ReadSecurityResult()

			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadSecurityResult() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ReadSecurityResult() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ReadSecurityResult() unexpected error = %v", err)
			}
		})
	}
}

// TestSecurityTypeSelectionPriority tests security type selection priority
func TestSecurityTypeSelectionPriority(t *testing.T) {
	tests := []struct {
		name           string
		availableTypes []byte
		preferVNCAuth  bool
		want           SecurityType
	}{
		{
			name:           "Prefer VNC, both available",
			availableTypes: []byte{uint8(SecurityTypeNone), uint8(SecurityTypeVNCAuth)},
			preferVNCAuth:  true,
			want:           SecurityTypeVNCAuth,
		},
		{
			name:           "Prefer None, both available",
			availableTypes: []byte{uint8(SecurityTypeNone), uint8(SecurityTypeVNCAuth)},
			preferVNCAuth:  false,
			want:           SecurityTypeNone,
		},
		{
			name:           "Only VNC available",
			availableTypes: []byte{uint8(SecurityTypeVNCAuth)},
			preferVNCAuth:  false,
			want:           SecurityTypeVNCAuth,
		},
		{
			name:           "Only None available",
			availableTypes: []byte{uint8(SecurityTypeNone)},
			preferVNCAuth:  true,
			want:           SecurityTypeNone,
		},
		{
			name:           "No supported types",
			availableTypes: []byte{0x10, 0x11},
			preferVNCAuth:  true,
			want:           SecurityTypeInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handshake{}
			got := h.selectSecurityType(tt.availableTypes, tt.preferVNCAuth)
			if got != tt.want {
				t.Errorf("selectSecurityType() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestNegotiateSecurityTypeWithoutVersion tests error when version not negotiated
func TestNegotiateSecurityTypeWithoutVersion(t *testing.T) {
	mock := newMockReadWriter()
	h := NewHandshake(mock)

	// Try to negotiate security type without version
	_, err := h.NegotiateSecurityType(true)
	if err == nil {
		t.Error("NegotiateSecurityType() expected error when version not negotiated, got nil")
	}
	if !contains(err.Error(), "version negotiation must be completed first") {
		t.Errorf("NegotiateSecurityType() error = %v, want error about version negotiation", err)
	}
}

// TestReadSecurityResultWithoutVersion tests error when version not negotiated
func TestReadSecurityResultWithoutVersion(t *testing.T) {
	mock := newMockReadWriter()
	h := NewHandshake(mock)

	// Try to read security result without version
	err := h.ReadSecurityResult()
	if err == nil {
		t.Error("ReadSecurityResult() expected error when version not negotiated, got nil")
	}
	if !contains(err.Error(), "version negotiation must be completed first") {
		t.Errorf("ReadSecurityResult() error = %v, want error about version negotiation", err)
	}
}

// TestGetNegotiatedVersion tests getting the negotiated version
func TestGetNegotiatedVersion(t *testing.T) {
	mock := newMockReadWriter()
	mock.readBuf.WriteString("RFB 003.008\n")

	h := NewHandshake(mock)

	// Before negotiation
	if v := h.GetNegotiatedVersion(); v != nil {
		t.Errorf("GetNegotiatedVersion() before negotiation = %v, want nil", v)
	}

	// Negotiate version
	version, err := h.NegotiateVersion()
	if err != nil {
		t.Fatalf("NegotiateVersion() error = %v", err)
	}

	// After negotiation
	if v := h.GetNegotiatedVersion(); v != version {
		t.Errorf("GetNegotiatedVersion() = %v, want %v", v, version)
	}
}

// TestNegotiateVersionEOF tests handling of EOF during version read
func TestNegotiateVersionEOF(t *testing.T) {
	mock := newMockReadWriter()
	mock.readBuf.WriteString("RFB 003") // Incomplete version

	h := NewHandshake(mock)
	_, err := h.NegotiateVersion()
	if err == nil {
		t.Error("NegotiateVersion() expected error for incomplete version, got nil")
	}
}

// encodeString encodes a string in RFB format (u32 length + data)
func encodeString(s string) []byte {
	buf := make([]byte, 4+len(s))
	buf[0] = byte(len(s) >> 24)
	buf[1] = byte(len(s) >> 16)
	buf[2] = byte(len(s) >> 8)
	buf[3] = byte(len(s))
	copy(buf[4:], s)
	return buf
}

// TestFormatSecurityTypes tests security type list formatting
func TestFormatSecurityTypes(t *testing.T) {
	tests := []struct {
		name  string
		types []byte
		want  string
	}{
		{
			name:  "Empty list",
			types: []byte{},
			want:  "none",
		},
		{
			name:  "Single type",
			types: []byte{uint8(SecurityTypeNone)},
			want:  "None",
		},
		{
			name:  "Multiple types",
			types: []byte{uint8(SecurityTypeNone), uint8(SecurityTypeVNCAuth)},
			want:  "None, VNC Authentication",
		},
		{
			name:  "Unknown type",
			types: []byte{0x99},
			want:  "Unknown(153)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSecurityTypes(tt.types)
			if got != tt.want {
				t.Errorf("formatSecurityTypes() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestHandshakeReadErrors tests error handling during reads
func TestHandshakeReadErrors(t *testing.T) {
	t.Run("Version read error", func(t *testing.T) {
		mock := newMockReadWriter()
		// Don't write anything - will cause EOF
		h := NewHandshake(mock)
		_, err := h.NegotiateVersion()
		if err == nil {
			t.Error("NegotiateVersion() expected EOF error, got nil")
		}
	})

	t.Run("Security type count read error (3.7)", func(t *testing.T) {
		mock := newMockReadWriter()
		mock.readBuf.WriteString("RFB 003.007\n")

		h := NewHandshake(mock)
		_, err := h.NegotiateVersion()
		if err != nil {
			t.Fatalf("NegotiateVersion() error = %v", err)
		}

		// Don't write security types - will cause EOF
		_, err = h.NegotiateSecurityType(true)
		if err == nil {
			t.Error("NegotiateSecurityType() expected EOF error, got nil")
		}
	})

	t.Run("Security result read error", func(t *testing.T) {
		mock := newMockReadWriter()
		mock.readBuf.WriteString("RFB 003.008\n")

		h := NewHandshake(mock)
		_, err := h.NegotiateVersion()
		if err != nil {
			t.Fatalf("NegotiateVersion() error = %v", err)
		}

		// Don't write security result - will cause EOF
		err = h.ReadSecurityResult()
		if err == nil {
			t.Error("ReadSecurityResult() expected EOF error, got nil")
		}
	})
}

// limitedReadWriter fails writes after a certain point
type limitedReadWriter struct {
	*mockReadWriter
	writeLimit int
	writeCount int
}

func (lrw *limitedReadWriter) Write(p []byte) (int, error) {
	if lrw.writeCount >= lrw.writeLimit {
		return 0, io.ErrShortWrite
	}
	n, err := lrw.mockReadWriter.Write(p)
	lrw.writeCount += n
	return n, err
}

// TestHandshakeWriteErrors tests error handling during writes
func TestHandshakeWriteErrors(t *testing.T) {
	t.Run("Version write error", func(t *testing.T) {
		mock := newMockReadWriter()
		mock.readBuf.WriteString("RFB 003.008\n")

		lrw := &limitedReadWriter{
			mockReadWriter: mock,
			writeLimit:     0, // Fail immediately
		}

		h := NewHandshake(lrw)
		_, err := h.NegotiateVersion()
		if err == nil {
			t.Error("NegotiateVersion() expected write error, got nil")
		}
	})

	t.Run("Security type write error (3.8)", func(t *testing.T) {
		mock := newMockReadWriter()
		mock.readBuf.WriteString("RFB 003.008\n")

		h := NewHandshake(mock)
		_, err := h.NegotiateVersion()
		if err != nil {
			t.Fatalf("NegotiateVersion() error = %v", err)
		}

		// Setup security types
		mock.readBuf.Write([]byte{0x01, uint8(SecurityTypeVNCAuth)})

		// Create limited writer after version negotiation
		lrw := &limitedReadWriter{
			mockReadWriter: mock,
			writeLimit:     0, // Fail on security type write
		}
		h.writer = NewProtocolWriter(lrw)

		_, err = h.NegotiateSecurityType(true)
		if err == nil {
			t.Error("NegotiateSecurityType() expected write error, got nil")
		}
	})
}

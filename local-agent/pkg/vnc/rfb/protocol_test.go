package rfb

import (
	"bytes"
	"io"
	"testing"
)

// TestParseProtocolVersion tests parsing of RFB protocol version strings
func TestParseProtocolVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		wantMajor   int
		wantMinor   int
		wantRaw     string
		wantErr     bool
		errContains string
	}{
		{
			name:      "RFB 3.3",
			input:     []byte("RFB 003.003\n"),
			wantMajor: 3,
			wantMinor: 3,
			wantRaw:   "RFB 003.003\n",
			wantErr:   false,
		},
		{
			name:      "RFB 3.7",
			input:     []byte("RFB 003.007\n"),
			wantMajor: 3,
			wantMinor: 7,
			wantRaw:   "RFB 003.007\n",
			wantErr:   false,
		},
		{
			name:      "RFB 3.8",
			input:     []byte("RFB 003.008\n"),
			wantMajor: 3,
			wantMinor: 8,
			wantRaw:   "RFB 003.008\n",
			wantErr:   false,
		},
		{
			name:        "Invalid length (too short)",
			input:       []byte("RFB 003.00"),
			wantErr:     true,
			errContains: "invalid version length",
		},
		{
			name:        "Invalid length (too long)",
			input:       []byte("RFB 003.008\n\n"),
			wantErr:     true,
			errContains: "invalid version length",
		},
		{
			name:        "Missing RFB prefix",
			input:       []byte("VNC 003.008\n"),
			wantErr:     true,
			errContains: "expected 'RFB ' prefix",
		},
		{
			name:        "Missing newline suffix",
			input:       []byte("RFB 003.008 "),
			wantErr:     true,
			errContains: "expected newline suffix",
		},
		{
			name:        "Invalid major version (non-digit)",
			input:       []byte("RFB 00A.008\n"),
			wantErr:     true,
			errContains: "invalid major version",
		},
		{
			name:        "Invalid minor version (non-digit)",
			input:       []byte("RFB 003.00X\n"),
			wantErr:     true,
			errContains: "invalid minor version",
		},
		{
			name:        "Missing dot separator",
			input:       []byte("RFB 003_008\n"),
			wantErr:     true,
			errContains: "expected '.' separator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := ParseProtocolVersion(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseProtocolVersion() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ParseProtocolVersion() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseProtocolVersion() unexpected error = %v", err)
				return
			}

			if version.Major != tt.wantMajor {
				t.Errorf("ParseProtocolVersion() major = %d, want %d", version.Major, tt.wantMajor)
			}
			if version.Minor != tt.wantMinor {
				t.Errorf("ParseProtocolVersion() minor = %d, want %d", version.Minor, tt.wantMinor)
			}
			if version.Raw != tt.wantRaw {
				t.Errorf("ParseProtocolVersion() raw = %q, want %q", version.Raw, tt.wantRaw)
			}
		})
	}
}

// TestProtocolVersionIsSupported tests version support detection
func TestProtocolVersionIsSupported(t *testing.T) {
	tests := []struct {
		name          string
		version       ProtocolVersion
		wantSupported bool
	}{
		{
			name:          "RFB 3.3 supported",
			version:       ProtocolVersion{Major: 3, Minor: 3},
			wantSupported: true,
		},
		{
			name:          "RFB 3.7 supported",
			version:       ProtocolVersion{Major: 3, Minor: 7},
			wantSupported: true,
		},
		{
			name:          "RFB 3.8 supported",
			version:       ProtocolVersion{Major: 3, Minor: 8},
			wantSupported: true,
		},
		{
			name:          "RFB 3.5 not supported",
			version:       ProtocolVersion{Major: 3, Minor: 5},
			wantSupported: false,
		},
		{
			name:          "RFB 4.0 not supported",
			version:       ProtocolVersion{Major: 4, Minor: 0},
			wantSupported: false,
		},
		{
			name:          "RFB 2.0 not supported",
			version:       ProtocolVersion{Major: 2, Minor: 0},
			wantSupported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.version.IsSupported(); got != tt.wantSupported {
				t.Errorf("IsSupported() = %v, want %v", got, tt.wantSupported)
			}
		})
	}
}

// TestProtocolVersionToWireFormat tests wire format conversion
func TestProtocolVersionToWireFormat(t *testing.T) {
	tests := []struct {
		name    string
		version ProtocolVersion
		want    string
	}{
		{
			name:    "RFB 3.3",
			version: ProtocolVersion{Major: 3, Minor: 3},
			want:    "RFB 003.003\n",
		},
		{
			name:    "RFB 3.7",
			version: ProtocolVersion{Major: 3, Minor: 7},
			want:    "RFB 003.007\n",
		},
		{
			name:    "RFB 3.8",
			version: ProtocolVersion{Major: 3, Minor: 8},
			want:    "RFB 003.008\n",
		},
		{
			name:    "RFB 10.15",
			version: ProtocolVersion{Major: 10, Minor: 15},
			want:    "RFB 010.015\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.version.ToWireFormat(); got != tt.want {
				t.Errorf("ToWireFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestProtocolVersionString tests string representation
func TestProtocolVersionString(t *testing.T) {
	tests := []struct {
		name    string
		version ProtocolVersion
		want    string
	}{
		{
			name:    "RFB 3.3",
			version: ProtocolVersion{Major: 3, Minor: 3},
			want:    "RFB 3.3",
		},
		{
			name:    "RFB 3.8",
			version: ProtocolVersion{Major: 3, Minor: 8},
			want:    "RFB 3.8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.version.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSecurityTypeString tests security type string representation
func TestSecurityTypeString(t *testing.T) {
	tests := []struct {
		name         string
		securityType SecurityType
		want         string
	}{
		{
			name:         "None",
			securityType: SecurityTypeNone,
			want:         "None",
		},
		{
			name:         "VNC Authentication",
			securityType: SecurityTypeVNCAuth,
			want:         "VNC Authentication",
		},
		{
			name:         "Unknown type",
			securityType: SecurityType(99),
			want:         "Unknown(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.securityType.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSecurityTypeIsSupported tests security type support detection
func TestSecurityTypeIsSupported(t *testing.T) {
	tests := []struct {
		name         string
		securityType SecurityType
		want         bool
	}{
		{
			name:         "None supported",
			securityType: SecurityTypeNone,
			want:         true,
		},
		{
			name:         "VNC Auth supported",
			securityType: SecurityTypeVNCAuth,
			want:         true,
		},
		{
			name:         "Unknown not supported",
			securityType: SecurityType(99),
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.securityType.IsSupported(); got != tt.want {
				t.Errorf("IsSupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestProtocolReader tests reading protocol data
func TestProtocolReader(t *testing.T) {
	t.Run("ReadU8", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte{0x42})
		pr := NewProtocolReader(buf)

		val, err := pr.ReadU8()
		if err != nil {
			t.Fatalf("ReadU8() error = %v", err)
		}
		if val != 0x42 {
			t.Errorf("ReadU8() = 0x%02x, want 0x42", val)
		}
	})

	t.Run("ReadU32", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte{0x00, 0x00, 0x01, 0x23})
		pr := NewProtocolReader(buf)

		val, err := pr.ReadU32()
		if err != nil {
			t.Fatalf("ReadU32() error = %v", err)
		}
		if val != 0x123 {
			t.Errorf("ReadU32() = 0x%08x, want 0x00000123", val)
		}
	})

	t.Run("ReadBytes", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte{0x01, 0x02, 0x03, 0x04})
		pr := NewProtocolReader(buf)

		data, err := pr.ReadBytes(4)
		if err != nil {
			t.Fatalf("ReadBytes() error = %v", err)
		}
		if !bytes.Equal(data, []byte{0x01, 0x02, 0x03, 0x04}) {
			t.Errorf("ReadBytes() = %v, want [1 2 3 4]", data)
		}
	})

	t.Run("ReadString", func(t *testing.T) {
		// String format: u32 length + data
		buf := bytes.NewBuffer([]byte{
			0x00, 0x00, 0x00, 0x05, // length = 5
			'h', 'e', 'l', 'l', 'o', // data
		})
		pr := NewProtocolReader(buf)

		str, err := pr.ReadString()
		if err != nil {
			t.Fatalf("ReadString() error = %v", err)
		}
		if str != "hello" {
			t.Errorf("ReadString() = %q, want %q", str, "hello")
		}
	})

	t.Run("ReadString empty", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte{0x00, 0x00, 0x00, 0x00})
		pr := NewProtocolReader(buf)

		str, err := pr.ReadString()
		if err != nil {
			t.Fatalf("ReadString() error = %v", err)
		}
		if str != "" {
			t.Errorf("ReadString() = %q, want empty string", str)
		}
	})

	t.Run("ReadString too large", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte{0x00, 0x01, 0x00, 0x00}) // 65536 bytes
		pr := NewProtocolReader(buf)

		_, err := pr.ReadString()
		if err == nil {
			t.Error("ReadString() expected error for large string, got nil")
		}
	})

	t.Run("ReadFull EOF", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte{0x01, 0x02})
		pr := NewProtocolReader(buf)

		data := make([]byte, 4)
		err := pr.ReadFull(data)
		if err == nil {
			t.Error("ReadFull() expected EOF error, got nil")
		}
	})
}

// TestProtocolWriter tests writing protocol data
func TestProtocolWriter(t *testing.T) {
	t.Run("WriteU8", func(t *testing.T) {
		buf := &bytes.Buffer{}
		pw := NewProtocolWriter(buf)

		err := pw.WriteU8(0x42)
		if err != nil {
			t.Fatalf("WriteU8() error = %v", err)
		}
		if !bytes.Equal(buf.Bytes(), []byte{0x42}) {
			t.Errorf("WriteU8() wrote %v, want [0x42]", buf.Bytes())
		}
	})

	t.Run("WriteU32", func(t *testing.T) {
		buf := &bytes.Buffer{}
		pw := NewProtocolWriter(buf)

		err := pw.WriteU32(0x123)
		if err != nil {
			t.Fatalf("WriteU32() error = %v", err)
		}
		if !bytes.Equal(buf.Bytes(), []byte{0x00, 0x00, 0x01, 0x23}) {
			t.Errorf("WriteU32() wrote %v, want [0x00 0x00 0x01 0x23]", buf.Bytes())
		}
	})

	t.Run("WriteString", func(t *testing.T) {
		buf := &bytes.Buffer{}
		pw := NewProtocolWriter(buf)

		err := pw.WriteString("hello")
		if err != nil {
			t.Fatalf("WriteString() error = %v", err)
		}
		if !bytes.Equal(buf.Bytes(), []byte("hello")) {
			t.Errorf("WriteString() wrote %v, want %v", buf.Bytes(), []byte("hello"))
		}
	})

	t.Run("Write", func(t *testing.T) {
		buf := &bytes.Buffer{}
		pw := NewProtocolWriter(buf)

		err := pw.Write([]byte{0x01, 0x02, 0x03})
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if !bytes.Equal(buf.Bytes(), []byte{0x01, 0x02, 0x03}) {
			t.Errorf("Write() wrote %v, want [1 2 3]", buf.Bytes())
		}
	})
}

// TestProtocolReaderWriter tests combined read/write operations
func TestProtocolReaderWriter(t *testing.T) {
	t.Run("Round-trip U8", func(t *testing.T) {
		buf := &bytes.Buffer{}
		pw := NewProtocolWriter(buf)
		pr := NewProtocolReader(buf)

		// Write
		if err := pw.WriteU8(0x42); err != nil {
			t.Fatalf("WriteU8() error = %v", err)
		}

		// Read
		val, err := pr.ReadU8()
		if err != nil {
			t.Fatalf("ReadU8() error = %v", err)
		}
		if val != 0x42 {
			t.Errorf("Round-trip U8 = 0x%02x, want 0x42", val)
		}
	})

	t.Run("Round-trip U32", func(t *testing.T) {
		buf := &bytes.Buffer{}
		pw := NewProtocolWriter(buf)
		pr := NewProtocolReader(buf)

		// Write
		if err := pw.WriteU32(0x12345678); err != nil {
			t.Fatalf("WriteU32() error = %v", err)
		}

		// Read
		val, err := pr.ReadU32()
		if err != nil {
			t.Fatalf("ReadU32() error = %v", err)
		}
		if val != 0x12345678 {
			t.Errorf("Round-trip U32 = 0x%08x, want 0x12345678", val)
		}
	})
}

// limitedWriter is a writer that fails after N bytes
type limitedWriter struct {
	limit int
	count int
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.count >= lw.limit {
		return 0, io.ErrShortWrite
	}
	n := len(p)
	if lw.count+n > lw.limit {
		n = lw.limit - lw.count
	}
	lw.count += n
	return n, nil
}

// TestProtocolWriterErrors tests error handling in writer
func TestProtocolWriterErrors(t *testing.T) {
	t.Run("Write error", func(t *testing.T) {
		lw := &limitedWriter{limit: 0}
		pw := NewProtocolWriter(lw)

		err := pw.Write([]byte{0x01})
		if err == nil {
			t.Error("Write() expected error, got nil")
		}
	})

	t.Run("WriteU8 error", func(t *testing.T) {
		lw := &limitedWriter{limit: 0}
		pw := NewProtocolWriter(lw)

		err := pw.WriteU8(0x42)
		if err == nil {
			t.Error("WriteU8() expected error, got nil")
		}
	})

	t.Run("WriteU32 incomplete", func(t *testing.T) {
		lw := &limitedWriter{limit: 2} // Only allow 2 bytes
		pw := NewProtocolWriter(lw)

		err := pw.WriteU32(0x12345678)
		if err == nil {
			t.Error("WriteU32() expected error for incomplete write, got nil")
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

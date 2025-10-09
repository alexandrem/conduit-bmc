package rfb

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"
)

// TestReverseBits tests bit reversal function
func TestReverseBits(t *testing.T) {
	tests := []struct {
		name  string
		input byte
		want  byte
	}{
		{
			name:  "0x00 -> 0x00",
			input: 0x00,
			want:  0x00,
		},
		{
			name:  "0xFF -> 0xFF",
			input: 0xFF,
			want:  0xFF,
		},
		{
			name:  "0x01 -> 0x80",
			input: 0x01,
			want:  0x80,
		},
		{
			name:  "0x80 -> 0x01",
			input: 0x80,
			want:  0x01,
		},
		{
			name:  "0xAA (10101010) -> 0x55 (01010101)",
			input: 0xAA,
			want:  0x55,
		},
		{
			name:  "0x55 (01010101) -> 0xAA (10101010)",
			input: 0x55,
			want:  0xAA,
		},
		{
			name:  "0xB2 (10110010) -> 0x4D (01001101)",
			input: 0xB2,
			want:  0x4D,
		},
		{
			name:  "0x4D (01001101) -> 0xB2 (10110010)",
			input: 0x4D,
			want:  0xB2,
		},
		{
			name:  "0x12 (00010010) -> 0x48 (01001000)",
			input: 0x12,
			want:  0x48,
		},
		{
			name:  "0xF0 (11110000) -> 0x0F (00001111)",
			input: 0xF0,
			want:  0x0F,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reverseBits(tt.input)
			if got != tt.want {
				t.Errorf("reverseBits(0x%02X) = 0x%02X, want 0x%02X", tt.input, got, tt.want)
			}

			// Test that reversal is symmetric (reversing twice returns original)
			gotReversed := reverseBits(got)
			if gotReversed != tt.input {
				t.Errorf("reverseBits(reverseBits(0x%02X)) = 0x%02X, want 0x%02X", tt.input, gotReversed, tt.input)
			}
		})
	}
}

// TestPrepareVNCKey tests VNC key preparation
func TestPrepareVNCKey(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantHex  string
	}{
		{
			name:     "Empty password",
			password: "",
			wantHex:  "0000000000000000", // 8 null bytes
		},
		{
			name:     "Single character",
			password: "a",
			// 'a' = 0x61 = 01100001 -> reversed = 10000110 = 0x86
			wantHex: "8600000000000000",
		},
		{
			name:     "8-byte password",
			password: "12345678",
			// '1' = 0x31, '2' = 0x32, '3' = 0x33, '4' = 0x34
			// '5' = 0x35, '6' = 0x36, '7' = 0x37, '8' = 0x38
			// After bit reversal:
			// 0x31 (00110001) -> 10001100 (0x8C)
			// 0x32 (00110010) -> 01001100 (0x4C)
			// 0x33 (00110011) -> 11001100 (0xCC)
			// 0x34 (00110100) -> 00101100 (0x2C)
			// 0x35 (00110101) -> 10101100 (0xAC)
			// 0x36 (00110110) -> 01101100 (0x6C)
			// 0x37 (00110111) -> 11101100 (0xEC)
			// 0x38 (00111000) -> 00011100 (0x1C)
			wantHex: "8c4ccc2cac6cec1c",
		},
		{
			name:     "Password longer than 8 bytes (truncated)",
			password: "verylongpassword",
			// Only first 8 bytes: "verylong"
			// 'v' = 0x76, 'e' = 0x65, 'r' = 0x72, 'y' = 0x79
			// 'l' = 0x6C, 'o' = 0x6F, 'n' = 0x6E, 'g' = 0x67
			// After bit reversal:
			// 0x76 (01110110) -> 01101110 (0x6E)
			// 0x65 (01100101) -> 10100110 (0xA6)
			// 0x72 (01110010) -> 01001110 (0x4E)
			// 0x79 (01111001) -> 10011110 (0x9E)
			// 0x6C (01101100) -> 00110110 (0x36)
			// 0x6F (01101111) -> 11110110 (0xF6)
			// 0x6E (01101110) -> 01110110 (0x76)
			// 0x67 (01100111) -> 11100110 (0xE6)
			wantHex: "6ea64e9e36f676e6",
		},
		{
			name:     "Password shorter than 8 bytes (null-padded)",
			password: "pass",
			// 'p' = 0x70, 'a' = 0x61, 's' = 0x73, 's' = 0x73, then 4 nulls
			// After bit reversal:
			// 0x70 (01110000) -> 00001110 (0x0E)
			// 0x61 (01100001) -> 10000110 (0x86)
			// 0x73 (01110011) -> 11001110 (0xCE)
			// 0x73 (01110011) -> 11001110 (0xCE)
			// 0x00 -> 0x00 (4 times)
			wantHex: "0e86cece00000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prepareVNCKey(tt.password)

			// Convert to hex for comparison
			gotHex := hex.EncodeToString(got)
			if gotHex != tt.wantHex {
				t.Errorf("prepareVNCKey(%q) = %s, want %s", tt.password, gotHex, tt.wantHex)
			}

			// Verify key is always 8 bytes
			if len(got) != 8 {
				t.Errorf("prepareVNCKey(%q) returned %d bytes, want 8", tt.password, len(got))
			}
		})
	}
}

// TestEncryptVNCChallenge tests VNC challenge encryption
func TestEncryptVNCChallenge(t *testing.T) {
	tests := []struct {
		name         string
		challenge    string // hex-encoded
		password     string
		wantResponse string // hex-encoded (will be calculated if empty)
		wantErr      bool
	}{
		{
			name:      "Password: password, known challenge",
			challenge: "8a847bcd5d6fdd580edb4a5cffd9e233",
			password:  "password",
			// Response will be verified via round-trip
			wantResponse: "",
			wantErr:      false,
		},
		{
			name:      "Password: test, sequential challenge",
			challenge: "0102030405060708090a0b0c0d0e0f10",
			password:  "test",
			// Response will be verified via round-trip
			wantResponse: "",
			wantErr:      false,
		},
		{
			name:         "Empty password, zero challenge",
			challenge:    "00000000000000000000000000000000",
			password:     "",
			wantResponse: "", // Will verify via round-trip
			wantErr:      false,
		},
		{
			name:      "Invalid challenge length (too short)",
			challenge: "0102030405060708",
			password:  "test",
			wantErr:   true,
		},
		{
			name:      "Invalid challenge length (too long)",
			challenge: "0102030405060708090a0b0c0d0e0f1011",
			password:  "test",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenge, err := hex.DecodeString(tt.challenge)
			if err != nil {
				t.Fatalf("Failed to decode challenge hex: %v", err)
			}

			response, err := encryptVNCChallenge(challenge, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Errorf("encryptVNCChallenge() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("encryptVNCChallenge() unexpected error: %v", err)
				return
			}

			// Verify response length
			if len(response) != VNCAuthChallengeLength {
				t.Errorf("Response length = %d, want %d", len(response), VNCAuthChallengeLength)
				return
			}

			// If expected response is provided, verify it matches
			if tt.wantResponse != "" {
				gotHex := hex.EncodeToString(response)
				if gotHex != tt.wantResponse {
					t.Errorf("encryptVNCChallenge() response = %s, want %s", gotHex, tt.wantResponse)
					return
				}
			}

			// Always verify round-trip (encrypt then decrypt should return original challenge)
			decrypted, err := DecryptVNCResponse(response, tt.password)
			if err != nil {
				t.Errorf("Failed to decrypt response: %v", err)
				return
			}

			if !bytes.Equal(decrypted, challenge) {
				t.Errorf("Round-trip failed: decrypted = %s, want %s",
					hex.EncodeToString(decrypted),
					tt.challenge)
			}
		})
	}
}

// TestDecryptVNCResponse tests VNC response decryption (for verification)
func TestDecryptVNCResponse(t *testing.T) {
	tests := []struct {
		name      string
		challenge string // hex-encoded (will be encrypted first)
		password  string
		wantErr   bool
	}{
		{
			name:      "Decrypt after encryption (password)",
			challenge: "8a847bcd5d6fdd580edb4a5cffd9e233",
			password:  "password",
			wantErr:   false,
		},
		{
			name:      "Decrypt after encryption (test)",
			challenge: "0102030405060708090a0b0c0d0e0f10",
			password:  "test",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First encrypt the challenge
			challengeBytes, err := hex.DecodeString(tt.challenge)
			if err != nil {
				t.Fatalf("Failed to decode challenge hex: %v", err)
			}

			response, err := encryptVNCChallenge(challengeBytes, tt.password)
			if err != nil {
				t.Fatalf("Failed to encrypt challenge: %v", err)
			}

			// Now decrypt it
			decrypted, err := DecryptVNCResponse(response, tt.password)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DecryptVNCResponse() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("DecryptVNCResponse() unexpected error: %v", err)
				return
			}

			// Verify it matches the original challenge
			if !bytes.Equal(decrypted, challengeBytes) {
				t.Errorf("DecryptVNCResponse() challenge = %s, want %s",
					hex.EncodeToString(decrypted),
					tt.challenge)
			}
		})
	}

	// Test invalid response length separately
	t.Run("Invalid response length", func(t *testing.T) {
		response := []byte{0x01, 0x02, 0x03}
		_, err := DecryptVNCResponse(response, "test")
		if err == nil {
			t.Error("DecryptVNCResponse() expected error for invalid length, got nil")
		}
	})
}

// TestEncryptDecryptRoundTrip tests that encryption and decryption are inverse operations
func TestEncryptDecryptRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		challenge string // hex-encoded
		password  string
	}{
		{
			name:      "Random challenge 1",
			challenge: "fedcba9876543210fedcba9876543210",
			password:  "mypass",
		},
		{
			name:      "Random challenge 2",
			challenge: "123456789abcdef0123456789abcdef0",
			password:  "secret",
		},
		{
			name:      "All zeros",
			challenge: "00000000000000000000000000000000",
			password:  "test",
		},
		{
			name:      "All ones",
			challenge: "ffffffffffffffffffffffffffffffff",
			password:  "admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenge, err := hex.DecodeString(tt.challenge)
			if err != nil {
				t.Fatalf("Failed to decode challenge hex: %v", err)
			}

			// Encrypt
			response, err := encryptVNCChallenge(challenge, tt.password)
			if err != nil {
				t.Fatalf("encryptVNCChallenge() error: %v", err)
			}

			// Decrypt
			decrypted, err := DecryptVNCResponse(response, tt.password)
			if err != nil {
				t.Fatalf("DecryptVNCResponse() error: %v", err)
			}

			// Verify round-trip
			if !bytes.Equal(decrypted, challenge) {
				t.Errorf("Round-trip failed: got %s, want %s",
					hex.EncodeToString(decrypted),
					hex.EncodeToString(challenge))
			}
		})
	}
}

// TestPerformVNCAuth tests the VNC authentication flow
func TestPerformVNCAuth(t *testing.T) {
	tests := []struct {
		name      string
		challenge string // hex-encoded
		password  string
		wantErr   bool
	}{
		{
			name:      "Valid authentication",
			challenge: "8a847bcd5d6fdd580edb4a5cffd9e233",
			password:  "password",
			wantErr:   false,
		},
		{
			name:      "Short password",
			challenge: "0102030405060708090a0b0c0d0e0f10",
			password:  "pw",
			wantErr:   false,
		},
		{
			name:      "Invalid challenge (too short)",
			challenge: "0102030405060708",
			password:  "test",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Decode challenge
			challengeBytes, err := hex.DecodeString(tt.challenge)
			if err != nil && !tt.wantErr {
				t.Fatalf("Failed to decode challenge hex: %v", err)
			}

			// Setup mock reader/writer with separate buffers
			readBuf := bytes.NewBuffer(challengeBytes) // Server sends challenge
			writeBuf := &bytes.Buffer{}                // Client writes response

			// Create a combined reader/writer
			type combinedRW struct {
				io.Reader
				io.Writer
			}
			rw := &combinedRW{Reader: readBuf, Writer: writeBuf}

			auth := NewAuthenticator(rw)
			err = auth.PerformVNCAuth(tt.password)

			if tt.wantErr {
				if err == nil {
					t.Errorf("PerformVNCAuth() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("PerformVNCAuth() unexpected error: %v", err)
				return
			}

			// Verify response was written
			responseBytes := writeBuf.Bytes()

			if len(responseBytes) != VNCAuthChallengeLength {
				t.Errorf("Response length = %d, want %d", len(responseBytes), VNCAuthChallengeLength)
				return
			}

			// Verify the response can be decrypted back to the challenge (round-trip test)
			decrypted, err := DecryptVNCResponse(responseBytes, tt.password)
			if err != nil {
				t.Errorf("Failed to decrypt response: %v", err)
				return
			}

			if !bytes.Equal(decrypted, challengeBytes) {
				t.Errorf("Decrypted challenge = %s, want %s",
					hex.EncodeToString(decrypted),
					hex.EncodeToString(challengeBytes))
			}
		})
	}
}

// TestPerformVNCAuthReadError tests error handling when reading challenge fails
func TestPerformVNCAuthReadError(t *testing.T) {
	// Empty buffer - will cause EOF when trying to read challenge
	buf := &bytes.Buffer{}

	auth := NewAuthenticator(buf)
	err := auth.PerformVNCAuth("password")

	if err == nil {
		t.Error("PerformVNCAuth() expected error for empty buffer, got nil")
	}

	if !contains(err.Error(), "failed to read VNC auth challenge") {
		t.Errorf("PerformVNCAuth() error = %v, want error about reading challenge", err)
	}
}

// TestPerformVNCAuthWriteError tests error handling when writing response fails
func TestPerformVNCAuthWriteError(t *testing.T) {
	// Setup with valid challenge but limited writer
	challenge, _ := hex.DecodeString("8a847bcd5d6fdd580edb4a5cffd9e233")

	readBuf := bytes.NewBuffer(challenge)
	writeBuf := &limitedWriter{limit: 0} // Fail writes immediately

	// Create a read/writer that reads from readBuf and writes to writeBuf
	type combinedRW struct {
		io.Reader
		io.Writer
	}
	rw := &combinedRW{Reader: readBuf, Writer: writeBuf}

	auth := NewAuthenticator(rw)
	err := auth.PerformVNCAuth("password")

	if err == nil {
		t.Error("PerformVNCAuth() expected error for write failure, got nil")
	}

	if !contains(err.Error(), "failed to send VNC auth response") {
		t.Errorf("PerformVNCAuth() error = %v, want error about sending response", err)
	}
}

// Benchmark VNC key preparation
func BenchmarkPrepareVNCKey(b *testing.B) {
	password := "password"
	for i := 0; i < b.N; i++ {
		_ = prepareVNCKey(password)
	}
}

// Benchmark VNC challenge encryption
func BenchmarkEncryptVNCChallenge(b *testing.B) {
	challenge, _ := hex.DecodeString("8a847bcd5d6fdd580edb4a5cffd9e233")
	password := "password"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = encryptVNCChallenge(challenge, password)
	}
}

// Benchmark bit reversal
func BenchmarkReverseBits(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = reverseBits(0xB2)
	}
}

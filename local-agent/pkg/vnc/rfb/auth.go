package rfb

import (
	"crypto/des"
	"fmt"
	"io"
)

// Authenticator handles VNC authentication
type Authenticator struct {
	reader *ProtocolReader
	writer *ProtocolWriter
}

// NewAuthenticator creates a new VNC authenticator
func NewAuthenticator(rw io.ReadWriter) *Authenticator {
	return &Authenticator{
		reader: NewProtocolReader(rw),
		writer: NewProtocolWriter(rw),
	}
}

// PerformVNCAuth performs VNC challenge-response authentication
//
// VNC Authentication flow:
// 1. Server sends 16-byte random challenge
// 2. Client encrypts challenge with DES using password as key
// 3. Client sends 16-byte encrypted response
// 4. Server validates and sends result (handled separately via ReadSecurityResult)
//
// VNC DES encryption quirk:
// - Password is truncated/padded to 8 bytes
// - Each byte has its bits reversed before use as DES key
// - Challenge is encrypted in two 8-byte blocks (ECB mode)
func (a *Authenticator) PerformVNCAuth(password string) error {
	// Read 16-byte challenge from server
	challenge, err := a.reader.ReadBytes(VNCAuthChallengeLength)
	if err != nil {
		return fmt.Errorf("failed to read VNC auth challenge: %w", err)
	}

	// Encrypt challenge with password
	response, err := encryptVNCChallenge(challenge, password)
	if err != nil {
		return fmt.Errorf("failed to encrypt VNC challenge: %w", err)
	}

	// Send encrypted response to server
	if err := a.writer.Write(response); err != nil {
		return fmt.Errorf("failed to send VNC auth response: %w", err)
	}

	return nil
}

// encryptVNCChallenge encrypts a 16-byte VNC challenge using DES
//
// VNC uses a non-standard DES encryption:
// 1. Password is truncated to 8 bytes (or padded with nulls)
// 2. Each byte of the password has its bits REVERSED (VNC quirk)
// 3. Challenge is encrypted in two 8-byte blocks using DES ECB mode
//
// This bit-reversal is specific to VNC and not part of standard DES.
func encryptVNCChallenge(challenge []byte, password string) ([]byte, error) {
	if len(challenge) != VNCAuthChallengeLength {
		return nil, fmt.Errorf("invalid challenge length: got %d bytes, expected %d", len(challenge), VNCAuthChallengeLength)
	}

	// Prepare 8-byte DES key from password
	key := prepareVNCKey(password)

	// Create DES cipher
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create DES cipher: %w", err)
	}

	// Encrypt challenge in two 8-byte blocks (ECB mode)
	response := make([]byte, VNCAuthChallengeLength)
	block.Encrypt(response[0:8], challenge[0:8])   // First 8 bytes
	block.Encrypt(response[8:16], challenge[8:16]) // Second 8 bytes

	return response, nil
}

// prepareVNCKey prepares an 8-byte DES key from a VNC password
//
// Steps:
// 1. Truncate password to 8 bytes (or pad with null bytes if shorter)
// 2. Reverse the bits in each byte (VNC-specific quirk)
//
// Example:
//
//	Input password: "mypass" (6 bytes)
//	After padding:  "mypass\x00\x00" (8 bytes)
//	After bit reversal: each byte has bits reversed
func prepareVNCKey(password string) []byte {
	key := make([]byte, 8)

	// Copy password bytes (truncate to 8 bytes if longer)
	n := len(password)
	if n > 8 {
		n = 8
	}
	copy(key, password[:n])
	// Remaining bytes are already zero (null padding)

	// Reverse bits in each byte (VNC quirk)
	for i := 0; i < 8; i++ {
		key[i] = reverseBits(key[i])
	}

	return key
}

// reverseBits reverses the bits in a byte
//
// Example:
//
//	Input:  0b10110010 (0xB2)
//	Output: 0b01001101 (0x4D)
//
// This is required by VNC's non-standard DES key preparation.
func reverseBits(b byte) byte {
	var result byte
	for i := 0; i < 8; i++ {
		// Shift result left to make room for next bit
		result <<= 1
		// Add the least significant bit of b to result
		result |= b & 1
		// Shift b right to process next bit
		b >>= 1
	}
	return result
}

// DecryptVNCResponse decrypts a VNC authentication response
// This is used for testing and verification purposes only.
// The server normally performs this operation, not the client.
func DecryptVNCResponse(response []byte, password string) ([]byte, error) {
	if len(response) != VNCAuthChallengeLength {
		return nil, fmt.Errorf("invalid response length: got %d bytes, expected %d", len(response), VNCAuthChallengeLength)
	}

	// Prepare 8-byte DES key from password
	key := prepareVNCKey(password)

	// Create DES cipher
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create DES cipher: %w", err)
	}

	// Decrypt response in two 8-byte blocks (ECB mode)
	challenge := make([]byte, VNCAuthChallengeLength)
	block.Decrypt(challenge[0:8], response[0:8])   // First 8 bytes
	block.Decrypt(challenge[8:16], response[8:16]) // Second 8 bytes

	return challenge, nil
}

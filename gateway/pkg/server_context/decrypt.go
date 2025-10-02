package server_context

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"core/auth"
)

// ServerContextDecryptor handles decryption of server context from JWT tokens
type ServerContextDecryptor struct {
	encryptionKey []byte
	signingKey    []byte
}

// NewServerContextDecryptor creates a new server context decryptor
func NewServerContextDecryptor(encryptionKey string) *ServerContextDecryptor {
	// Store original key for JWT signing
	signingKey := []byte(encryptionKey)

	// Use first 32 bytes of key for AES-256
	key := []byte(encryptionKey)
	if len(key) > 32 {
		key = key[:32]
	} else if len(key) < 32 {
		// Pad key to 32 bytes
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}

	return &ServerContextDecryptor{
		encryptionKey: key,
		signingKey:    signingKey, // Use original key for JWT signing
	}
}

// ExtractServerContextFromJWT extracts and decrypts server context from a JWT token
func (d *ServerContextDecryptor) ExtractServerContextFromJWT(tokenString string) (*auth.ServerContext, error) {
	// Parse JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return d.signingKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid JWT token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Extract encrypted server context
	encryptedContextStr, ok := claims["server_context"].(string)
	if !ok || encryptedContextStr == "" {
		return nil, fmt.Errorf("token does not contain server context")
	}

	// Decrypt server context
	serverContext, err := d.decryptServerContext(encryptedContextStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt server context: %w", err)
	}

	// Validate server context hasn't expired
	if time.Now().After(serverContext.ExpiresAt) {
		return nil, fmt.Errorf("server context has expired")
	}

	return serverContext, nil
}

// decryptServerContext decrypts server context using AES-256-GCM
func (d *ServerContextDecryptor) decryptServerContext(encryptedContext string) (*auth.ServerContext, error) {
	// Base64 decode
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedContext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(d.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce and ciphertext
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	contextJSON, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	// Unmarshal JSON
	var serverContext auth.ServerContext
	if err := json.Unmarshal(contextJSON, &serverContext); err != nil {
		return nil, fmt.Errorf("failed to unmarshal server context: %w", err)
	}

	return &serverContext, nil
}

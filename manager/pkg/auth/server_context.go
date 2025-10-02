package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"manager/pkg/models"
)

// ServerContext contains the BMC endpoint information that will be encrypted
// in JWT tokens.
type ServerContext struct {
	ServerID     string    `json:"server_id"`
	CustomerID   string    `json:"customer_id"`
	BMCEndpoint  string    `json:"bmc_endpoint"`
	BMCType      string    `json:"bmc_type"`
	Features     []string  `json:"features"`
	DatacenterID string    `json:"datacenter_id"`
	Permissions  []string  `json:"permissions"`
	IssuedAt     time.Time `json:"iat"`
	ExpiresAt    time.Time `json:"exp"`
}

// EncryptedJWT represents a JWT token with encrypted server context.
type EncryptedJWT struct {
	CustomerID    string `json:"customer_id"`
	Email         string `json:"email"`
	JTI           string `json:"jti"`
	IssuedAt      int64  `json:"iat"`
	ExpiresAt     int64  `json:"exp"`
	ServerContext string `json:"server_context,omitempty"` // Base64 encoded encrypted server context
}

// ServerContextService handles encryption and decryption of server context.
type ServerContextService struct {
	encryptionKey []byte
}

// NewServerContextService creates a new server context service.
func NewServerContextService(encryptionKey string) *ServerContextService {
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

	return &ServerContextService{
		encryptionKey: key,
	}
}

// CreateServerContext creates a server context with BMC endpoint information.
func (s *ServerContextService) CreateServerContext(server *models.Server, permissions []string) *ServerContext {
	now := time.Now()

	bmcEndpoint := ""
	bmcType := "ipmi" // Default

	// Extract BMC information from control endpoint
	if server.ControlEndpoint != nil {
		bmcEndpoint = server.ControlEndpoint.Endpoint
		bmcType = string(server.ControlEndpoint.Type)
	}

	return &ServerContext{
		ServerID:     server.ID,
		CustomerID:   server.CustomerID,
		BMCEndpoint:  bmcEndpoint,
		BMCType:      bmcType,
		Features:     server.Features,
		DatacenterID: server.DatacenterID,
		Permissions:  permissions,
		IssuedAt:     now,
		ExpiresAt:    now.Add(1 * time.Hour), // Server tokens expire in 1 hour
	}
}

// EncryptServerContext encrypts server context using AES-256-GCM.
func (s *ServerContextService) EncryptServerContext(serverContext *ServerContext) (string, error) {
	// Marshal server context to JSON
	contextJSON, err := json.Marshal(serverContext)
	if err != nil {
		return "", fmt.Errorf("failed to marshal server context: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the context
	ciphertext := gcm.Seal(nonce, nonce, contextJSON, nil)

	// Base64 encode the result
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptServerContext decrypts server context using AES-256-GCM.
func (s *ServerContextService) DecryptServerContext(encryptedContext string) (*ServerContext, error) {
	// Base64 decode
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedContext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.encryptionKey)
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
	var serverContext ServerContext
	if err := json.Unmarshal(contextJSON, &serverContext); err != nil {
		return nil, fmt.Errorf("failed to unmarshal server context: %w", err)
	}

	// Validate expiration
	if time.Now().After(serverContext.ExpiresAt) {
		return nil, fmt.Errorf("server context has expired")
	}

	return &serverContext, nil
}

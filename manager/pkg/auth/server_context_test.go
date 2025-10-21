package auth

import (
	"encoding/base64"
	"testing"
	"time"

	"core/types"
	"manager/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerContextService_CreateServerContext(t *testing.T) {
	service := NewServerContextService("test-encryption-key")

	server := &models.Server{
		ID:         "server-001",
		CustomerID: "customer-123",
		ControlEndpoints: []*models.BMCControlEndpoint{
			{
				Endpoint: "http://localhost:9001",
				Type:     models.BMCTypeRedfish,
			},
		},
		PrimaryProtocol: models.BMCTypeRedfish,
		Features: types.FeaturesToStrings([]types.Feature{
			types.FeaturePower,
			types.FeatureConsole,
			types.FeatureVNC,
		}),
		DatacenterID: "dc-local-01",
	}

	permissions := []string{"power:read", "power:write", "console:read"}

	context := service.CreateServerContext(server, permissions)

	assert.Equal(t, "server-001", context.ServerID)
	assert.Equal(t, "customer-123", context.CustomerID)
	assert.Equal(t, "http://localhost:9001", context.BMCEndpoint)
	assert.Equal(t, "redfish", context.BMCType)
	assert.Equal(t, types.FeaturesToStrings([]types.Feature{
		types.FeaturePower,
		types.FeatureConsole,
		types.FeatureVNC,
	}), context.Features)
	assert.Equal(t, "dc-local-01", context.DatacenterID)
	assert.Equal(t, permissions, context.Permissions)
	assert.True(t, time.Now().After(context.IssuedAt))
	assert.True(t, context.ExpiresAt.After(context.IssuedAt))
	assert.True(t, context.ExpiresAt.Sub(context.IssuedAt) <= time.Hour+time.Second)
}

func TestServerContextService_EncryptDecryptRoundTrip(t *testing.T) {
	service := NewServerContextService("test-encryption-key-for-validation")

	originalContext := &ServerContext{
		ServerID:    "server-001",
		CustomerID:  "customer-123",
		BMCEndpoint: "http://localhost:9001",
		BMCType:     "redfish",
		Features: types.FeaturesToStrings([]types.Feature{
			types.FeaturePower,
			types.FeatureConsole,
		}),
		DatacenterID: "dc-local-01",
		Permissions:  []string{"power:read", "power:write"},
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	// Encrypt the context
	encrypted, err := service.EncryptServerContext(originalContext)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// Decrypt the context
	decrypted, err := service.DecryptServerContext(encrypted)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, originalContext.ServerID, decrypted.ServerID)
	assert.Equal(t, originalContext.CustomerID, decrypted.CustomerID)
	assert.Equal(t, originalContext.BMCEndpoint, decrypted.BMCEndpoint)
	assert.Equal(t, originalContext.BMCType, decrypted.BMCType)
	assert.Equal(t, originalContext.Features, decrypted.Features)
	assert.Equal(t, originalContext.DatacenterID, decrypted.DatacenterID)
	assert.Equal(t, originalContext.Permissions, decrypted.Permissions)
	assert.WithinDuration(t, originalContext.IssuedAt, decrypted.IssuedAt, time.Second)
	assert.WithinDuration(t, originalContext.ExpiresAt, decrypted.ExpiresAt, time.Second)
}

func TestServerContextService_DecryptExpiredContext(t *testing.T) {
	service := NewServerContextService("test-encryption-key")

	expiredContext := &ServerContext{
		ServerID:    "server-001",
		CustomerID:  "customer-123",
		BMCEndpoint: "http://localhost:9001",
		BMCType:     "redfish",
		Features: types.FeaturesToStrings([]types.Feature{
			types.FeaturePower,
		}),
		DatacenterID: "dc-local-01",
		Permissions:  []string{"power:read"},
		IssuedAt:     time.Now().Add(-2 * time.Hour),
		ExpiresAt:    time.Now().Add(-time.Hour), // Expired 1 hour ago
	}

	encrypted, err := service.EncryptServerContext(expiredContext)
	require.NoError(t, err)

	// Should fail to decrypt expired context
	_, err = service.DecryptServerContext(encrypted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server context has expired")
}

func TestServerContextService_DecryptWithWrongKey(t *testing.T) {
	service1 := NewServerContextService("correct-key")
	service2 := NewServerContextService("wrong-key")

	context := &ServerContext{
		ServerID:    "server-001",
		CustomerID:  "customer-123",
		BMCEndpoint: "http://localhost:9001",
		BMCType:     "redfish",
		Features: types.FeaturesToStrings([]types.Feature{
			types.FeaturePower,
		}),
		DatacenterID: "dc-local-01",
		Permissions:  []string{"power:read"},
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	// Encrypt with correct key
	encrypted, err := service1.EncryptServerContext(context)
	require.NoError(t, err)

	// Try to decrypt with wrong key
	_, err = service2.DecryptServerContext(encrypted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt")
}

func TestServerContextService_EncryptionProducesUniqueResults(t *testing.T) {
	service := NewServerContextService("test-encryption-key")

	context := &ServerContext{
		ServerID:    "server-001",
		CustomerID:  "customer-123",
		BMCEndpoint: "http://localhost:9001",
		BMCType:     "redfish",
		Features: types.FeaturesToStrings([]types.Feature{
			types.FeaturePower,
		}),
		DatacenterID: "dc-local-01",
		Permissions:  []string{"power:read"},
		IssuedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	// Encrypt the same context multiple times
	encrypted1, err := service.EncryptServerContext(context)
	require.NoError(t, err)

	encrypted2, err := service.EncryptServerContext(context)
	require.NoError(t, err)

	// Should produce different encrypted results due to random nonce
	assert.NotEqual(t, encrypted1, encrypted2)

	// But both should decrypt to the same content
	decrypted1, err := service.DecryptServerContext(encrypted1)
	require.NoError(t, err)

	decrypted2, err := service.DecryptServerContext(encrypted2)
	require.NoError(t, err)

	assert.Equal(t, decrypted1.ServerID, decrypted2.ServerID)
	assert.Equal(t, decrypted1.BMCEndpoint, decrypted2.BMCEndpoint)
}

func TestServerContextService_InvalidBase64(t *testing.T) {
	service := NewServerContextService("test-encryption-key")

	// Try to decrypt invalid base64
	_, err := service.DecryptServerContext("invalid-base64!!!")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode base64")
}

func TestServerContextService_TruncatedCiphertext(t *testing.T) {
	service := NewServerContextService("test-encryption-key")

	// Create a valid base64 string that's too short for nonce
	// AES-GCM needs at least 12 bytes for nonce, create only 8 bytes
	shortData := make([]byte, 8)
	truncated := base64.StdEncoding.EncodeToString(shortData)

	_, err := service.DecryptServerContext(truncated)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

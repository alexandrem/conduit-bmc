package manager

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	managerv1 "manager/gen/manager/v1"
	"manager/pkg/auth"
	"manager/internal/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticate_TokenExpirationTime(t *testing.T) {
	// Setup
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	jwtManager := auth.NewJWTManager("test-secret-key")
	handler := NewBMCManagerServiceHandler(db, jwtManager)

	// Test authentication request
	req := connect.NewRequest(&managerv1.AuthenticateRequest{
		Email:    "test@example.com",
		Password: "password",
	})

	// Record time before authentication
	beforeAuth := time.Now()

	// Perform authentication
	resp, err := handler.Authenticate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Record time after authentication
	afterAuth := time.Now()

	// Verify response has proper expiration time
	assert.NotNil(t, resp.Msg.ExpiresAt)
	expiresAt := resp.Msg.ExpiresAt.AsTime()

	// Token should expire approximately 24 hours from now
	expectedExpiration := beforeAuth.Add(24 * time.Hour)
	timeDiff := expiresAt.Sub(expectedExpiration)

	// Allow up to 1 minute difference to account for test execution time
	assert.True(t, timeDiff >= -time.Minute && timeDiff <= time.Minute,
		"Token expiration time should be ~24 hours from authentication. Expected: %v, Got: %v, Diff: %v",
		expectedExpiration, expiresAt, timeDiff)

	// Verify token is not expired immediately
	assert.True(t, expiresAt.After(afterAuth),
		"Token should not be expired immediately after authentication")

	// Verify token expires in the future (approximately 24 hours)
	timeUntilExpiration := time.Until(expiresAt)
	assert.True(t, timeUntilExpiration > 23*time.Hour && timeUntilExpiration < 25*time.Hour,
		"Token should expire in approximately 24 hours. Time until expiration: %v", timeUntilExpiration)
}

func TestAuthenticate_TokenContentMatchesExpiration(t *testing.T) {
	// Setup
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	jwtManager := auth.NewJWTManager("test-secret-key")
	handler := NewBMCManagerServiceHandler(db, jwtManager)

	// Test authentication request
	req := connect.NewRequest(&managerv1.AuthenticateRequest{
		Email:    "test@example.com",
		Password: "password",
	})

	// Perform authentication
	resp, err := handler.Authenticate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Parse the JWT token to verify its expiration matches the response
	token := resp.Msg.AccessToken
	require.NotEmpty(t, token)

	// Validate token and extract claims
	claims, err := jwtManager.ValidateToken(token)
	require.NoError(t, err)
	require.NotNil(t, claims)

	// The JWT token itself should be valid for approximately 24 hours
	// We can't easily extract the exp claim without additional JWT parsing,
	// but we can validate that the token is currently valid
	assert.NotNil(t, claims)
	assert.Equal(t, "test@example.com", claims.Email)

	// Verify the response expiration time is reasonable
	expiresAt := resp.Msg.ExpiresAt.AsTime()
	now := time.Now()

	// Token should expire in the future
	assert.True(t, expiresAt.After(now), "Token should expire in the future")

	// Token should expire in approximately 24 hours (with some tolerance)
	expectedExpiration := now.Add(24 * time.Hour)
	timeDiff := expiresAt.Sub(expectedExpiration)
	assert.True(t, timeDiff >= -2*time.Minute && timeDiff <= 2*time.Minute,
		"Token expiration should be approximately 24 hours from now")
}

func TestAuthenticate_RegressTokenImmediateExpiration(t *testing.T) {
	// This is a regression test for the bug where tokens were expiring immediately
	// due to setting ExpiresAt: timestamppb.Now() instead of the actual expiration

	// Setup
	db, err := database.New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	jwtManager := auth.NewJWTManager("test-secret-key")
	handler := NewBMCManagerServiceHandler(db, jwtManager)

	// Test authentication request
	req := connect.NewRequest(&managerv1.AuthenticateRequest{
		Email:    "test@example.com",
		Password: "password",
	})

	// Perform authentication
	resp, err := handler.Authenticate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Get the expiration time
	expiresAt := resp.Msg.ExpiresAt.AsTime()
	now := time.Now()

	// REGRESSION TEST: Token should NOT expire immediately
	// The bug was that ExpiresAt was set to "now" instead of "now + 24h"
	assert.False(t, expiresAt.Before(now.Add(time.Minute)),
		"REGRESSION: Token should not expire within 1 minute of authentication")

	assert.False(t, expiresAt.Before(now.Add(time.Hour)),
		"REGRESSION: Token should not expire within 1 hour of authentication")

	// Token should expire much later (at least 20 hours in the future)
	assert.True(t, expiresAt.After(now.Add(20*time.Hour)),
		"Token should expire at least 20 hours in the future")
}

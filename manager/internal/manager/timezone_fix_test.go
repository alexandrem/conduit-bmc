package manager

import (
	"context"
	"testing"
	"time"

	managerv1 "manager/gen/manager/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticate_TokenTimezoneConsistency(t *testing.T) {
	handler := setupTestHandler(t)

	// Test authentication
	req := connect.NewRequest(&managerv1.AuthenticateRequest{
		Email:    "timezone-test@example.com",
		Password: "password123",
	})

	beforeAuth := time.Now().UTC()
	resp, err := handler.Authenticate(context.Background(), req)
	afterAuth := time.Now().UTC()

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify token expiration is reasonable (should be ~24 hours from now)
	expiresAt := resp.Msg.ExpiresAt.AsTime()

	// Token should expire between 23.5 and 24.5 hours from now to account for test execution time
	minExpiration := beforeAuth.Add(23*time.Hour + 30*time.Minute)
	maxExpiration := afterAuth.Add(24*time.Hour + 30*time.Minute)

	assert.True(t, expiresAt.After(minExpiration),
		"Token expiration %v should be after %v", expiresAt, minExpiration)
	assert.True(t, expiresAt.Before(maxExpiration),
		"Token expiration %v should be before %v", expiresAt, maxExpiration)

	// Verify JWT token contains consistent timing
	claims, err := handler.jwtManager.ValidateToken(resp.Msg.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "timezone-test@example.com", claims.CustomerID)
}

func TestAuthenticate_TokenNotExpiredImmediately(t *testing.T) {
	handler := setupTestHandler(t)

	// Test authentication
	req := connect.NewRequest(&managerv1.AuthenticateRequest{
		Email:    "immediate-test@example.com",
		Password: "password123",
	})

	resp, err := handler.Authenticate(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify token is not expired immediately after creation
	expiresAt := resp.Msg.ExpiresAt.AsTime()
	now := time.Now().UTC()

	assert.True(t, expiresAt.After(now),
		"Token should not be expired immediately. Expires: %v, Now: %v",
		expiresAt, now)

	// Verify token is valid for a reasonable time (at least 20 hours)
	minimumValidDuration := 20 * time.Hour
	assert.True(t, expiresAt.Sub(now) > minimumValidDuration,
		"Token should be valid for at least 20 hours. Valid for: %v",
		expiresAt.Sub(now))
}

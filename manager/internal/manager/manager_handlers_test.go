package manager

import (
	"context"
	"testing"
	"time"

	managerv1 "manager/gen/manager/v1"
	"manager/pkg/auth"
	"manager/pkg/database"
	"manager/pkg/models"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestHandler(t *testing.T) *BMCManagerServiceHandler {
	// Create in-memory database for testing
	db, err := database.New(":memory:")
	require.NoError(t, err)

	// Create JWT manager with test secret
	jwtManager := auth.NewJWTManager("test-secret-key")

	return NewBMCManagerServiceHandler(db, jwtManager)
}

func setupTestGateway(t *testing.T, handler *BMCManagerServiceHandler) *models.RegionalGateway {
	// Create a test gateway
	gateway := &models.RegionalGateway{
		ID:            "test-gateway-1",
		Region:        "us-test-1",
		Endpoint:      "http://localhost:8081",
		DatacenterIDs: []string{"dc-test-01"},
		Status:        "active",
		LastSeen:      time.Now(),
		CreatedAt:     time.Now(),
	}

	// Register the gateway
	err := handler.db.CreateRegionalGateway(gateway)
	require.NoError(t, err)

	return gateway
}

func TestListGateways_GeneratesDelegatedTokens(t *testing.T) {
	handler := setupTestHandler(t)

	// Setup test gateway
	setupTestGateway(t, handler)

	// Create test customer
	customer := &models.Customer{
		ID:    "test-customer-1",
		Email: "test@example.com",
	}

	// Create authenticated context
	ctx := context.WithValue(context.Background(), "customer_id", customer.ID)
	ctx = context.WithValue(ctx, "customer_email", customer.Email)

	// Create request
	req := connect.NewRequest(&managerv1.ListGatewaysRequest{})

	// Call ListGateways
	resp, err := handler.ListGateways(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify response
	gateways := resp.Msg.Gateways
	require.Len(t, gateways, 1)

	gateway := gateways[0]
	assert.Equal(t, "test-gateway-1", gateway.Id)
	assert.Equal(t, "us-test-1", gateway.Region)
	assert.Equal(t, "http://localhost:8081", gateway.Endpoint)

	// Verify delegated token is generated
	assert.NotEmpty(t, gateway.DelegatedToken, "Delegated token should be generated")
	assert.Greater(t, len(gateway.DelegatedToken), 50, "Delegated token should be substantial JWT")

	// Verify token can be validated
	claims, err := handler.jwtManager.ValidateToken(gateway.DelegatedToken)
	require.NoError(t, err)
	assert.Equal(t, customer.ID, claims.CustomerID)
}

func TestListGateways_RequiresAuthentication(t *testing.T) {
	handler := setupTestHandler(t)

	// Setup test gateway
	setupTestGateway(t, handler)

	// Create unauthenticated context (no customer_id)
	ctx := context.Background()

	// Create request
	req := connect.NewRequest(&managerv1.ListGatewaysRequest{})

	// Call ListGateways
	resp, err := handler.ListGateways(ctx, req)

	// Should fail due to missing authentication
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "customer not authenticated")
}

func TestListGateways_HandlesTokenGenerationError(t *testing.T) {
	// Create handler with invalid JWT secret to force token generation error
	db, err := database.New(":memory:")
	require.NoError(t, err)

	// Create JWT manager with empty secret (will cause errors)
	jwtManager := auth.NewJWTManager("")
	handler := NewBMCManagerServiceHandler(db, jwtManager)

	// Setup test gateway
	setupTestGateway(t, handler)

	// Create authenticated context
	ctx := context.WithValue(context.Background(), "customer_id", "test-customer-1")
	ctx = context.WithValue(ctx, "customer_email", "test@example.com")

	// Create request
	req := connect.NewRequest(&managerv1.ListGatewaysRequest{})

	// Call ListGateways
	resp, err := handler.ListGateways(ctx, req)
	require.NoError(t, err) // Should not fail completely
	require.NotNil(t, resp)

	// Verify response
	gateways := resp.Msg.Gateways
	require.Len(t, gateways, 1)

	gateway := gateways[0]
	assert.Equal(t, "test-gateway-1", gateway.Id)

	// Verify delegated token is empty due to generation error
	assert.Empty(t, gateway.DelegatedToken, "Delegated token should be empty when generation fails")
}

func TestListGateways_EmptyWhenNoGateways(t *testing.T) {
	handler := setupTestHandler(t)

	// Create authenticated context
	ctx := context.WithValue(context.Background(), "customer_id", "test-customer-1")
	ctx = context.WithValue(ctx, "customer_email", "test@example.com")

	// Create request
	req := connect.NewRequest(&managerv1.ListGatewaysRequest{})

	// Call ListGateways
	resp, err := handler.ListGateways(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify empty response
	gateways := resp.Msg.Gateways
	assert.Len(t, gateways, 0)
}

func TestListGateways_FiltersByRegion(t *testing.T) {
	handler := setupTestHandler(t)

	// Create gateways in different regions
	gateway1 := &models.RegionalGateway{
		ID:            "gateway-us-east",
		Region:        "us-east-1",
		Endpoint:      "http://localhost:8081",
		DatacenterIDs: []string{"dc-us-east-01"},
		Status:        "active",
		LastSeen:      time.Now(),
		CreatedAt:     time.Now(),
	}

	gateway2 := &models.RegionalGateway{
		ID:            "gateway-eu-west",
		Region:        "eu-west-1",
		Endpoint:      "http://localhost:8082",
		DatacenterIDs: []string{"dc-eu-west-01"},
		Status:        "active",
		LastSeen:      time.Now(),
		CreatedAt:     time.Now(),
	}

	err := handler.db.CreateRegionalGateway(gateway1)
	require.NoError(t, err)
	err = handler.db.CreateRegionalGateway(gateway2)
	require.NoError(t, err)

	// Create authenticated context
	ctx := context.WithValue(context.Background(), "customer_id", "test-customer-1")
	ctx = context.WithValue(ctx, "customer_email", "test@example.com")

	// Test filtering by region
	req := connect.NewRequest(&managerv1.ListGatewaysRequest{
		Region: "us-east-1",
	})

	resp, err := handler.ListGateways(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should only return US East gateway
	gateways := resp.Msg.Gateways
	require.Len(t, gateways, 1)
	assert.Equal(t, "gateway-us-east", gateways[0].Id)
	assert.Equal(t, "us-east-1", gateways[0].Region)
	assert.NotEmpty(t, gateways[0].DelegatedToken)
}

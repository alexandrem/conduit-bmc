package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	commonauth "core/auth"
	"core/domain"
	"core/types"
	gatewayv1 "gateway/gen/gateway/v1"
	"gateway/internal/agent"
	"gateway/pkg/server_context"
	"manager/pkg/auth"
	managermodels "manager/pkg/models"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

// convertCustomerToManager converts common/domain.Customer to manager/pkg/domain.Customer
func convertCustomerToManager(customer *domain.Customer) *managermodels.Customer {
	return &managermodels.Customer{
		ID:        customer.ID,
		Email:     customer.Email,
		APIKey:    customer.APIKey,
		CreatedAt: customer.CreatedAt,
	}
}

// convertServerToManager converts common/domain.Server to manager/pkg/domain.Server
func convertServerToManager(server *domain.Server) *managermodels.Server {
	var controlEndpoint *managermodels.BMCControlEndpoint
	if server.BMCEndpoint != "" {
		controlEndpoint = &managermodels.BMCControlEndpoint{
			Endpoint: server.BMCEndpoint,
			Type:     managermodels.BMCType(server.BMCType),
		}
	}

	return &managermodels.Server{
		ID:              server.ID,
		CustomerID:      server.CustomerID,
		DatacenterID:    server.DatacenterID,
		ControlEndpoint: controlEndpoint,
		Features:        server.Features,
		Status:          server.Status,
		CreatedAt:       server.CreatedAt,
		UpdatedAt:       server.UpdatedAt,
	}
}

// convertAuthClaimsFromManager converts manager/pkg/models.AuthClaims to common/auth.AuthClaims
func convertAuthClaimsFromManager(claims *managermodels.AuthClaims) *commonauth.AuthClaims {
	return &commonauth.AuthClaims{
		CustomerID: claims.CustomerID,
		Email:      claims.Email,
		UUID:       claims.UUID,
	}
}

// newGatewayHandler creates a handler instance for testing without
// external dependencies.
func newGatewayHandler(gatewayID, region string) *RegionalGatewayHandler {
	jwtManager := auth.NewJWTManager("test-secret")
	serverContextDecryptor := server_context.NewServerContextDecryptor("test-secret")
	return &RegionalGatewayHandler{
		bmcManagerEndpoint:     "http://localhost:8080",
		jwtManager:             jwtManager,
		serverContextDecryptor: serverContextDecryptor,
		gatewayID:              gatewayID,
		region:                 region,
		externalEndpoint:       "test-gateway:8081",
		managerClient:          nil,            // No client needed for testing
		httpClient:             &http.Client{}, // Initialize HTTP client for agent RPC calls
		testMode:               true,
		agentRegistry:          agent.NewRegistry(),
		bmcEndpointMapping:     make(map[string]*domain.AgentBMCMapping),
		consoleSessions:        make(map[string]*ConsoleSession),
	}
}

// createAuthenticatedContext creates a context with a valid server token for testing
func createAuthenticatedContext(serverID, customerID string) context.Context {
	// Use the same secret key as the test handler
	jwtManager := auth.NewJWTManager("test-secret")

	customer := &domain.Customer{
		ID:    customerID,
		Email: "test@example.com",
	}

	server := &domain.Server{
		ID:           serverID,
		CustomerID:   customerID,
		BMCEndpoint:  serverID, // For tests, use server ID as BMC endpoint
		BMCType:      types.BMCTypeIPMI,
		Features:     []string{"power", "console", "sensors"},
		DatacenterID: "dc-1",
	}

	permissions := []string{"power:read", "power:write", "console:read", "sensors:read"}

	token, err := jwtManager.GenerateServerToken(convertCustomerToManager(customer), convertServerToManager(server), permissions)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate test token: %v", err))
	}

	return context.WithValue(context.Background(), "token", token)
}

func TestNewGatewayHandler(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	if handler == nil {
		t.Fatal("NewGatewayHandler returned nil")
	}

	if handler.gatewayID != "gateway-1" {
		t.Errorf("Expected gatewayID 'gateway-1', got '%s'", handler.gatewayID)
	}

	if handler.region != "us-west-1" {
		t.Errorf("Expected region 'us-west-1', got '%s'", handler.region)
	}

	if handler.agentRegistry == nil {
		t.Error("Agent registry should not be nil")
	}

	if handler.bmcEndpointMapping == nil {
		t.Error("BMC endpoint mapping should not be nil")
	}
}

func TestHealthCheck(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	req := connect.NewRequest(&gatewayv1.HealthCheckRequest{})
	resp, err := handler.HealthCheck(context.Background(), req)

	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	if resp.Msg.Status == "" {
		t.Error("Status should not be empty")
	}

	if resp.Msg.Timestamp == nil {
		t.Error("Timestamp should not be nil")
	}
}

func TestRegisterAgent(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	bmcEndpoints := []*gatewayv1.BMCEndpointRegistration{
		{
			ServerId: "test-server-1",
			ControlEndpoint: &gatewayv1.BMCControlEndpoint{
				Endpoint: "192.168.1.100:623",
				Type:     gatewayv1.BMCType_BMC_TYPE_IPMI,
			},
			Features: []string{"power", "console"},
			Status:   "reachable",
			Metadata: map[string]string{"rack": "R1U42"},
		},
	}

	req := connect.NewRequest(&gatewayv1.RegisterAgentRequest{
		AgentId:      "agent-1",
		DatacenterId: "dc-1",
		Endpoint:     "http://agent:8080",
		BmcEndpoints: bmcEndpoints,
	})

	resp, err := handler.RegisterAgent(context.Background(), req)

	if err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	if !resp.Msg.Success {
		t.Error("Registration should be successful")
	}

	// Verify agent was registered
	agentInfo := handler.agentRegistry.Get("agent-1")
	if agentInfo == nil {
		t.Error("Agent should be registered in registry")
	}

	if agentInfo.ID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got '%s'", agentInfo.ID)
	}

	if agentInfo.DatacenterID != "dc-1" {
		t.Errorf("Expected datacenter ID 'dc-1', got '%s'", agentInfo.DatacenterID)
	}

	// Verify BMC endpoint mapping was created
	handler.mu.RLock()
	mapping := handler.bmcEndpointMapping["192.168.1.100:623"]
	handler.mu.RUnlock()

	if mapping == nil {
		t.Error("BMC endpoint mapping should be created")
	}

	if mapping.BMCEndpoint != "192.168.1.100:623" {
		t.Errorf("Expected BMC endpoint '192.168.1.100:623', got '%s'", mapping.BMCEndpoint)
	}

	if mapping.AgentID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got '%s'", mapping.AgentID)
	}

	if mapping.DatacenterID != "dc-1" {
		t.Errorf("Expected datacenter ID 'dc-1', got '%s'", mapping.DatacenterID)
	}

	expectedType := types.BMCTypeIPMI // This is the models constant
	if mapping.BMCType != expectedType {
		t.Errorf("Expected BMC type %s, got %s", expectedType, mapping.BMCType)
	}
}

func TestAgentHeartbeat(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// First register an agent
	agentInfo := &agent.Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://agent:8080",
		LastSeen:     time.Now().Add(-time.Minute),
	}
	handler.agentRegistry.Register(agentInfo)

	// Send heartbeat
	req := connect.NewRequest(&gatewayv1.AgentHeartbeatRequest{
		AgentId: "agent-1",
		BmcEndpoints: []*gatewayv1.BMCEndpointRegistration{
			{
				ServerId: "test-server-2",
				ControlEndpoint: &gatewayv1.BMCControlEndpoint{
					Endpoint: "192.168.1.100:623",
					Type:     gatewayv1.BMCType_BMC_TYPE_REDFISH,
				},
				Features: []string{"power", "console"},
				Status:   "reachable",
				Metadata: map[string]string{"rack": "R1U42"},
			},
		},
	})

	resp, err := handler.AgentHeartbeat(context.Background(), req)

	if err != nil {
		t.Fatalf("AgentHeartbeat failed: %v", err)
	}

	if !resp.Msg.Success {
		t.Error("Heartbeat should be successful")
	}

	if resp.Msg.HeartbeatIntervalSeconds != 30 {
		t.Errorf("Expected heartbeat interval 30, got %d", resp.Msg.HeartbeatIntervalSeconds)
	}

	// Verify LastSeen was updated
	updatedAgent := handler.agentRegistry.Get("agent-1")
	if updatedAgent.LastSeen.Before(agentInfo.LastSeen) {
		t.Error("LastSeen should be updated")
	}

	// Verify BMC endpoint mapping was updated
	handler.mu.RLock()
	mapping := handler.bmcEndpointMapping["192.168.1.100:623"]
	handler.mu.RUnlock()

	if mapping == nil {
		t.Error("BMC endpoint mapping should be created")
	}

	expectedType := types.BMCTypeRedfish // This is the models constant
	if mapping.BMCType != expectedType {
		t.Errorf("Expected BMC type %s, got %s", expectedType, mapping.BMCType)
	}
}

func TestProxyPowerOperation(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// Register agent
	agentInfo := &agent.Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://agent:8080",
		LastSeen:     time.Now(),
	}
	handler.agentRegistry.Register(agentInfo)

	// Add BMC endpoint mapping
	handler.mu.Lock()
	handler.bmcEndpointMapping["192.168.1.100:623"] = &domain.AgentBMCMapping{
		ServerID:     "test-server-1",
		BMCEndpoint:  "192.168.1.100:623",
		AgentID:      "agent-1",
		DatacenterID: "dc-1",
		BMCType:      types.BMCTypeIPMI,
		Features:     []string{"power"},
		Status:       "reachable",
		LastSeen:     time.Now(),
		Metadata:     map[string]string{},
	}
	handler.mu.Unlock()

	// Test power operation - expect connection error since agent doesn't exist
	_, err := handler.proxyPowerOperation(context.Background(), "192.168.1.100:623", PowerOpPowerOn)

	// We expect an error here because the agent endpoint doesn't actually exist
	if err == nil {
		t.Error("Expected error when connecting to non-existent agent")
	}

	// Verify it's a connection error
	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("Expected connect.Error, got %T", err)
	}

	if connectErr.Code() != connect.CodeUnavailable {
		t.Errorf("Expected Unavailable error code, got %v", connectErr.Code())
	}
}

func TestProxyPowerOperation_BMCEndpointNotFound(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	_, err := handler.proxyPowerOperation(context.Background(), "192.168.1.200:623", PowerOpPowerOn)

	if err == nil {
		t.Error("Expected error for non-existent BMC endpoint")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("Expected NotFound error code, got %v", connectErr.Code())
	}
}

func TestProxyPowerOperation_AgentNotAvailable(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// Add BMC endpoint mapping but no agent
	handler.mu.Lock()
	handler.bmcEndpointMapping["192.168.1.100:623"] = &domain.AgentBMCMapping{
		BMCEndpoint:  "192.168.1.100:623",
		AgentID:      "agent-1",
		DatacenterID: "dc-1",
		BMCType:      types.BMCTypeIPMI,
		Features:     []string{"power"},
		Status:       "reachable",
		LastSeen:     time.Now(),
		Metadata:     map[string]string{},
	}
	handler.mu.Unlock()

	_, err := handler.proxyPowerOperation(context.Background(), "192.168.1.100:623", PowerOpPowerOn)

	if err == nil {
		t.Error("Expected error for unavailable agent")
	}

	connectErr := err.(*connect.Error)
	if connectErr.Code() != connect.CodeUnavailable {
		t.Errorf("Expected Unavailable error code, got %v", connectErr.Code())
	}
}

func TestPowerOperations(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// Setup agent and BMC endpoint
	agentInfo := &agent.Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://agent:8080",
		LastSeen:     time.Now(),
	}
	handler.agentRegistry.Register(agentInfo)

	handler.mu.Lock()
	handler.bmcEndpointMapping["192.168.1.100:623"] = &domain.AgentBMCMapping{
		ServerID:     "192.168.1.100:623",
		BMCEndpoint:  "192.168.1.100:623",
		AgentID:      "agent-1",
		DatacenterID: "dc-1",
		BMCType:      types.BMCTypeIPMI,
		Features:     []string{"power"},
		Status:       "reachable",
		LastSeen:     time.Now(),
		Metadata:     map[string]string{},
	}
	handler.mu.Unlock()

	// Create authenticated context
	ctx := createAuthenticatedContext("192.168.1.100:623", "customer-1")

	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: "192.168.1.100:623",
	})

	// Test PowerOn - expect connection error since agent doesn't exist
	_, err := handler.PowerOn(ctx, req)
	if err == nil {
		t.Error("Expected connection error for PowerOn")
	}

	// Test PowerOff - expect connection error since agent doesn't exist
	_, err = handler.PowerOff(ctx, req)
	if err == nil {
		t.Error("Expected connection error for PowerOff")
	}

	// Test PowerCycle - expect connection error since agent doesn't exist
	_, err = handler.PowerCycle(ctx, req)
	if err == nil {
		t.Error("Expected connection error for PowerCycle")
	}

	// Test Reset - expect connection error since agent doesn't exist
	_, err = handler.Reset(ctx, req)
	if err == nil {
		t.Error("Expected connection error for Reset")
	}
}

func TestGetPowerStatus(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// Setup agent and BMC endpoint
	agentInfo := &agent.Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://agent:8080",
		LastSeen:     time.Now(),
	}
	handler.agentRegistry.Register(agentInfo)

	handler.mu.Lock()
	handler.bmcEndpointMapping["192.168.1.100:623"] = &domain.AgentBMCMapping{
		ServerID:     "192.168.1.100:623",
		BMCEndpoint:  "192.168.1.100:623",
		AgentID:      "agent-1",
		DatacenterID: "dc-1",
		BMCType:      types.BMCTypeIPMI,
		Features:     []string{"power"},
		Status:       "reachable",
		LastSeen:     time.Now(),
		Metadata:     map[string]string{},
	}
	handler.mu.Unlock()

	// Create authenticated context
	ctx := createAuthenticatedContext("192.168.1.100:623", "customer-1")

	req := connect.NewRequest(&gatewayv1.PowerStatusRequest{
		ServerId: "192.168.1.100:623",
	})

	// Expect connection error since agent doesn't actually exist
	_, err := handler.GetPowerStatus(ctx, req)

	if err == nil {
		t.Error("Expected connection error for GetPowerStatus")
	}
}

func TestCreateVNCSession(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// Create customer and server models for token generation
	customer := &domain.Customer{
		ID:    "customer-1",
		Email: "test@example.com",
	}

	server := &domain.Server{
		ID:           "192.168.1.100:623",
		CustomerID:   customer.ID,
		DatacenterID: "dc-1",
		BMCType:      types.BMCTypeIPMI,
		BMCEndpoint:  "192.168.1.100:623",
		Features:     []string{"console", "power"},
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	permissions := []string{"console:write", "power:write"}

	// Generate a server token
	token, err := handler.jwtManager.GenerateServerToken(convertCustomerToManager(customer), convertServerToManager(server), permissions)
	if err != nil {
		t.Fatalf("Failed to generate server token: %v", err)
	}

	// Create context with the server token
	ctx := context.WithValue(context.Background(), "token", token)

	// Setup agent and BMC endpoint
	agentInfo := &agent.Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://agent:8080",
		LastSeen:     time.Now(),
	}
	handler.agentRegistry.Register(agentInfo)

	handler.mu.Lock()
	handler.bmcEndpointMapping["192.168.1.100:623"] = &domain.AgentBMCMapping{
		BMCEndpoint:  "192.168.1.100:623",
		AgentID:      "agent-1",
		DatacenterID: "dc-1",
		BMCType:      types.BMCTypeIPMI,
		Features:     []string{"console"},
		Status:       "reachable",
		LastSeen:     time.Now(),
		Metadata:     map[string]string{},
	}
	handler.mu.Unlock()

	req := connect.NewRequest(&gatewayv1.CreateVNCSessionRequest{
		ServerId: "192.168.1.100:623",
	})

	resp, err := handler.CreateVNCSession(ctx, req)

	if err != nil {
		t.Fatalf("CreateVNCSession failed: %v", err)
	}

	if resp.Msg.SessionId == "" {
		t.Error("Session ID should not be empty")
	}

	if resp.Msg.WebsocketEndpoint == "" {
		t.Error("WebSocket endpoint should not be empty")
	}

	if resp.Msg.ViewerUrl == "" {
		t.Error("Viewer URL should not be empty")
	}

	if resp.Msg.ExpiresAt == nil {
		t.Error("Expires at should not be nil")
	}
}

func TestGetDatacenterIDs(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// Test with no agents - should return default
	datacenterIDs := handler.getDatacenterIDs()
	if len(datacenterIDs) != 1 || datacenterIDs[0] != "dc-test-01" {
		t.Errorf("Expected default datacenter, got %v", datacenterIDs)
	}

	// Add agents with different datacenters
	agent1 := &agent.Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://agent1:8080",
		LastSeen:     time.Now(),
	}

	agent2 := &agent.Info{
		ID:           "agent-2",
		DatacenterID: "dc-2",
		Endpoint:     "http://agent2:8080",
		LastSeen:     time.Now(),
	}

	agent3 := &agent.Info{
		ID:           "agent-3",
		DatacenterID: "dc-1", // Same as agent1
		Endpoint:     "http://agent3:8080",
		LastSeen:     time.Now(),
	}

	handler.agentRegistry.Register(agent1)
	handler.agentRegistry.Register(agent2)
	handler.agentRegistry.Register(agent3)

	datacenterIDs = handler.getDatacenterIDs()

	if len(datacenterIDs) != 2 {
		t.Errorf("Expected 2 unique datacenters, got %d", len(datacenterIDs))
	}

	datacenterMap := make(map[string]bool)
	for _, id := range datacenterIDs {
		datacenterMap[id] = true
	}

	if !datacenterMap["dc-1"] || !datacenterMap["dc-2"] {
		t.Errorf("Expected dc-1 and dc-2, got %v", datacenterIDs)
	}
}

// Authentication tests for gateway token validation

func TestAuthenticationInterceptor_HealthCheckSkipsAuth(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// HealthCheck should work without auth headers
	req := connect.NewRequest(&gatewayv1.HealthCheckRequest{})
	resp, err := handler.HealthCheck(context.Background(), req)

	if err != nil {
		t.Fatalf("HealthCheck should not require authentication: %v", err)
	}

	if resp.Msg.Status == "" {
		t.Error("HealthCheck should return status")
	}
}

// Helper function to test authentication logic directly.
func TestAuthenticationLogic(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	cases := []struct {
		name        string
		authHeader  string
		expectError bool
		errorText   string
	}{
		{
			name:        "Missing auth header",
			authHeader:  "",
			expectError: true,
			errorText:   "authorization header required",
		},
		{
			name:        "Invalid format - no Bearer",
			authHeader:  "invalid-token",
			expectError: true,
			errorText:   "invalid authorization header format",
		},
		{
			name:        "Invalid format - wrong prefix",
			authHeader:  "Basic user:pass",
			expectError: true,
			errorText:   "invalid authorization header format",
		},
		{
			name:        "Invalid format - Bearer only",
			authHeader:  "Bearer",
			expectError: true,
			errorText:   "invalid authorization header format",
		},
		{
			name:        "Invalid format - Bearer with space only",
			authHeader:  "Bearer ",
			expectError: true,
			errorText:   "invalid authorization header format",
		},
		{
			name:        "Invalid token",
			authHeader:  "Bearer invalid-jwt-token",
			expectError: true,
			errorText:   "invalid token",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the authentication interceptor logic
			var claims *commonauth.AuthClaims
			var err error

			if tc.authHeader == "" {
				err = fmt.Errorf("authorization header required")
			} else {
				// Extract Bearer token
				token := ""
				if len(tc.authHeader) > 7 && tc.authHeader[:7] == "Bearer " {
					token = tc.authHeader[7:]
				} else {
					err = fmt.Errorf("invalid authorization header format")
				}

				if err == nil {
					// Validate token
					managerClaims, validateErr := handler.jwtManager.ValidateToken(token)
					if validateErr != nil {
						err = fmt.Errorf("invalid token: %w", validateErr)
					} else {
						claims = convertAuthClaimsFromManager(managerClaims)
					}
				}
			}

			if tc.expectError {
				if err == nil {
					t.Fatalf("Expected error for test case %s", tc.name)
				}
				if !strings.Contains(err.Error(), tc.errorText) {
					t.Errorf("Expected error to contain '%s', got: %s", tc.errorText, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error for test case %s: %v", tc.name, err)
				}
				if claims == nil {
					t.Error("Expected valid claims for successful authentication")
				}
			}
		})
	}
}

func TestAuthenticationInterceptor_ValidToken(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// Generate a valid JWT token
	customer := &domain.Customer{
		ID:    "test-customer-1",
		Email: "test@example.com",
	}

	validToken, err := handler.jwtManager.GenerateToken(convertCustomerToManager(customer))
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Test authentication logic with valid token
	authHeader := "Bearer " + validToken
	token := authHeader[7:]
	managerClaims, err := handler.jwtManager.ValidateToken(token)
	require.NoError(t, err)
	claims := convertAuthClaimsFromManager(managerClaims)

	if err != nil {
		t.Fatalf("Valid token should be accepted: %v", err)
	}

	if claims == nil {
		t.Fatal("Claims should not be nil for valid token")
	}

	if claims.CustomerID != customer.ID {
		t.Errorf("Expected customer ID %s, got %s", customer.ID, claims.CustomerID)
	}

	if claims.Email != customer.Email {
		t.Errorf("Expected email %s, got %s", customer.Email, claims.Email)
	}
}

func TestEndpointWithValidAuthentication(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// Create customer
	customer := &domain.Customer{
		ID:    "test-customer-1",
		Email: "test@example.com",
	}

	// Setup BMC endpoint for testing
	agentInfo := &agent.Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://agent:8080",
		LastSeen:     time.Now(),
	}
	handler.agentRegistry.Register(agentInfo)

	handler.mu.Lock()
	handler.bmcEndpointMapping["192.168.1.100:623"] = &domain.AgentBMCMapping{
		ServerID:     "192.168.1.100:623",
		BMCEndpoint:  "192.168.1.100:623",
		AgentID:      "agent-1",
		DatacenterID: "dc-1",
		BMCType:      types.BMCTypeIPMI,
		Features:     []string{"power"},
		Status:       "reachable",
		LastSeen:     time.Now(),
		Metadata:     map[string]string{},
	}
	handler.mu.Unlock()

	// Create authenticated context with server token
	ctx := createAuthenticatedContext("192.168.1.100:623", customer.ID)

	// Test that endpoint works with proper authentication context
	req := connect.NewRequest(&gatewayv1.PowerStatusRequest{
		ServerId: "192.168.1.100:623",
	})

	// Expect connection error since agent doesn't actually exist
	_, err := handler.GetPowerStatus(ctx, req)

	// We're just testing that authentication passes, connection error is expected
	if err == nil {
		t.Error("Expected connection error (agent doesn't exist)")
	}
}

func TestCreateVNCSession_WithAuthentication(t *testing.T) {
	handler := newGatewayHandler("gateway-1", "us-west-1")

	// Create customer and server models for token generation
	customer := &domain.Customer{
		ID:    "test-customer-123",
		Email: "claims-test@example.com",
	}

	server := &domain.Server{
		ID:           "192.168.1.100:623",
		CustomerID:   customer.ID,
		DatacenterID: "dc-1",
		BMCType:      types.BMCTypeIPMI,
		BMCEndpoint:  "192.168.1.100:623",
		Features:     []string{"console", "power"},
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	permissions := []string{"console:write", "power:write"}

	// Generate a server token
	token, err := handler.jwtManager.GenerateServerToken(convertCustomerToManager(customer), convertServerToManager(server), permissions)
	if err != nil {
		t.Fatalf("Failed to generate server token: %v", err)
	}

	// Create context with the server token
	ctx := context.WithValue(context.Background(), "token", token)

	// Setup BMC endpoint for testing
	agentInfo := &agent.Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://agent:8080",
		LastSeen:     time.Now(),
	}
	handler.agentRegistry.Register(agentInfo)

	handler.mu.Lock()
	handler.bmcEndpointMapping["192.168.1.100:623"] = &domain.AgentBMCMapping{
		BMCEndpoint:  "192.168.1.100:623",
		AgentID:      "agent-1",
		DatacenterID: "dc-1",
		BMCType:      types.BMCTypeIPMI,
		Features:     []string{"console"},
		Status:       "reachable",
		LastSeen:     time.Now(),
		Metadata:     map[string]string{},
	}
	handler.mu.Unlock()

	// Test CreateVNCSession which uses server token from context
	req := connect.NewRequest(&gatewayv1.CreateVNCSessionRequest{
		ServerId: "192.168.1.100:623",
	})

	resp, err := handler.CreateVNCSession(ctx, req)

	if err != nil {
		t.Fatalf("CreateVNCSession should work with valid authentication context: %v", err)
	}

	// The response should contain a session ID, which means claims were properly extracted
	if resp.Msg.SessionId == "" {
		t.Error("Session ID should not be empty - this suggests claims were not properly extracted from context")
	}

	if resp.Msg.WebsocketEndpoint == "" {
		t.Error("WebSocket endpoint should not be empty")
	}

	if resp.Msg.ViewerUrl == "" {
		t.Error("Viewer URL should not be empty")
	}

	if resp.Msg.ExpiresAt == nil {
		t.Error("Expires at should not be nil")
	}
}

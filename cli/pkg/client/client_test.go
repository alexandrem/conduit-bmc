package client

import (
	"context"
	"testing"

	"cli/pkg/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{}
	client := New(cfg)

	if client == nil {
		t.Fatal("New returned nil client")
	}

	if client.config != cfg {
		t.Error("Config not set correctly")
	}

	if client.httpClient == nil {
		t.Error("HTTP client should not be nil")
	}

	if client.managerClient == nil {
		t.Error("Manager client should not be nil")
	}

	if client.gatewayCache == nil {
		t.Error("Gateway cache should not be nil")
	}

	if len(client.gatewayCache) != 0 {
		t.Error("Gateway cache should be empty initially")
	}
}

func TestServerInfo(t *testing.T) {
	server := ServerInfo{
		ID: "server-1",
		ControlEndpoint: &BMCControlEndpoint{
			Type: "ipmi",
		},
		Features:     []string{"power", "console"},
		Status:       "active",
		DatacenterID: "dc-1",
	}

	if server.ID != "server-1" {
		t.Errorf("Expected ID 'server-1', got '%s'", server.ID)
	}

	if server.ControlEndpoint.Type != "ipmi" {
		t.Errorf("Expected BMC type 'ipmi', got '%s'", server.ControlEndpoint.Type)
	}

	if len(server.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(server.Features))
	}

	if server.Features[0] != "power" || server.Features[1] != "console" {
		t.Errorf("Features not set correctly: %v", server.Features)
	}

	if server.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", server.Status)
	}

	if server.DatacenterID != "dc-1" {
		t.Errorf("Expected datacenter ID 'dc-1', got '%s'", server.DatacenterID)
	}
}

func TestProxySession(t *testing.T) {
	session := ProxySession{
		ID:        "session-1",
		Endpoint:  "http://proxy:8080",
		ExpiresAt: "2024-01-01T00:00:00Z",
	}

	if session.ID != "session-1" {
		t.Errorf("Expected ID 'session-1', got '%s'", session.ID)
	}

	if session.Endpoint != "http://proxy:8080" {
		t.Errorf("Expected endpoint 'http://proxy:8080', got '%s'", session.Endpoint)
	}

	if session.ExpiresAt != "2024-01-01T00:00:00Z" {
		t.Errorf("Expected expires at '2024-01-01T00:00:00Z', got '%s'", session.ExpiresAt)
	}
}

func TestVNCSession(t *testing.T) {
	session := VNCSession{
		ID:                "vnc-session-1",
		WebsocketEndpoint: "wss://gateway:8081/vnc/session-1",
		ViewerURL:         "https://gateway:8081/vnc/session-1",
		ExpiresAt:         "2024-01-01T01:00:00Z",
	}

	if session.ID != "vnc-session-1" {
		t.Errorf("Expected ID 'vnc-session-1', got '%s'", session.ID)
	}

	if session.WebsocketEndpoint != "wss://gateway:8081/vnc/session-1" {
		t.Errorf("Expected WebSocket endpoint 'wss://gateway:8081/vnc/session-1', got '%s'", session.WebsocketEndpoint)
	}

	if session.ViewerURL != "https://gateway:8081/vnc/session-1" {
		t.Errorf("Expected viewer URL 'https://gateway:8081/vnc/session-1', got '%s'", session.ViewerURL)
	}

	if session.ExpiresAt != "2024-01-01T01:00:00Z" {
		t.Errorf("Expected expires at '2024-01-01T01:00:00Z', got '%s'", session.ExpiresAt)
	}
}

func TestClient_AuthenticateError(t *testing.T) {
	cfg := &config.Config{
		Manager: config.ManagerConfig{
			Endpoint: "http://invalid:9999",
		},
	}

	client := New(cfg)
	ctx := context.Background()

	err := client.Authenticate(ctx, "invalid@example.com", "wrongpassword")
	if err == nil {
		t.Error("Expected authentication to fail with invalid credentials")
	}
}

func TestClient_ListServersNoGateways(t *testing.T) {
	cfg := &config.Config{
		Manager: config.ManagerConfig{
			Endpoint: "http://invalid:9999",
		},
	}

	client := New(cfg)
	ctx := context.Background()

	// This should fail due to invalid manager endpoint
	_, err := client.ListServers(ctx)
	if err == nil {
		t.Error("Expected ListServers to fail with invalid manager endpoint")
	}
}

func TestClient_GetServerError(t *testing.T) {
	cfg := &config.Config{
		Manager: config.ManagerConfig{
			Endpoint: "http://invalid:9999",
		},
	}

	client := New(cfg)
	ctx := context.Background()

	// This should fail due to invalid manager endpoint
	_, err := client.GetServer(ctx, "server-1")
	if err == nil {
		t.Error("Expected GetServer to fail with invalid manager endpoint")
	}
}

func TestClient_PowerOperationsError(t *testing.T) {
	cfg := &config.Config{
		Manager: config.ManagerConfig{
			Endpoint: "http://invalid:9999",
		},
	}

	client := New(cfg)
	ctx := context.Background()

	testCases := []struct {
		name      string
		operation func() error
	}{
		{
			name:      "PowerOn",
			operation: func() error { return client.PowerOn(ctx, "server-1") },
		},
		{
			name:      "PowerOff",
			operation: func() error { return client.PowerOff(ctx, "server-1") },
		},
		{
			name:      "PowerCycle",
			operation: func() error { return client.PowerCycle(ctx, "server-1") },
		},
		{
			name:      "Reset",
			operation: func() error { return client.Reset(ctx, "server-1") },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.operation()
			if err == nil {
				t.Errorf("Expected %s to fail with invalid manager endpoint", tc.name)
			}
		})
	}
}

func TestClient_GetPowerStatusError(t *testing.T) {
	cfg := &config.Config{
		Manager: config.ManagerConfig{
			Endpoint: "http://invalid:9999",
		},
	}

	client := New(cfg)
	ctx := context.Background()

	_, err := client.GetPowerStatus(ctx, "server-1")
	if err == nil {
		t.Error("Expected GetPowerStatus to fail with invalid manager endpoint")
	}
}

func TestClient_VNCSessionOperationsError(t *testing.T) {
	cfg := &config.Config{
		Manager: config.ManagerConfig{
			Endpoint: "http://invalid:9999",
		},
	}

	client := New(cfg)
	ctx := context.Background()

	// Test CreateVNCSession
	_, err := client.CreateVNCSession(ctx, "server-1")
	if err == nil {
		t.Error("Expected CreateVNCSession to fail with invalid manager endpoint")
	}

	// Test GetVNCSession with empty cache
	_, err = client.GetVNCSession(ctx, "session-1")
	if err == nil {
		t.Error("Expected GetVNCSession to fail with empty gateway cache")
	}

	// Test CloseVNCSession with empty cache
	err = client.CloseVNCSession(ctx, "session-1")
	if err == nil {
		t.Error("Expected CloseVNCSession to fail with empty gateway cache")
	}
}

func TestClient_GatewayCacheManagement(t *testing.T) {
	cfg := &config.Config{}
	client := New(cfg)

	// Verify initial state
	if len(client.gatewayCache) != 0 {
		t.Error("Gateway cache should be empty initially")
	}

	// Manually add a gateway client to cache for testing
	endpoint := "http://gateway.example.com:8081"
	gatewayClient := NewRegionalGatewayClient(cfg, endpoint, "test-token")
	client.gatewayCache[endpoint] = gatewayClient

	if len(client.gatewayCache) != 1 {
		t.Error("Gateway cache should contain one entry")
	}

	// Verify we can retrieve from cache
	cached, exists := client.gatewayCache[endpoint]
	if !exists {
		t.Error("Gateway client should exist in cache")
	}

	if cached != gatewayClient {
		t.Error("Cached client should be the same instance")
	}
}

func TestClient_VNCSessionWithMockGateways(t *testing.T) {
	cfg := &config.Config{}
	client := New(cfg)

	// Add mock gateway clients to cache
	endpoint1 := "http://gateway1.example.com:8081"
	endpoint2 := "http://gateway2.example.com:8081"

	client.gatewayCache[endpoint1] = NewRegionalGatewayClient(cfg, endpoint1, "token1")
	client.gatewayCache[endpoint2] = NewRegionalGatewayClient(cfg, endpoint2, "token2")

	ctx := context.Background()

	// Test GetVNCSession - should try all gateways
	_, err := client.GetVNCSession(ctx, "non-existent-session")
	if err == nil {
		t.Error("Expected GetVNCSession to fail for non-existent session")
	}
	if err.Error() != "VNC session not found: non-existent-session" {
		t.Errorf("Expected specific error message, got: %v", err)
	}

	// Test CloseVNCSession - should try all gateways
	err = client.CloseVNCSession(ctx, "non-existent-session")
	if err == nil {
		t.Error("Expected CloseVNCSession to fail for non-existent session")
	}
	if err.Error() != "VNC session not found: non-existent-session" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

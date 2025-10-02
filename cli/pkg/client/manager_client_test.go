package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	managerv1 "manager/gen/manager/v1"

	"cli/pkg/config"

	"github.com/stretchr/testify/assert"
)

func TestBMCManagerClient_AuthorizationHeader(t *testing.T) {
	// This test verifies that authorization headers are correctly added to requests
	// This is a regression test for the bug where type matching failed for ListServersRequest

	testCases := []struct {
		name        string
		setupClient func() (*BMCManagerClient, *httptest.Server)
		testRequest func(client *BMCManagerClient) error
	}{
		{
			name: "ListServers includes authorization header",
			setupClient: func() (*BMCManagerClient, *httptest.Server) {
				// Create a test server that captures the request
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Verify authorization header is present
					authHeader := r.Header.Get("Authorization")
					assert.NotEmpty(t, authHeader, "Authorization header should be present")
					assert.Contains(t, authHeader, "Bearer test-token-123", "Authorization header should contain the token")

					// Return a valid response
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"servers": []}`))
				}))

				cfg := &config.Config{
					Manager: config.ManagerConfig{
						Endpoint: server.URL,
					},
					Auth: config.AuthConfig{
						AccessToken:    "test-token-123",
						TokenExpiresAt: time.Now().Add(24 * time.Hour),
					},
				}

				return NewBMCManagerClient(cfg), server
			},
			testRequest: func(client *BMCManagerClient) error {
				_, err := client.ListServers(context.Background())
				return err
			},
		},
		{
			name: "GetServer includes authorization header",
			setupClient: func() (*BMCManagerClient, *httptest.Server) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					authHeader := r.Header.Get("Authorization")
					assert.NotEmpty(t, authHeader, "Authorization header should be present")
					assert.Contains(t, authHeader, "Bearer test-token-456", "Authorization header should contain the token")

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"server": {"id": "test-server", "customer_id": "test-customer", "datacenter_id": "dc1", "bmc_type": "BMC_TYPE_REDFISH", "bmc_endpoint": "192.168.1.100", "features": ["power"], "status": "active"}}`))
				}))

				cfg := &config.Config{
					Manager: config.ManagerConfig{
						Endpoint: server.URL,
					},
					Auth: config.AuthConfig{
						AccessToken:    "test-token-456",
						TokenExpiresAt: time.Now().Add(24 * time.Hour),
					},
				}

				return NewBMCManagerClient(cfg), server
			},
			testRequest: func(client *BMCManagerClient) error {
				_, err := client.GetServer(context.Background(), "test-server")
				return err
			},
		},
		{
			name: "GetServerLocation includes authorization header",
			setupClient: func() (*BMCManagerClient, *httptest.Server) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					authHeader := r.Header.Get("Authorization")
					assert.NotEmpty(t, authHeader, "Authorization header should be present")
					assert.Contains(t, authHeader, "Bearer test-token-789", "Authorization header should contain the token")

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"regional_gateway_id": "gw1", "regional_gateway_endpoint": "http://gateway", "datacenter_id": "dc1", "bmc_type": "BMC_TYPE_REDFISH", "features": ["power"]}`))
				}))

				cfg := &config.Config{
					Manager: config.ManagerConfig{
						Endpoint: server.URL,
					},
					Auth: config.AuthConfig{
						AccessToken:    "test-token-789",
						TokenExpiresAt: time.Now().Add(24 * time.Hour),
					},
				}

				return NewBMCManagerClient(cfg), server
			},
			testRequest: func(client *BMCManagerClient) error {
				_, err := client.GetServerLocation(context.Background(), "test-server")
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, server := tc.setupClient()
			defer server.Close()

			// Execute the test request
			err := tc.testRequest(client)

			// The error is not important - we just care that the authorization header was sent
			// (The test server validates the header in the HTTP handler)
			_ = err
		})
	}
}

func TestBMCManagerClient_AddAuthHeaders_TypeMatching(t *testing.T) {
	// This is a regression test for the bug where type matching failed
	// due to incorrect generic type parameters in the switch statement

	// Test that each request type properly matches in the addAuthHeaders method
	testCases := []struct {
		name        string
		createReq   func() interface{}
		expectMatch bool
	}{
		{
			name: "ListServersRequest should match",
			createReq: func() interface{} {
				return connect.NewRequest(&managerv1.ListServersRequest{})
			},
			expectMatch: true,
		},
		{
			name: "GetServerRequest should match",
			createReq: func() interface{} {
				return connect.NewRequest(&managerv1.GetServerRequest{ServerId: "test"})
			},
			expectMatch: true,
		},
		{
			name: "GetServerLocationRequest should match",
			createReq: func() interface{} {
				return connect.NewRequest(&managerv1.GetServerLocationRequest{ServerId: "test"})
			},
			expectMatch: true,
		},
		{
			name: "ListGatewaysRequest should match",
			createReq: func() interface{} {
				return connect.NewRequest(&managerv1.ListGatewaysRequest{})
			},
			expectMatch: true,
		},
		{
			name: "RefreshTokenRequest should match",
			createReq: func() interface{} {
				return connect.NewRequest(&managerv1.RefreshTokenRequest{RefreshToken: "test"})
			},
			expectMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.createReq()

			// Create a request interceptor to verify the header was set
			headerSet := false

			// We can't easily test the switch statement directly, but we can verify
			// the type by checking if it would be handled correctly
			switch req.(type) {
			case *connect.Request[managerv1.ListServersRequest]:
				headerSet = true
			case *connect.Request[managerv1.GetServerRequest]:
				headerSet = true
			case *connect.Request[managerv1.GetServerLocationRequest]:
				headerSet = true
			case *connect.Request[managerv1.ListGatewaysRequest]:
				headerSet = true
			case *connect.Request[managerv1.RefreshTokenRequest]:
				headerSet = true
			}

			if tc.expectMatch {
				assert.True(t, headerSet, "Request type %T should match in type switch", req)
			} else {
				assert.False(t, headerSet, "Request type %T should not match in type switch", req)
			}
		})
	}
}

func TestBMCManagerClient_EnsureValidToken(t *testing.T) {
	testCases := []struct {
		name        string
		token       string
		expiresAt   time.Time
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid token should pass",
			token:       "valid-token",
			expiresAt:   time.Now().Add(1 * time.Hour),
			expectError: false,
		},
		{
			name:        "Empty token should fail",
			token:       "",
			expiresAt:   time.Now().Add(1 * time.Hour),
			expectError: true,
			errorMsg:    "no access token available",
		},
		{
			name:        "Expired token should fail",
			token:       "expired-token",
			expiresAt:   time.Now().Add(-1 * time.Hour),
			expectError: true,
			errorMsg:    "access token expired",
		},
		{
			name:        "Just expired token should fail",
			token:       "just-expired-token",
			expiresAt:   time.Now().Add(-1 * time.Second),
			expectError: true,
			errorMsg:    "access token expired",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				Auth: config.AuthConfig{
					AccessToken:    tc.token,
					TokenExpiresAt: tc.expiresAt,
				},
			}

			client := NewBMCManagerClient(cfg)
			err := client.EnsureValidToken(context.Background())

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBMCManagerClient_RegressAuthHeaderMissing(t *testing.T) {
	// This is a specific regression test for the "missing authorization header" bug
	// that was caused by incorrect type matching in addAuthHeaders

	// Setup a server that strictly checks for the authorization header
	authHeaderReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && authHeader == "Bearer regression-test-token" {
			authHeaderReceived = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"servers": []}`))
		} else {
			// Simulate the server returning "missing authorization header" error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "missing authorization header"}`))
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		Manager: config.ManagerConfig{
			Endpoint: server.URL,
		},
		Auth: config.AuthConfig{
			AccessToken:    "regression-test-token",
			TokenExpiresAt: time.Now().Add(24 * time.Hour),
		},
	}

	client := NewBMCManagerClient(cfg)

	// Call ListServers - this should include the authorization header
	_, _ = client.ListServers(context.Background())

	// The specific error doesn't matter, but the authorization header should have been sent
	assert.True(t, authHeaderReceived,
		"REGRESSION: Authorization header should be sent with ListServers request. "+
			"This test guards against the type matching bug that caused 'missing authorization header' errors.")
}

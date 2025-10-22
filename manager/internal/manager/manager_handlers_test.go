package manager

import (
	"context"
	"testing"
	"time"

	"core/domain"
	commonv1 "core/gen/common/v1"
	"core/types"
	managerv1 "manager/gen/manager/v1"
	"manager/internal/database"
	"manager/pkg/auth"
	"manager/pkg/models"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestHandler(t *testing.T) *BMCManagerServiceHandler {
	t.Helper()

	// Create in-memory database for testing
	db, err := database.New(":memory:")
	require.NoError(t, err)

	// Create JWT manager with test secret
	jwtManager := auth.NewJWTManager("test-secret-key")

	return NewBMCManagerServiceHandler(db, jwtManager)
}

func setupTestGateway(t *testing.T, handler *BMCManagerServiceHandler) *models.RegionalGateway {
	t.Helper()

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
	err := handler.db.Gateways.Create(context.Background(), gateway)
	require.NoError(t, err)

	return gateway
}

// setupTestCustomer creates a test customer with default values
func setupTestCustomer(t *testing.T, id string) *models.Customer {
	t.Helper()

	if id == "" {
		id = "test-customer"
	}
	return &models.Customer{
		ID:        id,
		Email:     id + "@example.com",
		APIKey:    "test-api-key-" + id,
		CreatedAt: time.Now(),
	}
}

// setupAuthenticatedContext creates a context with JWT claims for testing
func setupAuthenticatedContext(t *testing.T, handler *BMCManagerServiceHandler, customer *models.Customer) context.Context {
	t.Helper()

	token, err := handler.jwtManager.GenerateToken(customer)
	require.NoError(t, err)

	claims, err := handler.jwtManager.ValidateToken(token)
	require.NoError(t, err)

	return context.WithValue(context.Background(), "claims", claims)
}

// setupCustomerContext creates a simple context with customer_id for testing
func setupCustomerContext(customerID string) context.Context {
	// Note: t.Helper() not needed here as this doesn't call any test assertions
	return context.WithValue(context.Background(), "customer_id", customerID)
}

func TestListGateways_GeneratesDelegatedTokens(t *testing.T) {
	handler := setupTestHandler(t)

	// Setup test gateway and customer
	setupTestGateway(t, handler)
	customer := setupTestCustomer(t, "test-customer-1")

	// Create authenticated context
	ctx := setupCustomerContext(customer.ID)
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

	// Setup test gateway and customer
	setupTestGateway(t, handler)
	customer := setupTestCustomer(t, "test-customer-1")

	// Create authenticated context
	ctx := setupCustomerContext(customer.ID)
	ctx = context.WithValue(ctx, "customer_email", customer.Email)

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
	customer := setupTestCustomer(t, "test-customer-1")

	// Create authenticated context
	ctx := setupCustomerContext(customer.ID)
	ctx = context.WithValue(ctx, "customer_email", customer.Email)

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

	err := handler.db.Gateways.Create(context.Background(), gateway1)
	require.NoError(t, err)
	err = handler.db.Gateways.Create(context.Background(), gateway2)
	require.NoError(t, err)

	customer := setupTestCustomer(t, "test-customer-1")

	// Create authenticated context
	ctx := setupCustomerContext(customer.ID)
	ctx = context.WithValue(ctx, "customer_email", customer.Email)

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

// TestReportAvailableEndpoints_PopulatesSOLAndVNCEndpoints tests that SOL and VNC
// endpoints are correctly populated when servers have "sol", "console", "vnc", or "kvm" features
func TestReportAvailableEndpoints_PopulatesSOLAndVNCEndpoints(t *testing.T) {
	handler := setupTestHandler(t)
	gateway := setupTestGateway(t, handler)

	testCases := []struct {
		name            string
		bmcType         commonv1.BMCType
		features        []string
		expectSOL       bool
		expectVNC       bool
		expectedSOLType types.SOLType
		expectedVNCType types.VNCType
	}{
		{
			name:    "IPMI with console and VNC features",
			bmcType: commonv1.BMCType_BMC_IPMI,
			features: types.FeaturesToStrings([]types.Feature{
				types.FeaturePower,
				types.FeatureSensors,
				types.FeatureConsole,
				types.FeatureVNC,
			}),
			expectSOL:       true,
			expectVNC:       true,
			expectedSOLType: types.SOLTypeIPMI,
			expectedVNCType: types.VNCTypeNative,
		},
		{
			name:    "Redfish with console and VNC features",
			bmcType: commonv1.BMCType_BMC_REDFISH,
			features: types.FeaturesToStrings([]types.Feature{
				types.FeaturePower,
				types.FeatureConsole,
				types.FeatureVNC,
			}),
			expectSOL:       true,
			expectVNC:       true,
			expectedSOLType: types.SOLTypeRedfishSerial,
			expectedVNCType: types.VNCTypeNative,
		},
		{
			name:    "IPMI with only power features",
			bmcType: commonv1.BMCType_BMC_IPMI,
			features: types.FeaturesToStrings([]types.Feature{
				types.FeaturePower,
				types.FeatureSensors,
			}),
			expectSOL: false,
			expectVNC: false,
		},
		{
			name:    "IPMI with only console",
			bmcType: commonv1.BMCType_BMC_IPMI,
			features: types.FeaturesToStrings([]types.Feature{
				types.FeaturePower,
				types.FeatureConsole,
			}),
			expectSOL:       true,
			expectVNC:       false,
			expectedSOLType: types.SOLTypeIPMI,
		},
		{
			name:    "IPMI with only VNC",
			bmcType: commonv1.BMCType_BMC_IPMI,
			features: types.FeaturesToStrings([]types.Feature{
				types.FeaturePower,
				types.FeatureVNC,
			}),
			expectSOL:       false,
			expectVNC:       true,
			expectedVNCType: types.VNCTypeNative,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test BMC endpoint
			bmcEndpoint := &managerv1.BMCEndpointAvailability{
				BmcEndpoint:  "192.168.1.100:623",
				AgentId:      "test-agent-1",
				DatacenterId: "dc-test-01",
				BmcType:      tc.bmcType,
				Features:     tc.features,
				Status:       "active",
				Username:     "admin",
				Capabilities: types.CapabilitiesToStrings([]types.Capability{
					types.CapabilityIPMIChassis,
				}),
			}

			// Report endpoints to manager
			req := connect.NewRequest(&managerv1.ReportAvailableEndpointsRequest{
				GatewayId:    gateway.ID,
				Region:       gateway.Region,
				BmcEndpoints: []*managerv1.BMCEndpointAvailability{bmcEndpoint},
			})

			resp, err := handler.ReportAvailableEndpoints(context.Background(), req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.True(t, resp.Msg.Success)

			// Retrieve the server and verify endpoints
			serverID := models.GenerateServerIDFromBMCEndpoint(bmcEndpoint.DatacenterId, bmcEndpoint.BmcEndpoint)
			server, err := handler.db.Servers.Get(context.Background(), serverID)
			require.NoError(t, err)
			require.NotNil(t, server)

			// Verify SOL endpoint
			if tc.expectSOL {
				require.NotNil(t, server.SOLEndpoint, "SOL endpoint should be populated")
				assert.Equal(t, tc.expectedSOLType, server.SOLEndpoint.Type)
				assert.Equal(t, bmcEndpoint.BmcEndpoint, server.SOLEndpoint.Endpoint)
				assert.Equal(t, bmcEndpoint.Username, server.SOLEndpoint.Username)
			} else {
				assert.Nil(t, server.SOLEndpoint, "SOL endpoint should not be populated")
			}

			// Verify VNC endpoint
			if tc.expectVNC {
				require.NotNil(t, server.VNCEndpoint, "VNC endpoint should be populated")
				assert.Equal(t, tc.expectedVNCType, server.VNCEndpoint.Type)
				assert.Equal(t, bmcEndpoint.BmcEndpoint, server.VNCEndpoint.Endpoint)
				assert.Equal(t, bmcEndpoint.Username, server.VNCEndpoint.Username)
			} else {
				assert.Nil(t, server.VNCEndpoint, "VNC endpoint should not be populated")
			}
		})
	}
}

// TestListServers_ReturnsSOLAndVNCEndpoints tests that ListServers correctly returns
// SOL and VNC endpoint information
func TestListServers_ReturnsSOLAndVNCEndpoints(t *testing.T) {
	handler := setupTestHandler(t)
	gateway := setupTestGateway(t, handler)

	// Create test customer and authenticated context
	customer := setupTestCustomer(t, "")
	ctx := setupAuthenticatedContext(t, handler, customer)

	// Report a server with console and VNC features
	bmcEndpoint := &managerv1.BMCEndpointAvailability{
		BmcEndpoint:  "192.168.1.100:623",
		AgentId:      "test-agent-1",
		DatacenterId: "dc-test-01",
		BmcType:      commonv1.BMCType_BMC_IPMI,
		Features: types.FeaturesToStrings([]types.Feature{
			types.FeaturePower,
			types.FeatureConsole,
			types.FeatureVNC,
			types.FeatureSensors,
		}),
		Status:   "active",
		Username: "admin",
		Capabilities: types.CapabilitiesToStrings([]types.Capability{
			types.CapabilityIPMISEL,
			types.CapabilityIPMISDR,
			types.CapabilityIPMIFRU,
		}),
	}

	reportReq := connect.NewRequest(&managerv1.ReportAvailableEndpointsRequest{
		GatewayId:    gateway.ID,
		Region:       gateway.Region,
		BmcEndpoints: []*managerv1.BMCEndpointAvailability{bmcEndpoint},
	})

	_, err := handler.ReportAvailableEndpoints(context.Background(), reportReq)
	require.NoError(t, err)

	// List servers
	listReq := connect.NewRequest(&managerv1.ListServersRequest{})
	resp, err := handler.ListServers(ctx, listReq)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Msg.Servers, 1)

	server := resp.Msg.Servers[0]

	// Verify SOL endpoint is returned
	require.NotNil(t, server.SolEndpoint, "SOL endpoint should be included in response")
	assert.Equal(t, commonv1.SOLType_SOL_IPMI, server.SolEndpoint.Type)
	assert.Equal(t, bmcEndpoint.BmcEndpoint, server.SolEndpoint.Endpoint)
	assert.Equal(t, bmcEndpoint.Username, server.SolEndpoint.Username)

	// Verify VNC endpoint is returned
	require.NotNil(t, server.VncEndpoint, "VNC endpoint should be included in response")
	assert.Equal(t, commonv1.VNCType_VNC_NATIVE, server.VncEndpoint.Type)
	assert.Equal(t, bmcEndpoint.BmcEndpoint, server.VncEndpoint.Endpoint)
	assert.Equal(t, bmcEndpoint.Username, server.VncEndpoint.Username)
}

// TestRegisterServer_PopulatesSOLAndVNCEndpoints tests the RegisterServer RPC method
// correctly populates SOL and VNC endpoints from features
func TestRegisterServer_PopulatesSOLAndVNCEndpoints(t *testing.T) {
	handler := setupTestHandler(t)

	// Create test customer and context
	customer := setupTestCustomer(t, "")
	ctx := setupCustomerContext(customer.ID)

	testCases := []struct {
		name            string
		bmcType         commonv1.BMCType
		features        []string
		expectSOL       bool
		expectVNC       bool
		expectedSOLType types.SOLType
	}{
		{
			name:    "IPMI with console and VNC features",
			bmcType: commonv1.BMCType_BMC_IPMI,
			features: types.FeaturesToStrings([]types.Feature{
				types.FeaturePower,
				types.FeatureConsole,
				types.FeatureVNC,
				types.FeatureSensors,
			}),
			expectSOL:       true,
			expectVNC:       true,
			expectedSOLType: types.SOLTypeIPMI,
		},
		{
			name:    "Redfish with console and VNC features",
			bmcType: commonv1.BMCType_BMC_REDFISH,
			features: types.FeaturesToStrings([]types.Feature{
				types.FeaturePower,
				types.FeatureConsole,
				types.FeatureVNC,
				types.FeatureSensors,
			}),
			expectSOL:       true,
			expectVNC:       true,
			expectedSOLType: types.SOLTypeRedfishSerial,
		},
		{
			name:    "IPMI without console features",
			bmcType: commonv1.BMCType_BMC_IPMI,
			features: types.FeaturesToStrings([]types.Feature{
				types.FeaturePower,
				types.FeatureSensors,
			}),
			expectSOL: false,
			expectVNC: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverID := tc.name
			bmcEndpoint := "192.168.1." + string(rune(100+i)) + ":623"

			req := connect.NewRequest(&managerv1.RegisterServerRequest{
				ServerId:          serverID,
				CustomerId:        customer.ID,
				DatacenterId:      "dc-test-01",
				RegionalGatewayId: "gateway-1",
				BmcProtocols: []*commonv1.BMCControlEndpoint{
					{
						Endpoint: bmcEndpoint,
						Type:     tc.bmcType,
					},
				},
				PrimaryProtocol: tc.bmcType,
				Features:        tc.features,
			})

			resp, err := handler.RegisterServer(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.True(t, resp.Msg.Success)

			// Verify the server was created with correct endpoints
			server, err := handler.db.Servers.Get(context.Background(), serverID)
			require.NoError(t, err)
			require.NotNil(t, server)

			if tc.expectSOL {
				require.NotNil(t, server.SOLEndpoint, "SOL endpoint should be populated")
				assert.Equal(t, tc.expectedSOLType, server.SOLEndpoint.Type)
				assert.Equal(t, bmcEndpoint, server.SOLEndpoint.Endpoint)
			} else {
				assert.Nil(t, server.SOLEndpoint, "SOL endpoint should not be populated")
			}

			if tc.expectVNC {
				require.NotNil(t, server.VNCEndpoint, "VNC endpoint should be populated")
				assert.Equal(t, types.VNCTypeNative, server.VNCEndpoint.Type)
				assert.Equal(t, bmcEndpoint, server.VNCEndpoint.Endpoint)
			} else {
				assert.Nil(t, server.VNCEndpoint, "VNC endpoint should not be populated")
			}
		})
	}
}

// TestDatabaseRoundTrip_PreservesSOLAndVNCEndpoints tests that SOL and VNC endpoints
// are correctly serialized and deserialized through the database
func TestDatabaseRoundTrip_PreservesSOLAndVNCEndpoints(t *testing.T) {
	handler := setupTestHandler(t)

	// Create a server with full SOL and VNC endpoint configuration
	server := &domain.Server{
		ID:           "test-server-roundtrip",
		CustomerID:   "test-customer",
		DatacenterID: "dc-test-01",
		ControlEndpoints: []*types.BMCControlEndpoint{
			{
				Endpoint: "192.168.1.100:623",
				Type:     types.BMCTypeIPMI,
				Username: "admin",
				Password: "",
				Capabilities: types.CapabilitiesToStrings([]types.Capability{
					types.CapabilityIPMIChassis,
					types.CapabilityIPMISDR,
				}),
			},
		},
		PrimaryProtocol: types.BMCTypeIPMI,
		SOLEndpoint: &types.SOLEndpoint{
			Type:     types.SOLTypeIPMI,
			Endpoint: "192.168.1.100:623",
			Username: "admin",
			Password: "",
			Config: &types.SOLConfig{
				BaudRate:       115200,
				FlowControl:    "none",
				TimeoutSeconds: 30,
			},
		},
		VNCEndpoint: &types.VNCEndpoint{
			Type:     types.VNCTypeNative,
			Endpoint: "192.168.1.100:5900",
			Username: "admin",
			Password: "",
			Config: &types.VNCConfig{
				Protocol: "vnc",
				Path:     "/vnc",
				Display:  1,
				ReadOnly: false,
			},
		},
		Features: types.FeaturesToStrings([]types.Feature{
			types.FeaturePower,
			types.FeatureConsole,
			types.FeatureVNC,
		}),
		Status:    "active",
		Metadata:  map[string]string{"location": "rack-1"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create the server in database
	err := handler.db.Servers.Create(context.Background(), server)
	require.NoError(t, err)

	// Retrieve the server
	retrieved, err := handler.db.Servers.Get(context.Background(), server.ID)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify SOL endpoint was preserved
	require.NotNil(t, retrieved.SOLEndpoint)
	assert.Equal(t, server.SOLEndpoint.Type, retrieved.SOLEndpoint.Type)
	assert.Equal(t, server.SOLEndpoint.Endpoint, retrieved.SOLEndpoint.Endpoint)
	assert.Equal(t, server.SOLEndpoint.Username, retrieved.SOLEndpoint.Username)
	require.NotNil(t, retrieved.SOLEndpoint.Config)
	assert.Equal(t, server.SOLEndpoint.Config.BaudRate, retrieved.SOLEndpoint.Config.BaudRate)
	assert.Equal(t, server.SOLEndpoint.Config.FlowControl, retrieved.SOLEndpoint.Config.FlowControl)

	// Verify VNC endpoint was preserved
	require.NotNil(t, retrieved.VNCEndpoint)
	assert.Equal(t, server.VNCEndpoint.Type, retrieved.VNCEndpoint.Type)
	assert.Equal(t, server.VNCEndpoint.Endpoint, retrieved.VNCEndpoint.Endpoint)
	assert.Equal(t, server.VNCEndpoint.Username, retrieved.VNCEndpoint.Username)
	require.NotNil(t, retrieved.VNCEndpoint.Config)
	assert.Equal(t, server.VNCEndpoint.Config.Protocol, retrieved.VNCEndpoint.Config.Protocol)
	assert.Equal(t, server.VNCEndpoint.Config.Display, retrieved.VNCEndpoint.Config.Display)

	// Verify metadata was preserved
	assert.Equal(t, server.Metadata["location"], retrieved.Metadata["location"])
}

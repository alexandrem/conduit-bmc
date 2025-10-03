package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"core/types"
	"manager/pkg/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *BunDB {
	t.Helper()

	// Use in-memory database for fast tests
	db, err := New(":memory:")
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// TestBunDB_WithDebugOption tests that debug option enables query logging
func TestBunDB_WithDebugOption(t *testing.T) {
	// Test with debug disabled (default)
	db1, err := New(":memory:")
	require.NoError(t, err)
	defer db1.Close()
	assert.NotNil(t, db1)

	// Test with debug enabled
	db2, err := New(":memory:", WithDebug(true))
	require.NoError(t, err)
	defer db2.Close()
	assert.NotNil(t, db2)

	// Test with debug explicitly disabled
	db3, err := New(":memory:", WithDebug(false))
	require.NoError(t, err)
	defer db3.Close()
	assert.NotNil(t, db3)
}

// TestBunDB_Initialization tests that database initializes correctly
func TestBunDB_Initialization(t *testing.T) {
	db := setupTestDB(t)

	assert.NotNil(t, db.DB())
	assert.NotNil(t, db.Servers)
	assert.NotNil(t, db.Customers)
	assert.NotNil(t, db.Agents)
	assert.NotNil(t, db.Gateways)
	assert.NotNil(t, db.Locations)
	assert.NotNil(t, db.Sessions)
}

// TestServerRepository_CRUD tests server CRUD operations
func TestServerRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create customer first (foreign key requirement)
	customer := &models.Customer{
		ID:        "customer-123",
		Email:     "test@example.com",
		APIKey:    "test-api-key",
		CreatedAt: time.Now(),
	}
	err := db.Customers.Create(ctx, customer)
	require.NoError(t, err)

	// Test Create
	server := &models.Server{
		ID:           "server-001",
		CustomerID:   "customer-123",
		DatacenterID: "dc-test-01",
		Features: types.FeaturesToStrings([]types.Feature{
			types.FeaturePower,
			types.FeatureConsole,
			types.FeatureVNC,
		}),
		ControlEndpoint: &models.BMCControlEndpoint{
			Endpoint: "192.168.1.100:623",
			Type:     models.BMCTypeIPMI,
			Username: "admin",
			Capabilities: types.CapabilitiesToStrings([]types.Capability{
				types.CapabilityIPMISEL,
				types.CapabilityIPMISDR,
			}),
		},
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = db.Servers.Create(ctx, server)
	require.NoError(t, err)

	// Test Get
	retrieved, err := db.Servers.Get(ctx, "server-001")
	require.NoError(t, err)
	assert.Equal(t, server.ID, retrieved.ID)
	assert.Equal(t, server.CustomerID, retrieved.CustomerID)
	assert.Equal(t, server.Features, retrieved.Features)
	assert.NotNil(t, retrieved.ControlEndpoint)
	assert.Equal(t, models.BMCTypeIPMI, retrieved.ControlEndpoint.Type)

	// Test List
	servers, err := db.Servers.List(ctx, "customer-123")
	require.NoError(t, err)
	assert.Len(t, servers, 1)
	assert.Equal(t, "server-001", servers[0].ID)

	// Test Update
	server.Status = "inactive"
	err = db.Servers.Update(ctx, server)
	require.NoError(t, err)

	updated, err := db.Servers.Get(ctx, "server-001")
	require.NoError(t, err)
	assert.Equal(t, "inactive", updated.Status)

	// Test Delete
	err = db.Servers.Delete(ctx, "server-001")
	require.NoError(t, err)

	_, err = db.Servers.Get(ctx, "server-001")
	assert.Error(t, err)
}

// TestServer_JSONFields tests that JSON fields are properly handled
func TestServer_JSONFields(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create customer
	customer := &models.Customer{
		ID:        "customer-123",
		Email:     "test@example.com",
		APIKey:    "test-api-key",
		CreatedAt: time.Now(),
	}
	err := db.Customers.Create(ctx, customer)
	require.NoError(t, err)

	// Create server with complex JSON fields
	server := &models.Server{
		ID:           "server-001",
		CustomerID:   "customer-123",
		DatacenterID: "dc-test-01",
		Features:     []string{"power", "console", "vnc", "sensors"}, // ✅ JSON array
		Status:       "active",
		SOLEndpoint: &models.SOLEndpoint{
			Type:     models.SOLTypeRedfishSerial,
			Endpoint: "http://192.168.1.100:8000",
			Username: "admin",
			Password: "password",
		},
		VNCEndpoint: &models.VNCEndpoint{
			Type:     models.VNCTypeNative,
			Endpoint: "192.168.1.100:5901",
			Username: "admin",
			Password: "password",
		},
		ControlEndpoint: &models.BMCControlEndpoint{
			Endpoint:     "http://192.168.1.100:8000",
			Type:         models.BMCTypeRedfish,
			Username:     "admin",
			Password:     "password",
			Capabilities: []string{"Systems", "Chassis"},
		},
		Metadata: map[string]string{
			"location": "rack-1",
			"model":    "Dell R750",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = db.Servers.Create(ctx, server)
	require.NoError(t, err)

	// Retrieve and verify JSON fields
	retrieved, err := db.Servers.Get(ctx, "server-001")
	require.NoError(t, err)

	// Verify features as proper array (not CSV string)
	assert.IsType(t, []string{}, retrieved.Features)
	assert.Len(t, retrieved.Features, 4)
	assert.Equal(t, "power", retrieved.Features[0])
	assert.Equal(t, "console", retrieved.Features[1])
	assert.Equal(t, "vnc", retrieved.Features[2])
	assert.Equal(t, "sensors", retrieved.Features[3])

	// Verify capabilities from control endpoint
	require.NotNil(t, retrieved.ControlEndpoint)
	assert.IsType(t, []string{}, retrieved.ControlEndpoint.Capabilities)
	assert.Len(t, retrieved.ControlEndpoint.Capabilities, 2)
	assert.Contains(t, retrieved.ControlEndpoint.Capabilities, "Systems")

	// Verify SOLEndpoint
	require.NotNil(t, retrieved.SOLEndpoint)
	assert.Equal(t, models.SOLTypeRedfishSerial, retrieved.SOLEndpoint.Type)
	assert.Equal(t, "http://192.168.1.100:8000", retrieved.SOLEndpoint.Endpoint)

	// Verify VNCEndpoint
	require.NotNil(t, retrieved.VNCEndpoint)
	assert.Equal(t, models.VNCTypeNative, retrieved.VNCEndpoint.Type)
	assert.Equal(t, "192.168.1.100:5901", retrieved.VNCEndpoint.Endpoint)

	// Verify ControlEndpoint (already checked above, but double-check Type)
	assert.Equal(t, models.BMCTypeRedfish, retrieved.ControlEndpoint.Type)

	// Verify Metadata
	assert.Len(t, retrieved.Metadata, 2)
	assert.Equal(t, "rack-1", retrieved.Metadata["location"])
	assert.Equal(t, "Dell R750", retrieved.Metadata["model"])
}

// TestGatewayRepository_CRUD tests gateway CRUD operations
func TestGatewayRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	gateway := &models.RegionalGateway{
		ID:            "gateway-us-east-1",
		Region:        "us-east-1",
		Endpoint:      "http://gateway-us-east:8081",
		DatacenterIDs: []string{"dc-us-east-1a", "dc-us-east-1b"},
		Status:        "active",
		LastSeen:      time.Now(),
		CreatedAt:     time.Now(),
	}

	// Test Create
	err := db.Gateways.Create(ctx, gateway)
	require.NoError(t, err)

	// Test Get
	retrieved, err := db.Gateways.Get(ctx, "gateway-us-east-1")
	require.NoError(t, err)
	assert.Equal(t, gateway.ID, retrieved.ID)
	assert.Equal(t, gateway.Endpoint, retrieved.Endpoint)
	assert.Equal(t, gateway.DatacenterIDs, retrieved.DatacenterIDs)

	// Test List
	gateways, err := db.Gateways.List(ctx)
	require.NoError(t, err)
	assert.Len(t, gateways, 1)
}

// TestServerLocationRepository_Upsert tests upsert functionality
func TestServerLocationRepository_Upsert(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	location := &models.ServerLocation{
		ServerID:          "server-001",
		CustomerID:        "customer-123",
		DatacenterID:      "dc-us-east-1a",
		RegionalGatewayID: "gateway-us-east-1",
		BMCType:           models.BMCTypeIPMI,
		Features:          []string{"power", "console"},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// First upsert (insert)
	err := db.Locations.Upsert(ctx, location)
	require.NoError(t, err)

	retrieved, err := db.Locations.Get(ctx, "server-001")
	require.NoError(t, err)
	assert.Equal(t, "dc-us-east-1a", retrieved.DatacenterID)

	// Second upsert (update)
	location.DatacenterID = "dc-us-east-1b"
	location.Features = []string{"power", "console", "vnc"}
	err = db.Locations.Upsert(ctx, location)
	require.NoError(t, err)

	updated, err := db.Locations.Get(ctx, "server-001")
	require.NoError(t, err)
	assert.Equal(t, "dc-us-east-1b", updated.DatacenterID)
	assert.Len(t, updated.Features, 3)
}

// TestCustomerRepository_UniqueConstraints tests unique constraints
func TestCustomerRepository_UniqueConstraints(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	customer1 := &models.Customer{
		ID:        "customer-1",
		Email:     "test@example.com",
		APIKey:    "api-key-1",
		CreatedAt: time.Now(),
	}

	err := db.Customers.Create(ctx, customer1)
	require.NoError(t, err)

	// Try to create another customer with same email (should fail)
	customer2 := &models.Customer{
		ID:        "customer-2",
		Email:     "test@example.com", // Duplicate email
		APIKey:    "api-key-2",
		CreatedAt: time.Now(),
	}

	err = db.Customers.Create(ctx, customer2)
	assert.Error(t, err)
}

// TestProxySessionRepository_ListActive tests active session filtering
func TestProxySessionRepository_ListActive(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create dependencies
	customer := &models.Customer{
		ID:        "customer-123",
		Email:     "test@example.com",
		APIKey:    "test-api-key",
		CreatedAt: time.Now(),
	}
	db.Customers.Create(ctx, customer)

	server := &models.Server{
		ID:           "server-001",
		CustomerID:   "customer-123",
		DatacenterID: "dc-01",
		Features:     []string{"power"},
		ControlEndpoint: &models.BMCControlEndpoint{
			Endpoint: "192.168.1.100:623",
			Type:     models.BMCTypeIPMI,
		},
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	db.Servers.Create(ctx, server)

	agent := &models.Agent{
		ID:           "agent-001",
		DatacenterID: "dc-01",
		Endpoint:     "http://agent:8082",
		Status:       "active",
		LastSeen:     time.Now(),
		CreatedAt:    time.Now(),
	}
	db.Agents.Create(ctx, agent)

	// Create active session
	activeSession := &models.ProxySession{
		ID:         "session-active",
		CustomerID: "customer-123",
		ServerID:   "server-001",
		AgentID:    "agent-001",
		Status:     "active",
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}
	db.Sessions.Create(ctx, activeSession)

	// Create expired session
	expiredSession := &models.ProxySession{
		ID:         "session-expired",
		CustomerID: "customer-123",
		ServerID:   "server-001",
		AgentID:    "agent-001",
		Status:     "active",
		CreatedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt:  time.Now().Add(-1 * time.Hour),
	}
	db.Sessions.Create(ctx, expiredSession)

	// List active sessions
	active, err := db.Sessions.ListActive(ctx)
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Equal(t, "session-active", active[0].ID)
}

// TestBunDB_Close tests database cleanup
func TestBunDB_Close(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)
}

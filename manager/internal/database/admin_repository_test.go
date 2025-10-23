package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"core/domain"
	"core/types"
	"manager/pkg/models"
)

func TestAdminRepository_GetDashboardMetrics(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test data
	customer := &models.Customer{
		ID:      "customer1",
		Email:   "customer1@example.com",
		IsAdmin: false,
	}
	require.NoError(t, db.Customers.Create(context.Background(), customer))

	server := &domain.Server{
		ID:           "server1",
		CustomerID:   "customer1",
		DatacenterID: "dc1",
		ControlEndpoints: []*types.BMCControlEndpoint{
			{Endpoint: "192.168.1.100:623", Type: types.BMCTypeIPMI},
		},
		PrimaryProtocol: types.BMCTypeIPMI,
		Features:        []string{"power"},
		Status:          "active",
	}
	require.NoError(t, db.Servers.Create(context.Background(), server))

	gateway := &models.RegionalGateway{
		ID:            "gateway1",
		Region:        "us-west-1",
		Endpoint:      "https://gw.example.com",
		DatacenterIDs: []string{"dc1"},
		Status:        "active",
		LastSeen:      time.Now().UTC(),
	}
	require.NoError(t, db.Gateways.Create(context.Background(), gateway))

	// Create server location to link server to customer and gateway
	serverLocation := &models.ServerLocation{
		ServerID:          "server1",
		CustomerID:        "customer1",
		DatacenterID:      "dc1",
		RegionalGatewayID: "gateway1",
		ControlEndpoints: []*types.BMCControlEndpoint{
			{Endpoint: "192.168.1.100:623", Type: types.BMCTypeIPMI},
		},
		PrimaryProtocol: types.BMCTypeIPMI,
		CreatedAt:       time.Now().UTC(),
	}
	require.NoError(t, db.Locations.Create(context.Background(), serverLocation))

	// Get metrics
	metrics, err := db.Admin.GetDashboardMetrics(context.Background())
	require.NoError(t, err)

	// Assertions
	assert.Equal(t, int32(1), metrics.TotalBmcs)
	assert.Equal(t, int32(1), metrics.OnlineBmcs)
	assert.Equal(t, int32(0), metrics.OfflineBmcs)
	assert.Equal(t, int32(1), metrics.TotalGateways)
	assert.Equal(t, int32(1), metrics.ActiveGateways)
	assert.Equal(t, int32(1), metrics.TotalCustomers)
}

func TestAdminRepository_ListAllServersWithFilters(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test data
	customer := &models.Customer{
		ID:      "customer1",
		Email:   "customer1@example.com",
		IsAdmin: false,
	}
	require.NoError(t, db.Customers.Create(context.Background(), customer))

	server := &domain.Server{
		ID:           "server1",
		CustomerID:   "customer1",
		DatacenterID: "dc1",
		ControlEndpoints: []*types.BMCControlEndpoint{
			{Endpoint: "192.168.1.100:623", Type: types.BMCTypeIPMI},
		},
		PrimaryProtocol: types.BMCTypeIPMI,
		Features:        []string{"power"},
		Status:          "active",
	}
	require.NoError(t, db.Servers.Create(context.Background(), server))

	gateway := &models.RegionalGateway{
		ID:            "gateway1",
		Region:        "us-west-1",
		Endpoint:      "https://gw.example.com",
		DatacenterIDs: []string{"dc1"},
		Status:        "active",
	}
	require.NoError(t, db.Gateways.Create(context.Background(), gateway))

	serverLocation := &models.ServerLocation{
		ServerID:          "server1",
		CustomerID:        "customer1",
		DatacenterID:      "dc1",
		RegionalGatewayID: "gateway1",
		ControlEndpoints: []*types.BMCControlEndpoint{
			{Endpoint: "192.168.1.100:623", Type: types.BMCTypeIPMI},
		},
		PrimaryProtocol: types.BMCTypeIPMI,
		Features:        []string{"power"},
	}
	require.NoError(t, db.Locations.Create(context.Background(), serverLocation))

	// List servers
	filters := &AdminServerFilters{
		PageSize: 10,
		Offset:   0,
	}
	servers, err := db.Admin.ListAllServersWithFilters(context.Background(), filters)
	require.NoError(t, err)

	// Assertions
	assert.Len(t, servers, 1)
	assert.Equal(t, "server1", servers[0].ServerId)
	assert.Equal(t, "customer1", servers[0].CustomerId)
	assert.Equal(t, "active", servers[0].Status)
}

func TestAdminRepository_ListAllCustomersWithCounts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test data
	customer := &models.Customer{
		ID:      "customer1",
		Email:   "customer1@example.com",
		IsAdmin: false,
	}
	require.NoError(t, db.Customers.Create(context.Background(), customer))

	server := &domain.Server{
		ID:           "server1",
		CustomerID:   "customer1",
		DatacenterID: "dc1",
		ControlEndpoints: []*types.BMCControlEndpoint{
			{Endpoint: "192.168.1.100:623", Type: types.BMCTypeIPMI},
		},
		PrimaryProtocol: types.BMCTypeIPMI,
		Features:        []string{"power"},
		Status:          "active",
	}
	require.NoError(t, db.Servers.Create(context.Background(), server))

	// List customers
	customers, err := db.Admin.ListAllCustomersWithCounts(context.Background(), 10, 0)
	require.NoError(t, err)

	// Assertions
	assert.Len(t, customers, 1)
	assert.Equal(t, "customer1", customers[0].CustomerId)
	assert.Equal(t, "customer1@example.com", customers[0].Email)
	assert.Equal(t, int32(1), customers[0].ServerCount)
	assert.Equal(t, int32(1), customers[0].OnlineServerCount)
}

func TestAdminRepository_GetRegions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test gateways
	gateway1 := &models.RegionalGateway{
		ID:            "gateway1",
		Region:        "us-west-1",
		Endpoint:      "https://gw1.example.com",
		DatacenterIDs: []string{"dc1"},
		Status:        "active",
	}
	require.NoError(t, db.Gateways.Create(context.Background(), gateway1))

	gateway2 := &models.RegionalGateway{
		ID:            "gateway2",
		Region:        "us-east-1",
		Endpoint:      "https://gw2.example.com",
		DatacenterIDs: []string{"dc2"},
		Status:        "active",
	}
	require.NoError(t, db.Gateways.Create(context.Background(), gateway2))

	// Get regions
	regions, err := db.Admin.GetRegions(context.Background())
	require.NoError(t, err)

	// Assertions
	assert.Len(t, regions, 2)
	assert.Contains(t, regions, "us-west-1")
	assert.Contains(t, regions, "us-east-1")
}

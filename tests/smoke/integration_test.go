package functional

import (
	"context"
	"testing"
	"time"

	"tests/synthetic"
	"local-agent/pkg/ipmi"
	"local-agent/pkg/redfish"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEndBMCDiscovery(t *testing.T) {
	// Skip if running in CI without Docker
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create multiple synthetic BMC servers
	redfishServer, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create Redfish server")

	err = redfishServer.Start()
	require.NoError(t, err, "Failed to start Redfish server")
	defer redfishServer.Stop()

	// Give servers time to start
	time.Sleep(200 * time.Millisecond)

	// Create clients
	ipmiClient := ipmi.NewClient()
	redfishClient := redfish.NewClient()

	ctx := context.Background()

	// Test that we can discover the synthetic Redfish server
	accessible := redfishClient.IsAccessible(ctx, redfishServer.Endpoint)
	assert.True(t, accessible, "Synthetic Redfish server should be accessible")

	// Test IPMI client initialization (doesn't need actual server for this test)
	assert.NotNil(t, ipmiClient, "IPMI client should be created successfully")

	// For this test, we'll just verify the clients work
	// In a real discovery service, this would scan networks and find endpoints
	t.Log("Discovery test completed - clients can detect BMC endpoints")
}

func TestBMCOperationFlow(t *testing.T) {
	// Create synthetic Redfish server
	server, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create synthetic Redfish server")

	err = server.Start()
	require.NoError(t, err, "Failed to start synthetic Redfish server")
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Test full BMC operation flow
	redfishClient := redfish.NewClient()
	ctx := context.Background()

	// 1. Discovery phase
	accessible := redfishClient.IsAccessible(ctx, server.Endpoint)
	assert.True(t, accessible, "Server should be discoverable")

	// 2. Information gathering
	info, err := redfishClient.GetBMCInfo(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should get BMC info")
	t.Logf("Discovered BMC: %s %s (Redfish %s)", info.Vendor, info.Model, info.RedfishVersion)

	// 3. Initial power state check
	initialState, err := redfishClient.GetPowerState(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should get initial power state")
	t.Logf("Initial power state: %s", initialState)

	// 4. Power operation
	err = redfishClient.PowerOff(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should be able to power off")

	// 5. Verify state change
	assert.Equal(t, "Off", server.GetPowerState(), "Power state should change to Off")

	// 6. Power back on
	err = redfishClient.PowerOn(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should be able to power on")

	// 7. Final state verification
	assert.Equal(t, "On", server.GetPowerState(), "Power state should be On")

	t.Log("BMC operation flow completed successfully")
}

func TestConcurrentBMCAccess(t *testing.T) {
	// Create synthetic Redfish server
	server, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create synthetic Redfish server")

	err = server.Start()
	require.NoError(t, err, "Failed to start synthetic Redfish server")
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Test concurrent access to BMC
	redfishClient := redfish.NewClient()
	ctx := context.Background()

	// Channel to collect results
	results := make(chan error, 10)

	// Launch multiple concurrent operations
	for i := 0; i < 10; i++ {
		go func(id int) {
			// Each goroutine performs a power status check
			_, err := redfishClient.GetPowerState(ctx, server.Endpoint, server.Username, server.Password)
			results <- err
		}(i)
	}

	// Collect all results
	for i := 0; i < 10; i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent power state check should succeed")
	}

	t.Log("Concurrent BMC access test completed")
}

func TestBMCFailureRecovery(t *testing.T) {
	// Create synthetic Redfish server
	server, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create synthetic Redfish server")

	err = server.Start()
	require.NoError(t, err, "Failed to start synthetic Redfish server")

	time.Sleep(100 * time.Millisecond)

	redfishClient := redfish.NewClient()
	ctx := context.Background()

	// Verify server is initially accessible
	accessible := redfishClient.IsAccessible(ctx, server.Endpoint)
	assert.True(t, accessible, "Server should be initially accessible")

	// Stop the server to simulate failure
	err = server.Stop()
	require.NoError(t, err, "Should be able to stop server")

	// Verify server is no longer accessible
	accessible = redfishClient.IsAccessible(ctx, server.Endpoint)
	assert.False(t, accessible, "Server should not be accessible after stop")

	// Test that operations fail gracefully
	err = redfishClient.PowerOn(ctx, server.Endpoint, server.Username, server.Password)
	assert.Error(t, err, "Operations should fail when server is down")

	t.Log("BMC failure recovery test completed")
}

func TestBMCSensorData(t *testing.T) {
	// Create synthetic Redfish server
	server, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create synthetic Redfish server")

	err = server.Start()
	require.NoError(t, err, "Failed to start synthetic Redfish server")
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	redfishClient := redfish.NewClient()
	ctx := context.Background()

	// Test getting sensor data
	sensors, err := redfishClient.GetSensors(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should be able to get sensor data")

	// Verify we got some sensor data
	assert.NotEmpty(t, sensors, "Should have sensor data")

	// Check for expected sensor types
	_, hasCPUTemp := sensors["cpu_temperature"]
	_, hasPower := sensors["power_consumption"]
	_, hasVoltage := sensors["voltage_12v"]

	assert.True(t, hasCPUTemp, "Should have CPU temperature sensor")
	assert.True(t, hasPower, "Should have power consumption sensor")
	assert.True(t, hasVoltage, "Should have voltage sensor")

	t.Logf("Retrieved %d sensor readings", len(sensors))
}
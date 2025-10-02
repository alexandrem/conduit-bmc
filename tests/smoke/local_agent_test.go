package functional

import (
	"context"
	"testing"
	"time"

	"tests/synthetic"
	"local-agent/pkg/redfish"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalAgentRedfishDiscovery(t *testing.T) {
	// Create synthetic Redfish server
	server, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create synthetic Redfish server")

	// Start the server
	err = server.Start()
	require.NoError(t, err, "Failed to start synthetic Redfish server")
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is accessible
	assert.True(t, server.IsAccessible(), "Synthetic server should be accessible")

	// Test Redfish client can discover the server
	redfishClient := redfish.NewClient()
	ctx := context.Background()

	// Test accessibility check
	accessible := redfishClient.IsAccessible(ctx, server.Endpoint)
	assert.True(t, accessible, "Redfish client should detect synthetic server")
}

func TestLocalAgentRedfishPowerOperations(t *testing.T) {
	// Create synthetic Redfish server
	server, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create synthetic Redfish server")

	// Start the server
	err = server.Start()
	require.NoError(t, err, "Failed to start synthetic Redfish server")
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create Redfish client
	redfishClient := redfish.NewClient()
	ctx := context.Background()

	// Test power status
	powerState, err := redfishClient.GetPowerState(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should be able to get power state")
	assert.Equal(t, redfish.PowerStateOn, powerState, "Initial power state should be On")

	// Test power off
	err = redfishClient.PowerOff(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should be able to power off")

	// Verify power state changed
	assert.Equal(t, "Off", server.GetPowerState(), "Server power state should be Off")

	// Test power on
	err = redfishClient.PowerOn(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should be able to power on")

	// Verify power state changed
	assert.Equal(t, "On", server.GetPowerState(), "Server power state should be On")

	// Test power cycle
	err = redfishClient.PowerCycle(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should be able to power cycle")

	// Verify power state (should end up On after cycle)
	assert.Equal(t, "On", server.GetPowerState(), "Server power state should be On after cycle")
}

func TestLocalAgentRedfishBMCInfo(t *testing.T) {
	// Create synthetic Redfish server
	server, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create synthetic Redfish server")

	// Start the server
	err = server.Start()
	require.NoError(t, err, "Failed to start synthetic Redfish server")
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create Redfish client
	redfishClient := redfish.NewClient()
	ctx := context.Background()

	// Test getting BMC info
	info, err := redfishClient.GetBMCInfo(ctx, server.Endpoint, server.Username, server.Password)
	require.NoError(t, err, "Should be able to get BMC info")

	assert.Equal(t, "Redfish BMC", info.Model, "Should get expected model")
	assert.Contains(t, info.Features, "power", "Should support power features")
	assert.Contains(t, info.Features, "sensors", "Should support sensor features")
	assert.NotEmpty(t, info.RedfishVersion, "Should have Redfish version")
}

func TestLocalAgentMultipleBMCs(t *testing.T) {
	// Create multiple synthetic Redfish servers
	server1, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create first synthetic Redfish server")

	server2, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create second synthetic Redfish server")

	// Start both servers
	err = server1.Start()
	require.NoError(t, err, "Failed to start first synthetic Redfish server")
	defer server1.Stop()

	err = server2.Start()
	require.NoError(t, err, "Failed to start second synthetic Redfish server")
	defer server2.Stop()

	// Give servers time to start
	time.Sleep(200 * time.Millisecond)

	// Create Redfish client
	redfishClient := redfish.NewClient()
	ctx := context.Background()

	// Test both servers are accessible
	accessible1 := redfishClient.IsAccessible(ctx, server1.Endpoint)
	accessible2 := redfishClient.IsAccessible(ctx, server2.Endpoint)

	assert.True(t, accessible1, "First Redfish server should be accessible")
	assert.True(t, accessible2, "Second Redfish server should be accessible")

	// Test independent power operations
	err = redfishClient.PowerOff(ctx, server1.Endpoint, server1.Username, server1.Password)
	require.NoError(t, err, "Should be able to power off first server")

	// Verify only first server powered off
	assert.Equal(t, "Off", server1.GetPowerState(), "First server should be Off")
	assert.Equal(t, "On", server2.GetPowerState(), "Second server should still be On")
}

func TestLocalAgentRedfishErrorHandling(t *testing.T) {
	// Create Redfish client
	redfishClient := redfish.NewClient()
	ctx := context.Background()

	// Test with non-existent server
	accessible := redfishClient.IsAccessible(ctx, "http://localhost:99999")
	assert.False(t, accessible, "Non-existent server should not be accessible")

	// Test power operations on non-existent server
	err := redfishClient.PowerOn(ctx, "http://localhost:99999", "admin", "admin123")
	assert.Error(t, err, "Power operations should fail on non-existent server")

	// Test with wrong credentials
	server, err := synthetic.NewHTTPRedfishServer()
	require.NoError(t, err, "Failed to create synthetic Redfish server")

	err = server.Start()
	require.NoError(t, err, "Failed to start synthetic Redfish server")
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Test with wrong password
	err = redfishClient.PowerOn(ctx, server.Endpoint, server.Username, "wrongpassword")
	assert.Error(t, err, "Power operations should fail with wrong credentials")
}
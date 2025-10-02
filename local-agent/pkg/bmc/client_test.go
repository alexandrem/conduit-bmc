package bmc

import (
	"context"
	"testing"

	"local-agent/internal/discovery"
	"local-agent/pkg/ipmi"
	"local-agent/pkg/redfish"
)

// Note: These tests verify the routing logic and error handling in the BMC client.
// The actual IPMI and Redfish client functionality is tested in their respective packages.
// These tests will skip if no real BMC is available, focusing on the routing and validation logic.

func TestNewClient(t *testing.T) {
	ipmiClient := ipmi.NewClient()
	redfishClient := redfish.NewClient()

	client := NewClient(ipmiClient, redfishClient)

	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.ipmiClient == nil {
		t.Error("Expected ipmiClient to be set")
	}

	if client.redfishClient == nil {
		t.Error("Expected redfishClient to be set")
	}
}

func TestClient_GetPowerState_NoControlEndpoint(t *testing.T) {
	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: nil,
	}

	ctx := context.Background()
	_, err := client.GetPowerState(ctx, server)

	if err == nil {
		t.Error("Expected error for missing control endpoint")
	}

	if err.Error() != "server has no control endpoint" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestClient_GetPowerState_UnsupportedType(t *testing.T) {
	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: &discovery.BMCControlEndpoint{
			Endpoint: "unknown://192.168.1.100",
			Type:     "unknown",
			Username: "admin",
			Password: "password",
		},
	}

	ctx := context.Background()
	_, err := client.GetPowerState(ctx, server)

	if err == nil {
		t.Error("Expected error for unsupported BMC type")
	}

	expectedErr := "unsupported BMC type: unknown"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got: %v", expectedErr, err)
	}
}

func TestClient_PowerOn_NoControlEndpoint(t *testing.T) {
	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: nil,
	}

	ctx := context.Background()
	err := client.PowerOn(ctx, server)

	if err == nil {
		t.Error("Expected error for missing control endpoint")
	}

	if err.Error() != "server has no control endpoint" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestClient_PowerOff_UnsupportedType(t *testing.T) {
	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: &discovery.BMCControlEndpoint{
			Endpoint: "unknown://192.168.1.100",
			Type:     "unsupported",
			Username: "admin",
			Password: "password",
		},
	}

	ctx := context.Background()
	err := client.PowerOff(ctx, server)

	if err == nil {
		t.Error("Expected error for unsupported BMC type")
	}

	expectedErr := "unsupported BMC type: unsupported"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got: %v", expectedErr, err)
	}
}

func TestClient_PowerCycle_NoControlEndpoint(t *testing.T) {
	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: nil,
	}

	ctx := context.Background()
	err := client.PowerCycle(ctx, server)

	if err == nil {
		t.Error("Expected error for missing control endpoint")
	}
}

func TestClient_Reset_NoControlEndpoint(t *testing.T) {
	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: nil,
	}

	ctx := context.Background()
	err := client.Reset(ctx, server)

	if err == nil {
		t.Error("Expected error for missing control endpoint")
	}
}

func TestClient_AllOperations_NoControlEndpoint(t *testing.T) {
	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: nil,
	}

	ctx := context.Background()

	// Test all operations return error for missing control endpoint
	operations := []struct {
		name string
		fn   func() error
	}{
		{"PowerOn", func() error { return client.PowerOn(ctx, server) }},
		{"PowerOff", func() error { return client.PowerOff(ctx, server) }},
		{"PowerCycle", func() error { return client.PowerCycle(ctx, server) }},
		{"Reset", func() error { return client.Reset(ctx, server) }},
	}

	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			err := op.fn()
			if err == nil {
				t.Errorf("%s: Expected error for missing control endpoint", op.name)
			}
			if err.Error() != "server has no control endpoint" {
				t.Errorf("%s: Expected specific error message, got: %v", op.name, err)
			}
		})
	}
}

func TestClient_AllOperations_UnsupportedType(t *testing.T) {
	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: &discovery.BMCControlEndpoint{
			Endpoint: "unknown://192.168.1.100",
			Type:     "invalid_type",
			Username: "admin",
			Password: "password",
		},
	}

	ctx := context.Background()

	// Test all operations return error for unsupported type
	operations := []struct {
		name string
		fn   func() error
	}{
		{"PowerOn", func() error { return client.PowerOn(ctx, server) }},
		{"PowerOff", func() error { return client.PowerOff(ctx, server) }},
		{"PowerCycle", func() error { return client.PowerCycle(ctx, server) }},
		{"Reset", func() error { return client.Reset(ctx, server) }},
	}

	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {
			err := op.fn()
			if err == nil {
				t.Errorf("%s: Expected error for unsupported BMC type", op.name)
			}
			expectedErr := "unsupported BMC type: invalid_type"
			if err.Error() != expectedErr {
				t.Errorf("%s: Expected error %q, got: %v", op.name, expectedErr, err)
			}
		})
	}
}

// TestClient_IPMI_Routing verifies IPMI endpoints are correctly routed
// Note: This test requires real IPMI BMC hardware and is skipped in unit tests
func TestClient_IPMI_Routing(t *testing.T) {
	t.Skip("Skipping integration test - requires real IPMI BMC hardware")

	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: &discovery.BMCControlEndpoint{
			Endpoint: "192.168.1.100:623",
			Type:     "ipmi",
			Username: "admin",
			Password: "password",
		},
	}

	ctx := context.Background()

	// Test that IPMI routing works with real hardware
	_, err := client.GetPowerState(ctx, server)

	// Should get IPMI-specific error, not routing error
	if err != nil {
		if err.Error() == "server has no control endpoint" || err.Error() == "unsupported BMC type: ipmi" {
			t.Error("Routing failed - should have reached IPMI client")
		}
	}
}

// TestClient_Redfish_Routing verifies Redfish endpoints are correctly routed
// Note: This test requires real Redfish BMC hardware and is skipped in unit tests
func TestClient_Redfish_Routing(t *testing.T) {
	t.Skip("Skipping integration test - requires real Redfish BMC hardware")

	client := NewClient(ipmi.NewClient(), redfish.NewClient())

	server := &discovery.Server{
		ControlEndpoint: &discovery.BMCControlEndpoint{
			Endpoint: "https://192.168.1.100",
			Type:     "redfish",
			Username: "admin",
			Password: "password",
		},
	}

	ctx := context.Background()

	// Test that Redfish routing works with real hardware
	_, err := client.GetPowerState(ctx, server)

	// Should get Redfish-specific error, not routing error
	if err != nil {
		if err.Error() == "server has no control endpoint" || err.Error() == "unsupported BMC type: redfish" {
			t.Error("Routing failed - should have reached Redfish client")
		}
	}
}

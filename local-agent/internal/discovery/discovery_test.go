package discovery

import (
	"context"
	"testing"

	"local-agent/pkg/config"
	"local-agent/pkg/ipmi"
	"local-agent/pkg/redfish"
)

func TestNewService(t *testing.T) {
	ipmiClient := ipmi.NewClient()
	redfishClient := redfish.NewClient()
	cfg := &config.Config{}

	service := NewService(ipmiClient, redfishClient, cfg)

	if service == nil {
		t.Fatal("Expected service to be created")
	}

	if service.ipmiClient == nil {
		t.Error("Expected ipmiClient to be set")
	}

	if service.redfishClient == nil {
		t.Error("Expected redfishClient to be set")
	}

	if service.config == nil {
		t.Error("Expected config to be set")
	}
}

func TestService_LoadStaticServers(t *testing.T) {
	cfg := &config.Config{
		Static: config.StaticConfig{
			Hosts: []config.BMCHost{
				{
					ID: "test-server-1",
					ControlEndpoint: &config.BMCControlEndpoint{
						Type:     "ipmi",
						Endpoint: "192.168.1.100:623",
						Username: "admin",
						Password: "password",
					},
				},
				{
					ID: "test-server-2",
					ControlEndpoint: &config.BMCControlEndpoint{
						Type:     "redfish",
						Endpoint: "https://192.168.1.101",
						Username: "root",
						Password: "secret",
					},
				},
			},
		},
	}

	service := NewService(ipmi.NewClient(), redfish.NewClient(), cfg)
	servers := service.loadStaticServers()

	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Check first server
	if servers[0].ID != "test-server-1" {
		t.Errorf("Expected ID 'test-server-1', got '%s'", servers[0].ID)
	}

	if servers[0].ControlEndpoint == nil {
		t.Fatal("Expected ControlEndpoint to be set")
	}

	if servers[0].ControlEndpoint.Type != "ipmi" {
		t.Errorf("Expected type 'ipmi', got '%s'", servers[0].ControlEndpoint.Type)
	}

	if servers[0].ControlEndpoint.Endpoint != "192.168.1.100:623" {
		t.Errorf("Expected endpoint '192.168.1.100:623', got '%s'", servers[0].ControlEndpoint.Endpoint)
	}

	// Check second server
	if servers[1].ID != "test-server-2" {
		t.Errorf("Expected ID 'test-server-2', got '%s'", servers[1].ID)
	}

	if servers[1].ControlEndpoint.Type != "redfish" {
		t.Errorf("Expected type 'redfish', got '%s'", servers[1].ControlEndpoint.Type)
	}
}

func TestService_LoadStaticServers_Empty(t *testing.T) {
	cfg := &config.Config{
		Static: config.StaticConfig{
			Hosts: []config.BMCHost{},
		},
	}

	service := NewService(ipmi.NewClient(), redfish.NewClient(), cfg)
	servers := service.loadStaticServers()

	if len(servers) != 0 {
		t.Errorf("Expected 0 servers, got %d", len(servers))
	}
}

func TestService_DiscoverServers_StaticOnly(t *testing.T) {
	cfg := &config.Config{
		Agent: config.AgentConfig{
			BMCDiscovery: config.BMCDiscoveryConfig{
				Enabled: false, // Disable auto-discovery
			},
		},
		Static: config.StaticConfig{
			Hosts: []config.BMCHost{
				{
					ID: "static-server-1",
					ControlEndpoint: &config.BMCControlEndpoint{
						Type:     "ipmi",
						Endpoint: "192.168.1.100:623",
						Username: "admin",
						Password: "password",
					},
				},
			},
		},
	}

	service := NewService(ipmi.NewClient(), redfish.NewClient(), cfg)
	ctx := context.Background()

	servers, err := service.DiscoverServers(ctx)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(servers))
	}

	if servers[0].ID != "static-server-1" {
		t.Errorf("Expected ID 'static-server-1', got '%s'", servers[0].ID)
	}
}

func TestService_FilterDuplicates(t *testing.T) {
	service := NewService(ipmi.NewClient(), redfish.NewClient(), &config.Config{})

	existing := []*Server{
		{
			ID: "server-1",
			ControlEndpoint: &BMCControlEndpoint{
				Endpoint: "192.168.1.100:623",
				Type:     "ipmi",
			},
		},
		{
			ID: "server-2",
			ControlEndpoint: &BMCControlEndpoint{
				Endpoint: "192.168.1.101:623",
				Type:     "ipmi",
			},
		},
	}

	discovered := []*Server{
		{
			ID: "discovered-1",
			ControlEndpoint: &BMCControlEndpoint{
				Endpoint: "192.168.1.100:623", // Duplicate
				Type:     "ipmi",
			},
		},
		{
			ID: "discovered-2",
			ControlEndpoint: &BMCControlEndpoint{
				Endpoint: "192.168.1.102:623", // New
				Type:     "ipmi",
			},
		},
	}

	filtered := service.filterDuplicates(existing, discovered)

	// Should only have the new server (192.168.1.102:623)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 server after filtering, got %d", len(filtered))
	}

	if filtered[0].ControlEndpoint.Endpoint != "192.168.1.102:623" {
		t.Errorf("Expected endpoint '192.168.1.102:623', got '%s'", filtered[0].ControlEndpoint.Endpoint)
	}
}

func TestServer_Features(t *testing.T) {
	server := &Server{
		ID: "test-server",
		ControlEndpoint: &BMCControlEndpoint{
			Endpoint:     "192.168.1.100:623",
			Type:         "ipmi",
			Capabilities: []string{"power", "sensors"},
		},
		Features: []string{"power_management", "monitoring"},
		Status:   "active",
	}

	if len(server.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(server.Features))
	}

	if server.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", server.Status)
	}

	if len(server.ControlEndpoint.Capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(server.ControlEndpoint.Capabilities))
	}
}

func TestBMCControlEndpoint_Validation(t *testing.T) {
	tests := []struct {
		name     string
		endpoint *BMCControlEndpoint
		valid    bool
	}{
		{
			name: "valid IPMI endpoint",
			endpoint: &BMCControlEndpoint{
				Endpoint: "192.168.1.100:623",
				Type:     "ipmi",
				Username: "admin",
				Password: "password",
			},
			valid: true,
		},
		{
			name: "valid Redfish endpoint",
			endpoint: &BMCControlEndpoint{
				Endpoint: "https://192.168.1.100",
				Type:     "redfish",
				Username: "root",
				Password: "secret",
			},
			valid: true,
		},
		{
			name: "missing type",
			endpoint: &BMCControlEndpoint{
				Endpoint: "192.168.1.100:623",
				Type:     "",
				Username: "admin",
				Password: "password",
			},
			valid: false,
		},
		{
			name: "missing endpoint",
			endpoint: &BMCControlEndpoint{
				Endpoint: "",
				Type:     "ipmi",
				Username: "admin",
				Password: "password",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation checks
			hasEndpoint := tt.endpoint.Endpoint != ""
			hasType := tt.endpoint.Type != ""
			isValid := hasEndpoint && hasType

			if isValid != tt.valid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.valid, isValid)
			}
		})
	}
}

func TestSOLEndpoint_Types(t *testing.T) {
	tests := []struct {
		name     string
		solType  string
		expected bool
	}{
		{"IPMI SOL", "ipmi", true},
		{"Redfish Serial", "redfish_serial", true},
		{"Invalid", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sol := &SOLEndpoint{
				Type:     tt.solType,
				Endpoint: "test-endpoint",
			}

			// Check if type is one of the expected values
			isValid := sol.Type == "ipmi" || sol.Type == "redfish_serial"

			if isValid != tt.expected {
				t.Errorf("Expected valid=%v for type '%s'", tt.expected, tt.solType)
			}
		})
	}
}

func TestVNCEndpoint_Types(t *testing.T) {
	tests := []struct {
		name    string
		vncType string
		valid   bool
	}{
		{"BMC Native", "bmc_native", true},
		{"NoVNC Proxy", "novnc_proxy", true},
		{"External KVM", "external_kvm", true},
		{"Invalid", "invalid_type", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vnc := &VNCEndpoint{
				Type:     tt.vncType,
				Endpoint: "ws://test:6080",
			}

			// Check if type is one of the valid values
			validTypes := map[string]bool{
				"bmc_native":   true,
				"novnc_proxy":  true,
				"external_kvm": true,
			}

			isValid := validTypes[vnc.Type]

			if isValid != tt.valid {
				t.Errorf("Expected valid=%v for type '%s'", tt.valid, tt.vncType)
			}
		})
	}
}

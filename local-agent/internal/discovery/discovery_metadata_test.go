package discovery

import (
	"testing"

	"core/domain"
	"core/types"
	"local-agent/pkg/config"
)

func TestBuildDiscoveryMetadata(t *testing.T) {
	// Create a test service
	cfg := &config.Config{
		Agent: config.AgentConfig{
			ID: "test-agent-01",
		},
	}
	service := &Service{
		config: cfg,
	}

	tests := []struct {
		name             string
		server           *domain.Server
		discoveryMethod  types.DiscoveryMethod
		configSource     string
		wantProtocol     bool
		wantEndpoints    bool
		wantSecurity     bool
		wantNetwork      bool
		wantCapabilities bool
		wantVendor       bool
	}{
		{
			name: "Redfish server with TLS",
			server: &domain.Server{
				ID:         "test-server-1",
				CustomerID: "customer-1",
				ControlEndpoints: []*types.BMCControlEndpoint{
					{
						Endpoint: "https://192.168.1.100:8000",
						Type:     types.BMCTypeRedfish,
						Username: "admin",
						Password: "password",
						TLS: &types.TLSConfig{
							Enabled:            true,
							InsecureSkipVerify: true,
						},
					},
				},
				PrimaryProtocol: types.BMCTypeRedfish,
				SOLEndpoint: &types.SOLEndpoint{
					Type:     types.SOLTypeRedfishSerial,
					Endpoint: "https://192.168.1.100:8000/redfish/v1/Systems/1/SerialConsole",
					Username: "admin",
					Password: "password",
				},
				VNCEndpoint: &types.VNCEndpoint{
					Type:     types.VNCTypeWebSocket,
					Endpoint: "ws://novnc:6080/websockify",
					Username: "",
					Password: "vncpassword",
				},
				Features: []string{"power", "console", "vnc"},
				Status:   "active",
				Metadata: map[string]string{
					"vendor": "Dell",
				},
			},
			discoveryMethod:  types.DiscoveryMethodStaticConfig,
			configSource:     "config.yaml",
			wantProtocol:     true,
			wantEndpoints:    true,
			wantSecurity:     true,
			wantNetwork:      true,
			wantCapabilities: true,
			wantVendor:       true,
		},
		{
			name: "IPMI server without TLS",
			server: &domain.Server{
				ID:         "test-server-2",
				CustomerID: "customer-1",
				ControlEndpoints: []*types.BMCControlEndpoint{
					{
						Endpoint: "192.168.1.101:623",
						Type:     types.BMCTypeIPMI,
						Username: "admin",
						Password: "password",
					},
				},
				PrimaryProtocol: types.BMCTypeIPMI,
				SOLEndpoint: &types.SOLEndpoint{
					Type:     types.SOLTypeIPMI,
					Endpoint: "192.168.1.101:623",
					Username: "admin",
					Password: "password",
				},
				Features: []string{"power", "console"},
				Status:   "active",
				Metadata: make(map[string]string),
			},
			discoveryMethod:  types.DiscoveryMethodNetworkScan,
			configSource:     "auto-discovery",
			wantProtocol:     true,
			wantEndpoints:    true,
			wantSecurity:     true,
			wantNetwork:      true,
			wantCapabilities: true,
			wantVendor:       false,
		},
		{
			name: "Redfish with IPMI fallback",
			server: &domain.Server{
				ID:         "test-server-3",
				CustomerID: "customer-1",
				ControlEndpoints: []*types.BMCControlEndpoint{
					{
						Endpoint: "https://192.168.1.102:8000",
						Type:     types.BMCTypeRedfish,
						Username: "admin",
						Password: "password",
						TLS: &types.TLSConfig{
							Enabled:            true,
							InsecureSkipVerify: false,
						},
					},
				},
				PrimaryProtocol: types.BMCTypeRedfish,
				SOLEndpoint: &types.SOLEndpoint{
					Type:     types.SOLTypeIPMI,
					Endpoint: "192.168.1.102:623",
					Username: "admin",
					Password: "password",
				},
				Features: []string{"power", "console"},
				Status:   "active",
				Metadata: map[string]string{
					"vendor":       "Contoso",
					"sol_fallback": "ipmi",
				},
			},
			discoveryMethod:  types.DiscoveryMethodStaticConfig,
			configSource:     "config.yaml",
			wantProtocol:     true,
			wantEndpoints:    true,
			wantSecurity:     true,
			wantNetwork:      true,
			wantCapabilities: true,
			wantVendor:       true,
		},
		{
			name: "Server with discovery error",
			server: &domain.Server{
				ID:         "test-server-4",
				CustomerID: "customer-1",
				ControlEndpoints: []*types.BMCControlEndpoint{
					{
						Endpoint: "https://192.168.1.103:8000",
						Type:     types.BMCTypeRedfish,
						Username: "admin",
						Password: "password",
					},
				},
				PrimaryProtocol: types.BMCTypeRedfish,
				Features:        []string{"power"},
				Status:          "active",
				Metadata: map[string]string{
					"discovery_error": "failed to connect to BMC",
				},
			},
			discoveryMethod:  types.DiscoveryMethodNetworkScan,
			configSource:     "auto-discovery",
			wantProtocol:     true,
			wantEndpoints:    true,
			wantSecurity:     true,
			wantNetwork:      true,
			wantCapabilities: true,
			wantVendor:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := service.buildDiscoveryMetadata(tt.server, tt.discoveryMethod, tt.configSource)

			// Check basic fields
			if metadata.DiscoveryMethod != tt.discoveryMethod {
				t.Errorf("DiscoveryMethod = %v, want %v", metadata.DiscoveryMethod, tt.discoveryMethod)
			}

			if metadata.DiscoverySource != "test-agent-01" {
				t.Errorf("DiscoverySource = %v, want test-agent-01", metadata.DiscoverySource)
			}

			if metadata.ConfigSource != tt.configSource {
				t.Errorf("ConfigSource = %v, want %v", metadata.ConfigSource, tt.configSource)
			}

			// Check protocol configuration
			if tt.wantProtocol {
				if metadata.Protocol == nil {
					t.Error("Expected Protocol to be non-nil")
				} else {
					if metadata.Protocol.PrimaryProtocol != string(tt.server.GetPrimaryControlEndpoint().Type) {
						t.Errorf("Protocol.PrimaryProtocol = %v, want %v", metadata.Protocol.PrimaryProtocol, tt.server.GetPrimaryControlEndpoint().Type)
					}

					if tt.server.SOLEndpoint != nil {
						if metadata.Protocol.ConsoleType != string(tt.server.SOLEndpoint.Type) {
							t.Errorf("Protocol.ConsoleType = %v, want %v", metadata.Protocol.ConsoleType, tt.server.SOLEndpoint.Type)
						}
					}

					if tt.server.VNCEndpoint != nil {
						if metadata.Protocol.VNCTransport != string(tt.server.VNCEndpoint.Type) {
							t.Errorf("Protocol.VNCTransport = %v, want %v", metadata.Protocol.VNCTransport, tt.server.VNCEndpoint.Type)
						}
					}

					// Check for IPMI fallback
					if val, ok := tt.server.Metadata["sol_fallback"]; ok && val == "ipmi" {
						if metadata.Protocol.FallbackProtocol != "ipmi" {
							t.Errorf("Protocol.FallbackProtocol = %v, want ipmi", metadata.Protocol.FallbackProtocol)
						}
						if metadata.Protocol.FallbackReason == "" {
							t.Error("Expected Protocol.FallbackReason to be non-empty")
						}
					}
				}
			}

			// Check endpoint details
			if tt.wantEndpoints {
				if metadata.Endpoints == nil {
					t.Error("Expected Endpoints to be non-nil")
				} else {
					if metadata.Endpoints.ControlEndpoint != tt.server.GetPrimaryControlEndpoint().Endpoint {
						t.Errorf("Endpoints.ControlEndpoint = %v, want %v", metadata.Endpoints.ControlEndpoint, tt.server.GetPrimaryControlEndpoint().Endpoint)
					}

					if tt.server.SOLEndpoint != nil {
						if metadata.Endpoints.ConsoleEndpoint != tt.server.SOLEndpoint.Endpoint {
							t.Errorf("Endpoints.ConsoleEndpoint = %v, want %v", metadata.Endpoints.ConsoleEndpoint, tt.server.SOLEndpoint.Endpoint)
						}
					}

					if tt.server.VNCEndpoint != nil {
						if metadata.Endpoints.VNCEndpoint != tt.server.VNCEndpoint.Endpoint {
							t.Errorf("Endpoints.VNCEndpoint = %v, want %v", metadata.Endpoints.VNCEndpoint, tt.server.VNCEndpoint.Endpoint)
						}
					}
				}
			}

			// Check security configuration
			if tt.wantSecurity {
				if metadata.Security == nil {
					t.Error("Expected Security to be non-nil")
				} else {
					if len(tt.server.ControlEndpoints) > 0 && tt.server.GetPrimaryControlEndpoint().TLS != nil {
						if metadata.Security.TLSEnabled != tt.server.GetPrimaryControlEndpoint().TLS.Enabled {
							t.Errorf("Security.TLSEnabled = %v, want %v", metadata.Security.TLSEnabled, tt.server.GetPrimaryControlEndpoint().TLS.Enabled)
						}
						if metadata.Security.TLSVerify == tt.server.GetPrimaryControlEndpoint().TLS.InsecureSkipVerify {
							t.Errorf("Security.TLSVerify = %v, want %v", metadata.Security.TLSVerify, !tt.server.GetPrimaryControlEndpoint().TLS.InsecureSkipVerify)
						}
					}

					if tt.server.VNCEndpoint != nil && tt.server.VNCEndpoint.Password != "" {
						if metadata.Security.VNCAuthType != "password" {
							t.Errorf("Security.VNCAuthType = %v, want password", metadata.Security.VNCAuthType)
						}
						if metadata.Security.VNCPasswordLength != int32(len(tt.server.VNCEndpoint.Password)) {
							t.Errorf("Security.VNCPasswordLength = %v, want %v", metadata.Security.VNCPasswordLength, len(tt.server.VNCEndpoint.Password))
						}
					}
				}
			}

			// Check network information
			if tt.wantNetwork {
				if metadata.Network == nil {
					t.Error("Expected Network to be non-nil")
				} else {
					if !metadata.Network.Reachable {
						t.Error("Expected Network.Reachable to be true")
					}
					if metadata.Network.IPAddress == "" {
						t.Error("Expected Network.IPAddress to be non-empty")
					}
				}
			}

			// Check capabilities
			if tt.wantCapabilities {
				if metadata.Capabilities == nil {
					t.Error("Expected Capabilities to be non-nil")
				} else {
					if len(metadata.Capabilities.SupportedFeatures) != len(tt.server.Features) {
						t.Errorf("Capabilities.SupportedFeatures length = %v, want %v", len(metadata.Capabilities.SupportedFeatures), len(tt.server.Features))
					}

					if discoveryError, ok := tt.server.Metadata["discovery_error"]; ok {
						if len(metadata.Capabilities.DiscoveryErrors) == 0 {
							t.Error("Expected Capabilities.DiscoveryErrors to be non-empty")
						} else if metadata.Capabilities.DiscoveryErrors[0] != discoveryError {
							t.Errorf("Capabilities.DiscoveryErrors[0] = %v, want %v", metadata.Capabilities.DiscoveryErrors[0], discoveryError)
						}
					}
				}
			}

			// Check vendor information
			if tt.wantVendor {
				if metadata.Vendor == nil {
					t.Error("Expected Vendor to be non-nil")
				} else {
					if vendor, ok := tt.server.Metadata["vendor"]; ok {
						if metadata.Vendor.Manufacturer != vendor {
							t.Errorf("Vendor.Manufacturer = %v, want %v", metadata.Vendor.Manufacturer, vendor)
						}
					}
				}
			} else {
				if metadata.Vendor != nil {
					t.Error("Expected Vendor to be nil")
				}
			}
		})
	}
}

func TestDiscoveryMetadata_Integration(t *testing.T) {
	// Create a test service
	cfg := &config.Config{
		Agent: config.AgentConfig{
			ID: "test-agent-integration",
		},
		Static: config.StaticConfig{
			Hosts: []config.BMCHost{
				{
					ID:         "test-server-static",
					CustomerID: "customer-1",
					ControlEndpoints: []*config.ConfigBMCControlEndpoint{{
						Endpoint: "192.168.1.100:623", // Use IPMI to avoid Redfish client call
						Type:     "ipmi",
						Username: "admin",
						Password: "password",
					}},
					SOLEndpoint: &config.ConfigSOLEndpoint{
						Endpoint: "192.168.1.100:623",
						Type:     "ipmi",
						Username: "admin",
						Password: "password",
					},
					Features: []string{"power", "console"},
				},
			},
		},
	}

	// Create service (without actual clients since we're using IPMI)
	service := &Service{
		config: cfg,
	}

	// Load static servers
	servers := service.loadStaticServers()

	if len(servers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(servers))
	}

	server := servers[0]

	// Verify discovery metadata is populated
	if server.DiscoveryMetadata == nil {
		t.Fatal("Expected DiscoveryMetadata to be non-nil")
	}

	// Check discovery method
	if server.DiscoveryMetadata.DiscoveryMethod != types.DiscoveryMethodStaticConfig {
		t.Errorf("DiscoveryMethod = %v, want static_config", server.DiscoveryMetadata.DiscoveryMethod)
	}

	// Check discovery source
	if server.DiscoveryMetadata.DiscoverySource != "test-agent-integration" {
		t.Errorf("DiscoverySource = %v, want test-agent-integration", server.DiscoveryMetadata.DiscoverySource)
	}

	// Check discovered_at is set
	if server.DiscoveryMetadata.DiscoveredAt.IsZero() {
		t.Error("Expected DiscoveredAt to be non-zero")
	}

	// Check protocol is set
	if server.DiscoveryMetadata.Protocol == nil {
		t.Error("Expected Protocol to be non-nil")
	} else {
		if server.DiscoveryMetadata.Protocol.PrimaryProtocol != "ipmi" {
			t.Errorf("Protocol.PrimaryProtocol = %v, want ipmi", server.DiscoveryMetadata.Protocol.PrimaryProtocol)
		}
	}

	// Check endpoints are set
	if server.DiscoveryMetadata.Endpoints == nil {
		t.Error("Expected Endpoints to be non-nil")
	}

	// Check security is set
	if server.DiscoveryMetadata.Security == nil {
		t.Error("Expected Security to be non-nil")
	}

	// Check network is set
	if server.DiscoveryMetadata.Network == nil {
		t.Error("Expected Network to be non-nil")
	}

	// Check capabilities are set
	if server.DiscoveryMetadata.Capabilities == nil {
		t.Error("Expected Capabilities to be non-nil")
	}
}

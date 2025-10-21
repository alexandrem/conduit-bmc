package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"core/types"
)

func TestAgentConfigLoad(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test configuration file
	configFile := filepath.Join(tempDir, "agent.yaml")
	configContent := `
log:
  level: debug
  format: text

agent:
  id: test-agent-001
  name: Test Agent
  region: us-east-1
  bmc_discovery:
    enabled: false
    scan_interval: 10m
    network_ranges:
      - 10.0.1.0/24
      - 10.0.2.0/24
    ipmi_ports:
      - 623
      - 624
    redfish_ports:
      - 443
      - 8443
    scan_timeout: 20s
    max_concurrent: 100
    default_credentials:
      - username: testuser
        password: testpass
  bmc_operations:
    operation_timeout: 60s
    power_operation_timeout: 120s
    max_retries: 5
    max_concurrent_operations: 20
    ipmi:
      interface: lan
      cipher_suite: "17"
      privilege_level: USER
      sol_baud_rate: 57600
      sol_authentication: false
    redfish:
      http_timeout: 45s
      insecure_skip_verify: true
      auth_method: session
      session_timeout: 60m
  vnc:
    enabled: false
    port: 5901
    bind_address: 192.168.1.100
    max_connections: 10
    frame_rate: 30
    quality: 9
    enable_authentication: false
  serial_console:
    enabled: false
    default_baud_rate: 57600
    buffer_size: 16384
    max_sessions: 20
  connection_management:
    connect_timeout: 20s
    reconnect_interval: 60s
    heartbeat_interval: 45s
    max_connections: 200
  health_monitoring:
    enabled: false
    check_interval: 120s
    cpu_threshold: 90.0
    memory_threshold: 95.0
    disk_threshold: 95.0
  security:
    enable_tls_verification: false
    allowed_networks:
      - 192.168.0.0/16
      - 10.0.0.0/8
    deny_private_networks: true
    enable_audit_logging: false

tls:
  enabled: true

# Static hosts configuration (RFD 006 multi-protocol format)
static:
  hosts:
    - id: test-server-001
      customer_id: test-customer
      control_endpoints:
        - endpoint: https://192.168.1.100
          type: redfish
          username: admin
          password: password
      features:
        - power
        - sensors
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create environment file
	envFile := filepath.Join(tempDir, "agent.env")
	envContent := `
AGENT_GATEWAY_ENDPOINT=http://test-gateway:8081
AGENT_DATACENTER_ID=dc-test-01
AGENT_ENCRYPTION_KEY=test-encryption-key-32-characters
AGENT_ID=env-agent-001
`

	err = os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write env file: %v", err)
	}

	// Load configuration
	cfg, err := Load(configFile, envFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Cleanup environment variables set by environment file
	defer func() {
		os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
		os.Unsetenv("AGENT_DATACENTER_ID")
		os.Unsetenv("AGENT_ENCRYPTION_KEY")
		os.Unsetenv("AGENT_ID")
	}()

	// Test YAML values
	if cfg.Log.Level != "debug" {
		t.Errorf("Expected Log.Level 'debug', got '%s'", cfg.Log.Level)
	}

	// Test environment variable override
	if cfg.Agent.ID != "env-agent-001" {
		t.Errorf("Expected Agent.ID 'env-agent-001' (from env), got '%s'", cfg.Agent.ID)
	}

	if cfg.Agent.Name != "Test Agent" {
		t.Errorf("Expected Agent.Name 'Test Agent', got '%s'", cfg.Agent.Name)
	}

	if cfg.Agent.Region != "us-east-1" {
		t.Errorf("Expected Agent.Region 'us-east-1', got '%s'", cfg.Agent.Region)
	}

	if cfg.Agent.GatewayEndpoint != "http://test-gateway:8081" {
		t.Errorf("Expected GatewayEndpoint from env, got '%s'", cfg.Agent.GatewayEndpoint)
	}

	if cfg.Agent.DatacenterID != "dc-test-01" {
		t.Errorf("Expected DatacenterID from env, got '%s'", cfg.Agent.DatacenterID)
	}

	// Test BMC discovery configuration
	if cfg.Agent.BMCDiscovery.Enabled {
		t.Errorf("Expected BMCDiscovery.Enabled false, got %v", cfg.Agent.BMCDiscovery.Enabled)
	}

	if cfg.Agent.BMCDiscovery.ScanInterval != 10*time.Minute {
		t.Errorf("Expected ScanInterval 10m, got %v", cfg.Agent.BMCDiscovery.ScanInterval)
	}

	if len(cfg.Agent.BMCDiscovery.NetworkRanges) != 2 {
		t.Errorf("Expected 2 network ranges, got %d", len(cfg.Agent.BMCDiscovery.NetworkRanges))
	}

	if cfg.Agent.BMCDiscovery.NetworkRanges[0] != "10.0.1.0/24" {
		t.Errorf("Expected first network range '10.0.1.0/24', got '%s'", cfg.Agent.BMCDiscovery.NetworkRanges[0])
	}

	if len(cfg.Agent.BMCDiscovery.IPMIPorts) != 2 {
		t.Errorf("Expected 2 IPMI ports, got %d", len(cfg.Agent.BMCDiscovery.IPMIPorts))
	}

	if cfg.Agent.BMCDiscovery.IPMIPorts[1] != 624 {
		t.Errorf("Expected second IPMI port 624, got %d", cfg.Agent.BMCDiscovery.IPMIPorts[1])
	}

	if len(cfg.Agent.BMCDiscovery.DefaultCredentials) != 1 {
		t.Errorf("Expected 1 default credential, got %d", len(cfg.Agent.BMCDiscovery.DefaultCredentials))
	}

	if cfg.Agent.BMCDiscovery.DefaultCredentials[0].Username != "testuser" {
		t.Errorf("Expected credential username 'testuser', got '%s'", cfg.Agent.BMCDiscovery.DefaultCredentials[0].Username)
	}

	// Test BMC operations configuration
	if cfg.Agent.BMCOperations.OperationTimeout != 60*time.Second {
		t.Errorf("Expected OperationTimeout 60s, got %v", cfg.Agent.BMCOperations.OperationTimeout)
	}

	if cfg.Agent.BMCOperations.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries 5, got %d", cfg.Agent.BMCOperations.MaxRetries)
	}

	if cfg.Agent.BMCOperations.MaxConcurrentOperations != 20 {
		t.Errorf("Expected MaxConcurrentOperations 20, got %d", cfg.Agent.BMCOperations.MaxConcurrentOperations)
	}

	// Test IPMI configuration
	if cfg.Agent.BMCOperations.IPMIConfig.Interface != "lan" {
		t.Errorf("Expected IPMI Interface 'lan', got '%s'", cfg.Agent.BMCOperations.IPMIConfig.Interface)
	}

	if cfg.Agent.BMCOperations.IPMIConfig.CipherSuite != "17" {
		t.Errorf("Expected IPMI CipherSuite '17', got '%s'", cfg.Agent.BMCOperations.IPMIConfig.CipherSuite)
	}

	if cfg.Agent.BMCOperations.IPMIConfig.SOLBaudRate != 57600 {
		t.Errorf("Expected SOL BaudRate 57600, got %d", cfg.Agent.BMCOperations.IPMIConfig.SOLBaudRate)
	}

	if cfg.Agent.BMCOperations.IPMIConfig.SOLAuthentication {
		t.Errorf("Expected SOL Authentication false, got %v", cfg.Agent.BMCOperations.IPMIConfig.SOLAuthentication)
	}

	// Test Redfish configuration
	if cfg.Agent.BMCOperations.RedfishConfig.HTTPTimeout != 45*time.Second {
		t.Errorf("Expected Redfish HTTPTimeout 45s, got %v", cfg.Agent.BMCOperations.RedfishConfig.HTTPTimeout)
	}

	if !cfg.Agent.BMCOperations.RedfishConfig.InsecureSkipVerify {
		t.Errorf("Expected Redfish InsecureSkipVerify true, got %v", cfg.Agent.BMCOperations.RedfishConfig.InsecureSkipVerify)
	}

	if cfg.Agent.BMCOperations.RedfishConfig.AuthMethod != "session" {
		t.Errorf("Expected Redfish AuthMethod 'session', got '%s'", cfg.Agent.BMCOperations.RedfishConfig.AuthMethod)
	}

	// Test VNC configuration (from YAML)
	if cfg.Agent.VNCConfig.Port != 5901 {
		t.Errorf("Expected VNC Port 5901 (from YAML), got %d", cfg.Agent.VNCConfig.Port)
	}

	if cfg.Agent.VNCConfig.Enabled {
		t.Errorf("Expected VNC Enabled false, got %v", cfg.Agent.VNCConfig.Enabled)
	}

	if cfg.Agent.VNCConfig.BindAddress != "192.168.1.100" {
		t.Errorf("Expected VNC BindAddress '192.168.1.100', got '%s'", cfg.Agent.VNCConfig.BindAddress)
	}

	if cfg.Agent.VNCConfig.FrameRate != 30 {
		t.Errorf("Expected VNC FrameRate 30, got %d", cfg.Agent.VNCConfig.FrameRate)
	}

	// Test security configuration
	if cfg.Agent.Security.EnableTLSVerification {
		t.Errorf("Expected Security.EnableTLSVerification false, got %v", cfg.Agent.Security.EnableTLSVerification)
	}

	if len(cfg.Agent.Security.AllowedNetworks) != 2 {
		t.Errorf("Expected 2 allowed networks, got %d", len(cfg.Agent.Security.AllowedNetworks))
	}

	if !cfg.Agent.Security.DenyPrivateNetworks {
		t.Errorf("Expected Security.DenyPrivateNetworks true, got %v", cfg.Agent.Security.DenyPrivateNetworks)
	}

	// Test legacy static hosts
	if len(cfg.Static.Hosts) != 1 {
		t.Errorf("Expected 1 static host, got %d", len(cfg.Static.Hosts))
	}

	if cfg.Static.Hosts[0].ID != "test-server-001" {
		t.Errorf("Expected static host ID 'test-server-001', got '%s'", cfg.Static.Hosts[0].ID)
	}

	if len(cfg.Static.Hosts[0].ControlEndpoints) == 0 {
		t.Errorf("Expected static host to have at least one control endpoint, got 0")
	} else if cfg.Static.Hosts[0].ControlEndpoints[0].Type != "redfish" {
		t.Errorf("Expected static host type 'redfish', got '%s'", cfg.Static.Hosts[0].ControlEndpoints[0].Type)
	}
}

func TestAgentConfigDefaults(t *testing.T) {
	// Set required environment variables
	os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
	os.Setenv("AGENT_DATACENTER_ID", "dc-test")
	defer os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
	defer os.Unsetenv("AGENT_DATACENTER_ID")

	// Load with no config files (should use defaults)
	cfg, err := Load("", "")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test default values
	if cfg.Agent.Region != "default" {
		t.Errorf("Expected default Agent.Region 'default', got '%s'", cfg.Agent.Region)
	}

	// Test BMC discovery defaults
	if !cfg.Agent.BMCDiscovery.Enabled {
		t.Errorf("Expected default BMCDiscovery.Enabled true, got %v", cfg.Agent.BMCDiscovery.Enabled)
	}

	if cfg.Agent.BMCDiscovery.ScanInterval != 5*time.Minute {
		t.Errorf("Expected default ScanInterval 5m, got %v", cfg.Agent.BMCDiscovery.ScanInterval)
	}

	if cfg.Agent.BMCDiscovery.ScanTimeout != 10*time.Second {
		t.Errorf("Expected default ScanTimeout 10s, got %v", cfg.Agent.BMCDiscovery.ScanTimeout)
	}

	if cfg.Agent.BMCDiscovery.MaxConcurrent != 50 {
		t.Errorf("Expected default MaxConcurrent 50, got %d", cfg.Agent.BMCDiscovery.MaxConcurrent)
	}

	// Test BMC operations defaults
	if cfg.Agent.BMCOperations.OperationTimeout != 30*time.Second {
		t.Errorf("Expected default OperationTimeout 30s, got %v", cfg.Agent.BMCOperations.OperationTimeout)
	}

	if cfg.Agent.BMCOperations.MaxRetries != 3 {
		t.Errorf("Expected default MaxRetries 3, got %d", cfg.Agent.BMCOperations.MaxRetries)
	}

	// Test IPMI defaults
	if cfg.Agent.BMCOperations.IPMIConfig.Interface != "lanplus" {
		t.Errorf("Expected default IPMI Interface 'lanplus', got '%s'", cfg.Agent.BMCOperations.IPMIConfig.Interface)
	}

	if cfg.Agent.BMCOperations.IPMIConfig.SOLBaudRate != 115200 {
		t.Errorf("Expected default SOL BaudRate 115200, got %d", cfg.Agent.BMCOperations.IPMIConfig.SOLBaudRate)
	}

	// Test VNC defaults
	if !cfg.Agent.VNCConfig.Enabled {
		t.Errorf("Expected default VNC Enabled true, got %v", cfg.Agent.VNCConfig.Enabled)
	}

	if cfg.Agent.VNCConfig.Port != 5900 {
		t.Errorf("Expected default VNC Port 5900, got %d", cfg.Agent.VNCConfig.Port)
	}

	if cfg.Agent.VNCConfig.BindAddress != "127.0.0.1" {
		t.Errorf("Expected default VNC BindAddress '127.0.0.1', got '%s'", cfg.Agent.VNCConfig.BindAddress)
	}

	// Test serial console defaults
	if !cfg.Agent.SerialConsole.Enabled {
		t.Errorf("Expected default SerialConsole Enabled true, got %v", cfg.Agent.SerialConsole.Enabled)
	}

	if cfg.Agent.SerialConsole.DefaultBaudRate != 115200 {
		t.Errorf("Expected default SerialConsole BaudRate 115200, got %d", cfg.Agent.SerialConsole.DefaultBaudRate)
	}

	// Test health monitoring defaults
	if !cfg.Agent.HealthMonitoring.Enabled {
		t.Errorf("Expected default HealthMonitoring Enabled true, got %v", cfg.Agent.HealthMonitoring.Enabled)
	}

	if cfg.Agent.HealthMonitoring.CPUThreshold != 80.0 {
		t.Errorf("Expected default CPU threshold 80.0, got %v", cfg.Agent.HealthMonitoring.CPUThreshold)
	}

	// Test defaults are set for supported baud rates and flow control modes
	if len(cfg.Agent.SerialConsole.SupportedBaudRates) == 0 {
		t.Errorf("Expected default supported baud rates to be set")
	}

	if len(cfg.Agent.SerialConsole.FlowControlModes) == 0 {
		t.Errorf("Expected default flow control modes to be set")
	}
}

func TestAgentConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func()
		expectError bool
		errorText   string
	}{
		{
			name: "missing gateway endpoint",
			setupEnv: func() {
				os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
				os.Setenv("AGENT_DATACENTER_ID", "dc-test")
			},
			expectError: false,
		},
		{
			name: "missing datacenter ID",
			setupEnv: func() {
				os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
				os.Unsetenv("AGENT_DATACENTER_ID")
			},
			expectError: true,
			errorText:   "agent datacenter id is required",
		},
		{
			name: "invalid agent region",
			setupEnv: func() {
				os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
				os.Setenv("AGENT_DATACENTER_ID", "dc-test")
				os.Setenv("AGENT_REGION", "")
			},
			expectError: false, // Empty region will use default
			errorText:   "",
		},
		{
			name: "invalid VNC port",
			setupEnv: func() {
				os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
				os.Setenv("AGENT_DATACENTER_ID", "dc-test")
				// VNC port validation now happens at config file level, not env vars
			},
			expectError: false, // This test no longer relevant since VNC_PORT env var was removed
			errorText:   "",
		},
		{
			name: "valid configuration",
			setupEnv: func() {
				os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
				os.Setenv("AGENT_DATACENTER_ID", "dc-test")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment
			os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
			os.Unsetenv("AGENT_DATACENTER_ID")
			os.Unsetenv("BMC_DISCOVERY_NETWORK_RANGES")

			// Setup test environment
			tt.setupEnv()

			// Attempt to load configuration
			_, err := Load("", "")

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("Expected error containing '%s', got '%v'", tt.errorText, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestAgentConfigIPMIValidation(t *testing.T) {
	// Set required environment variables
	os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
	os.Setenv("AGENT_DATACENTER_ID", "dc-test")
	defer os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
	defer os.Unsetenv("AGENT_DATACENTER_ID")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "agent.yaml")

	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorText   string
	}{
		{
			name: "invalid IPMI interface",
			configYAML: `
agent:
  bmc_operations:
    ipmi:
      interface: invalid_interface
`,
			expectError: true,
			errorText:   "invalid IPMI interface: invalid_interface",
		},
		{
			name: "valid lan interface",
			configYAML: `
agent:
  bmc_operations:
    ipmi:
      interface: lan
`,
			expectError: false,
		},
		{
			name: "valid lanplus interface",
			configYAML: `
agent:
  bmc_operations:
    ipmi:
      interface: lanplus
`,
			expectError: false,
		},
		{
			name: "valid serial interface",
			configYAML: `
agent:
  bmc_operations:
    ipmi:
      interface: serial
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile(configFile, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			_, err = Load(configFile, "")

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("Expected error containing '%s', got '%v'", tt.errorText, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestAgentConfigVNCValidation(t *testing.T) {
	// Set required environment variables
	os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
	os.Setenv("AGENT_DATACENTER_ID", "dc-test")
	defer os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
	defer os.Unsetenv("AGENT_DATACENTER_ID")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "agent.yaml")

	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorText   string
	}{
		{
			name: "VNC frame rate too high",
			configYAML: `
agent:
  vnc:
    frame_rate: 100
`,
			expectError: true,
			errorText:   "VNC frame rate must be between 1 and 60",
		},
		{
			name: "VNC frame rate too low",
			configYAML: `
agent:
  vnc:
    frame_rate: 0
`,
			expectError: true,
			errorText:   "VNC frame rate must be between 1 and 60",
		},
		{
			name: "VNC quality too high",
			configYAML: `
agent:
  vnc:
    quality: 10
`,
			expectError: true,
			errorText:   "VNC quality must be between 0 and 9",
		},
		{
			name: "VNC quality too low",
			configYAML: `
agent:
  vnc:
    quality: -1
`,
			expectError: true,
			errorText:   "VNC quality must be between 0 and 9",
		},
		{
			name: "valid VNC config",
			configYAML: `
agent:
  vnc:
    frame_rate: 30
    quality: 6
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile(configFile, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			_, err = Load(configFile, "")

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("Expected error containing '%s', got '%v'", tt.errorText, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestAgentConfigHealthMonitoringValidation(t *testing.T) {
	// Set required environment variables
	os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
	os.Setenv("AGENT_DATACENTER_ID", "dc-test")
	defer os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
	defer os.Unsetenv("AGENT_DATACENTER_ID")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "agent.yaml")

	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorText   string
	}{
		{
			name: "CPU threshold too high",
			configYAML: `
agent:
  health_monitoring:
    cpu_threshold: 150.0
`,
			expectError: true,
			errorText:   "CPU threshold must be between 0 and 100",
		},
		{
			name: "memory threshold negative",
			configYAML: `
agent:
  health_monitoring:
    memory_threshold: -10.0
`,
			expectError: true,
			errorText:   "memory threshold must be between 0 and 100",
		},
		{
			name: "disk threshold too high",
			configYAML: `
agent:
  health_monitoring:
    disk_threshold: 101.0
`,
			expectError: true,
			errorText:   "disk threshold must be between 0 and 100",
		},
		{
			name: "valid health monitoring config",
			configYAML: `
agent:
  health_monitoring:
    cpu_threshold: 80.0
    memory_threshold: 85.0
    disk_threshold: 90.0
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile(configFile, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			_, err = Load(configFile, "")

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("Expected error containing '%s', got '%v'", tt.errorText, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestAgentConfigGetVNCListenAddress(t *testing.T) {
	// Set required environment variables
	os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
	os.Setenv("AGENT_DATACENTER_ID", "dc-test")
	defer os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
	defer os.Unsetenv("AGENT_DATACENTER_ID")

	tests := []struct {
		name        string
		bindAddress string
		port        int
		expected    string
	}{
		{
			name:        "default address",
			bindAddress: "127.0.0.1",
			port:        5900,
			expected:    "127.0.0.1:5900",
		},
		{
			name:        "all interfaces",
			bindAddress: "0.0.0.0",
			port:        5901,
			expected:    "0.0.0.0:5901",
		},
		{
			name:        "specific IP",
			bindAddress: "192.168.1.100",
			port:        5902,
			expected:    "192.168.1.100:5902",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "agent.yaml")

			configContent := fmt.Sprintf(`
agent:
  gateway_endpoint: http://localhost:8081
  datacenter_id: dc-test
  vnc:
    bind_address: %s
    port: %d
`, tt.bindAddress, tt.port)

			err := os.WriteFile(configFile, []byte(configContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			cfg, err := Load(configFile, "")
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}

			address := cfg.GetVNCListenAddress()
			if address != tt.expected {
				t.Errorf("Expected VNC address '%s', got '%s'", tt.expected, address)
			}
		})
	}
}

func TestBMCControlEndpointInferType(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		explicitType string
		expected     types.BMCType
	}{
		{
			name:         "HTTPS endpoint infers redfish",
			endpoint:     "https://192.168.1.100",
			explicitType: "",
			expected:     types.BMCTypeRedfish,
		},
		{
			name:         "HTTP endpoint infers redfish",
			endpoint:     "http://192.168.1.100",
			explicitType: "",
			expected:     types.BMCTypeRedfish,
		},
		{
			name:         "IPMI scheme infers ipmi",
			endpoint:     "ipmi://192.168.1.100",
			explicitType: "",
			expected:     types.BMCTypeIPMI,
		},
		{
			name:         "Host:port infers ipmi",
			endpoint:     "192.168.1.100:623",
			explicitType: "",
			expected:     types.BMCTypeIPMI,
		},
		{
			name:         "Explicit type overrides inference",
			endpoint:     "https://192.168.1.100",
			explicitType: types.BMCTypeIPMI.String(),
			expected:     types.BMCTypeIPMI,
		},
		{
			name:         "No scheme defaults to ipmi",
			endpoint:     "192.168.1.100",
			explicitType: "",
			expected:     types.BMCTypeIPMI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bmc := &ConfigBMCControlEndpoint{
				Type:     tt.explicitType,
				Endpoint: tt.endpoint,
			}

			result := bmc.ToTypesEndpoint().Type
			if result != tt.expected {
				t.Errorf("Expected type '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSOLEndpointInferType(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		explicitType string
		expected     types.SOLType
	}{
		{
			name:         "HTTPS endpoint infers redfish_serial",
			endpoint:     "https://192.168.1.100/redfish/v1/Systems/1/SerialInterfaces/1",
			explicitType: "",
			expected:     types.SOLTypeRedfishSerial,
		},
		{
			name:         "HTTP endpoint infers redfish_serial",
			endpoint:     "http://192.168.1.100/redfish/v1/Systems/1/SerialInterfaces/1",
			explicitType: "",
			expected:     types.SOLTypeRedfishSerial,
		},
		{
			name:         "IPMI scheme infers ipmi",
			endpoint:     "ipmi://192.168.1.100",
			explicitType: "",
			expected:     types.SOLTypeIPMI,
		},
		{
			name:         "Host:port infers ipmi",
			endpoint:     "192.168.1.100:623",
			explicitType: "",
			expected:     types.SOLTypeIPMI,
		},
		{
			name:         "Explicit type overrides inference",
			endpoint:     "https://192.168.1.100/sol",
			explicitType: types.SOLTypeIPMI.String(),
			expected:     types.SOLTypeIPMI,
		},
		{
			name:         "No scheme defaults to ipmi",
			endpoint:     "192.168.1.100",
			explicitType: "",
			expected:     types.SOLTypeIPMI,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sol := &ConfigSOLEndpoint{
				Type:     tt.explicitType,
				Endpoint: tt.endpoint,
			}

			result := sol.ToTypesEndpoint().Type
			if result != tt.expected {
				t.Errorf("Expected type '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestVNCEndpointInferType(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		explicitType string
		expected     types.VNCType
	}{
		{
			name:         "WS scheme infers websocket",
			endpoint:     "ws://192.168.1.100/kvm/0",
			explicitType: "",
			expected:     types.VNCTypeWebSocket,
		},
		{
			name:         "WSS scheme infers websocket",
			endpoint:     "wss://192.168.1.100/redfish/v1/Systems/1/GraphicalConsole",
			explicitType: "",
			expected:     types.VNCTypeWebSocket,
		},
		{
			name:         "VNC scheme infers native",
			endpoint:     "vnc://192.168.1.100:5900",
			explicitType: "",
			expected:     types.VNCTypeNative,
		},
		{
			name:         "Host:port infers native",
			endpoint:     "192.168.1.100:5900",
			explicitType: "",
			expected:     types.VNCTypeNative,
		},
		{
			name:         "Explicit type overrides inference",
			endpoint:     "ws://192.168.1.100/vnc",
			explicitType: types.VNCTypeNative.String(),
			expected:     types.VNCTypeNative,
		},
		{
			name:         "No scheme defaults to native",
			endpoint:     "192.168.1.100",
			explicitType: "",
			expected:     types.VNCTypeNative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vnc := &ConfigVNCEndpoint{
				Type:     tt.explicitType,
				Endpoint: tt.endpoint,
			}

			result := vnc.ToTypesEndpoint().Type
			if result != tt.expected {
				t.Errorf("Expected type '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestStaticHostTypeInference(t *testing.T) {
	// Set required environment variables
	os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
	os.Setenv("AGENT_DATACENTER_ID", "dc-test")
	defer os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
	defer os.Unsetenv("AGENT_DATACENTER_ID")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "agent.yaml")

	// Config without explicit type fields
	configContent := `
static:
  hosts:
    - id: test-server-redfish
      control_endpoint:
        endpoint: https://192.168.1.100
        type: redfish
      sol_endpoint:
        endpoint: https://192.168.1.100/redfish/v1/Systems/1/SerialInterfaces/1
        username: admin
        password: password
      vnc_endpoint:
        endpoint: wss://192.168.1.100/redfish/v1/Systems/1/GraphicalConsole
        username: admin
        password: password
    - id: test-server-ipmi
      control_endpoint:
        endpoint: 192.168.1.101
        type: ipmi
      sol_endpoint:
        endpoint: 192.168.1.101:623
        username: admin
        password: password
      vnc_endpoint:
        endpoint: vnc://192.168.1.101:5900
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile, "")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Static.Hosts) != 2 {
		t.Fatalf("Expected 2 static hosts, got %d", len(cfg.Static.Hosts))
	}

	// Test Redfish server
	redfishHost := cfg.Static.Hosts[0]
	if redfishHost.SOLEndpoint.ToTypesEndpoint().Type != types.SOLTypeRedfishSerial {
		t.Errorf("Expected SOL type '%s' for HTTPS endpoint, got '%s'", types.SOLTypeRedfishSerial, redfishHost.SOLEndpoint.ToTypesEndpoint().Type)
	}
	if redfishHost.VNCEndpoint.ToTypesEndpoint().Type != types.VNCTypeWebSocket {
		t.Errorf("Expected VNC type '%s' for WSS endpoint, got '%s'", types.VNCTypeWebSocket, redfishHost.VNCEndpoint.ToTypesEndpoint().Type)
	}

	// Test IPMI server
	ipmiHost := cfg.Static.Hosts[1]
	if ipmiHost.SOLEndpoint.ToTypesEndpoint().Type != types.SOLTypeIPMI {
		t.Errorf("Expected SOL type '%s' for host:port endpoint, got '%s'", types.SOLTypeIPMI, ipmiHost.SOLEndpoint.ToTypesEndpoint().Type)
	}
	if ipmiHost.VNCEndpoint.ToTypesEndpoint().Type != types.VNCTypeNative {
		t.Errorf("Expected VNC type '%s' for vnc:// endpoint, got '%s'", types.VNCTypeNative, ipmiHost.VNCEndpoint.ToTypesEndpoint().Type)
	}
}

func TestAgentConfigLegacyMethods(t *testing.T) {
	// Set required environment variables
	os.Setenv("AGENT_GATEWAY_ENDPOINT", "http://localhost:8081")
	os.Setenv("AGENT_DATACENTER_ID", "dc-test")
	defer os.Unsetenv("AGENT_GATEWAY_ENDPOINT")
	defer os.Unsetenv("AGENT_DATACENTER_ID")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "agent.yaml")
	configContent := `
static:
  hosts:
    - id: test-server
      control_endpoints:
        - endpoint: https://192.168.1.100
          type: redfish
      sol_endpoint:
        endpoint: https://192.168.1.100/sol
        type: redfish_serial
      vnc_endpoint:
        endpoint: vnc://192.168.1.100:5900
        type: novnc_proxy
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configFile, "")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Static.Hosts) != 1 {
		t.Fatalf("Expected 1 static host, got %d", len(cfg.Static.Hosts))
	}

	host := cfg.Static.Hosts[0]

	// Test legacy methods
	controlEndpoint := host.GetControlEndpoint()
	if controlEndpoint != "https://192.168.1.100" {
		t.Errorf("Expected control endpoint 'https://192.168.1.100', got '%s'", controlEndpoint)
	}

	solEndpoint := host.GetSOLEndpoint()
	if solEndpoint != "https://192.168.1.100/sol" {
		t.Errorf("Expected SOL endpoint 'https://192.168.1.100/sol', got '%s'", solEndpoint)
	}

	vncEndpoint := host.GetVNCEndpoint()
	if vncEndpoint != "vnc://192.168.1.100:5900" {
		t.Errorf("Expected VNC endpoint 'vnc://192.168.1.100:5900', got '%s'", vncEndpoint)
	}

	// Test with nil endpoints
	emptyHost := BMCHost{}
	if emptyHost.GetControlEndpoint() != "" {
		t.Errorf("Expected empty control endpoint for nil, got '%s'", emptyHost.GetControlEndpoint())
	}

	if emptyHost.GetSOLEndpoint() != "" {
		t.Errorf("Expected empty SOL endpoint for nil, got '%s'", emptyHost.GetSOLEndpoint())
	}

	if emptyHost.GetVNCEndpoint() != "" {
		t.Errorf("Expected empty VNC endpoint for nil, got '%s'", emptyHost.GetVNCEndpoint())
	}
}

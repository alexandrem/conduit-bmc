package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGatewayConfigLoad(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test configuration file
	configFile := filepath.Join(tempDir, "gateway.yaml")
	configContent := `
log:
  level: debug
  format: text

gateway:
  host: 127.0.0.1
  port: 9091
  region: us-west-2
  datacenters:
    - dc-01
    - dc-02
  proxy:
    read_timeout: 60s
    write_timeout: 60s
    bmc_timeout: 120s
    max_retries: 5
  websocket:
    read_buffer_size: 8192
    write_buffer_size: 8192
    vnc_frame_rate: 30
    vnc_quality: 8
  session_management:
    proxy_session_ttl: 2h
    vnc_session_ttl: 8h
    redis_database: 1
    use_in_memory_store: false
  webui:
    enabled: false
    title: Test BMC Console
    theme_color: "#ff0000"
  agent_connections:
    max_connections: 200
    connection_timeout: 45s
    load_balancer:
      algorithm: least_conn
      health_check_enabled: false
  rate_limit:
    enabled: false
    requests_per_minute: 2000

tls:
  enabled: true
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create environment file
	envFile := filepath.Join(tempDir, "gateway.env")
	envContent := `
BMC_MANAGER_ENDPOINT=http://test-manager:8080
GATEWAY_PORT=8888
REDIS_ENDPOINT=localhost:6379
REDIS_PASSWORD=test-password
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

	// Test YAML values
	if cfg.Log.Level != "debug" {
		t.Errorf("Expected Log.Level 'debug', got '%s'", cfg.Log.Level)
	}

	if cfg.Gateway.Host != "127.0.0.1" {
		t.Errorf("Expected Gateway.Host '127.0.0.1', got '%s'", cfg.Gateway.Host)
	}

	// Test environment variable override
	if cfg.Gateway.Port != 8888 {
		t.Errorf("Expected Gateway.Port 8888 (from env), got %d", cfg.Gateway.Port)
	}

	if cfg.Gateway.ManagerEndpoint != "http://test-manager:8080" {
		t.Errorf("Expected ManagerEndpoint from env, got '%s'", cfg.Gateway.ManagerEndpoint)
	}

	if cfg.Gateway.Region != "us-west-2" {
		t.Errorf("Expected Region 'us-west-2', got '%s'", cfg.Gateway.Region)
	}

	if len(cfg.Gateway.Datacenters) != 2 {
		t.Errorf("Expected 2 datacenters, got %d", len(cfg.Gateway.Datacenters))
	}

	if cfg.Gateway.Datacenters[0] != "dc-01" {
		t.Errorf("Expected first datacenter 'dc-01', got '%s'", cfg.Gateway.Datacenters[0])
	}

	// Test proxy configuration
	if cfg.Gateway.Proxy.ReadTimeout != 60*time.Second {
		t.Errorf("Expected Proxy.ReadTimeout 60s, got %v", cfg.Gateway.Proxy.ReadTimeout)
	}

	if cfg.Gateway.Proxy.BMCTimeout != 120*time.Second {
		t.Errorf("Expected Proxy.BMCTimeout 120s, got %v", cfg.Gateway.Proxy.BMCTimeout)
	}

	if cfg.Gateway.Proxy.MaxRetries != 5 {
		t.Errorf("Expected Proxy.MaxRetries 5, got %d", cfg.Gateway.Proxy.MaxRetries)
	}

	// Test WebSocket configuration
	if cfg.Gateway.WebSocket.ReadBufferSize != 8192 {
		t.Errorf("Expected WebSocket.ReadBufferSize 8192, got %d", cfg.Gateway.WebSocket.ReadBufferSize)
	}

	if cfg.Gateway.WebSocket.VNCFrameRate != 30 {
		t.Errorf("Expected WebSocket.VNCFrameRate 30, got %d", cfg.Gateway.WebSocket.VNCFrameRate)
	}

	if cfg.Gateway.WebSocket.VNCQuality != 8 {
		t.Errorf("Expected WebSocket.VNCQuality 8, got %d", cfg.Gateway.WebSocket.VNCQuality)
	}

	// Test session management
	if cfg.Gateway.SessionManagement.ProxySessionTTL != 2*time.Hour {
		t.Errorf("Expected ProxySessionTTL 2h, got %v", cfg.Gateway.SessionManagement.ProxySessionTTL)
	}

	if cfg.Gateway.SessionManagement.VNCSessionTTL != 8*time.Hour {
		t.Errorf("Expected VNCSessionTTL 8h, got %v", cfg.Gateway.SessionManagement.VNCSessionTTL)
	}

	if cfg.Gateway.SessionManagement.UseInMemoryStore {
		t.Errorf("Expected UseInMemoryStore false, got %v", cfg.Gateway.SessionManagement.UseInMemoryStore)
	}

	// Test Web UI configuration
	if cfg.Gateway.WebUI.Enabled {
		t.Errorf("Expected WebUI.Enabled false, got %v", cfg.Gateway.WebUI.Enabled)
	}

	if cfg.Gateway.WebUI.Title != "Test BMC Console" {
		t.Errorf("Expected WebUI.Title 'Test BMC Console', got '%s'", cfg.Gateway.WebUI.Title)
	}

	// Test agent connections
	if cfg.Gateway.AgentConnections.MaxConnections != 200 {
		t.Errorf("Expected AgentConnections.MaxConnections 200, got %d", cfg.Gateway.AgentConnections.MaxConnections)
	}

	if cfg.Gateway.AgentConnections.ConnectionTimeout != 45*time.Second {
		t.Errorf("Expected ConnectionTimeout 45s, got %v", cfg.Gateway.AgentConnections.ConnectionTimeout)
	}

	// Test rate limiting
	if cfg.Gateway.RateLimit.Enabled {
		t.Errorf("Expected RateLimit.Enabled false, got %v", cfg.Gateway.RateLimit.Enabled)
	}

	if cfg.Gateway.RateLimit.RequestsPerMinute != 2000 {
		t.Errorf("Expected RequestsPerMinute 2000, got %d", cfg.Gateway.RateLimit.RequestsPerMinute)
	}

	// Gateway auth config is minimal (just JWT secret validation)

	// Test TLS config
	if !cfg.TLS.Enabled {
		t.Errorf("Expected TLS.Enabled true, got %v", cfg.TLS.Enabled)
	}
}

func TestGatewayConfigDefaults(t *testing.T) {
	// Clean up any potential environment variable pollution
	os.Unsetenv("GATEWAY_PORT")
	os.Unsetenv("GATEWAY_HOST")
	defer os.Unsetenv("GATEWAY_PORT")
	defer os.Unsetenv("GATEWAY_HOST")

	// Set required environment variables
	os.Setenv("BMC_MANAGER_ENDPOINT", "http://localhost:8080")
	defer os.Unsetenv("BMC_MANAGER_ENDPOINT")

	// Load with no config files (should use defaults)
	cfg, err := Load("", "")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test default values
	if cfg.Gateway.Host != "0.0.0.0" {
		t.Errorf("Expected default Gateway.Host '0.0.0.0', got '%s'", cfg.Gateway.Host)
	}

	if cfg.Gateway.Port != 8081 {
		t.Errorf("Expected default Gateway.Port 8081, got %d", cfg.Gateway.Port)
	}

	if cfg.Gateway.Region != "default" {
		t.Errorf("Expected default Gateway.Region 'default', got '%s'", cfg.Gateway.Region)
	}

	// Test proxy defaults
	if cfg.Gateway.Proxy.ReadTimeout != 30*time.Second {
		t.Errorf("Expected default Proxy.ReadTimeout 30s, got %v", cfg.Gateway.Proxy.ReadTimeout)
	}

	if cfg.Gateway.Proxy.BMCTimeout != 60*time.Second {
		t.Errorf("Expected default Proxy.BMCTimeout 60s, got %v", cfg.Gateway.Proxy.BMCTimeout)
	}

	if cfg.Gateway.Proxy.MaxRetries != 3 {
		t.Errorf("Expected default Proxy.MaxRetries 3, got %d", cfg.Gateway.Proxy.MaxRetries)
	}

	// Test WebSocket defaults
	if cfg.Gateway.WebSocket.ReadBufferSize != 4096 {
		t.Errorf("Expected default WebSocket.ReadBufferSize 4096, got %d", cfg.Gateway.WebSocket.ReadBufferSize)
	}

	if cfg.Gateway.WebSocket.VNCFrameRate != 15 {
		t.Errorf("Expected default WebSocket.VNCFrameRate 15, got %d", cfg.Gateway.WebSocket.VNCFrameRate)
	}

	// Test session management defaults
	if cfg.Gateway.SessionManagement.ProxySessionTTL != 1*time.Hour {
		t.Errorf("Expected default ProxySessionTTL 1h, got %v", cfg.Gateway.SessionManagement.ProxySessionTTL)
	}

	if cfg.Gateway.SessionManagement.VNCSessionTTL != 4*time.Hour {
		t.Errorf("Expected default VNCSessionTTL 4h, got %v", cfg.Gateway.SessionManagement.VNCSessionTTL)
	}

	if !cfg.Gateway.SessionManagement.UseInMemoryStore {
		t.Errorf("Expected default UseInMemoryStore true, got %v", cfg.Gateway.SessionManagement.UseInMemoryStore)
	}

	// Test Web UI defaults
	if !cfg.Gateway.WebUI.Enabled {
		t.Errorf("Expected default WebUI.Enabled true, got %v", cfg.Gateway.WebUI.Enabled)
	}

	if cfg.Gateway.WebUI.Title != "BMC Management Console" {
		t.Errorf("Expected default WebUI.Title 'BMC Management Console', got '%s'", cfg.Gateway.WebUI.Title)
	}

	// Test agent connections defaults
	if cfg.Gateway.AgentConnections.MaxConnections != 100 {
		t.Errorf("Expected default AgentConnections.MaxConnections 100, got %d", cfg.Gateway.AgentConnections.MaxConnections)
	}

	// Test rate limiting defaults
	if !cfg.Gateway.RateLimit.Enabled {
		t.Errorf("Expected default RateLimit.Enabled true, got %v", cfg.Gateway.RateLimit.Enabled)
	}
}

func TestGatewayConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func()
		expectError bool
		errorText   string
	}{
		{
			name: "missing manager endpoint",
			setupEnv: func() {
				os.Unsetenv("BMC_MANAGER_ENDPOINT")
			},
			expectError: false,
		},
		{
			name: "invalid port",
			setupEnv: func() {
				os.Setenv("BMC_MANAGER_ENDPOINT", "http://localhost:8080")
				os.Setenv("GATEWAY_PORT", "70000")
			},
			expectError: true,
			errorText:   "gateway port must be between 1 and 65535",
		},
		{
			name: "valid configuration",
			setupEnv: func() {
				os.Setenv("BMC_MANAGER_ENDPOINT", "http://localhost:8080")
				os.Setenv("GATEWAY_PORT", "8081")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment
			os.Unsetenv("BMC_MANAGER_ENDPOINT")
			os.Unsetenv("GATEWAY_PORT")

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

func TestGatewayConfigWebSocketValidation(t *testing.T) {
	// Set required environment variables
	os.Setenv("BMC_MANAGER_ENDPOINT", "http://localhost:8080")
	defer os.Unsetenv("BMC_MANAGER_ENDPOINT")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "gateway.yaml")

	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorText   string
	}{
		{
			name: "zero read buffer size",
			configYAML: `
gateway:
  websocket:
    read_buffer_size: 0
`,
			expectError: true,
			errorText:   "WebSocket read buffer size must be positive",
		},
		{
			name: "zero write buffer size",
			configYAML: `
gateway:
  websocket:
    write_buffer_size: 0
`,
			expectError: true,
			errorText:   "WebSocket write buffer size must be positive",
		},
		{
			name: "VNC frame rate too high",
			configYAML: `
gateway:
  websocket:
    vnc_frame_rate: 100
`,
			expectError: true,
			errorText:   "VNC frame rate must be between 1 and 60",
		},
		{
			name: "VNC frame rate too low",
			configYAML: `
gateway:
  websocket:
    vnc_frame_rate: 0
`,
			expectError: true,
			errorText:   "VNC frame rate must be between 1 and 60",
		},
		{
			name: "valid WebSocket config",
			configYAML: `
gateway:
  websocket:
    read_buffer_size: 4096
    write_buffer_size: 4096
    vnc_frame_rate: 30
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

func TestGatewayConfigGetListenAddress(t *testing.T) {
	// Set required environment variables
	os.Setenv("BMC_MANAGER_ENDPOINT", "http://localhost:8080")
	defer os.Unsetenv("BMC_MANAGER_ENDPOINT")

	tests := []struct {
		name     string
		host     string
		port     string
		expected string
	}{
		{
			name:     "default address",
			host:     "0.0.0.0",
			port:     "8081",
			expected: "0.0.0.0:8081",
		},
		{
			name:     "localhost address",
			host:     "127.0.0.1",
			port:     "9091",
			expected: "127.0.0.1:9091",
		},
		{
			name:     "all interfaces (empty host)",
			host:     "",
			port:     "8081",
			expected: ":8081",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("GATEWAY_HOST", tt.host)
			defer os.Unsetenv("GATEWAY_HOST")
			os.Setenv("GATEWAY_PORT", tt.port)
			defer os.Unsetenv("GATEWAY_PORT")

			cfg, err := Load("", "")
			if err != nil {
				t.Fatalf("Load failed: %v", err)
			}

			address := cfg.GetListenAddress()
			if address != tt.expected {
				t.Errorf("Expected address '%s', got '%s'", tt.expected, address)
			}
		})
	}
}

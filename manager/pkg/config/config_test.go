package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagerConfigLoad(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test configuration file
	configFile := filepath.Join(tempDir, "manager.yaml")
	configContent := `
log:
  level: debug
  format: text

manager:
  host: 127.0.0.1
  port: 9090
  gateway_discovery:
    enabled: false
    update_interval: 60s
  server_management:
    auto_registration: false
    max_servers_per_customer: 50
  customer_management:
    allow_self_registration: true
    password_min_length: 12
  rate_limit:
    enabled: false
    requests_per_minute: 200

database:
  driver: postgres
  max_open_conns: 50

auth:
  token_ttl: 12h
  refresh_token_ttl: 336h

tls:
  enabled: true
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create environment file
	envFile := filepath.Join(tempDir, "manager.env")
	envContent := `
JWT_SECRET_KEY=test-jwt-secret-key-at-least-32-characters-long
DATABASE_URL=postgres://test:test@localhost/test_db
MANAGER_PORT=8888
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
		os.Unsetenv("JWT_SECRET_KEY")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("MANAGER_PORT")
	}()

	// Test YAML values
	if cfg.Log.Level != "debug" {
		t.Errorf("Expected Log.Level 'debug', got '%s'", cfg.Log.Level)
	}

	if cfg.Log.Format != "text" {
		t.Errorf("Expected Log.Format 'text', got '%s'", cfg.Log.Format)
	}

	if cfg.Manager.Host != "127.0.0.1" {
		t.Errorf("Expected Manager.Host '127.0.0.1', got '%s'", cfg.Manager.Host)
	}

	// Test environment variable override
	if cfg.Manager.Port != 8888 {
		t.Errorf("Expected Manager.Port 8888 (from env), got %d", cfg.Manager.Port)
	}

	if cfg.Manager.GatewayDiscovery.Enabled != false {
		t.Errorf("Expected GatewayDiscovery.Enabled false, got %v", cfg.Manager.GatewayDiscovery.Enabled)
	}

	if cfg.Manager.GatewayDiscovery.UpdateInterval != 60*time.Second {
		t.Errorf("Expected GatewayDiscovery.UpdateInterval 60s, got %v", cfg.Manager.GatewayDiscovery.UpdateInterval)
	}

	// Test server management
	if cfg.Manager.ServerManagement.AutoRegistration {
		t.Errorf("Expected ServerManagement.AutoRegistration false, got %v", cfg.Manager.ServerManagement.AutoRegistration)
	}

	if cfg.Manager.ServerManagement.MaxServersPerCustomer != 50 {
		t.Errorf("Expected MaxServersPerCustomer 50, got %d", cfg.Manager.ServerManagement.MaxServersPerCustomer)
	}

	// Test customer management
	if !cfg.Manager.CustomerManagement.AllowSelfRegistration {
		t.Errorf("Expected AllowSelfRegistration true, got %v", cfg.Manager.CustomerManagement.AllowSelfRegistration)
	}

	if cfg.Manager.CustomerManagement.PasswordMinLength != 12 {
		t.Errorf("Expected PasswordMinLength 12, got %d", cfg.Manager.CustomerManagement.PasswordMinLength)
	}

	// Test rate limiting
	if cfg.Manager.RateLimit.Enabled {
		t.Errorf("Expected RateLimit.Enabled false, got %v", cfg.Manager.RateLimit.Enabled)
	}

	if cfg.Manager.RateLimit.RequestsPerMinute != 200 {
		t.Errorf("Expected RequestsPerMinute 200, got %d", cfg.Manager.RateLimit.RequestsPerMinute)
	}

	// Test database config
	if cfg.Database.Driver != "postgres" {
		t.Errorf("Expected Database.Driver 'postgres', got '%s'", cfg.Database.Driver)
	}

	if cfg.Database.DSN != "postgres://test:test@localhost/test_db" {
		t.Errorf("Expected Database.DSN from env, got '%s'", cfg.Database.DSN)
	}

	if cfg.Database.MaxOpenConns != 50 {
		t.Errorf("Expected Database.MaxOpenConns 50, got %d", cfg.Database.MaxOpenConns)
	}

	// Test auth config
	if cfg.Auth.JWTSecretKey != "test-jwt-secret-key-at-least-32-characters-long" {
		t.Errorf("Expected JWT secret from env, got '%s'", cfg.Auth.JWTSecretKey)
	}

	if cfg.Auth.TokenTTL != 12*time.Hour {
		t.Errorf("Expected Auth.TokenTTL 12h, got %v", cfg.Auth.TokenTTL)
	}

	if cfg.Auth.RefreshTokenTTL != 336*time.Hour {
		t.Errorf("Expected Auth.RefreshTokenTTL 336h, got %v", cfg.Auth.RefreshTokenTTL)
	}

	// Test TLS config
	if !cfg.TLS.Enabled {
		t.Errorf("Expected TLS.Enabled true, got %v", cfg.TLS.Enabled)
	}
}

func TestManagerConfigDefaults(t *testing.T) {
	// Set required environment variables but unset DATABASE_URL to test default
	os.Setenv("JWT_SECRET_KEY", "test-jwt-secret-key-at-least-32-characters-long")
	os.Unsetenv("DATABASE_URL") // Unset to test default value
	defer os.Unsetenv("JWT_SECRET_KEY")

	// Load with no config files (should use defaults)
	cfg, err := Load("", "")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test default values
	if cfg.Manager.Host != "0.0.0.0" {
		t.Errorf("Expected default Manager.Host '0.0.0.0', got '%s'", cfg.Manager.Host)
	}

	if cfg.Manager.Port != 8080 {
		t.Errorf("Expected default Manager.Port 8080, got %d", cfg.Manager.Port)
	}

	if !cfg.Manager.GatewayDiscovery.Enabled {
		t.Errorf("Expected default GatewayDiscovery.Enabled true, got %v", cfg.Manager.GatewayDiscovery.Enabled)
	}

	if cfg.Manager.GatewayDiscovery.UpdateInterval != 30*time.Second {
		t.Errorf("Expected default GatewayDiscovery.UpdateInterval 30s, got %v", cfg.Manager.GatewayDiscovery.UpdateInterval)
	}

	if cfg.Manager.ServerManagement.MaxServersPerCustomer != 100 {
		t.Errorf("Expected default MaxServersPerCustomer 100, got %d", cfg.Manager.ServerManagement.MaxServersPerCustomer)
	}

	if cfg.Manager.CustomerManagement.PasswordMinLength != 8 {
		t.Errorf("Expected default PasswordMinLength 8, got %d", cfg.Manager.CustomerManagement.PasswordMinLength)
	}

	if !cfg.Manager.RateLimit.Enabled {
		t.Errorf("Expected default RateLimit.Enabled true, got %v", cfg.Manager.RateLimit.Enabled)
	}

	if cfg.Manager.RateLimit.RequestsPerMinute != 100 {
		t.Errorf("Expected default RequestsPerMinute 100, got %d", cfg.Manager.RateLimit.RequestsPerMinute)
	}

	// Test common config defaults
	if cfg.Log.Level != "info" {
		t.Errorf("Expected default Log.Level 'info', got '%s'", cfg.Log.Level)
	}

	// Test database defaults
	if cfg.Database.Driver != "sqlite3" {
		t.Errorf("Expected default Database.Driver 'sqlite3', got '%s'", cfg.Database.Driver)
	}

	if cfg.Database.DSN != "file:./manager.db" {
		t.Errorf("Expected default Database.DSN 'file:./manager.db', got '%s'", cfg.Database.DSN)
	}
}

func TestManagerConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func()
		expectError bool
		errorText   string
	}{
		{
			name: "missing JWT secret",
			setupEnv: func() {
				os.Unsetenv("JWT_SECRET_KEY")
				os.Setenv("DATABASE_URL", "file:./test.db")
			},
			expectError: true,
			errorText:   "JWT_SECRET_KEY environment variable is required",
		},
		{
			name: "short JWT secret",
			setupEnv: func() {
				os.Setenv("JWT_SECRET_KEY", "short")
				os.Setenv("DATABASE_URL", "file:./test.db")
			},
			expectError: true,
			errorText:   "JWT_SECRET_KEY must be at least 32 characters long",
		},
		{
			name: "missing database URL uses default",
			setupEnv: func() {
				os.Setenv("JWT_SECRET_KEY", "test-jwt-secret-key-at-least-32-characters-long")
				os.Unsetenv("DATABASE_URL")
			},
			expectError: false,
			errorText:   "",
		},
		{
			name: "invalid port",
			setupEnv: func() {
				os.Setenv("JWT_SECRET_KEY", "test-jwt-secret-key-at-least-32-characters-long")
				os.Setenv("DATABASE_URL", "file:./test.db")
				os.Setenv("MANAGER_PORT", "70000")
			},
			expectError: true,
			errorText:   "manager port must be between 1 and 65535",
		},
		{
			name: "valid configuration",
			setupEnv: func() {
				os.Setenv("JWT_SECRET_KEY", "test-jwt-secret-key-at-least-32-characters-long")
				os.Setenv("DATABASE_URL", "file:./test.db")
				os.Setenv("MANAGER_PORT", "8080")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment
			os.Unsetenv("JWT_SECRET_KEY")
			os.Unsetenv("DATABASE_URL")
			os.Unsetenv("MANAGER_PORT")

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

func TestManagerConfigGetListenAddress(t *testing.T) {
	// Set required environment variables
	os.Setenv("JWT_SECRET_KEY", "test-jwt-secret-key-at-least-32-characters-long")
	os.Setenv("DATABASE_URL", "file:./test.db")
	defer os.Unsetenv("JWT_SECRET_KEY")
	defer os.Unsetenv("DATABASE_URL")

	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "default address",
			host:     "0.0.0.0",
			port:     8080,
			expected: "0.0.0.0:8080",
		},
		{
			name:     "localhost address",
			host:     "127.0.0.1",
			port:     9090,
			expected: "127.0.0.1:9090",
		},
		{
			name:     "all interfaces",
			host:     "",
			port:     8080,
			expected: ":8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("MANAGER_HOST", tt.host)
			os.Setenv("MANAGER_PORT", fmt.Sprintf("%d", tt.port))
			defer os.Unsetenv("MANAGER_HOST")
			defer os.Unsetenv("MANAGER_PORT")

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

func TestManagerConfigRateLimitValidation(t *testing.T) {
	// Set required environment variables
	os.Setenv("JWT_SECRET_KEY", "test-jwt-secret-key-at-least-32-characters-long")
	os.Setenv("DATABASE_URL", "file:./test.db")
	defer os.Unsetenv("JWT_SECRET_KEY")
	defer os.Unsetenv("DATABASE_URL")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "manager.yaml")

	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorText   string
	}{
		{
			name: "negative requests per minute",
			configYAML: `
manager:
  rate_limit:
    enabled: true
    requests_per_minute: -1
`,
			expectError: true,
			errorText:   "rate limit requests per minute must be positive",
		},
		{
			name: "zero burst size",
			configYAML: `
manager:
  rate_limit:
    enabled: true
    requests_per_minute: 100
    burst_size: 0
`,
			expectError: true,
			errorText:   "rate limit burst size must be positive",
		},
		{
			name: "valid rate limit config",
			configYAML: `
manager:
  rate_limit:
    enabled: true
    requests_per_minute: 100
    burst_size: 20
`,
			expectError: false,
		},
		{
			name: "disabled rate limit",
			configYAML: `
manager:
  rate_limit:
    enabled: false
    requests_per_minute: -1
    burst_size: 0
`,
			expectError: false, // Should not validate when disabled
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

func TestManagerConfigSessionTimeouts(t *testing.T) {
	// Set required environment variables
	os.Setenv("JWT_SECRET_KEY", "test-jwt-secret-key-at-least-32-characters-long")
	os.Setenv("DATABASE_URL", "file:./test.db")
	defer os.Unsetenv("JWT_SECRET_KEY")
	defer os.Unsetenv("DATABASE_URL")

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "manager.yaml")

	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorText   string
	}{
		{
			name: "zero proxy session TTL",
			configYAML: `
manager:
  session_management:
    proxy_session_ttl: 0s
`,
			expectError: true,
			errorText:   "proxy session TTL must be positive",
		},
		{
			name: "zero VNC session TTL",
			configYAML: `
manager:
  session_management:
    vnc_session_ttl: 0s
`,
			expectError: true,
			errorText:   "VNC session TTL must be positive",
		},
		{
			name: "zero console session TTL",
			configYAML: `
manager:
  session_management:
    console_session_ttl: 0s
`,
			expectError: true,
			errorText:   "console session TTL must be positive",
		},
		{
			name: "valid session timeouts",
			configYAML: `
manager:
  session_management:
    proxy_session_ttl: 1h
    vnc_session_ttl: 4h
    console_session_ttl: 2h
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

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestConfig_Load(t *testing.T) {
	// Reset viper for test
	viper.Reset()

	// Test loading with defaults
	config, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if config == nil {
		t.Fatal("Config should not be nil")
	}

	// Check defaults
	if config.Manager.Endpoint != "http://localhost:8080" {
		t.Errorf("Expected default Manager endpoint 'http://localhost:8080', got '%s'", config.Manager.Endpoint)
	}

	if config.Gateway.URL != "http://localhost:8081" {
		t.Errorf("Expected default Gateway URL 'http://localhost:8081', got '%s'", config.Gateway.URL)
	}
}

func TestConfig_LoadWithFile(t *testing.T) {
	// Create temp config file
	tempDir, err := os.MkdirTemp("", "bmc-cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
manager:
  endpoint: "http://test.example.com:9090"
auth:
  access_token: "test-token"
  email: "test@example.com"
gateway:
  url: "http://gateway.example.com:8081"
`

	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Reset viper and set config file
	viper.Reset()
	viper.SetConfigFile(configFile)

	// Manually read config since Load() looks for config in predefined paths
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Check loaded values
	if config.Manager.Endpoint != "http://test.example.com:9090" {
		t.Errorf("Expected Manager endpoint 'http://test.example.com:9090', got '%s'", config.Manager.Endpoint)
	}

	if config.Auth.AccessToken != "test-token" {
		t.Errorf("Expected access token 'test-token', got '%s'", config.Auth.AccessToken)
	}

	if config.Auth.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", config.Auth.Email)
	}

	if config.Gateway.URL != "http://gateway.example.com:8081" {
		t.Errorf("Expected Gateway URL 'http://gateway.example.com:8081', got '%s'", config.Gateway.URL)
	}
}

func TestConfig_LoadWithEnvironmentVariables(t *testing.T) {
	// Set environment variables - viper replaces dots with underscores for env vars
	originalEndpoint := os.Getenv("BMC_MANAGER_ENDPOINT")
	originalToken := os.Getenv("BMC_AUTH_ACCESS_TOKEN")

	os.Setenv("BMC_MANAGER_ENDPOINT", "http://env.example.com:8080")
	os.Setenv("BMC_AUTH_ACCESS_TOKEN", "env-token")

	defer func() {
		if originalEndpoint == "" {
			os.Unsetenv("BMC_MANAGER_ENDPOINT")
		} else {
			os.Setenv("BMC_MANAGER_ENDPOINT", originalEndpoint)
		}
		if originalToken == "" {
			os.Unsetenv("BMC_AUTH_ACCESS_TOKEN")
		} else {
			os.Setenv("BMC_AUTH_ACCESS_TOKEN", originalToken)
		}
	}()

	// Reset viper for test and configure it the same way as Load()
	viper.Reset()
	viper.SetEnvPrefix("BMC")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicitly bind environment variables for nested config
	viper.BindEnv("manager.endpoint")
	viper.BindEnv("auth.access_token")
	viper.BindEnv("auth.refresh_token")
	viper.BindEnv("auth.email")
	viper.BindEnv("gateway.url")

	// Set defaults
	viper.SetDefault("manager.endpoint", "http://localhost:8080")
	viper.SetDefault("gateway.url", "http://localhost:8081")

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Check environment values override defaults
	if config.Manager.Endpoint != "http://env.example.com:8080" {
		t.Errorf("Expected Manager endpoint from env 'http://env.example.com:8080', got '%s'", config.Manager.Endpoint)
	}

	if config.Auth.AccessToken != "env-token" {
		t.Errorf("Expected access token from env 'env-token', got '%s'", config.Auth.AccessToken)
	}
}

func TestConfig_Save(t *testing.T) {
	// Create temp directory for config
	tempDir, err := os.MkdirTemp("", "bmc-cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	config := &Config{
		Manager: ManagerConfig{
			Endpoint: "http://test.example.com:8080",
		},
		Auth: AuthConfig{
			AccessToken:    "test-access-token",
			RefreshToken:   "test-refresh-token",
			TokenExpiresAt: time.Now().Add(time.Hour),
			Email:          "test@example.com",
			APIKey:         "test-api-key",
			Token:          "test-token",
		},
		Gateway: GatewayConfig{
			URL: "http://gateway.example.com:8081",
		},
	}

	err = config.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify config file was created
	configFile := filepath.Join(tempDir, ".bmc-cli", "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Reset viper and load saved config
	viper.Reset()
	viper.SetConfigFile(configFile)

	// Manually read config since Load() looks for config in predefined paths
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read saved config file: %v", err)
	}

	var loadedConfig Config
	if err := viper.Unmarshal(&loadedConfig); err != nil {
		t.Fatalf("Failed to unmarshal saved config: %v", err)
	}

	// Verify saved values
	if loadedConfig.Manager.Endpoint != config.Manager.Endpoint {
		t.Errorf("Manager endpoint not saved correctly. Expected '%s', got '%s'",
			config.Manager.Endpoint, loadedConfig.Manager.Endpoint)
	}

	if loadedConfig.Auth.AccessToken != config.Auth.AccessToken {
		t.Errorf("Access token not saved correctly. Expected '%s', got '%s'",
			config.Auth.AccessToken, loadedConfig.Auth.AccessToken)
	}

	if loadedConfig.Auth.Email != config.Auth.Email {
		t.Errorf("Email not saved correctly. Expected '%s', got '%s'",
			config.Auth.Email, loadedConfig.Auth.Email)
	}

	if loadedConfig.Gateway.URL != config.Gateway.URL {
		t.Errorf("Gateway URL not saved correctly. Expected '%s', got '%s'",
			config.Gateway.URL, loadedConfig.Gateway.URL)
	}
}

func TestConfig_SaveCreateDirectory(t *testing.T) {
	// Create temp directory without .bmc-cli subdirectory
	tempDir, err := os.MkdirTemp("", "bmc-cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock home directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	config := &Config{
		Manager: ManagerConfig{
			Endpoint: "http://test.example.com:8080",
		},
	}

	// Verify .bmc-cli directory doesn't exist yet
	configDir := filepath.Join(tempDir, ".bmc-cli")
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		t.Error("Config directory should not exist yet")
	}

	err = config.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("Config directory was not created")
	}

	// Verify config file exists
	configFile := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestConfig_LoadInvalidYAML(t *testing.T) {
	// Create temp config file with invalid YAML
	tempDir, err := os.MkdirTemp("", "bmc-cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "config.yaml")
	invalidYAML := `
bmc_manager:
  endpoint: invalid yaml content
    missing colon
malformed
`

	err = os.WriteFile(configFile, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	// Reset viper and set config file
	viper.Reset()
	viper.SetConfigFile(configFile)

	// Try to read the invalid config file
	err = viper.ReadInConfig()
	if err == nil {
		t.Error("Expected error when reading invalid YAML")
	}
}

func TestConfig_ManagerConfig(t *testing.T) {
	config := ManagerConfig{
		Endpoint: "http://manager.example.com:8080",
	}

	if config.Endpoint != "http://manager.example.com:8080" {
		t.Errorf("Expected endpoint 'http://manager.example.com:8080', got '%s'", config.Endpoint)
	}
}

func TestConfig_GatewayConfig(t *testing.T) {
	config := GatewayConfig{
		URL: "http://gateway.example.com:8081",
	}

	if config.URL != "http://gateway.example.com:8081" {
		t.Errorf("Expected URL 'http://gateway.example.com:8081', got '%s'", config.URL)
	}
}

func TestConfig_AuthConfig(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)

	config := AuthConfig{
		AccessToken:    "access-token",
		RefreshToken:   "refresh-token",
		TokenExpiresAt: expiresAt,
		Email:          "user@example.com",
		APIKey:         "api-key",
		Token:          "legacy-token",
	}

	if config.AccessToken != "access-token" {
		t.Errorf("Expected access token 'access-token', got '%s'", config.AccessToken)
	}

	if config.RefreshToken != "refresh-token" {
		t.Errorf("Expected refresh token 'refresh-token', got '%s'", config.RefreshToken)
	}

	if !config.TokenExpiresAt.Equal(expiresAt) {
		t.Errorf("Expected expires at %v, got %v", expiresAt, config.TokenExpiresAt)
	}

	if config.Email != "user@example.com" {
		t.Errorf("Expected email 'user@example.com', got '%s'", config.Email)
	}

	if config.APIKey != "api-key" {
		t.Errorf("Expected API key 'api-key', got '%s'", config.APIKey)
	}

	if config.Token != "legacy-token" {
		t.Errorf("Expected token 'legacy-token', got '%s'", config.Token)
	}
}

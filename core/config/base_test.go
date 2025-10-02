package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestConfig is a test configuration struct
type TestConfig struct {
	CommonConfig `yaml:",inline"`
	Test         TestSection `yaml:"test"`
}

type TestSection struct {
	StringValue    string        `yaml:"string_value" env:"TEST_STRING" default:"default_string"`
	IntValue       int           `yaml:"int_value" env:"TEST_INT" default:"42"`
	BoolValue      bool          `yaml:"bool_value" env:"TEST_BOOL" default:"true"`
	DurationValue  time.Duration `yaml:"duration_value" env:"TEST_DURATION" default:"5m"`
	RequiredValue  string        `yaml:"required_value" env:"TEST_REQUIRED"`
	NestedSection  NestedSection `yaml:"nested"`
}

type NestedSection struct {
	NestedString string `yaml:"nested_string" env:"NESTED_STRING" default:"nested_default"`
	NestedInt    int    `yaml:"nested_int" env:"NESTED_INT" default:"100"`
}

func TestConfigLoader_SetDefaults(t *testing.T) {
	loader := NewConfigLoader(LoaderConfig{
		ServiceName: "test",
	})

	config := &TestConfig{}
	err := loader.setDefaults(config)
	if err != nil {
		t.Fatalf("setDefaults failed: %v", err)
	}

	// Test default values
	if config.Test.StringValue != "default_string" {
		t.Errorf("Expected StringValue 'default_string', got '%s'", config.Test.StringValue)
	}

	if config.Test.IntValue != 42 {
		t.Errorf("Expected IntValue 42, got %d", config.Test.IntValue)
	}

	if !config.Test.BoolValue {
		t.Errorf("Expected BoolValue true, got %v", config.Test.BoolValue)
	}

	if config.Test.DurationValue != 5*time.Minute {
		t.Errorf("Expected DurationValue 5m, got %v", config.Test.DurationValue)
	}

	// Test nested defaults
	if config.Test.NestedSection.NestedString != "nested_default" {
		t.Errorf("Expected NestedString 'nested_default', got '%s'", config.Test.NestedSection.NestedString)
	}

	if config.Test.NestedSection.NestedInt != 100 {
		t.Errorf("Expected NestedInt 100, got %d", config.Test.NestedSection.NestedInt)
	}

	// Test common config - no defaults at common level anymore
	// (defaults are now managed by individual services)
}

func TestConfigLoader_LoadFromYAML(t *testing.T) {
	// Create temporary YAML file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test.yaml")

	yamlContent := `
log:
  level: debug
  format: text

test:
  string_value: yaml_string
  int_value: 123
  bool_value: false
  duration_value: 10m
  nested:
    nested_string: yaml_nested
    nested_int: 200
`

	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	loader := NewConfigLoader(LoaderConfig{
		ConfigFile:  configFile,
		ServiceName: "test",
	})

	config := &TestConfig{}
	err = loader.Load(config)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test YAML values override defaults
	if config.Test.StringValue != "yaml_string" {
		t.Errorf("Expected StringValue 'yaml_string', got '%s'", config.Test.StringValue)
	}

	if config.Test.IntValue != 123 {
		t.Errorf("Expected IntValue 123, got %d", config.Test.IntValue)
	}

	if config.Test.BoolValue {
		t.Errorf("Expected BoolValue false, got %v", config.Test.BoolValue)
	}

	if config.Test.DurationValue != 10*time.Minute {
		t.Errorf("Expected DurationValue 10m, got %v", config.Test.DurationValue)
	}

	// Test nested YAML values
	if config.Test.NestedSection.NestedString != "yaml_nested" {
		t.Errorf("Expected NestedString 'yaml_nested', got '%s'", config.Test.NestedSection.NestedString)
	}

	if config.Test.NestedSection.NestedInt != 200 {
		t.Errorf("Expected NestedInt 200, got %d", config.Test.NestedSection.NestedInt)
	}

	// Test common config values
	if config.Log.Level != "debug" {
		t.Errorf("Expected Log.Level 'debug', got '%s'", config.Log.Level)
	}

	if config.Log.Format != "text" {
		t.Errorf("Expected Log.Format 'text', got '%s'", config.Log.Format)
	}
}

func TestConfigLoader_LoadFromEnv(t *testing.T) {
	// Set environment variables
	envVars := map[string]string{
		"TEST_STRING":    "env_string",
		"TEST_INT":       "456",
		"TEST_BOOL":      "false",
		"TEST_DURATION":  "15m",
		"NESTED_STRING":  "env_nested",
		"NESTED_INT":     "300",
	}

	// Set environment variables
	for key, value := range envVars {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	loader := NewConfigLoader(LoaderConfig{
		ServiceName: "test",
	})

	config := &TestConfig{}
	err := loader.Load(config)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test environment values override defaults
	if config.Test.StringValue != "env_string" {
		t.Errorf("Expected StringValue 'env_string', got '%s'", config.Test.StringValue)
	}

	if config.Test.IntValue != 456 {
		t.Errorf("Expected IntValue 456, got %d", config.Test.IntValue)
	}

	if config.Test.BoolValue {
		t.Errorf("Expected BoolValue false, got %v", config.Test.BoolValue)
	}

	if config.Test.DurationValue != 15*time.Minute {
		t.Errorf("Expected DurationValue 15m, got %v", config.Test.DurationValue)
	}

	// Test nested environment values
	if config.Test.NestedSection.NestedString != "env_nested" {
		t.Errorf("Expected NestedString 'env_nested', got '%s'", config.Test.NestedSection.NestedString)
	}

	if config.Test.NestedSection.NestedInt != 300 {
		t.Errorf("Expected NestedInt 300, got %d", config.Test.NestedSection.NestedInt)
	}

	// Common config no longer supports env variables - that's handled by services
}

func TestConfigLoader_ServiceSpecificOverrides(t *testing.T) {
	// Set both general and service-specific environment variables
	envVars := map[string]string{
		"TEST_STRING":      "general",  // General setting
		"TEST_TEST_STRING": "specific", // Service-specific override
	}

	for key, value := range envVars {
		os.Setenv(key, value)
		defer os.Unsetenv(key)
	}

	loader := NewConfigLoader(LoaderConfig{
		ServiceName: "test",
	})

	config := &TestConfig{}
	err := loader.Load(config)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Service-specific should override general
	if config.Test.StringValue != "specific" {
		t.Errorf("Expected StringValue 'specific' (service-specific), got '%s'", config.Test.StringValue)
	}
}

func TestConfigLoader_LoadEnvironmentFile(t *testing.T) {
	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, "test.env")

	envContent := `
# Test environment file
TEST_STRING=file_string
TEST_INT=789
TEST_BOOL=true
TEST_DURATION=20m

# Quoted values
QUOTED_VALUE="quoted string"
SINGLE_QUOTED='single quoted'
`

	err := os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test env file: %v", err)
	}

	loader := NewConfigLoader(LoaderConfig{
		EnvironmentFile: envFile,
		ServiceName:     "test",
	})

	config := &TestConfig{}
	err = loader.Load(config)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Test environment file values
	if config.Test.StringValue != "file_string" {
		t.Errorf("Expected StringValue 'file_string', got '%s'", config.Test.StringValue)
	}

	if config.Test.IntValue != 789 {
		t.Errorf("Expected IntValue 789, got %d", config.Test.IntValue)
	}

	if !config.Test.BoolValue {
		t.Errorf("Expected BoolValue true, got %v", config.Test.BoolValue)
	}

	if config.Test.DurationValue != 20*time.Minute {
		t.Errorf("Expected DurationValue 20m, got %v", config.Test.DurationValue)
	}
}

func TestConfigLoader_PrecedenceOrder(t *testing.T) {
	tempDir := t.TempDir()

	// Create YAML config file
	configFile := filepath.Join(tempDir, "test.yaml")
	yamlContent := `
test:
  string_value: yaml_value
  int_value: 100
`
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create environment file
	envFile := filepath.Join(tempDir, "test.env")
	envContent := `TEST_STRING=env_file_value`
	err = os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write env file: %v", err)
	}

	// Clean up any existing environment variables from previous tests
	os.Unsetenv("TEST_INT")
	defer os.Unsetenv("TEST_INT")

	// Set environment variable (should have highest precedence)
	os.Setenv("TEST_STRING", "env_var_value")
	defer os.Unsetenv("TEST_STRING")

	loader := NewConfigLoader(LoaderConfig{
		ConfigFile:      configFile,
		EnvironmentFile: envFile,
		ServiceName:     "test",
	})

	config := &TestConfig{}
	err = loader.Load(config)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Environment variable should have highest precedence
	if config.Test.StringValue != "env_var_value" {
		t.Errorf("Expected StringValue 'env_var_value' (env var), got '%s'", config.Test.StringValue)
	}

	// YAML value should be used for int_value (no env override)
	if config.Test.IntValue != 100 {
		t.Errorf("Expected IntValue 100 (YAML), got %d", config.Test.IntValue)
	}
}

func TestConfigLoader_InvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid.yaml")

	// Write invalid YAML
	invalidYAML := `
test:
  string_value: yaml_value
  invalid_indent:
bad_yaml
`
	err := os.WriteFile(configFile, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	loader := NewConfigLoader(LoaderConfig{
		ConfigFile:  configFile,
		ServiceName: "test",
	})

	config := &TestConfig{}
	err = loader.Load(config)
	if err == nil {
		t.Errorf("Expected error for invalid YAML, but got none")
	}

	if !strings.Contains(err.Error(), "failed to parse config file") {
		t.Errorf("Expected parse error message, got: %v", err)
	}
}

func TestConfigLoader_InvalidEnvironmentFile(t *testing.T) {
	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, "invalid.env")

	// Write invalid environment file
	invalidEnv := `
VALID_VAR=value
INVALID LINE WITHOUT EQUALS
ANOTHER_VALID=value
`
	err := os.WriteFile(envFile, []byte(invalidEnv), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid env file: %v", err)
	}

	loader := NewConfigLoader(LoaderConfig{
		EnvironmentFile: envFile,
		ServiceName:     "test",
	})

	config := &TestConfig{}
	err = loader.Load(config)
	if err == nil {
		t.Errorf("Expected error for invalid environment file, but got none")
	}

	if !strings.Contains(err.Error(), "invalid line") {
		t.Errorf("Expected invalid line error message, got: %v", err)
	}
}

func TestConfigLoader_MissingFiles(t *testing.T) {
	// Test with non-existent files (should not error)
	loader := NewConfigLoader(LoaderConfig{
		ConfigFile:      "/non/existent/config.yaml",
		EnvironmentFile: "/non/existent/.env",
		ServiceName:     "test",
	})

	config := &TestConfig{}
	err := loader.Load(config)
	if err != nil {
		t.Fatalf("Load should not fail for missing optional files: %v", err)
	}

	// Should use default values
	if config.Test.StringValue != "default_string" {
		t.Errorf("Expected default StringValue, got '%s'", config.Test.StringValue)
	}
}

func TestFindConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Change to temp directory
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Test 1: No config file exists
	configFile := FindConfigFile("test")
	expectedDefault := ""
	if configFile != expectedDefault {
		t.Errorf("Expected empty string when no config exists, got '%s'", configFile)
	}

	// Test 2: Config file in current directory
	currentConfigFile := "./test.yaml"
	err = os.WriteFile(currentConfigFile, []byte("test: value"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	configFile = FindConfigFile("test")
	expectedFoundFile := "test.yaml"
	if configFile != expectedFoundFile {
		t.Errorf("Expected current directory config '%s', got '%s'", expectedFoundFile, configFile)
	}

	// Test 3: Config file in config subdirectory
	err = os.Mkdir("config", 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	configSubdirFile := "config/test.yaml"
	err = os.WriteFile(configSubdirFile, []byte("test: value"), 0644)
	if err != nil {
		t.Fatalf("Failed to create config subdir file: %v", err)
	}

	// Remove current directory file so config subdir is found
	os.Remove(currentConfigFile)

	configFile = FindConfigFile("test")
	if configFile != configSubdirFile {
		t.Errorf("Expected config subdir file '%s', got '%s'", configSubdirFile, configFile)
	}
}

func TestFindEnvironmentFile(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Change to temp directory
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Test 1: No env file exists
	envFile := FindEnvironmentFile("test")
	if envFile != "" {
		t.Errorf("Expected empty path for non-existent file, got '%s'", envFile)
	}

	// Test 2: .env file in current directory
	currentEnvFile := ".env"
	err = os.WriteFile(currentEnvFile, []byte("TEST=value"), 0644)
	if err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}

	envFile = FindEnvironmentFile("test")
	if envFile != currentEnvFile {
		t.Errorf("Expected current directory .env '%s', got '%s'", currentEnvFile, envFile)
	}

	// Test 3: Service-specific env file
	serviceEnvFile := "test.env"
	err = os.WriteFile(serviceEnvFile, []byte("TEST=value"), 0644)
	if err != nil {
		t.Fatalf("Failed to create service env file: %v", err)
	}

	// .env should still be preferred
	envFile = FindEnvironmentFile("test")
	if envFile != currentEnvFile {
		t.Errorf("Expected .env to be preferred over service-specific, got '%s'", envFile)
	}

	// Remove .env so service-specific is found
	os.Remove(currentEnvFile)

	envFile = FindEnvironmentFile("test")
	if envFile != serviceEnvFile {
		t.Errorf("Expected service-specific env file '%s', got '%s'", serviceEnvFile, envFile)
	}
}

func TestSetFieldValue_StringTypes(t *testing.T) {
	loader := &ConfigLoader{}

	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"simple string", "hello", "hello"},
		{"empty string", "", ""},
		{"string with spaces", "hello world", "hello world"},
		{"string with special chars", "hello@world.com", "hello@world.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			field := reflect.ValueOf(&result).Elem()

			err := loader.setFieldValue(field, tt.value)
			if err != nil {
				t.Fatalf("setFieldValue failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSetFieldValue_BooleanTypes(t *testing.T) {
	loader := &ConfigLoader{}

	tests := []struct {
		name     string
		value    string
		expected bool
		hasError bool
	}{
		{"true", "true", true, false},
		{"false", "false", false, false},
		{"1", "1", true, false},
		{"0", "0", false, false},
		{"yes", "yes", true, false},
		{"no", "no", false, false},
		{"on", "on", true, false},
		{"off", "off", false, false},
		{"TRUE", "TRUE", true, false},
		{"FALSE", "FALSE", false, false},
		{"invalid", "invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result bool
			field := reflect.ValueOf(&result).Elem()

			err := loader.setFieldValue(field, tt.value)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for value '%s', but got none", tt.value)
				}
				return
			}

			if err != nil {
				t.Fatalf("setFieldValue failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSetFieldValue_IntegerTypes(t *testing.T) {
	loader := &ConfigLoader{}

	tests := []struct {
		name     string
		value    string
		expected int64
		hasError bool
	}{
		{"positive int", "42", 42, false},
		{"negative int", "-42", -42, false},
		{"zero", "0", 0, false},
		{"large int", "9223372036854775807", 9223372036854775807, false},
		{"invalid int", "not_a_number", 0, true},
		{"float as int", "42.5", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int64
			field := reflect.ValueOf(&result).Elem()

			err := loader.setFieldValue(field, tt.value)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for value '%s', but got none", tt.value)
				}
				return
			}

			if err != nil {
				t.Fatalf("setFieldValue failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestSetFieldValue_DurationTypes(t *testing.T) {
	loader := &ConfigLoader{}

	tests := []struct {
		name     string
		value    string
		expected time.Duration
		hasError bool
	}{
		{"seconds", "30s", 30 * time.Second, false},
		{"minutes", "5m", 5 * time.Minute, false},
		{"hours", "2h", 2 * time.Hour, false},
		{"mixed", "1h30m", 90 * time.Minute, false},
		{"nanoseconds", "500ns", 500 * time.Nanosecond, false},
		{"invalid duration", "invalid", 0, true},
		{"number without unit", "30", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result time.Duration
			field := reflect.ValueOf(&result).Elem()

			err := loader.setFieldValue(field, tt.value)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for value '%s', but got none", tt.value)
				}
				return
			}

			if err != nil {
				t.Fatalf("setFieldValue failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
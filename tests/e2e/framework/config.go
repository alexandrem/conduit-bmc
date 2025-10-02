// tests/e2e/framework/config.go
package framework

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type TestConfig struct {
	// BMC Management System Endpoints
	ManagerEndpoint string `yaml:"manager_endpoint"`
	GatewayEndpoint string `yaml:"gateway_endpoint"`
	AgentEndpoint   string `yaml:"agent_endpoint"`

	// Real IPMI Endpoints
	IPMIEndpoints []IPMIEndpoint `yaml:"ipmi_endpoints"`

	// Test Configuration
	TestTimeout     time.Duration `yaml:"test_timeout"`
	RetryAttempts   int           `yaml:"retry_attempts"`
	ConcurrentTests int           `yaml:"concurrent_tests"`

	// Test Users for Multi-tenant Testing
	TestCustomers []TestCustomer `yaml:"test_customers"`

	// Performance Test Configuration
	PerformanceTests PerformanceConfig `yaml:"performance_tests"`

	// Test Scenarios Configuration
	TestScenarios ScenarioConfig `yaml:"test_scenarios"`

	// Logging Configuration
	Logging LoggingConfig `yaml:"logging"`

	// Reporting Configuration
	Reporting ReportingConfig `yaml:"reporting"`
}

type IPMIEndpoint struct {
	ID               string   `yaml:"id"`
	Address          string   `yaml:"address"`
	Username         string   `yaml:"username"`
	Password         string   `yaml:"password"`
	Datacenter       string   `yaml:"datacenter"`
	Description      string   `yaml:"description"`
	ExpectedFeatures []string `yaml:"expected_features"`
}

type TestCustomer struct {
	Email           string   `yaml:"email"`
	APIKey          string   `yaml:"api_key"`
	Password        string   `yaml:"password"`
	Name            string   `yaml:"name"`
	Permissions     []string `yaml:"permissions"`
	AssignedServers []string `yaml:"assigned_servers"`
}

type PerformanceConfig struct {
	LoadTest   LoadTestConfig   `yaml:"load_test"`
	StressTest StressTestConfig `yaml:"stress_test"`
}

type LoadTestConfig struct {
	ConcurrentUsers   int           `yaml:"concurrent_users"`
	OperationsPerUser int           `yaml:"operations_per_user"`
	TestDuration      time.Duration `yaml:"test_duration"`
	RampUpTime        time.Duration `yaml:"ramp_up_time"`
}

type StressTestConfig struct {
	ConcurrentUsers   int           `yaml:"concurrent_users"`
	OperationsPerUser int           `yaml:"operations_per_user"`
	TestDuration      time.Duration `yaml:"test_duration"`
	RampUpTime        time.Duration `yaml:"ramp_up_time"`
}

type ScenarioConfig struct {
	PowerOperations PowerOperationsConfig `yaml:"power_operations"`
	ConsoleAccess   ConsoleAccessConfig   `yaml:"console_access"`
	Authentication  AuthenticationConfig  `yaml:"authentication"`
	MultiTenant     MultiTenantConfig     `yaml:"multi_tenant"`
}

type PowerOperationsConfig struct {
	Enabled    bool              `yaml:"enabled"`
	Operations []string          `yaml:"operations"`
	WaitTimes  map[string]string `yaml:"wait_times"`
}

type ConsoleAccessConfig struct {
	Enabled           bool                 `yaml:"enabled"`
	ConnectionTimeout time.Duration        `yaml:"connection_timeout"`
	CommandTimeout    time.Duration        `yaml:"command_timeout"`
	TestCommands      []ConsoleTestCommand `yaml:"test_commands"`
}

type ConsoleTestCommand struct {
	Command  string `yaml:"command"`
	Expected string `yaml:"expected"`
}

type AuthenticationConfig struct {
	Enabled           bool          `yaml:"enabled"`
	TokenTTL          time.Duration `yaml:"token_ttl"`
	TestInvalidTokens bool          `yaml:"test_invalid_tokens"`
	TestExpiredTokens bool          `yaml:"test_expired_tokens"`
}

type MultiTenantConfig struct {
	Enabled                bool `yaml:"enabled"`
	TestIsolation          bool `yaml:"test_isolation"`
	TestUnauthorizedAccess bool `yaml:"test_unauthorized_access"`
}

type LoggingConfig struct {
	Level             string `yaml:"level"`
	Output            string `yaml:"output"`
	IncludeTimestamps bool   `yaml:"include_timestamps"`
	IncludeRequestIds bool   `yaml:"include_request_ids"`
}

type ReportingConfig struct {
	OutputFormat           string `yaml:"output_format"`
	OutputFile             string `yaml:"output_file"`
	IncludeMetrics         bool   `yaml:"include_metrics"`
	IncludePerformanceData bool   `yaml:"include_performance_data"`
}

// LoadConfig loads test configuration from file or returns default
func LoadConfig() TestConfig {
	configFile := os.Getenv("E2E_TEST_CONFIG")
	fmt.Println("LOADING CONFIG: " + configFile)

	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read config file: %v\n", err)
		os.Exit(1)
	}

	var config TestConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot unmarshal config file: %v\n", err)
		os.Exit(1)
	}

	config.applyDefaults()

	return config
}

func (c *TestConfig) applyDefaults() {
	for i := range c.TestCustomers {
		if c.TestCustomers[i].Password == "" {
			c.TestCustomers[i].Password = c.TestCustomers[i].APIKey
		}
	}
}

// ParseWaitTime converts string duration to time.Duration
func (c *TestConfig) ParseWaitTime(operation string) time.Duration {
	waitTimeStr, exists := c.TestScenarios.PowerOperations.WaitTimes[operation]
	if !exists {
		return 5 * time.Second // Default wait time
	}

	duration, err := time.ParseDuration(waitTimeStr)
	if err != nil {
		return 5 * time.Second // Default on parse error
	}

	return duration
}

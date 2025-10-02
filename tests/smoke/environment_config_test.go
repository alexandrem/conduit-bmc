package functional

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestLocalAgentConfigurationRegression tests that the local-agent
// Air configuration supports both Docker and local environments correctly.
// This is a regression test for the environment configuration changes.
func TestLocalAgentConfigurationRegression(t *testing.T) {
	// Get the project root (go up two levels from tests/functional)
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Should be able to get project root path")

	localAgentDir := filepath.Join(projectRoot, "local-agent")

	t.Run("LocalAgentConfigExists", func(t *testing.T) {
		// Test that local-agent.yaml exists and is valid
		localConfigPath := filepath.Join(localAgentDir, "config", "local-agent.yaml")
		require.FileExists(t, localConfigPath, "local-agent.yaml config should exist")

		// Verify it's valid YAML
		content, err := os.ReadFile(localConfigPath)
		require.NoError(t, err, "Should be able to read local-agent.yaml")

		var config map[string]interface{}
		err = yaml.Unmarshal(content, &config)
		require.NoError(t, err, "local-agent.yaml should be valid YAML")

		// Verify it has expected local development settings
		agent, ok := config["agent"].(map[string]interface{})
		require.True(t, ok, "Config should have agent section")

		endpoint, ok := agent["endpoint"].(string)
		require.True(t, ok, "Agent should have endpoint")
		assert.Contains(t, endpoint, "localhost", "Local config should use localhost")

		gatewayEndpoint, ok := agent["regional_gateway_endpoint"].(string)
		require.True(t, ok, "Agent should have regional_gateway_endpoint")
		assert.Contains(t, gatewayEndpoint, "localhost", "Local config should use localhost for gateway")
	})

	t.Run("DockerAgentConfigExists", func(t *testing.T) {
		// Test that docker-agent.yaml exists and is valid
		dockerConfigPath := filepath.Join(localAgentDir, "config", "docker-agent.yaml")
		require.FileExists(t, dockerConfigPath, "docker-agent.yaml config should exist")

		// Verify it's valid YAML
		content, err := os.ReadFile(dockerConfigPath)
		require.NoError(t, err, "Should be able to read docker-agent.yaml")

		var config map[string]interface{}
		err = yaml.Unmarshal(content, &config)
		require.NoError(t, err, "docker-agent.yaml should be valid YAML")

		// Verify it has expected Docker development settings
		agent, ok := config["agent"].(map[string]interface{})
		require.True(t, ok, "Config should have agent section")

		endpoint, ok := agent["endpoint"].(string)
		require.True(t, ok, "Agent should have endpoint")
		assert.Contains(t, endpoint, "local-agent", "Docker config should use container name")

		gatewayEndpoint, ok := agent["regional_gateway_endpoint"].(string)
		require.True(t, ok, "Agent should have regional_gateway_endpoint")
		assert.Contains(t, gatewayEndpoint, "gateway", "Docker config should use container name for gateway")
	})

	t.Run("E2EAgentConfigExists", func(t *testing.T) {
		// Test that e2e-agent.yaml exists (used by e2e tests)
		testConfigPath := filepath.Join(localAgentDir, "config", "e2e-agent.yaml")
		require.FileExists(t, testConfigPath, "e2e-agent.yaml config should exist")

		// Verify it's valid YAML
		content, err := os.ReadFile(testConfigPath)
		require.NoError(t, err, "Should be able to read e2e-agent.yaml")

		var config map[string]interface{}
		err = yaml.Unmarshal(content, &config)
		require.NoError(t, err, "e2e-agent.yaml should be valid YAML")

		// Verify it has expected test settings
		agent, ok := config["agent"].(map[string]interface{})
		require.True(t, ok, "Config should have agent section")

		_, ok = agent["endpoint"].(string)
		require.True(t, ok, "Agent should have endpoint")
	})
}

// TestLocalEnvironmentConfiguration verifies that the local development
// environment can start properly with the new configuration.
func TestLocalEnvironmentConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping environment configuration test in short mode")
	}

	// Get the project root
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Should be able to get project root path")

	localAgentDir := filepath.Join(projectRoot, "local-agent")

	t.Run("LocalConfigurationIsValidForAgent", func(t *testing.T) {
		// This test verifies that the local-agent.yaml config is properly structured
		// for the agent to start correctly in local development

		localConfigPath := filepath.Join(localAgentDir, "config", "local-agent.yaml")
		content, err := os.ReadFile(localConfigPath)
		require.NoError(t, err, "Should be able to read local-agent.yaml")

		var config map[string]interface{}
		err = yaml.Unmarshal(content, &config)
		require.NoError(t, err, "Config should be valid YAML")

		// Check that static hosts are configured for local development
		static, ok := config["static"].(map[string]interface{})
		require.True(t, ok, "Config should have static section")

		hosts, ok := static["hosts"].([]interface{})
		require.True(t, ok, "Static section should have hosts")
		assert.Greater(t, len(hosts), 0, "Should have at least one static host configured")

		// Verify first host has localhost configuration
		if len(hosts) > 0 {
			firstHost, ok := hosts[0].(map[string]interface{})
			require.True(t, ok, "First host should be a map")

			ip, ok := firstHost["ip"].(string)
			require.True(t, ok, "Host should have IP")
			assert.Equal(t, "localhost", ip, "Local config should use localhost for BMC hosts")

			customerID, ok := firstHost["customer_id"].(string)
			require.True(t, ok, "Host should have customer_id")
			assert.Equal(t, "test@company.com", customerID, "Local config should use consistent test customer ID")
		}
	})
}

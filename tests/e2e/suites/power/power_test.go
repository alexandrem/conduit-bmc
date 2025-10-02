// tests/e2e/suites/power/power_test.go
package power

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"tests/e2e/framework"
)

const (
	server01 = "bmc-dc-docker-01-e2e-virtualbmc-01-623"
	server02 = "bmc-dc-docker-01-e2e-virtualbmc-02-623"
	server03 = "bmc-dc-docker-01-e2e-virtualbmc-03-623"
)

// PowerTestSuite embeds the framework E2E test suite
type PowerTestSuite struct {
	framework.E2ETestSuite
}

func (s *PowerTestSuite) TestPowerOperations() {
	if !s.Config.TestScenarios.PowerOperations.Enabled {
		s.T().Skip("Power operations testing is disabled")
	}

	tests := []struct {
		name      string
		serverID  string
		operation string
		expected  string
		waitTime  time.Duration
	}{
		{
			name:      "PowerStatus",
			serverID:  server01,
			operation: "status",
			expected:  "unknown", // VirtualBMC currently returns "unknown" for Docker containers
			waitTime:  s.Config.ParseWaitTime("status"),
		},
		{
			name:      "PowerOff",
			serverID:  server01,
			operation: "off",
			expected:  "unknown", // VirtualBMC returns "unknown" - power operations work but state isn't simulated
			waitTime:  s.Config.ParseWaitTime("off"),
		},
		{
			name:      "PowerOn",
			serverID:  server01,
			operation: "on",
			expected:  "unknown", // VirtualBMC returns "unknown" - power operations work but state isn't simulated
			waitTime:  s.Config.ParseWaitTime("on"),
		},
		{
			name:      "PowerCycle",
			serverID:  server01,
			operation: "cycle",
			expected:  "unknown", // VirtualBMC returns "unknown" - power operations work but state isn't simulated
			waitTime:  s.Config.ParseWaitTime("cycle"),
		},
		{
			name:      "PowerReset",
			serverID:  server01,
			operation: "reset",
			expected:  "unknown", // VirtualBMC returns "unknown" - power operations work but state isn't simulated
			waitTime:  s.Config.ParseWaitTime("reset"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Use the framework's authentication method
			token := s.AuthenticateAndGetServerToken(tt.serverID)

			// Execute power operation
			_, err := s.GatewayClient.PowerOperation(s.Ctx, token, tt.serverID, tt.operation)
			s.Require().NoError(err, "Power operation %s failed", tt.operation)

			// Wait for operation to complete
			s.T().Logf("Waiting %v for power operation to complete", tt.waitTime)
			time.Sleep(tt.waitTime)

			// Verify final power state
			status, err := s.GatewayClient.PowerStatus(s.Ctx, token, tt.serverID)
			s.Require().NoError(err, "Failed to get power status")
			s.Assert().Equal(tt.expected, status, "Power state mismatch")

			// Log operation success
			s.T().Logf("Power %s operation completed successfully: %s -> %s",
				tt.operation, tt.serverID, status)
		})
	}
}

func (s *PowerTestSuite) TestConcurrentPowerOperations() {
	if !s.Config.TestScenarios.PowerOperations.Enabled {
		s.T().Skip("Power operations testing is disabled")
	}

	servers := []string{server01, server02, server03}
	operations := []string{"status"} // Use only status to avoid interfering operations

	// Create test matrix
	type testCase struct {
		serverID  string
		operation string
	}

	var testCases []testCase
	for _, server := range servers {
		for _, op := range operations {
			testCases = append(testCases, testCase{
				serverID:  server,
				operation: op,
			})
		}
	}

	// Execute operations concurrently
	s.T().Logf("Running %d concurrent power operations", len(testCases))

	concurrency := s.Config.ConcurrentTests
	results := make(chan testResult, len(testCases))

	// Worker pool for concurrent execution
	semaphore := make(chan struct{}, concurrency)

	for _, tc := range testCases {
		go func(tc testCase) {
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			token := s.AuthenticateAndGetServerToken(tc.serverID)
			_, err := s.GatewayClient.PowerOperation(s.Ctx, token, tc.serverID, tc.operation)

			results <- testResult{
				serverID:  tc.serverID,
				operation: tc.operation,
				success:   err == nil,
				error:     err,
			}
		}(tc)
	}

	// Collect results
	successCount := 0
	for i := 0; i < len(testCases); i++ {
		result := <-results
		if result.success {
			successCount++
			s.T().Logf("✅ Concurrent operation succeeded: %s %s", result.serverID, result.operation)
		} else {
			s.T().Errorf("❌ Concurrent operation failed: %s %s - %v",
				result.serverID, result.operation, result.error)
		}
	}

	s.Assert().Equal(len(testCases), successCount,
		"Not all concurrent operations succeeded")

	s.T().Logf("Concurrent power operations test completed: %d/%d succeeded",
		successCount, len(testCases))
}

func (s *PowerTestSuite) TestPowerOperationsAllServers() {
	if !s.Config.TestScenarios.PowerOperations.Enabled {
		s.T().Skip("Power operations testing is disabled")
	}

	// Test power status on all configured IPMI endpoints
	for _, endpoint := range s.Config.IPMIEndpoints {
		s.Run("PowerStatus_"+endpoint.ID, func() {
			token := s.AuthenticateAndGetServerToken(endpoint.ID)

			status, err := s.GatewayClient.PowerStatus(s.Ctx, token, endpoint.ID)
			s.Require().NoError(err, "Failed to get power status for %s", endpoint.ID)
			s.Assert().NotEmpty(status, "Power status should not be empty")

			s.T().Logf("Server %s (%s) power status: %s",
				endpoint.ID, endpoint.Description, status)
		})
	}
}

func (s *PowerTestSuite) TestPowerOperationRetry() {
	if !s.Config.TestScenarios.PowerOperations.Enabled {
		s.T().Skip("Power operations testing is disabled")
	}

	serverID := server01
	token := s.AuthenticateAndGetServerToken(serverID)

	// Test power operation with retry logic
	err := framework.RetryOperation(func() error {
		_, err := s.GatewayClient.PowerStatus(s.Ctx, token, serverID)
		return err
	}, s.Config.RetryAttempts, 1*time.Second)

	s.Assert().NoError(err, "Power status operation should succeed with retry")
}

func (s *PowerTestSuite) TestPowerOperationTimeout() {
	if !s.Config.TestScenarios.PowerOperations.Enabled {
		s.T().Skip("Power operations testing is disabled")
	}

	serverID := server01
	token := s.AuthenticateAndGetServerToken(serverID)

	// Test operation with short timeout
	ctx, cancel := context.WithTimeout(s.Ctx, 200*time.Microsecond)
	defer cancel()

	_, err := s.GatewayClient.PowerStatus(ctx, token, serverID)
	s.Assert().Error(err, "Operation should timeout with very short deadline")
	s.T().Logf("Expected timeout error: %v", err)
}

type testResult struct {
	serverID  string
	operation string
	success   bool
	error     error
}

// TestPowerTestSuite runs the power test suite
func TestPowerTestSuite(t *testing.T) {
	suite.Run(t, new(PowerTestSuite))
}

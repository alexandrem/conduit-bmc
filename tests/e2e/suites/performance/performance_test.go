// tests/e2e/suites/performance/performance_test.go
package performance

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"tests/e2e/framework"
)

// PerformanceTestSuite embeds the framework E2E test suite
type PerformanceTestSuite struct {
	framework.E2ETestSuite
}

func (s *PerformanceTestSuite) TestConcurrentUserLoad() {
	config := s.Config.PerformanceTests.LoadTest
	totalOperations := config.ConcurrentUsers * config.OperationsPerUser

	if config.ConcurrentUsers == 0 {
		s.T().Skipf("ConcurrentUsers is zero")
	}

	s.T().Logf("Starting concurrent load test: %d users, %d operations each (total: %d)",
		config.ConcurrentUsers, config.OperationsPerUser, totalOperations)

	startTime := time.Now()
	results := make(chan loadTestResult, totalOperations)
	var wg sync.WaitGroup

	// Ramp up users gradually
	userInterval := config.RampUpTime / time.Duration(config.ConcurrentUsers)

	// Create concurrent users
	for i := 0; i < config.ConcurrentUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			s.runUserLoadTest(userID, config.OperationsPerUser, results)
		}(i)

		// Stagger user startup
		if i < config.ConcurrentUsers-1 {
			time.Sleep(userInterval)
		}
	}

	// Wait for all users to complete
	wg.Wait()
	close(results)

	// Analyze results
	s.analyzeLoadTestResults(results, totalOperations, time.Since(startTime))
}

func (s *PerformanceTestSuite) TestStressTest() {
	config := s.Config.PerformanceTests.StressTest
	totalOperations := config.ConcurrentUsers * config.OperationsPerUser

	if config.ConcurrentUsers == 0 {
		s.T().Skip("ConcurrentUsers is zero")
	}

	s.T().Logf("Starting stress test: %d users, %d operations each (total: %d)",
		config.ConcurrentUsers, config.OperationsPerUser, totalOperations)

	startTime := time.Now()
	results := make(chan loadTestResult, totalOperations)
	var wg sync.WaitGroup

	// Ramp up users more aggressively for stress test
	userInterval := config.RampUpTime / time.Duration(config.ConcurrentUsers)

	// Create concurrent users
	for i := 0; i < config.ConcurrentUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			s.runUserStressTest(userID, config.OperationsPerUser, results)
		}(i)

		// Shorter stagger for stress test
		if i < config.ConcurrentUsers-1 {
			time.Sleep(userInterval / 2)
		}
	}

	// Wait for all users to complete
	wg.Wait()
	close(results)

	// Analyze results
	s.analyzeStressTestResults(results, totalOperations, time.Since(startTime))
}

func (s *PerformanceTestSuite) runUserLoadTest(userID, operations int, results chan<- loadTestResult) {
	// Create user and get server access
	customer := s.CreateTestCustomerWithID(fmt.Sprintf("loadtest-user-%d", userID))
	server := s.GetRandomTestServer()

	// Authenticate
	if _, err := s.ManagerClient.Authenticate(s.Ctx, customer.Email, customer.Password); err != nil {
		for i := 0; i < operations; i++ {
			results <- loadTestResult{
				userID:    userID,
				operation: i,
				duration:  0,
				success:   false,
				error:     fmt.Errorf("authentication failed for user %d: %v", userID, err),
			}
		}
		return
	}

	token, err := s.ManagerClient.GenerateServerToken(s.Ctx, customer, server, []string{"power", "status"})
	if err != nil {
		for i := 0; i < operations; i++ {
			results <- loadTestResult{
				userID:    userID,
				operation: i,
				duration:  0,
				success:   false,
				error:     fmt.Errorf("token generation failed for user %d: %v", userID, err),
			}
		}
		return
	}

	for i := 0; i < operations; i++ {
		startTime := time.Now()

		// Perform power status operation
		_, err := s.GatewayClient.PowerStatus(s.Ctx, token, server.ID)

		duration := time.Since(startTime)

		results <- loadTestResult{
			userID:    userID,
			operation: i,
			duration:  duration,
			success:   err == nil,
			error:     err,
		}

		// Brief delay between operations to simulate realistic usage
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *PerformanceTestSuite) runUserStressTest(userID, operations int, results chan<- loadTestResult) {
	// Similar to load test but with more aggressive timing and mixed operations
	customer := s.CreateTestCustomerWithID(fmt.Sprintf("stresstest-user-%d", userID))
	server := s.GetRandomTestServer()

	// Authenticate
	if _, err := s.ManagerClient.Authenticate(s.Ctx, customer.Email, customer.Password); err != nil {
		for i := 0; i < operations; i++ {
			results <- loadTestResult{
				userID:    userID,
				operation: i,
				duration:  0,
				success:   false,
				error:     fmt.Errorf("authentication failed for user %d: %v", userID, err),
			}
		}
		return
	}

	token, err := s.ManagerClient.GenerateServerToken(s.Ctx, customer, server, []string{"power", "status"})
	if err != nil {
		for i := 0; i < operations; i++ {
			results <- loadTestResult{
				userID:    userID,
				operation: i,
				duration:  0,
				success:   false,
				error:     fmt.Errorf("token generation failed for user %d: %v", userID, err),
			}
		}
		return
	}

	for i := 0; i < operations; i++ {
		startTime := time.Now()

		// Mix of operations for stress test
		var err error
		switch i % 3 {
		case 0:
			// Power status
			_, err = s.GatewayClient.PowerStatus(s.Ctx, token, server.ID)
		case 1:
			// Token validation
			_ = s.GatewayClient.ValidateToken(s.Ctx, token)
		case 2:
			// Power status again (most common operation)
			_, err = s.GatewayClient.PowerStatus(s.Ctx, token, server.ID)
		}

		duration := time.Since(startTime)

		results <- loadTestResult{
			userID:    userID,
			operation: i,
			duration:  duration,
			success:   err == nil,
			error:     err,
		}

		// No delay for stress test - maximum pressure
	}
}

func (s *PerformanceTestSuite) analyzeLoadTestResults(results <-chan loadTestResult, expected int, totalTime time.Duration) {
	var (
		successCount  int
		totalDuration time.Duration
		maxDuration   time.Duration
		minDuration   = time.Hour // Initialize to large value
		errors        []error
	)

	count := 0
	for result := range results {
		count++

		if result.success {
			successCount++
			totalDuration += result.duration

			if result.duration > maxDuration {
				maxDuration = result.duration
			}
			if result.duration < minDuration {
				minDuration = result.duration
			}
		} else {
			errors = append(errors, result.error)
		}
	}

	// Calculate metrics
	successRate := float64(successCount) / float64(count) * 100
	var avgDuration time.Duration
	if successCount > 0 {
		avgDuration = totalDuration / time.Duration(successCount)
	}
	throughput := float64(count) / totalTime.Seconds()

	// Report results
	s.T().Logf("🧪 Load Test Results:")
	s.T().Logf("  Total Operations: %d", count)
	s.T().Logf("  Successful: %d (%.1f%%)", successCount, successRate)
	s.T().Logf("  Failed: %d", len(errors))
	s.T().Logf("  Total Time: %v", totalTime)
	s.T().Logf("  Throughput: %.2f ops/sec", throughput)
	if successCount > 0 {
		s.T().Logf("  Average Latency: %v", avgDuration)
		s.T().Logf("  Min Latency: %v", minDuration)
		s.T().Logf("  Max Latency: %v", maxDuration)
	}

	// Log some sample errors
	if len(errors) > 0 {
		s.T().Logf("  Sample errors:")
		for i, err := range errors {
			if i >= 5 { // Limit to first 5 errors
				s.T().Logf("  ... and %d more errors", len(errors)-i)
				break
			}
			s.T().Logf("    - %v", err)
		}
	}

	// Assertions for load test (more lenient than stress test)
	s.Assert().GreaterOrEqual(successRate, 95.0, "Load test success rate should be at least 95%%")
	if successCount > 0 {
		s.Assert().Less(avgDuration, 10*time.Second, "Average latency should be under 10 seconds for load test")
	}
	s.Assert().GreaterOrEqual(throughput, 0.5, "Throughput should be at least 0.5 op/sec for load test")

	s.T().Logf("✅ Load test completed successfully")
}

func (s *PerformanceTestSuite) analyzeStressTestResults(results <-chan loadTestResult, expected int, totalTime time.Duration) {
	var (
		successCount  int
		totalDuration time.Duration
		maxDuration   time.Duration
		minDuration   = time.Hour // Initialize to large value
		errors        []error
		timeouts      int
	)

	count := 0
	for result := range results {
		count++

		if result.success {
			successCount++
			totalDuration += result.duration

			if result.duration > maxDuration {
				maxDuration = result.duration
			}
			if result.duration < minDuration {
				minDuration = result.duration
			}
		} else {
			errors = append(errors, result.error)
			if result.error != nil && (result.error.Error() == "context deadline exceeded" ||
				result.error.Error() == "timeout") {
				timeouts++
			}
		}
	}

	// Calculate metrics
	successRate := float64(successCount) / float64(count) * 100
	var avgDuration time.Duration
	if successCount > 0 {
		avgDuration = totalDuration / time.Duration(successCount)
	}
	throughput := float64(count) / totalTime.Seconds()
	timeoutRate := float64(timeouts) / float64(count) * 100

	// Report results
	s.T().Logf("⚡ Stress Test Results:")
	s.T().Logf("  Total Operations: %d", count)
	s.T().Logf("  Successful: %d (%.1f%%)", successCount, successRate)
	s.T().Logf("  Failed: %d", len(errors))
	s.T().Logf("  Timeouts: %d (%.1f%%)", timeouts, timeoutRate)
	s.T().Logf("  Total Time: %v", totalTime)
	s.T().Logf("  Throughput: %.2f ops/sec", throughput)
	if successCount > 0 {
		s.T().Logf("  Average Latency: %v", avgDuration)
		s.T().Logf("  Min Latency: %v", minDuration)
		s.T().Logf("  Max Latency: %v", maxDuration)
	}

	// Log error distribution
	if len(errors) > 0 {
		errorCounts := make(map[string]int)
		for _, err := range errors {
			errorType := "unknown"
			if err != nil {
				errStr := err.Error()
				if len(errStr) > 50 {
					errStr = errStr[:50] + "..."
				}
				errorType = errStr
			}
			errorCounts[errorType]++
		}

		s.T().Logf("  Error distribution:")
		for errorType, count := range errorCounts {
			s.T().Logf("    - %s: %d", errorType, count)
		}
	}

	// Assertions for stress test (more tolerant of failures)
	s.Assert().GreaterOrEqual(successRate, 80.0, "Stress test success rate should be at least 80%%")
	if successCount > 0 {
		s.Assert().Less(avgDuration, 30*time.Second, "Average latency should be under 30 seconds for stress test")
	}
	s.Assert().GreaterOrEqual(throughput, 0.1, "Throughput should be at least 0.1 op/sec for stress test")
	s.Assert().Less(timeoutRate, 50.0, "Timeout rate should be less than 50%% for stress test")

	s.T().Logf("✅ Stress test completed successfully")
}

func (s *PerformanceTestSuite) TestLatencyMeasurement() {
	serverID := "bmc-dc-docker-01-e2e-virtualbmc-01-623"
	token := s.AuthenticateAndGetServerToken(serverID)

	measurements := make([]time.Duration, 0, 10)

	// Take multiple latency measurements
	for i := 0; i < 10; i++ {
		startTime := time.Now()

		_, err := s.GatewayClient.PowerStatus(s.Ctx, token, serverID)
		s.Require().NoError(err, "Power status operation failed in latency test")

		latency := time.Since(startTime)
		measurements = append(measurements, latency)

		time.Sleep(500 * time.Millisecond) // Wait between measurements
	}

	// Calculate statistics
	var total time.Duration
	min := measurements[0]
	max := measurements[0]

	for _, measurement := range measurements {
		total += measurement
		if measurement < min {
			min = measurement
		}
		if measurement > max {
			max = measurement
		}
	}

	avg := total / time.Duration(len(measurements))

	s.T().Logf("📊 Latency Measurements:")
	s.T().Logf("  Samples: %d", len(measurements))
	s.T().Logf("  Average: %v", avg)
	s.T().Logf("  Min: %v", min)
	s.T().Logf("  Max: %v", max)
	s.T().Logf("  Total: %v", total)

	// Basic assertions
	s.Assert().Less(avg, 5*time.Second, "Average latency should be under 5 seconds")
	s.Assert().Less(max, 10*time.Second, "Maximum latency should be under 10 seconds")
}

type loadTestResult struct {
	userID    int
	operation int
	duration  time.Duration
	success   bool
	error     error
}

// TestPerformanceTestSuite runs the performance test suite
func TestPerformanceTestSuite(t *testing.T) {
	suite.Run(t, new(PerformanceTestSuite))
}

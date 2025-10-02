// tests/e2e/framework/reporting.go
package framework

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type TestReport struct {
	Timestamp   time.Time     `json:"timestamp"`
	Duration    time.Duration `json:"duration"`
	Environment string        `json:"environment"`
	TestSuites  []SuiteResult `json:"test_suites"`
	Summary     TestSummary   `json:"summary"`
	Config      TestConfig    `json:"config,omitempty"`
}

type SuiteResult struct {
	Name     string        `json:"name"`
	Tests    []TestCase    `json:"tests"`
	Duration time.Duration `json:"duration"`
	Passed   int           `json:"passed"`
	Failed   int           `json:"failed"`
	Skipped  int           `json:"skipped"`
}

type TestCase struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"` // "passed", "failed", "skipped"
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
	ServerID  string        `json:"server_id,omitempty"`
	Operation string        `json:"operation,omitempty"`
	Metadata  TestMetadata  `json:"metadata,omitempty"`
}

type TestMetadata struct {
	UserID       string            `json:"user_id,omitempty"`
	CustomerID   string            `json:"customer_id,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	Latency      time.Duration     `json:"latency,omitempty"`
	Throughput   float64           `json:"throughput,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

type TestSummary struct {
	TotalTests    int           `json:"total_tests"`
	PassedTests   int           `json:"passed_tests"`
	FailedTests   int           `json:"failed_tests"`
	SkippedTests  int           `json:"skipped_tests"`
	SuccessRate   float64       `json:"success_rate"`
	TotalDuration time.Duration `json:"total_duration"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
}

func GenerateTestReport(results []SuiteResult, config TestConfig) TestReport {
	summary := calculateSummary(results)

	return TestReport{
		Timestamp:   time.Now(),
		Duration:    summary.TotalDuration,
		Environment: "ipmi-local-dev",
		TestSuites:  results,
		Summary:     summary,
		Config:      config,
	}
}

func calculateSummary(results []SuiteResult) TestSummary {
	var (
		totalTests    int
		passedTests   int
		failedTests   int
		skippedTests  int
		totalDuration time.Duration
		startTime     time.Time
		endTime       time.Time
	)

	// Initialize start/end times
	startTime = time.Now()
	endTime = time.Time{}

	for _, suite := range results {
		totalTests += suite.Passed + suite.Failed + suite.Skipped
		passedTests += suite.Passed
		failedTests += suite.Failed
		skippedTests += suite.Skipped
		totalDuration += suite.Duration

		// Track overall test execution time window
		for range suite.Tests {
			// Note: In real implementation, we'd track actual test start/end times
			if startTime.IsZero() {
				startTime = time.Now().Add(-suite.Duration)
			}
		}
	}

	if endTime.IsZero() {
		endTime = startTime.Add(totalDuration)
	}

	var successRate float64
	if totalTests > 0 {
		successRate = float64(passedTests) / float64(totalTests) * 100
	}

	return TestSummary{
		TotalTests:    totalTests,
		PassedTests:   passedTests,
		FailedTests:   failedTests,
		SkippedTests:  skippedTests,
		SuccessRate:   successRate,
		TotalDuration: totalDuration,
		StartTime:     startTime,
		EndTime:       endTime,
	}
}

func (r TestReport) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

func (r TestReport) PrintSummary() {
	fmt.Printf("\n🧪 E2E IPMI Test Report\n")
	fmt.Printf("========================\n")
	fmt.Printf("Environment: %s\n", r.Environment)
	fmt.Printf("Timestamp: %s\n", r.Timestamp.Format(time.RFC3339))
	fmt.Printf("Duration: %v\n", r.Duration)
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  Total Tests: %d\n", r.Summary.TotalTests)
	fmt.Printf("  Passed: %d (%.1f%%)\n", r.Summary.PassedTests,
		float64(r.Summary.PassedTests)/float64(r.Summary.TotalTests)*100)
	fmt.Printf("  Failed: %d (%.1f%%)\n", r.Summary.FailedTests,
		float64(r.Summary.FailedTests)/float64(r.Summary.TotalTests)*100)
	fmt.Printf("  Skipped: %d (%.1f%%)\n", r.Summary.SkippedTests,
		float64(r.Summary.SkippedTests)/float64(r.Summary.TotalTests)*100)
	fmt.Printf("  Success Rate: %.1f%%\n", r.Summary.SuccessRate)

	// Print suite breakdown
	if len(r.TestSuites) > 0 {
		fmt.Printf("\n📋 Test Suites:\n")
		for _, suite := range r.TestSuites {
			status := "✅"
			if suite.Failed > 0 {
				status = "❌"
			}
			fmt.Printf("  %s %s: %d/%d passed (%.1f%%) [%v]\n",
				status, suite.Name, suite.Passed, suite.Passed+suite.Failed,
				float64(suite.Passed)/float64(suite.Passed+suite.Failed)*100, suite.Duration)
		}
	}

	// Print failed tests if any
	if r.Summary.FailedTests > 0 {
		fmt.Printf("\n❌ Failed Tests:\n")
		for _, suite := range r.TestSuites {
			for _, test := range suite.Tests {
				if test.Status == "failed" {
					fmt.Printf("  - %s.%s", suite.Name, test.Name)
					if test.ServerID != "" {
						fmt.Printf(" [%s]", test.ServerID)
					}
					if test.Error != "" {
						fmt.Printf(": %s", truncateString(test.Error, 80))
					}
					fmt.Printf("\n")
				}
			}
		}
	}

	// Print performance metrics if available
	r.printPerformanceMetrics()

	fmt.Printf("\n")
}

func (r TestReport) printPerformanceMetrics() {
	var (
		totalLatency   time.Duration
		latencyCount   int
		totalThroughput float64
		throughputCount int
	)

	for _, suite := range r.TestSuites {
		for _, test := range suite.Tests {
			if test.Metadata.Latency > 0 {
				totalLatency += test.Metadata.Latency
				latencyCount++
			}
			if test.Metadata.Throughput > 0 {
				totalThroughput += test.Metadata.Throughput
				throughputCount++
			}
		}
	}

	if latencyCount > 0 || throughputCount > 0 {
		fmt.Printf("\n⚡ Performance Metrics:\n")

		if latencyCount > 0 {
			avgLatency := totalLatency / time.Duration(latencyCount)
			fmt.Printf("  Average Latency: %v (%d samples)\n", avgLatency, latencyCount)
		}

		if throughputCount > 0 {
			avgThroughput := totalThroughput / float64(throughputCount)
			fmt.Printf("  Average Throughput: %.2f ops/sec (%d samples)\n", avgThroughput, throughputCount)
		}
	}
}

func (r TestReport) PrintDetailedReport() {
	r.PrintSummary()

	fmt.Printf("🔍 Detailed Results:\n")
	fmt.Printf("====================\n")

	for _, suite := range r.TestSuites {
		fmt.Printf("\n📁 Suite: %s [%v]\n", suite.Name, suite.Duration)
		fmt.Printf("   Passed: %d, Failed: %d, Skipped: %d\n",
			suite.Passed, suite.Failed, suite.Skipped)

		if len(suite.Tests) > 0 {
			fmt.Printf("   Tests:\n")
			for _, test := range suite.Tests {
				status := getStatusEmoji(test.Status)
				fmt.Printf("     %s %s", status, test.Name)

				if test.Duration > 0 {
					fmt.Printf(" [%v]", test.Duration)
				}

				if test.ServerID != "" {
					fmt.Printf(" {%s}", test.ServerID)
				}

				if test.Status == "failed" && test.Error != "" {
					fmt.Printf("\n       Error: %s", truncateString(test.Error, 100))
				}

				if test.Metadata.Latency > 0 {
					fmt.Printf("\n       Latency: %v", test.Metadata.Latency)
				}

				fmt.Printf("\n")
			}
		}
	}

	// Print configuration summary
	fmt.Printf("\n⚙️  Configuration:\n")
	fmt.Printf("   IPMI Endpoints: %d\n", len(r.Config.IPMIEndpoints))
	fmt.Printf("   Test Customers: %d\n", len(r.Config.TestCustomers))
	fmt.Printf("   Concurrent Tests: %d\n", r.Config.ConcurrentTests)
	fmt.Printf("   Test Timeout: %v\n", r.Config.TestTimeout)
}

func getStatusEmoji(status string) string {
	switch status {
	case "passed":
		return "✅"
	case "failed":
		return "❌"
	case "skipped":
		return "⏭️"
	default:
		return "❓"
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (r TestReport) ExportToCSV(filename string) error {
	csvData := [][]string{
		{"Suite", "Test", "Status", "Duration", "Server", "Operation", "Error"},
	}

	for _, suite := range r.TestSuites {
		for _, test := range suite.Tests {
			row := []string{
				suite.Name,
				test.Name,
				test.Status,
				test.Duration.String(),
				test.ServerID,
				test.Operation,
				test.Error,
			}
			csvData = append(csvData, row)
		}
	}

	// Create CSV content
	csvContent := ""
	for _, row := range csvData {
		for i, cell := range row {
			if i > 0 {
				csvContent += ","
			}
			csvContent += fmt.Sprintf("\"%s\"", cell)
		}
		csvContent += "\n"
	}

	return os.WriteFile(filename, []byte(csvContent), 0644)
}

// Global report collector for gathering results during test execution
var globalReportCollector = &ReportCollector{
	suites: make(map[string]*SuiteResult),
}

type ReportCollector struct {
	suites map[string]*SuiteResult
}

func (rc *ReportCollector) AddTestResult(suiteName, testName, status string, duration time.Duration, metadata TestMetadata, err error) {
	if rc.suites[suiteName] == nil {
		rc.suites[suiteName] = &SuiteResult{
			Name:  suiteName,
			Tests: make([]TestCase, 0),
		}
	}

	suite := rc.suites[suiteName]

	testCase := TestCase{
		Name:     testName,
		Status:   status,
		Duration: duration,
		Metadata: metadata,
	}

	if err != nil {
		testCase.Error = err.Error()
	}

	suite.Tests = append(suite.Tests, testCase)
	suite.Duration += duration

	switch status {
	case "passed":
		suite.Passed++
	case "failed":
		suite.Failed++
	case "skipped":
		suite.Skipped++
	}
}

func (rc *ReportCollector) GetReport(config TestConfig) TestReport {
	results := make([]SuiteResult, 0, len(rc.suites))
	for _, suite := range rc.suites {
		results = append(results, *suite)
	}

	return GenerateTestReport(results, config)
}

func (rc *ReportCollector) Reset() {
	rc.suites = make(map[string]*SuiteResult)
}
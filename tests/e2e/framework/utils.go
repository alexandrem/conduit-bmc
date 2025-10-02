// tests/e2e/framework/utils.go
package framework

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// TestIPMIConnection tests direct IPMI connectivity to an endpoint
func TestIPMIConnection(address, username, password string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ipmitool", "-I", "lanplus", "-H", address, "-U", username, "-P", password, "power", "status")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	// Check if output contains expected IPMI response
	outputStr := strings.ToLower(string(output))
	return strings.Contains(outputStr, "chassis power") || strings.Contains(outputStr, "power")
}

// WaitForCondition waits for a condition to be true with timeout and polling
func WaitForCondition(condition func() bool, timeout, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}

	return false
}

// RetryOperation retries an operation with exponential backoff
func RetryOperation(operation func() error, maxAttempts int, initialDelay time.Duration) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt < maxAttempts {
			delay := initialDelay * time.Duration(1<<uint(attempt-1)) // Exponential backoff
			time.Sleep(delay)
		}
	}

	return lastErr
}

// StringInSlice checks if a string exists in a slice
func StringInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// MergeMaps merges two string maps, with values from the second map taking precedence
func MergeMaps(map1, map2 map[string]string) map[string]string {
	result := make(map[string]string)

	for k, v := range map1 {
		result[k] = v
	}

	for k, v := range map2 {
		result[k] = v
	}

	return result
}

// GenerateTestID generates a unique test ID based on timestamp
func GenerateTestID() string {
	return "test-" + time.Now().Format("20060102-150405")
}

// SanitizeForLog sanitizes sensitive information for logging
func SanitizeForLog(input string) string {
	// Replace potential passwords and tokens with masked values
	if len(input) > 10 {
		return input[:4] + "***" + input[len(input)-4:]
	}
	return "***"
}
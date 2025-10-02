package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestMain_NonExistentConfigFile verifies that the agent fails hard when
// given a config file path that doesn't exist
func TestMain_NonExistentConfigFile(t *testing.T) {
	// Build the binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/test-agent", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build agent binary: %v", err)
	}
	defer os.Remove("/tmp/test-agent")

	// Run with non-existent config file
	cmd := exec.Command("/tmp/test-agent", "-config", "/nonexistent/path/to/config.yaml")
	output, err := cmd.CombinedOutput()

	// Should fail
	if err == nil {
		t.Error("Expected agent to fail with non-existent config file")
	}

	// Should contain error message about missing config
	outputStr := string(output)
	if !strings.Contains(outputStr, "Configuration file does not exist") &&
		!strings.Contains(outputStr, "config_path") {
		t.Errorf("Expected error message about missing config file, got: %s", outputStr)
	}

	// Check exit code
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("Expected exit code 1, got: %d", exitErr.ExitCode())
		}
	}
}

// TestMain_ValidConfigFile verifies that the agent starts when given a valid config
func TestMain_ValidConfigFile(t *testing.T) {
	// Create a temporary valid config file
	tmpfile, err := os.CreateTemp("", "agent-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	// Write minimal valid config
	configContent := `
agent:
  datacenter_id: test-dc
  gateway_endpoint: http://localhost:8081
log:
  level: info
`
	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpfile.Close()

	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "/tmp/test-agent", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build agent binary: %v", err)
	}
	defer os.Remove("/tmp/test-agent")

	// Run with valid config file (but kill it quickly)
	cmd := exec.Command("/tmp/test-agent", "-config", tmpfile.Name())
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	// Kill it immediately (we just want to verify it starts)
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("Warning: failed to kill process: %v", err)
	}

	// Wait for process to exit
	cmd.Wait()
}

// TestMain_NoConfigFlag verifies agent behavior when no config flag is provided
func TestMain_NoConfigFlag(t *testing.T) {
	// Build the binary
	buildCmd := exec.Command("go", "build", "-o", "/tmp/test-agent", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build agent binary: %v", err)
	}
	defer os.Remove("/tmp/test-agent")

	// Run without config flag (will try to find config)
	cmd := exec.Command("/tmp/test-agent")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start agent: %v", err)
	}

	// Kill it immediately
	if err := cmd.Process.Kill(); err != nil {
		t.Logf("Warning: failed to kill process: %v", err)
	}

	// The agent should start and try to use defaults or discover config
	// (it might fail later due to missing datacenter_id, but not immediately)
	cmd.Wait()
}

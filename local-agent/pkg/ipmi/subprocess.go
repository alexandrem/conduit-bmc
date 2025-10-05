package ipmi

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// SubprocessClient implements IPMI operations using ipmitool subprocess calls
// This is more resilient than go-ipmi library which can panic on edge cases
type SubprocessClient struct {
	timeout time.Duration
}

// NewSubprocessClient creates a new subprocess-based IPMI client
func NewSubprocessClient() *SubprocessClient {
	return &SubprocessClient{
		timeout: 10 * time.Second,
	}
}

// runIPMITool executes ipmitool with the given arguments
func (c *SubprocessClient) runIPMITool(ctx context.Context, endpoint, username, password string, args ...string) (string, error) {
	// Parse endpoint
	host := endpoint
	if strings.Contains(endpoint, ":") {
		parts := strings.Split(endpoint, ":")
		host = parts[0]
	}

	// Build ipmitool command
	// Try lanplus first, will fallback to lan if it fails
	cmdArgs := []string{
		"-I", "lanplus",
		"-H", host,
		"-U", username,
		"-P", password,
	}
	cmdArgs = append(cmdArgs, args...)

	// Create command with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "ipmitool", cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Debug().
		Str("endpoint", endpoint).
		Strs("args", args).
		Msg("Executing ipmitool command")

	err := cmd.Run()
	if err != nil {
		// If lanplus fails, try legacy lan interface
		if strings.Contains(stderr.String(), "lanplus") || strings.Contains(err.Error(), "exit status") {
			log.Debug().Msg("Trying legacy lan interface")
			cmdArgs[1] = "lan" // Change -I lanplus to -I lan

			cmd = exec.CommandContext(timeoutCtx, "ipmitool", cmdArgs...)
			stdout.Reset()
			stderr.Reset()
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err = cmd.Run()
			if err != nil {
				return "", fmt.Errorf("ipmitool failed: %w, stderr: %s", err, stderr.String())
			}
		} else {
			return "", fmt.Errorf("ipmitool failed: %w, stderr: %s", err, stderr.String())
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}

// PowerOn powers on the server using ipmitool
func (c *SubprocessClient) PowerOn(ctx context.Context, endpoint, username, password string) error {
	log.Debug().Str("endpoint", endpoint).Msg("Powering on server via ipmitool")

	_, err := c.runIPMITool(ctx, endpoint, username, password, "chassis", "power", "on")
	if err != nil {
		return fmt.Errorf("failed to power on: %w", err)
	}

	log.Info().Str("endpoint", endpoint).Msg("Server powered on successfully")
	return nil
}

// PowerOff powers off the server using ipmitool
func (c *SubprocessClient) PowerOff(ctx context.Context, endpoint, username, password string) error {
	log.Debug().Str("endpoint", endpoint).Msg("Powering off server via ipmitool")

	_, err := c.runIPMITool(ctx, endpoint, username, password, "chassis", "power", "off")
	if err != nil {
		return fmt.Errorf("failed to power off: %w", err)
	}

	log.Info().Str("endpoint", endpoint).Msg("Server powered off successfully")
	return nil
}

// PowerCycle power cycles the server using ipmitool
func (c *SubprocessClient) PowerCycle(ctx context.Context, endpoint, username, password string) error {
	log.Debug().Str("endpoint", endpoint).Msg("Power cycling server via ipmitool")

	_, err := c.runIPMITool(ctx, endpoint, username, password, "chassis", "power", "cycle")
	if err != nil {
		return fmt.Errorf("failed to power cycle: %w", err)
	}

	log.Info().Str("endpoint", endpoint).Msg("Server power cycled successfully")
	return nil
}

// Reset resets the server using ipmitool
func (c *SubprocessClient) Reset(ctx context.Context, endpoint, username, password string) error {
	log.Debug().Str("endpoint", endpoint).Msg("Resetting server via ipmitool")

	_, err := c.runIPMITool(ctx, endpoint, username, password, "chassis", "power", "reset")
	if err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	log.Info().Str("endpoint", endpoint).Msg("Server reset successfully")
	return nil
}

// GetPowerState gets the current power state using ipmitool
func (c *SubprocessClient) GetPowerState(ctx context.Context, endpoint, username, password string) (PowerState, error) {
	log.Debug().Str("endpoint", endpoint).Msg("Getting power state via ipmitool")

	output, err := c.runIPMITool(ctx, endpoint, username, password, "chassis", "power", "status")
	if err != nil {
		log.Warn().Err(err).Str("endpoint", endpoint).Msg("Failed to get power state")
		return PowerStateUnknown, fmt.Errorf("failed to get power state: %w", err)
	}

	// Parse output: "Chassis Power is on" or "Chassis Power is off"
	outputLower := strings.ToLower(output)
	if strings.Contains(outputLower, "is on") {
		return PowerStateOn, nil
	} else if strings.Contains(outputLower, "is off") {
		return PowerStateOff, nil
	}

	log.Warn().Str("output", output).Msg("Unknown power state output")
	return PowerStateUnknown, nil
}

// GetBMCInfo gets BMC information using ipmitool
func (c *SubprocessClient) GetBMCInfo(ctx context.Context, endpoint, username, password string) (*BMCInfo, error) {
	output, err := c.runIPMITool(ctx, endpoint, username, password, "bmc", "info")
	if err != nil {
		return nil, fmt.Errorf("failed to get BMC info: %w", err)
	}

	info := &BMCInfo{
		Features: []string{"power", "sensors", "console"},
	}

	// Parse output for version, vendor, etc.
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Firmware Revision") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				info.FirmwareVersion = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "Manufacturer") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				info.Vendor = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "Product") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				info.Model = strings.TrimSpace(parts[1])
			}
		}
	}

	return info, nil
}

// IsAccessible checks if IPMI is accessible using ipmitool
func (c *SubprocessClient) IsAccessible(ctx context.Context, endpoint string) bool {
	// Use a simple command with default/no credentials to test accessibility
	timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	host := endpoint
	if strings.Contains(endpoint, ":") {
		parts := strings.Split(endpoint, ":")
		host = parts[0]
	}

	cmd := exec.CommandContext(timeoutCtx, "ipmitool", "-I", "lanplus", "-H", host, "chassis", "status")
	err := cmd.Run()

	// If lanplus fails, try lan
	if err != nil {
		cmd = exec.CommandContext(timeoutCtx, "ipmitool", "-I", "lan", "-H", host, "chassis", "status")
		err = cmd.Run()
	}

	return err == nil
}

package ipmi

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// Client handles IPMI BMC communications using ipmitool subprocess
type Client struct {
	timeout          time.Duration
	subprocessClient *SubprocessClient
}

// PowerState represents the power state of a server
type PowerState string

const (
	PowerStateOn      PowerState = "on"
	PowerStateOff     PowerState = "off"
	PowerStateUnknown PowerState = "unknown"
)

// BMCInfo represents information about an IPMI BMC
type BMCInfo struct {
	Vendor          string
	Model           string
	FirmwareVersion string
	Features        []string
}

func NewClient() *Client {
	return &Client{
		timeout:          10 * time.Second,
		subprocessClient: NewSubprocessClient(),
	}
}

// IsAccessible checks if an IPMI BMC is accessible at the given endpoint
func (c *Client) IsAccessible(ctx context.Context, endpoint string) bool {
	return c.subprocessClient.IsAccessible(ctx, endpoint)
}

// GetBMCInfo retrieves information about the BMC
func (c *Client) GetBMCInfo(ctx context.Context, endpoint, username, password string) (*BMCInfo, error) {
	return c.subprocessClient.GetBMCInfo(ctx, endpoint, username, password)
}

// GetPowerState retrieves the current power state of the server
func (c *Client) GetPowerState(ctx context.Context, endpoint, username, password string) (PowerState, error) {
	return c.subprocessClient.GetPowerState(ctx, endpoint, username, password)
}

// PowerOn powers on the server
func (c *Client) PowerOn(ctx context.Context, endpoint, username, password string) error {
	return c.subprocessClient.PowerOn(ctx, endpoint, username, password)
}

// PowerOff powers off the server
func (c *Client) PowerOff(ctx context.Context, endpoint, username, password string) error {
	return c.subprocessClient.PowerOff(ctx, endpoint, username, password)
}

// PowerCycle power cycles the server
func (c *Client) PowerCycle(ctx context.Context, endpoint, username, password string) error {
	return c.subprocessClient.PowerCycle(ctx, endpoint, username, password)
}

// Reset resets the server
func (c *Client) Reset(ctx context.Context, endpoint, username, password string) error {
	return c.subprocessClient.Reset(ctx, endpoint, username, password)
}

// GetSensors retrieves sensor readings from the BMC
func (c *Client) GetSensors(ctx context.Context, endpoint, username, password string) (map[string]interface{}, error) {
	log.Debug().Str("endpoint", endpoint).Msg("Getting sensors")

	// TODO: Implement actual IPMI SDR (Sensor Data Record) commands
	// This would involve reading the SDR repository and getting sensor readings

	// Simulated sensor data
	sensors := map[string]interface{}{
		"cpu_temperature":    65.5,
		"system_temperature": 32.0,
		"fan_speed_1":        3500,
		"fan_speed_2":        3600,
		"voltage_12v":        12.1,
		"voltage_5v":         5.0,
	}

	return sensors, nil
}

// GetMCInfo retrieves Management Controller information from the BMC
func (c *Client) GetMCInfo(ctx context.Context, endpoint, username, password string) (map[string]string, error) {
	return c.subprocessClient.GetMCInfo(ctx, endpoint, username, password)
}

// StartSOLSession starts a Serial-over-LAN console session
func (c *Client) StartSOLSession(ctx context.Context, endpoint, username, password string) error {
	log.Debug().Str("endpoint", endpoint).Msg("Starting SOL session")

	// TODO: Implement IPMI SOL (Serial over LAN)
	// This is complex and involves:
	// 1. Activating SOL payload
	// 2. Setting up SOL configuration
	// 3. Managing bidirectional console data flow

	return fmt.Errorf("SOL session not yet implemented")
}

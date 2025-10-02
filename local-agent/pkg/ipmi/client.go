package ipmi

import (
	"context"
	"fmt"
	"net"
	"time"

	goipmi "github.com/bougou/go-ipmi"
	"github.com/rs/zerolog/log"
)

// Client handles IPMI BMC communications
type Client struct {
	timeout time.Duration
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
		timeout: 10 * time.Second,
	}
}

// createIPMIClient creates a go-ipmi client for the given endpoint
func (c *Client) createIPMIClient(endpoint, username, password string) (*goipmi.Client, error) {
	// Parse endpoint to extract host and port
	host, portStr, err := net.SplitHostPort(endpoint)
	port := 623 // Default IPMI port

	if err != nil {
		// If no port specified, use entire endpoint as host
		host = endpoint
	} else {
		// Convert port string to int
		var portErr error
		port, portErr = net.LookupPort("udp", portStr)
		if portErr != nil {
			port = 623 // Fallback to default
		}
	}

	// Create IPMI client using IPMI v2.0 RMCP+ (lanplus)
	// This provides:
	// - RAKP authentication (vs MD2/MD5 in v1.5)
	// - AES-CBC-128 encryption (vs plaintext in v1.5)
	// - SHA1 HMAC integrity checks
	// - Cipher suite negotiation
	// - 20-char password support (vs 16 in v1.5)
	client := &goipmi.Client{
		Interface: goipmi.InterfaceLanplus, // IPMI v2.0 RMCP+ only
		Host:      host,
		Port:      port,
		Username:  username,
		Password:  password,
	}

	return client, nil
}

// IsAccessible checks if an IPMI BMC is accessible at the given endpoint
func (c *Client) IsAccessible(ctx context.Context, endpoint string) bool {
	log.Debug().Str("endpoint", endpoint).Msg("Checking IPMI accessibility")

	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Try to establish UDP connection to IPMI port (623)
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(timeoutCtx, "udp", endpoint)
	if err != nil {
		log.Debug().Str("endpoint", endpoint).Err(err).Msg("IPMI connection failed")
		return false
	}
	defer conn.Close()

	// TODO: In a real implementation, we would:
	// 1. Send IPMI Get Device ID command
	// 2. Parse the response to confirm it's a valid BMC
	// 3. Check authentication requirements

	// For now, just simulate IPMI presence check
	// This is a placeholder - real IPMI detection would use the IPMI protocol
	log.Debug().Str("endpoint", endpoint).Msg("IPMI BMC detected (simulated)")
	return true
}

// GetBMCInfo retrieves information about the BMC
func (c *Client) GetBMCInfo(ctx context.Context, endpoint, username, password string) (*BMCInfo, error) {
	log.Debug().Str("endpoint", endpoint).Msg("Getting BMC info")

	client, err := c.createIPMIClient(endpoint, username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to create IPMI client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		log.Warn().Err(err).Str("endpoint", endpoint).Msg("Failed to connect to IPMI for device info")
		// Return simulated info as fallback
		return &BMCInfo{
			Vendor:          "Unknown",
			Model:           "IPMI BMC",
			FirmwareVersion: "Unknown",
			Features:        []string{"power", "sensors", "console"},
		}, nil
	}
	defer client.Close(ctx)

	// Get device ID
	deviceID, err := client.GetDeviceID(ctx)
	if err != nil {
		log.Warn().Err(err).Str("endpoint", endpoint).Msg("Failed to get device ID")
		return &BMCInfo{
			Vendor:          "Unknown",
			Model:           "IPMI BMC",
			FirmwareVersion: "Unknown",
			Features:        []string{"power", "sensors", "console"},
		}, nil
	}

	// Parse firmware version
	firmwareVersion := fmt.Sprintf("%d.%02d", deviceID.MajorFirmwareRevision, deviceID.MinorFirmwareRevision)

	info := &BMCInfo{
		Vendor:          fmt.Sprintf("Manufacturer ID: %d", deviceID.ManufacturerID),
		Model:           fmt.Sprintf("Product ID: %d", deviceID.ProductID),
		FirmwareVersion: firmwareVersion,
		Features:        []string{"power", "sensors", "console"},
	}

	return info, nil
}

// GetPowerState retrieves the current power state of the server
func (c *Client) GetPowerState(ctx context.Context, endpoint, username, password string) (PowerState, error) {
	log.Debug().Str("endpoint", endpoint).Msg("Getting power state")

	client, err := c.createIPMIClient(endpoint, username, password)
	if err != nil {
		return PowerStateUnknown, fmt.Errorf("failed to create IPMI client: %w", err)
	}

	// Connect to IPMI
	if err := client.Connect(ctx); err != nil {
		log.Warn().Err(err).Str("endpoint", endpoint).Msg("Failed to connect to IPMI, returning unknown state")
		return PowerStateUnknown, fmt.Errorf("failed to connect to IPMI: %w", err)
	}
	defer client.Close(ctx)

	// Get chassis status
	res, err := client.GetChassisStatus(ctx)
	if err != nil {
		log.Warn().Err(err).Str("endpoint", endpoint).Msg("Failed to get chassis status")
		return PowerStateUnknown, fmt.Errorf("failed to get chassis status: %w", err)
	}

	if res.PowerIsOn {
		return PowerStateOn, nil
	}
	return PowerStateOff, nil
}

// PowerOn powers on the server
func (c *Client) PowerOn(ctx context.Context, endpoint, username, password string) error {
	log.Debug().Str("endpoint", endpoint).Msg("Powering on server")

	client, err := c.createIPMIClient(endpoint, username, password)
	if err != nil {
		return fmt.Errorf("failed to create IPMI client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to IPMI: %w", err)
	}
	defer client.Close(ctx)

	// Send chassis power on command
	if _, err := client.ChassisControl(ctx, goipmi.ChassisControlPowerUp); err != nil {
		return fmt.Errorf("failed to power on: %w", err)
	}

	log.Info().Str("endpoint", endpoint).Msg("Server powered on successfully")
	return nil
}

// PowerOff powers off the server
func (c *Client) PowerOff(ctx context.Context, endpoint, username, password string) error {
	log.Debug().Str("endpoint", endpoint).Msg("Powering off server")

	client, err := c.createIPMIClient(endpoint, username, password)
	if err != nil {
		return fmt.Errorf("failed to create IPMI client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to IPMI: %w", err)
	}
	defer client.Close(ctx)

	// Send chassis power down command (hard power off)
	if _, err := client.ChassisControl(ctx, goipmi.ChassisControlPowerDown); err != nil {
		return fmt.Errorf("failed to power off: %w", err)
	}

	log.Info().Str("endpoint", endpoint).Msg("Server powered off successfully")
	return nil
}

// PowerCycle power cycles the server
func (c *Client) PowerCycle(ctx context.Context, endpoint, username, password string) error {
	log.Debug().Str("endpoint", endpoint).Msg("Power cycling server")

	client, err := c.createIPMIClient(endpoint, username, password)
	if err != nil {
		return fmt.Errorf("failed to create IPMI client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to IPMI: %w", err)
	}
	defer client.Close(ctx)

	// Send chassis power cycle command
	if _, err := client.ChassisControl(ctx, goipmi.ChassisControlPowerCycle); err != nil {
		return fmt.Errorf("failed to power cycle: %w", err)
	}

	log.Info().Str("endpoint", endpoint).Msg("Server power cycle initiated successfully")
	return nil
}

// Reset resets the server
func (c *Client) Reset(ctx context.Context, endpoint, username, password string) error {
	log.Debug().Str("endpoint", endpoint).Msg("Resetting server")

	client, err := c.createIPMIClient(endpoint, username, password)
	if err != nil {
		return fmt.Errorf("failed to create IPMI client: %w", err)
	}

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to IPMI: %w", err)
	}
	defer client.Close(ctx)

	// Send chassis hard reset command
	if _, err := client.ChassisControl(ctx, goipmi.ChassisControlHardReset); err != nil {
		return fmt.Errorf("failed to reset: %w", err)
	}

	log.Info().Str("endpoint", endpoint).Msg("Server reset successfully")
	return nil
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

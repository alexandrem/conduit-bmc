package redfish

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Client handles Redfish BMC communications
type Client struct {
	httpClient *http.Client
	timeout    time.Duration
}

// PowerState represents the power state of a server
type PowerState string

const (
	PowerStateOn      PowerState = "On"
	PowerStateOff     PowerState = "Off"
	PowerStateUnknown PowerState = "Unknown"
)

// ServiceRoot represents the Redfish service root response
type ServiceRoot struct {
	ID             string `json:"Id"`
	Name           string `json:"Name"`
	RedfishVersion string `json:"RedfishVersion"`
	UUID           string `json:"UUID"`
	Systems        struct {
		ODataID string `json:"@odata.id"`
	} `json:"Systems"`
	Chassis struct {
		ODataID string `json:"@odata.id"`
	} `json:"Chassis"`
}

// ComputerSystem represents a Redfish computer system
type ComputerSystem struct {
	ID           string     `json:"Id"`
	Name         string     `json:"Name"`
	Manufacturer string     `json:"Manufacturer"`
	Model        string     `json:"Model"`
	SerialNumber string     `json:"SerialNumber"`
	PowerState   PowerState `json:"PowerState"`
	Actions      struct {
		ComputerSystemReset struct {
			Target                   string   `json:"target"`
			ResetTypeAllowableValues []string `json:"ResetType@Redfish.AllowableValues"`
		} `json:"#ComputerSystem.Reset"`
	} `json:"Actions"`
}

// BMCInfo represents information about a Redfish BMC
type BMCInfo struct {
	Vendor          string
	Model           string
	FirmwareVersion string
	RedfishVersion  string
	Features        []string
}

// Manager represents a Redfish Manager (BMC)
type Manager struct {
	ID              string     `json:"Id"`
	Name            string     `json:"Name"`
	ManagerType     string     `json:"ManagerType"`
	Model           string     `json:"Model"`
	FirmwareVersion string     `json:"FirmwareVersion"`
	Manufacturer    string     `json:"Manufacturer"`
	PowerState      PowerState `json:"PowerState"`
	Status          struct {
		State  string `json:"State"`
		Health string `json:"Health"`
	} `json:"Status"`
	NetworkProtocol struct {
		ODataID string `json:"@odata.id"`
	} `json:"NetworkProtocol"`
}

// NetworkProtocol represents Redfish network protocol information
type NetworkProtocol struct {
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Status      struct {
		State  string `json:"State"`
		Health string `json:"Health"`
	} `json:"Status"`
	HTTP struct {
		ProtocolEnabled bool  `json:"ProtocolEnabled"`
		Port            int32 `json:"Port"`
	} `json:"HTTP"`
	HTTPS struct {
		ProtocolEnabled bool  `json:"ProtocolEnabled"`
		Port            int32 `json:"Port"`
	} `json:"HTTPS"`
	SSH struct {
		ProtocolEnabled bool  `json:"ProtocolEnabled"`
		Port            int32 `json:"Port"`
	} `json:"SSH"`
	IPMI struct {
		ProtocolEnabled bool  `json:"ProtocolEnabled"`
		Port            int32 `json:"Port"`
	} `json:"IPMI"`
}

func NewClient() *Client {
	// Create HTTP client with insecure TLS (common for BMCs)
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // BMCs often use self-signed certificates
			},
		},
	}

	return &Client{
		httpClient: httpClient,
		timeout:    10 * time.Second,
	}
}

// IsAccessible checks if a Redfish BMC is accessible at the given endpoint
func (c *Client) IsAccessible(ctx context.Context, endpoint string) bool {
	log.Debug().Str("endpoint", endpoint).Msg("Checking Redfish accessibility")

	// Create request context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Try to access the Redfish service root
	serviceRootURL := strings.TrimSuffix(endpoint, "/") + "/redfish/v1/"
	req, err := http.NewRequestWithContext(reqCtx, "GET", serviceRootURL, nil)
	if err != nil {
		log.Debug().Str("url", serviceRootURL).Err(err).Msg("Failed to create request")
		return false
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Debug().Str("url", serviceRootURL).Err(err).Msg("Redfish connection failed")
		return false
	}
	defer resp.Body.Close()

	// Check if we got a successful response or authentication required
	if (resp.StatusCode >= 200 && resp.StatusCode < 300) || resp.StatusCode == http.StatusUnauthorized {
		log.Debug().Str("endpoint", endpoint).Msg("Redfish BMC detected")
		return true
	}

	log.Debug().Str("url", serviceRootURL).Int("status", resp.StatusCode).Msg("Redfish check failed")
	return false
}

// GetBMCInfo retrieves information about the Redfish BMC
func (c *Client) GetBMCInfo(ctx context.Context, endpoint, username, password string) (*BMCInfo, error) {
	log.Debug().Str("endpoint", endpoint).Msg("Getting BMC info")

	serviceRoot, err := c.getServiceRoot(ctx, endpoint, username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to get service root: %w", err)
	}

	info := &BMCInfo{
		Vendor:          "Unknown", // Would be extracted from Manager or System info
		Model:           "Redfish BMC",
		RedfishVersion:  serviceRoot.RedfishVersion,
		FirmwareVersion: "Unknown", // Would be extracted from Manager info
		Features:        []string{"power", "sensors", "console", "storage", "network", "virtual_media"},
	}

	return info, nil
}

// getServiceRoot retrieves the Redfish service root
func (c *Client) getServiceRoot(ctx context.Context, endpoint, username, password string) (*ServiceRoot, error) {
	serviceRootURL := strings.TrimSuffix(endpoint, "/") + "/redfish/v1/"

	req, err := http.NewRequestWithContext(ctx, "GET", serviceRootURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var serviceRoot ServiceRoot
	if err := json.NewDecoder(resp.Body).Decode(&serviceRoot); err != nil {
		return nil, fmt.Errorf("failed to decode service root: %w", err)
	}

	return &serviceRoot, nil
}

// GetPowerState retrieves the current power state of the server
func (c *Client) GetPowerState(ctx context.Context, endpoint, username, password string) (PowerState, error) {
	log.Debug().Str("endpoint", endpoint).Msg("Getting power state")

	system, err := c.getComputerSystem(ctx, endpoint, username, password)
	if err != nil {
		return PowerStateUnknown, fmt.Errorf("failed to get computer system: %w", err)
	}

	return system.PowerState, nil
}

// getComputerSystem retrieves the first computer system
func (c *Client) getComputerSystem(ctx context.Context, endpoint, username, password string) (*ComputerSystem, error) {
	serviceRoot, err := c.getServiceRoot(ctx, endpoint, username, password)
	if err != nil {
		return nil, err
	}

	// Get systems collection
	systemsURL := strings.TrimSuffix(endpoint, "/") + serviceRoot.Systems.ODataID
	req, err := http.NewRequestWithContext(ctx, "GET", systemsURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var systemsCollection struct {
		Members []struct {
			ODataID string `json:"@odata.id"`
		} `json:"Members"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&systemsCollection); err != nil {
		return nil, fmt.Errorf("failed to decode systems collection: %w", err)
	}

	if len(systemsCollection.Members) == 0 {
		return nil, fmt.Errorf("no computer systems found")
	}

	// Get the first system
	systemURL := strings.TrimSuffix(endpoint, "/") + systemsCollection.Members[0].ODataID
	req, err = http.NewRequestWithContext(ctx, "GET", systemURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var system ComputerSystem
	if err := json.NewDecoder(resp.Body).Decode(&system); err != nil {
		return nil, fmt.Errorf("failed to decode computer system: %w", err)
	}

	return &system, nil
}

// PowerOn powers on the server
func (c *Client) PowerOn(ctx context.Context, endpoint, username, password string) error {
	return c.performPowerAction(ctx, endpoint, username, password, "On")
}

// PowerOff powers off the server
func (c *Client) PowerOff(ctx context.Context, endpoint, username, password string) error {
	return c.performPowerAction(ctx, endpoint, username, password, "ForceOff")
}

// PowerCycle power cycles the server
func (c *Client) PowerCycle(ctx context.Context, endpoint, username, password string) error {
	return c.performPowerAction(ctx, endpoint, username, password, "PowerCycle")
}

// Reset resets the server
func (c *Client) Reset(ctx context.Context, endpoint, username, password string) error {
	return c.performPowerAction(ctx, endpoint, username, password, "ForceRestart")
}

// performPowerAction performs a power action on the server
func (c *Client) performPowerAction(ctx context.Context, endpoint, username, password, action string) error {
	log.Debug().Str("action", action).Str("endpoint", endpoint).Msg("Performing power action")

	system, err := c.getComputerSystem(ctx, endpoint, username, password)
	if err != nil {
		return fmt.Errorf("failed to get computer system: %w", err)
	}

	// Perform the reset action
	resetURL := strings.TrimSuffix(endpoint, "/") + system.Actions.ComputerSystemReset.Target

	resetPayload := map[string]string{
		"ResetType": action,
	}

	payloadBytes, err := json.Marshal(resetPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal reset payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", resetURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("power action failed: HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	log.Debug().Str("action", action).Msg("Power action completed")
	return nil
}

// GetManagerInfo retrieves Manager (BMC) information from Redfish
func (c *Client) GetManagerInfo(ctx context.Context, endpoint, username, password string) (*Manager, *NetworkProtocol, error) {
	log.Debug().Str("endpoint", endpoint).Msg("Getting Manager info")

	// Get Managers collection
	managersURL := strings.TrimSuffix(endpoint, "/") + "/redfish/v1/Managers"
	req, err := http.NewRequestWithContext(ctx, "GET", managersURL, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", "application/json")
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var managersCollection struct {
		Members []struct {
			ODataID string `json:"@odata.id"`
		} `json:"Members"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&managersCollection); err != nil {
		return nil, nil, fmt.Errorf("failed to decode managers collection: %w", err)
	}

	if len(managersCollection.Members) == 0 {
		return nil, nil, fmt.Errorf("no managers found")
	}

	// Get the first manager (BMC)
	managerURL := strings.TrimSuffix(endpoint, "/") + managersCollection.Members[0].ODataID
	req, err = http.NewRequestWithContext(ctx, "GET", managerURL, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", "application/json")
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var manager Manager
	if err := json.NewDecoder(resp.Body).Decode(&manager); err != nil {
		return nil, nil, fmt.Errorf("failed to decode manager: %w", err)
	}

	// Get NetworkProtocol if available
	var netProto *NetworkProtocol
	if manager.NetworkProtocol.ODataID != "" {
		netProtoURL := strings.TrimSuffix(endpoint, "/") + manager.NetworkProtocol.ODataID
		req, err = http.NewRequestWithContext(ctx, "GET", netProtoURL, nil)
		if err == nil {
			req.Header.Set("Accept", "application/json")
			if username != "" && password != "" {
				req.SetBasicAuth(username, password)
			}

			resp, err = c.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				var np NetworkProtocol
				if err := json.NewDecoder(resp.Body).Decode(&np); err == nil {
					netProto = &np
				}
			}
		}
	}

	return &manager, netProto, nil
}

// GetSensors retrieves sensor readings from the BMC
func (c *Client) GetSensors(ctx context.Context, endpoint, username, password string) (map[string]interface{}, error) {
	log.Debug().Str("endpoint", endpoint).Msg("Getting sensors")

	// TODO: Implement actual Redfish sensor reading
	// This would involve accessing /redfish/v1/Chassis/{id}/Thermal and /Power endpoints

	// Simulated sensor data
	sensors := map[string]interface{}{
		"cpu_temperature":     62.0,
		"inlet_temperature":   28.0,
		"exhaust_temperature": 45.0,
		"fan_speed_1":         3200,
		"fan_speed_2":         3300,
		"power_consumption":   180.5,
		"voltage_12v":         12.05,
		"voltage_5v":          4.98,
	}

	return sensors, nil
}

package bmc

import (
	"context"
	"fmt"
	"time"

	gatewayv1 "gateway/gen/gateway/v1"

	"core/types"
	"local-agent/internal/discovery"
	"local-agent/pkg/ipmi"
	"local-agent/pkg/redfish"
)

// Client provides unified interface for BMC operations across IPMI and Redfish
type Client struct {
	ipmiClient    *ipmi.Client
	redfishClient *redfish.Client
}

// NewClient creates a new BMC client
func NewClient(ipmiClient *ipmi.Client, redfishClient *redfish.Client) *Client {
	return &Client{
		ipmiClient:    ipmiClient,
		redfishClient: redfishClient,
	}
}

// GetPowerState retrieves the current power state of a server
func (c *Client) GetPowerState(ctx context.Context, server *discovery.Server) (string, error) {
	if server == nil {
		return "", fmt.Errorf("server is nil")
	}

	if server.ControlEndpoint == nil {
		return "", fmt.Errorf("server has no control endpoint")
	}

	if c.ipmiClient == nil && server.ControlEndpoint.Type == types.BMCTypeIPMI {
		return "", fmt.Errorf("IPMI client is nil")
	}

	if c.redfishClient == nil && server.ControlEndpoint.Type == types.BMCTypeRedfish {
		return "", fmt.Errorf("Redfish client is nil")
	}

	endpoint := server.ControlEndpoint.Endpoint
	username := server.ControlEndpoint.Username
	password := server.ControlEndpoint.Password

	switch server.ControlEndpoint.Type {
	case types.BMCTypeIPMI:
		state, err := c.ipmiClient.GetPowerState(ctx, endpoint, username, password)
		if err != nil {
			return "", fmt.Errorf("IPMI GetPowerState failed: %w", err)
		}
		return string(state), nil

	case types.BMCTypeRedfish:
		state, err := c.redfishClient.GetPowerState(ctx, endpoint, username, password)
		if err != nil {
			return "", fmt.Errorf("Redfish GetPowerState failed: %w", err)
		}
		return string(state), nil

	default:
		return "", fmt.Errorf("unsupported BMC type: %s", server.ControlEndpoint.Type)
	}
}

// PowerOn powers on a server
func (c *Client) PowerOn(ctx context.Context, server *discovery.Server) error {
	if server == nil {
		return fmt.Errorf("server is nil")
	}

	if server.ControlEndpoint == nil {
		return fmt.Errorf("server has no control endpoint")
	}

	if c.ipmiClient == nil && server.ControlEndpoint.Type == types.BMCTypeIPMI {
		return fmt.Errorf("IPMI client is nil")
	}

	if c.redfishClient == nil && server.ControlEndpoint.Type == types.BMCTypeRedfish {
		return fmt.Errorf("Redfish client is nil")
	}

	endpoint := server.ControlEndpoint.Endpoint
	username := server.ControlEndpoint.Username
	password := server.ControlEndpoint.Password

	switch server.ControlEndpoint.Type {
	case types.BMCTypeIPMI:
		if err := c.ipmiClient.PowerOn(ctx, endpoint, username, password); err != nil {
			return fmt.Errorf("IPMI PowerOn failed: %w", err)
		}
		return nil

	case types.BMCTypeRedfish:
		if err := c.redfishClient.PowerOn(ctx, endpoint, username, password); err != nil {
			return fmt.Errorf("Redfish PowerOn failed: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported BMC type: %s", server.ControlEndpoint.Type)
	}
}

// PowerOff powers off a server
func (c *Client) PowerOff(ctx context.Context, server *discovery.Server) error {
	if server == nil {
		return fmt.Errorf("server is nil")
	}

	if server.ControlEndpoint == nil {
		return fmt.Errorf("server has no control endpoint")
	}

	if c.ipmiClient == nil && server.ControlEndpoint.Type == types.BMCTypeIPMI {
		return fmt.Errorf("IPMI client is nil")
	}

	if c.redfishClient == nil && server.ControlEndpoint.Type == types.BMCTypeRedfish {
		return fmt.Errorf("Redfish client is nil")
	}

	endpoint := server.ControlEndpoint.Endpoint
	username := server.ControlEndpoint.Username
	password := server.ControlEndpoint.Password

	switch server.ControlEndpoint.Type {
	case types.BMCTypeIPMI:
		if err := c.ipmiClient.PowerOff(ctx, endpoint, username, password); err != nil {
			return fmt.Errorf("IPMI PowerOff failed: %w", err)
		}
		return nil

	case types.BMCTypeRedfish:
		if err := c.redfishClient.PowerOff(ctx, endpoint, username, password); err != nil {
			return fmt.Errorf("Redfish PowerOff failed: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported BMC type: %s", server.ControlEndpoint.Type)
	}
}

// PowerCycle power cycles a server
func (c *Client) PowerCycle(ctx context.Context, server *discovery.Server) error {
	if server == nil {
		return fmt.Errorf("server is nil")
	}

	if server.ControlEndpoint == nil {
		return fmt.Errorf("server has no control endpoint")
	}

	if c.ipmiClient == nil && server.ControlEndpoint.Type == types.BMCTypeIPMI {
		return fmt.Errorf("IPMI client is nil")
	}

	if c.redfishClient == nil && server.ControlEndpoint.Type == types.BMCTypeRedfish {
		return fmt.Errorf("Redfish client is nil")
	}

	endpoint := server.ControlEndpoint.Endpoint
	username := server.ControlEndpoint.Username
	password := server.ControlEndpoint.Password

	switch server.ControlEndpoint.Type {
	case types.BMCTypeIPMI:
		if err := c.ipmiClient.PowerCycle(ctx, endpoint, username, password); err != nil {
			return fmt.Errorf("IPMI PowerCycle failed: %w", err)
		}
		return nil

	case types.BMCTypeRedfish:
		if err := c.redfishClient.PowerCycle(ctx, endpoint, username, password); err != nil {
			return fmt.Errorf("Redfish PowerCycle failed: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported BMC type: %s", server.ControlEndpoint.Type)
	}
}

// Reset resets a server
func (c *Client) Reset(ctx context.Context, server *discovery.Server) error {
	if server == nil {
		return fmt.Errorf("server is nil")
	}

	if server.ControlEndpoint == nil {
		return fmt.Errorf("server has no control endpoint")
	}

	if c.ipmiClient == nil && server.ControlEndpoint.Type == types.BMCTypeIPMI {
		return fmt.Errorf("IPMI client is nil")
	}

	if c.redfishClient == nil && server.ControlEndpoint.Type == types.BMCTypeRedfish {
		return fmt.Errorf("Redfish client is nil")
	}

	endpoint := server.ControlEndpoint.Endpoint
	username := server.ControlEndpoint.Username
	password := server.ControlEndpoint.Password

	switch server.ControlEndpoint.Type {
	case types.BMCTypeIPMI:
		if err := c.ipmiClient.Reset(ctx, endpoint, username, password); err != nil {
			return fmt.Errorf("IPMI Reset failed: %w", err)
		}
		return nil

	case types.BMCTypeRedfish:
		if err := c.redfishClient.Reset(ctx, endpoint, username, password); err != nil {
			return fmt.Errorf("Redfish Reset failed: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported BMC type: %s", server.ControlEndpoint.Type)
	}
}

// GetBMCInfo retrieves detailed BMC hardware information
func (c *Client) GetBMCInfo(ctx context.Context, server *discovery.Server) (*gatewayv1.BMCInfo, error) {
	if server == nil {
		return nil, fmt.Errorf("server is nil")
	}

	if server.ControlEndpoint == nil {
		return nil, fmt.Errorf("server has no control endpoint")
	}

	endpoint := server.ControlEndpoint.Endpoint
	username := server.ControlEndpoint.Username
	password := server.ControlEndpoint.Password

	switch server.ControlEndpoint.Type {
	case types.BMCTypeIPMI:
		if c.ipmiClient == nil {
			return nil, fmt.Errorf("IPMI client is nil")
		}

		mcInfo, err := c.ipmiClient.GetMCInfo(ctx, endpoint, username, password)
		if err != nil {
			return nil, fmt.Errorf("IPMI GetMCInfo failed: %w", err)
		}

		// Parse additional device support
		additionalSupport := []string{}
		if support, ok := mcInfo["Additional Device Support"]; ok && support != "" {
			// Split comma-separated values
			parts := splitAndTrim(support, ",")
			additionalSupport = parts
		}

		// Convert to protobuf format
		ipmiInfo := &gatewayv1.IPMIInfo{
			DeviceId:                mcInfo["Device ID"],
			DeviceRevision:          mcInfo["Device Revision"],
			FirmwareRevision:        mcInfo["Firmware Revision"],
			IpmiVersion:             mcInfo["IPMI Version"],
			ManufacturerId:          mcInfo["Manufacturer ID"],
			ManufacturerName:        mcInfo["Manufacturer Name"],
			ProductId:               mcInfo["Product ID"],
			DeviceAvailable:         mcInfo["Device Available"] == "yes",
			ProvidesDeviceSdrs:      mcInfo["Provides Device SDRs"] == "yes",
			AdditionalDeviceSupport: additionalSupport,
		}

		return &gatewayv1.BMCInfo{
			BmcType: "ipmi",
			Details: &gatewayv1.BMCInfo_IpmiInfo{IpmiInfo: ipmiInfo},
		}, nil

	case types.BMCTypeRedfish:
		if c.redfishClient == nil {
			return nil, fmt.Errorf("Redfish client is nil")
		}

		// Add timeout wrapper to prevent hanging on slow/unresponsive BMCs
		infoCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		manager, netProto, err := c.redfishClient.GetManagerInfo(infoCtx, endpoint, username, password)
		if err != nil {
			return nil, fmt.Errorf("Redfish GetManagerInfo failed: %w", err)
		}

		// Build network protocols list
		networkProtocols := []*gatewayv1.NetworkProtocol{}
		if netProto != nil {
			if netProto.SSH.ProtocolEnabled {
				networkProtocols = append(networkProtocols, &gatewayv1.NetworkProtocol{
					Name:    "SSH",
					Port:    netProto.SSH.Port,
					Enabled: true,
				})
			}
			if netProto.HTTPS.ProtocolEnabled {
				networkProtocols = append(networkProtocols, &gatewayv1.NetworkProtocol{
					Name:    "HTTPS",
					Port:    netProto.HTTPS.Port,
					Enabled: true,
				})
			}
			if netProto.HTTP.ProtocolEnabled {
				networkProtocols = append(networkProtocols, &gatewayv1.NetworkProtocol{
					Name:    "HTTP",
					Port:    netProto.HTTP.Port,
					Enabled: true,
				})
			}
			if netProto.IPMI.ProtocolEnabled {
				networkProtocols = append(networkProtocols, &gatewayv1.NetworkProtocol{
					Name:    "IPMI",
					Port:    netProto.IPMI.Port,
					Enabled: true,
				})
			}
		}

		// Build status string
		status := manager.Status.State
		if manager.Status.Health != "" {
			status = status + " (" + manager.Status.Health + ")"
		}

		// Fetch system information with timeout to prevent hanging
		var systemStatus *gatewayv1.SystemStatus
		systemCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		system, err := c.redfishClient.GetSystemInfo(systemCtx, endpoint, username, password)
		if err == nil && system != nil {
			// Build OEM health map (Dell-specific)
			oemHealth := make(map[string]string)
			if system.Oem.Dell.DellSystem.CPURollupStatus != "" {
				oemHealth["cpu_rollup_status"] = system.Oem.Dell.DellSystem.CPURollupStatus
			}
			if system.Oem.Dell.DellSystem.StorageRollupStatus != "" {
				oemHealth["storage_rollup_status"] = system.Oem.Dell.DellSystem.StorageRollupStatus
			}
			if system.Oem.Dell.DellSystem.TempRollupStatus != "" {
				oemHealth["temp_rollup_status"] = system.Oem.Dell.DellSystem.TempRollupStatus
			}
			if system.Oem.Dell.DellSystem.VoltRollupStatus != "" {
				oemHealth["volt_rollup_status"] = system.Oem.Dell.DellSystem.VoltRollupStatus
			}
			if system.Oem.Dell.DellSystem.FanRollupStatus != "" {
				oemHealth["fan_rollup_status"] = system.Oem.Dell.DellSystem.FanRollupStatus
			}
			if system.Oem.Dell.DellSystem.PSRollupStatus != "" {
				oemHealth["ps_rollup_status"] = system.Oem.Dell.DellSystem.PSRollupStatus
			}
			if system.Oem.Dell.DellSystem.BatteryRollupStatus != "" {
				oemHealth["battery_rollup_status"] = system.Oem.Dell.DellSystem.BatteryRollupStatus
			}
			if system.Oem.Dell.DellSystem.SystemHealthRollupStatus != "" {
				oemHealth["system_health_rollup_status"] = system.Oem.Dell.DellSystem.SystemHealthRollupStatus
			}

			systemStatus = &gatewayv1.SystemStatus{
				SystemId:        system.ID,
				BootProgress:    system.BootProgress.LastState,
				BootProgressOem: system.BootProgress.OemLastState,
				PostState:       system.PostState,
				BootSource: &gatewayv1.BootSourceOverride{
					Target:  system.Boot.BootSourceOverrideTarget,
					Enabled: system.Boot.BootSourceOverrideEnabled,
					Mode:    system.Boot.BootSourceOverrideMode,
				},
				BiosVersion:   system.BiosVersion,
				SerialNumber:  system.SerialNumber,
				Sku:           system.SKU,
				Hostname:      system.HostName,
				LastResetTime: system.LastResetTime,
				OemHealth:     oemHealth,
				BootOrder:     system.Boot.BootOrder,
			}
		}
		// If system info fetch fails, continue without it (graceful degradation)

		redfishInfo := &gatewayv1.RedfishInfo{
			ManagerId:        manager.ID,
			Name:             manager.Name,
			Model:            manager.Model,
			Manufacturer:     manager.Manufacturer,
			FirmwareVersion:  manager.FirmwareVersion,
			Status:           status,
			PowerState:       string(manager.PowerState),
			NetworkProtocols: networkProtocols,
			SystemStatus:     systemStatus,
		}

		return &gatewayv1.BMCInfo{
			BmcType: "redfish",
			Details: &gatewayv1.BMCInfo_RedfishInfo{RedfishInfo: redfishInfo},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported BMC type: %s", server.ControlEndpoint.Type)
	}
}

// splitAndTrim splits a string by delimiter and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	parts := []string{}
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

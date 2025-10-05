package bmc

import (
	"context"
	"fmt"

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

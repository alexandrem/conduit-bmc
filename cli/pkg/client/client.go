package client

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"

	"core/types"
	gatewayv1 "gateway/gen/gateway/v1"

	"cli/pkg/config"
)

// Client orchestrates between BMC Manager and Regional Gateways
type Client struct {
	config        *config.Config
	httpClient    *http.Client
	managerClient *BMCManagerClient
	gatewayCache  map[string]*RegionalGatewayClient
}

func New(cfg *config.Config) *Client {
	return &Client{
		config:        cfg,
		httpClient:    &http.Client{},
		managerClient: NewBMCManagerClient(cfg),
		gatewayCache:  make(map[string]*RegionalGatewayClient),
	}
}

// Authenticate performs initial authentication with BMC Manager
func (c *Client) Authenticate(ctx context.Context, email, password string) error {
	result, err := c.managerClient.Authenticate(ctx, email, password)
	if err != nil {
		return err
	}

	fmt.Printf("Authenticated as %s\n", result.Customer.Email)
	fmt.Printf("Access token expires at: %s\n", result.ExpiresAt.Format("2006-01-02 15:04:05"))

	return nil
}

// getGatewayClientWithServerToken returns a gateway client with server-specific token
func (c *Client) getGatewayClientWithServerToken(ctx context.Context, serverID string) (*RegionalGatewayClient, string, error) {
	// Ensure we have a valid token
	if err := c.managerClient.EnsureValidToken(ctx); err != nil {
		return nil, "", fmt.Errorf("failed to ensure valid token: %w", err)
	}

	// Get server-specific token with encrypted BMC context
	serverToken, err := c.managerClient.GetServerToken(ctx, serverID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get server token: %w", err)
	}

	// Get server location from BMC Manager
	location, err := c.managerClient.GetServerLocation(ctx, serverID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get server location: %w", err)
	}

	// Check cache for existing gateway client
	if client, exists := c.gatewayCache[location.RegionalGatewayEndpoint]; exists {
		return client, serverToken.Token, nil
	}

	// Create new gateway client (we'll use server token instead of delegated token)
	gatewayClient := NewRegionalGatewayClient(c.config, location.RegionalGatewayEndpoint, "")
	c.gatewayCache[location.RegionalGatewayEndpoint] = gatewayClient

	return gatewayClient, serverToken.Token, nil
}

// getGatewayClient returns a cached or new Regional Gateway client for a server (legacy method)
func (c *Client) getGatewayClient(ctx context.Context, serverID string) (*RegionalGatewayClient, error) {
	// For non-BMC operations, we still use the old method
	client, _, err := c.getGatewayClientWithServerToken(ctx, serverID)
	return client, err
}

// Updated methods that use the new architecture

type ServerInfo struct {
	ID                string                   `json:"id"`
	ControlEndpoint   *BMCControlEndpoint      `json:"control_endpoint"` // Deprecated: use ControlEndpoints
	ControlEndpoints  []*BMCControlEndpoint    `json:"control_endpoints"`
	PrimaryProtocol   string                   `json:"primary_protocol"`
	SOLEndpoint       *SOLEndpoint             `json:"sol_endpoint"`
	VNCEndpoint       *VNCEndpoint             `json:"vnc_endpoint"`
	Features          []string                 `json:"features"`
	Status            string                   `json:"status"`
	DatacenterID      string                   `json:"datacenter_id"`
	Metadata          map[string]string        `json:"metadata"`
	DiscoveryMetadata *types.DiscoveryMetadata `json:"discovery_metadata,omitempty"`
}

// GetPrimaryControlEndpoint returns the primary control endpoint.
// Looks for endpoint matching PrimaryProtocol, otherwise returns first endpoint or the deprecated ControlEndpoint.
func (s *ServerInfo) GetPrimaryControlEndpoint() *BMCControlEndpoint {
	if len(s.ControlEndpoints) > 0 {
		// Try to find endpoint matching PrimaryProtocol
		if s.PrimaryProtocol != "" {
			for _, ep := range s.ControlEndpoints {
				if ep.Type == s.PrimaryProtocol {
					return ep
				}
			}
		}
		// Fallback to first endpoint
		return s.ControlEndpoints[0]
	}
	// Fallback to deprecated ControlEndpoint field
	return s.ControlEndpoint
}

type BMCControlEndpoint struct {
	Endpoint     string     `json:"endpoint"`
	Type         string     `json:"type"`
	Username     string     `json:"username"`
	Password     string     `json:"password"`
	TLS          *TLSConfig `json:"tls"`
	Capabilities []string   `json:"capabilities"`
}

type SOLEndpoint struct {
	Type     string     `json:"type"`
	Endpoint string     `json:"endpoint"`
	Username string     `json:"username"`
	Password string     `json:"password"`
	Config   *SOLConfig `json:"config"`
}

type VNCEndpoint struct {
	Type     string     `json:"type"`
	Endpoint string     `json:"endpoint"`
	Username string     `json:"username"`
	Password string     `json:"password"`
	Config   *VNCConfig `json:"config"`
}

type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	CACert             string `json:"ca_cert"`
}

type SOLConfig struct {
	BaudRate       int    `json:"baud_rate"`
	FlowControl    string `json:"flow_control"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type VNCConfig struct {
	Protocol string `json:"protocol"`
	Path     string `json:"path"`
	Display  int    `json:"display"`
	ReadOnly bool   `json:"read_only"`
}

type ProxySession struct {
	ID        string `json:"id"`
	Endpoint  string `json:"endpoint"`
	ExpiresAt string `json:"expires_at"`
}

func (c *Client) GetServer(ctx context.Context, serverID string) (*ServerInfo, error) {
	// Ensure we have a valid token
	if err := c.managerClient.EnsureValidToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure valid token: %w", err)
	}

	// Get server from BMC Manager (new BMC-centric architecture)
	server, err := c.managerClient.GetServer(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server from manager: %w", err)
	}

	// Convert from manager Server type to client ServerInfo type
	serverInfo := &ServerInfo{
		ID:           server.ID,
		Features:     server.Features,
		Status:       server.Status,
		DatacenterID: server.DatacenterID,
		Metadata:     server.Metadata,
	}

	// Convert control endpoints
	serverInfo.ControlEndpoints = make([]*BMCControlEndpoint, 0, len(server.ControlEndpoints))
	for _, endpoint := range server.ControlEndpoints {
		bmcEndpoint := &BMCControlEndpoint{
			Endpoint:     endpoint.Endpoint,
			Type:         endpoint.Type,
			Username:     endpoint.Username,
			Password:     endpoint.Password,
			Capabilities: endpoint.Capabilities,
		}
		if endpoint.TLS != nil {
			bmcEndpoint.TLS = &TLSConfig{
				Enabled:            endpoint.TLS.Enabled,
				InsecureSkipVerify: endpoint.TLS.InsecureSkipVerify,
				CACert:             endpoint.TLS.CACert,
			}
		}
		serverInfo.ControlEndpoints = append(serverInfo.ControlEndpoints, bmcEndpoint)
	}
	serverInfo.PrimaryProtocol = server.PrimaryProtocol

	// For backwards compatibility, also set the deprecated ControlEndpoint field
	if server.GetPrimaryControlEndpoint() != nil {
		serverInfo.ControlEndpoint = serverInfo.GetPrimaryControlEndpoint()
	}

	// Convert SOL endpoint
	if server.SOLEndpoint != nil {
		serverInfo.SOLEndpoint = &SOLEndpoint{
			Type:     server.SOLEndpoint.Type,
			Endpoint: server.SOLEndpoint.Endpoint,
			Username: server.SOLEndpoint.Username,
			Password: server.SOLEndpoint.Password,
		}
		if server.SOLEndpoint.Config != nil {
			serverInfo.SOLEndpoint.Config = &SOLConfig{
				BaudRate:       int(server.SOLEndpoint.Config.BaudRate),
				FlowControl:    server.SOLEndpoint.Config.FlowControl,
				TimeoutSeconds: int(server.SOLEndpoint.Config.TimeoutSeconds),
			}
		}
	}

	// Convert VNC endpoint
	if server.VNCEndpoint != nil {
		serverInfo.VNCEndpoint = &VNCEndpoint{
			Type:     server.VNCEndpoint.Type,
			Endpoint: server.VNCEndpoint.Endpoint,
			Username: server.VNCEndpoint.Username,
			Password: server.VNCEndpoint.Password,
		}
		if server.VNCEndpoint.Config != nil {
			serverInfo.VNCEndpoint.Config = &VNCConfig{
				Protocol: server.VNCEndpoint.Config.Protocol,
				Path:     server.VNCEndpoint.Config.Path,
				Display:  int(server.VNCEndpoint.Config.Display),
				ReadOnly: server.VNCEndpoint.Config.ReadOnly,
			}
		}
	}

	// Copy discovery metadata
	serverInfo.DiscoveryMetadata = server.DiscoveryMetadata

	return serverInfo, nil
}

func (c *Client) ListServers(ctx context.Context) ([]ServerInfo, error) {
	// Ensure we have a valid token
	if err := c.managerClient.EnsureValidToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure valid token: %w", err)
	}

	// Get servers from BMC Manager (new BMC-centric architecture)
	servers, err := c.managerClient.ListServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers from manager: %w", err)
	}

	// Convert from manager Server type to client ServerInfo type
	var serverInfos []ServerInfo
	for _, server := range servers {
		serverInfo := ServerInfo{
			ID:           server.ID,
			Features:     server.Features,
			Status:       server.Status,
			DatacenterID: server.DatacenterID,
			Metadata:     server.Metadata,
		}

		// Convert control endpoint (use primary/first endpoint for backwards compatibility)
		if server.GetPrimaryControlEndpoint() != nil {
			serverInfo.ControlEndpoint = &BMCControlEndpoint{
				Endpoint:     server.GetPrimaryControlEndpoint().Endpoint,
				Type:         server.GetPrimaryControlEndpoint().Type,
				Username:     server.GetPrimaryControlEndpoint().Username,
				Password:     server.GetPrimaryControlEndpoint().Password,
				Capabilities: server.GetPrimaryControlEndpoint().Capabilities,
			}
			if server.GetPrimaryControlEndpoint().TLS != nil {
				serverInfo.ControlEndpoint.TLS = &TLSConfig{
					Enabled:            server.GetPrimaryControlEndpoint().TLS.Enabled,
					InsecureSkipVerify: server.GetPrimaryControlEndpoint().TLS.InsecureSkipVerify,
					CACert:             server.GetPrimaryControlEndpoint().TLS.CACert,
				}
			}
		}

		// Convert SOL endpoint
		if server.SOLEndpoint != nil {
			serverInfo.SOLEndpoint = &SOLEndpoint{
				Type:     server.SOLEndpoint.Type,
				Endpoint: server.SOLEndpoint.Endpoint,
				Username: server.SOLEndpoint.Username,
				Password: server.SOLEndpoint.Password,
			}
			if server.SOLEndpoint.Config != nil {
				serverInfo.SOLEndpoint.Config = &SOLConfig{
					BaudRate:       int(server.SOLEndpoint.Config.BaudRate),
					FlowControl:    server.SOLEndpoint.Config.FlowControl,
					TimeoutSeconds: int(server.SOLEndpoint.Config.TimeoutSeconds),
				}
			}
		}

		// Convert VNC endpoint
		if server.VNCEndpoint != nil {
			serverInfo.VNCEndpoint = &VNCEndpoint{
				Type:     server.VNCEndpoint.Type,
				Endpoint: server.VNCEndpoint.Endpoint,
				Username: server.VNCEndpoint.Username,
				Password: server.VNCEndpoint.Password,
			}
			if server.VNCEndpoint.Config != nil {
				serverInfo.VNCEndpoint.Config = &VNCConfig{
					Protocol: server.VNCEndpoint.Config.Protocol,
					Path:     server.VNCEndpoint.Config.Path,
					Display:  int(server.VNCEndpoint.Config.Display),
					ReadOnly: server.VNCEndpoint.Config.ReadOnly,
				}
			}
		}

		serverInfos = append(serverInfos, serverInfo)
	}

	return serverInfos, nil
}

// BMC operation methods that delegate to regional gateways using server tokens

func (c *Client) PowerOn(ctx context.Context, serverID string) error {
	gatewayClient, serverToken, err := c.getGatewayClientWithServerToken(ctx, serverID)
	if err != nil {
		return err
	}
	return gatewayClient.PowerOnWithToken(ctx, serverID, serverToken)
}

func (c *Client) PowerOff(ctx context.Context, serverID string) error {
	gatewayClient, serverToken, err := c.getGatewayClientWithServerToken(ctx, serverID)
	if err != nil {
		return err
	}
	return gatewayClient.PowerOffWithToken(ctx, serverID, serverToken)
}

func (c *Client) PowerCycle(ctx context.Context, serverID string) error {
	gatewayClient, serverToken, err := c.getGatewayClientWithServerToken(ctx, serverID)
	if err != nil {
		return err
	}
	return gatewayClient.PowerCycleWithToken(ctx, serverID, serverToken)
}

func (c *Client) GetPowerStatus(ctx context.Context, serverID string) (string, error) {
	gatewayClient, serverToken, err := c.getGatewayClientWithServerToken(ctx, serverID)
	if err != nil {
		return "", err
	}
	return gatewayClient.GetPowerStatusWithToken(ctx, serverID, serverToken)
}

func (c *Client) Reset(ctx context.Context, serverID string) error {
	gatewayClient, serverToken, err := c.getGatewayClientWithServerToken(ctx, serverID)
	if err != nil {
		return err
	}
	return gatewayClient.ResetWithToken(ctx, serverID, serverToken)
}

func (c *Client) GetBMCInfo(ctx context.Context, serverID string) (*gatewayv1.BMCInfo, error) {
	gatewayClient, serverToken, err := c.getGatewayClientWithServerToken(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return gatewayClient.GetBMCInfoWithToken(ctx, serverID, serverToken)
}

// VNC session management methods

type VNCSession struct {
	ID                string `json:"id"`
	WebsocketEndpoint string `json:"websocket_endpoint"`
	ViewerURL         string `json:"viewer_url"`
	ExpiresAt         string `json:"expires_at"`
}

type SOLSession struct {
	ID                string `json:"id"`
	WebsocketEndpoint string `json:"websocket_endpoint"`
	ConsoleURL        string `json:"console_url"`
	ExpiresAt         string `json:"expires_at"`
}

func (c *Client) CreateVNCSession(ctx context.Context, serverID string) (*VNCSession, error) {
	gatewayClient, serverToken, err := c.getGatewayClientWithServerToken(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return gatewayClient.CreateVNCSessionWithToken(ctx, serverID, serverToken)
}

func (c *Client) GetVNCSession(ctx context.Context, sessionID string) (*VNCSession, error) {
	// For getting VNC session by ID, we need to try all gateway clients
	// In practice, we'd store which gateway a session belongs to
	for _, gatewayClient := range c.gatewayCache {
		session, err := gatewayClient.GetVNCSession(ctx, sessionID)
		if err == nil {
			return session, nil
		}
	}
	return nil, fmt.Errorf("VNC session not found: %s", sessionID)
}

func (c *Client) CloseVNCSession(ctx context.Context, sessionID string) error {
	// For closing VNC session by ID, we need to try all gateway clients
	// In practice, we'd store which gateway a session belongs to
	for _, gatewayClient := range c.gatewayCache {
		err := gatewayClient.CloseVNCSession(ctx, sessionID)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("VNC session not found: %s", sessionID)
}

// SOL session management methods

func (c *Client) CreateSOLSession(ctx context.Context, serverID string) (*SOLSession, error) {
	gatewayClient, serverToken, err := c.getGatewayClientWithServerToken(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return gatewayClient.CreateSOLSessionWithToken(ctx, serverID, serverToken)
}

func (c *Client) GetSOLSession(ctx context.Context, sessionID string) (*SOLSession, error) {
	// For getting SOL session by ID, we need to try all gateway clients
	// In practice, we'd store which gateway a session belongs to
	for _, gatewayClient := range c.gatewayCache {
		session, err := gatewayClient.GetSOLSession(ctx, sessionID)
		if err == nil {
			return session, nil
		}
	}
	return nil, fmt.Errorf("SOL session not found: %s", sessionID)
}

func (c *Client) CloseSOLSession(ctx context.Context, sessionID string) error {
	// For closing SOL session by ID, we need to try all gateway clients
	// In practice, we'd store which gateway a session belongs to
	for _, gatewayClient := range c.gatewayCache {
		err := gatewayClient.CloseSOLSession(ctx, sessionID)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("SOL session not found: %s", sessionID)
}

// StreamConsoleData opens a bidirectional stream for console data
func (c *Client) StreamConsoleData(ctx context.Context, serverID, sessionID string) (*connect.BidiStreamForClient[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk], error) {
	gatewayClient, serverToken, err := c.getGatewayClientWithServerToken(ctx, serverID)
	if err != nil {
		return nil, err
	}
	return gatewayClient.StreamConsoleDataWithToken(ctx, sessionID, serverID, serverToken)
}

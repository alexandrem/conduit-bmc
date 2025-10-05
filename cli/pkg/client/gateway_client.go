package client

import (
	"context"
	"fmt"
	"net/http"

	gatewayv1 "gateway/gen/gateway/v1"
	"gateway/gen/gateway/v1/gatewayv1connect"

	"connectrpc.com/connect"

	"cli/pkg/config"
)

// RegionalGatewayClient handles BMC operations via a specific Regional Gateway
type RegionalGatewayClient struct {
	client         gatewayv1connect.GatewayServiceClient
	config         *config.Config
	endpoint       string
	delegatedToken string
	httpClient     *http.Client
}

func NewRegionalGatewayClient(cfg *config.Config, endpoint, delegatedToken string) *RegionalGatewayClient {
	httpClient := &http.Client{}
	client := gatewayv1connect.NewGatewayServiceClient(httpClient, endpoint)

	return &RegionalGatewayClient{
		client:         client,
		config:         cfg,
		endpoint:       endpoint,
		delegatedToken: delegatedToken,
		httpClient:     httpClient,
	}
}

// BMC Power Operations

func (c *RegionalGatewayClient) PowerOn(ctx context.Context, serverID string) error {
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: serverID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.PowerOn(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to power on server: %w", err)
	}

	if !resp.Msg.Success {
		return fmt.Errorf("power on failed: %s", resp.Msg.Message)
	}

	return nil
}

func (c *RegionalGatewayClient) PowerOff(ctx context.Context, serverID string) error {
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: serverID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.PowerOff(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to power off server: %w", err)
	}

	if !resp.Msg.Success {
		return fmt.Errorf("power off failed: %s", resp.Msg.Message)
	}

	return nil
}

func (c *RegionalGatewayClient) PowerCycle(ctx context.Context, serverID string) error {
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: serverID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.PowerCycle(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to power cycle server: %w", err)
	}

	if !resp.Msg.Success {
		return fmt.Errorf("power cycle failed: %s", resp.Msg.Message)
	}

	return nil
}

func (c *RegionalGatewayClient) Reset(ctx context.Context, serverID string) error {
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: serverID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.Reset(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to reset server: %w", err)
	}

	if !resp.Msg.Success {
		return fmt.Errorf("reset failed: %s", resp.Msg.Message)
	}

	return nil
}

func (c *RegionalGatewayClient) GetPowerStatus(ctx context.Context, serverID string) (string, error) {
	req := connect.NewRequest(&gatewayv1.PowerStatusRequest{
		ServerId: serverID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.GetPowerStatus(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get power status: %w", err)
	}

	return resp.Msg.State.String(), nil
}

// BMC Power Operations with server-specific tokens
func (c *RegionalGatewayClient) PowerOnWithToken(ctx context.Context, serverID, serverToken string) error {
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: serverID,
	})

	c.addAuthHeadersWithToken(req, serverToken)

	resp, err := c.client.PowerOn(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to power on server: %w", err)
	}

	if !resp.Msg.Success {
		return fmt.Errorf("power on failed: %s", resp.Msg.Message)
	}

	return nil
}

func (c *RegionalGatewayClient) PowerOffWithToken(ctx context.Context, serverID, serverToken string) error {
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: serverID,
	})

	c.addAuthHeadersWithToken(req, serverToken)

	resp, err := c.client.PowerOff(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to power off server: %w", err)
	}

	if !resp.Msg.Success {
		return fmt.Errorf("power off failed: %s", resp.Msg.Message)
	}

	return nil
}

func (c *RegionalGatewayClient) PowerCycleWithToken(ctx context.Context, serverID, serverToken string) error {
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: serverID,
	})

	c.addAuthHeadersWithToken(req, serverToken)

	resp, err := c.client.PowerCycle(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to power cycle server: %w", err)
	}

	if !resp.Msg.Success {
		return fmt.Errorf("power cycle failed: %s", resp.Msg.Message)
	}

	return nil
}

func (c *RegionalGatewayClient) ResetWithToken(ctx context.Context, serverID, serverToken string) error {
	req := connect.NewRequest(&gatewayv1.PowerOperationRequest{
		ServerId: serverID,
	})

	c.addAuthHeadersWithToken(req, serverToken)

	resp, err := c.client.Reset(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to reset server: %w", err)
	}

	if !resp.Msg.Success {
		return fmt.Errorf("reset failed: %s", resp.Msg.Message)
	}

	return nil
}

func (c *RegionalGatewayClient) GetPowerStatusWithToken(ctx context.Context, serverID, serverToken string) (string, error) {
	req := connect.NewRequest(&gatewayv1.PowerStatusRequest{
		ServerId: serverID,
	})

	c.addAuthHeadersWithToken(req, serverToken)

	resp, err := c.client.GetPowerStatus(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get power status: %w", err)
	}

	return resp.Msg.State.String(), nil
}

func (c *RegionalGatewayClient) GetBMCInfoWithToken(ctx context.Context, serverID, serverToken string) (*gatewayv1.BMCInfo, error) {
	req := connect.NewRequest(&gatewayv1.GetBMCInfoRequest{
		ServerId: serverID,
	})

	c.addAuthHeadersWithToken(req, serverToken)

	resp, err := c.client.GetBMCInfo(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get BMC info: %w", err)
	}

	return resp.Msg.Info, nil
}

// CreateVNCSession creates a new VNC console session
func (c *RegionalGatewayClient) CreateVNCSession(ctx context.Context, serverID string) (*VNCSession, error) {
	req := connect.NewRequest(&gatewayv1.CreateVNCSessionRequest{
		ServerId: serverID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.CreateVNCSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create VNC session: %w", err)
	}

	session := &VNCSession{
		ID:                resp.Msg.SessionId,
		WebsocketEndpoint: resp.Msg.WebsocketEndpoint,
		ViewerURL:         resp.Msg.ViewerUrl,
	}

	if resp.Msg.ExpiresAt != nil {
		session.ExpiresAt = resp.Msg.ExpiresAt.AsTime().String()
	}

	return session, nil
}

// CreateVNCSessionWithToken creates a new VNC console session using server-specific token
func (c *RegionalGatewayClient) CreateVNCSessionWithToken(ctx context.Context, serverID, serverToken string) (*VNCSession, error) {
	req := connect.NewRequest(&gatewayv1.CreateVNCSessionRequest{
		ServerId: serverID,
	})

	c.addAuthHeadersWithToken(req, serverToken)

	resp, err := c.client.CreateVNCSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create VNC session: %w", err)
	}

	session := &VNCSession{
		ID:                resp.Msg.SessionId,
		WebsocketEndpoint: resp.Msg.WebsocketEndpoint,
		ViewerURL:         resp.Msg.ViewerUrl,
	}

	if resp.Msg.ExpiresAt != nil {
		session.ExpiresAt = resp.Msg.ExpiresAt.AsTime().String()
	}

	return session, nil
}

// GetVNCSession retrieves information about an existing VNC session
func (c *RegionalGatewayClient) GetVNCSession(ctx context.Context, sessionID string) (*VNCSession, error) {
	req := connect.NewRequest(&gatewayv1.GetVNCSessionRequest{
		SessionId: sessionID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.GetVNCSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get VNC session: %w", err)
	}

	session := &VNCSession{
		ID:                resp.Msg.Session.Id,
		WebsocketEndpoint: resp.Msg.Session.WebsocketEndpoint,
		ViewerURL:         resp.Msg.Session.ViewerUrl,
	}

	if resp.Msg.Session.ExpiresAt != nil {
		session.ExpiresAt = resp.Msg.Session.ExpiresAt.AsTime().String()
	}

	return session, nil
}

// CloseVNCSession terminates an active VNC session
func (c *RegionalGatewayClient) CloseVNCSession(ctx context.Context, sessionID string) error {
	req := connect.NewRequest(&gatewayv1.CloseVNCSessionRequest{
		SessionId: sessionID,
	})

	c.addAuthHeaders(req)

	_, err := c.client.CloseVNCSession(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to close VNC session: %w", err)
	}

	return nil
}

// SOL Session Management

func (c *RegionalGatewayClient) CreateSOLSession(ctx context.Context, serverID string) (*SOLSession, error) {
	req := connect.NewRequest(&gatewayv1.CreateSOLSessionRequest{
		ServerId: serverID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.CreateSOLSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOL session: %w", err)
	}

	session := &SOLSession{
		ID:                resp.Msg.SessionId,
		WebsocketEndpoint: resp.Msg.WebsocketEndpoint,
		ConsoleURL:        resp.Msg.ConsoleUrl,
	}

	if resp.Msg.ExpiresAt != nil {
		session.ExpiresAt = resp.Msg.ExpiresAt.AsTime().String()
	}

	return session, nil
}

// CreateSOLSessionWithToken creates a new SOL console session using server-specific token
func (c *RegionalGatewayClient) CreateSOLSessionWithToken(ctx context.Context, serverID, serverToken string) (*SOLSession, error) {
	req := connect.NewRequest(&gatewayv1.CreateSOLSessionRequest{
		ServerId: serverID,
	})

	c.addAuthHeadersWithToken(req, serverToken)

	resp, err := c.client.CreateSOLSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOL session: %w", err)
	}

	session := &SOLSession{
		ID:                resp.Msg.SessionId,
		WebsocketEndpoint: resp.Msg.WebsocketEndpoint,
		ConsoleURL:        resp.Msg.ConsoleUrl,
	}

	if resp.Msg.ExpiresAt != nil {
		session.ExpiresAt = resp.Msg.ExpiresAt.AsTime().String()
	}

	return session, nil
}

// GetSOLSession retrieves information about an existing SOL session
func (c *RegionalGatewayClient) GetSOLSession(ctx context.Context, sessionID string) (*SOLSession, error) {
	req := connect.NewRequest(&gatewayv1.GetSOLSessionRequest{
		SessionId: sessionID,
	})

	c.addAuthHeaders(req)

	resp, err := c.client.GetSOLSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get SOL session: %w", err)
	}

	if resp.Msg.Session == nil {
		return nil, fmt.Errorf("SOL session not found")
	}

	session := &SOLSession{
		ID:                resp.Msg.Session.Id,
		WebsocketEndpoint: resp.Msg.Session.WebsocketEndpoint,
		ConsoleURL:        resp.Msg.Session.ConsoleUrl,
	}

	if resp.Msg.Session.ExpiresAt != nil {
		session.ExpiresAt = resp.Msg.Session.ExpiresAt.AsTime().String()
	}

	return session, nil
}

// CloseSOLSession closes an active SOL session
func (c *RegionalGatewayClient) CloseSOLSession(ctx context.Context, sessionID string) error {
	req := connect.NewRequest(&gatewayv1.CloseSOLSessionRequest{
		SessionId: sessionID,
	})
	c.addAuthHeaders(req)
	_, err := c.client.CloseSOLSession(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to close SOL session: %w", err)
	}
	return nil
}

// StreamConsoleData opens a bidirectional stream for SOL console data
func (c *RegionalGatewayClient) StreamConsoleData(ctx context.Context, sessionID, serverID string) (*connect.BidiStreamForClient[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk], error) {
	// Create bidirectional stream
	stream := c.client.StreamConsoleData(ctx)

	// Send initial handshake
	handshake := &gatewayv1.ConsoleDataChunk{
		SessionId:   sessionID,
		ServerId:    serverID,
		IsHandshake: true,
	}

	if err := stream.Send(handshake); err != nil {
		return nil, fmt.Errorf("failed to send handshake: %w", err)
	}

	return stream, nil
}

// StreamConsoleDataWithToken opens a bidirectional stream for SOL console data using server token
func (c *RegionalGatewayClient) StreamConsoleDataWithToken(ctx context.Context, sessionID, serverID, serverToken string) (*connect.BidiStreamForClient[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk], error) {
	// Note: Connect doesn't support adding headers to streaming RPCs after creation
	// We need to use HTTP interceptors or context metadata instead
	// For now, we'll use the regular method and rely on session authentication
	return c.StreamConsoleData(ctx, sessionID, serverID)
}

func addAuthHeaders[T any](req *connect.Request[T], token string) {
	if token != "" {
		req.Header().Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
}

func (c *RegionalGatewayClient) addAuthHeaders(req interface{}) {
	// Use delegated token for gateway authentication
	token := c.delegatedToken
	if token == "" {
		token = c.config.Auth.AccessToken // Fallback to config token
	}

	switch r := req.(type) {
	case *connect.Request[gatewayv1.PowerOperationRequest]:
		addAuthHeaders(r, token)
	case *connect.Request[gatewayv1.PowerStatusRequest]:
		addAuthHeaders(r, token)
	case *connect.Request[gatewayv1.CreateVNCSessionRequest]:
		addAuthHeaders(r, token)
	case *connect.Request[gatewayv1.GetVNCSessionRequest]:
		addAuthHeaders(r, token)
	case *connect.Request[gatewayv1.CloseVNCSessionRequest]:
		addAuthHeaders(r, token)
	}
}

func (c *RegionalGatewayClient) addAuthHeadersWithToken(req interface{}, serverToken string) {
	// Use server-specific token for gateway authentication
	switch r := req.(type) {
	case *connect.Request[gatewayv1.PowerOperationRequest]:
		addAuthHeaders(r, serverToken)
	case *connect.Request[gatewayv1.PowerStatusRequest]:
		addAuthHeaders(r, serverToken)
	case *connect.Request[gatewayv1.GetBMCInfoRequest]:
		addAuthHeaders(r, serverToken)
	case *connect.Request[gatewayv1.CreateVNCSessionRequest]:
		addAuthHeaders(r, serverToken)
	case *connect.Request[gatewayv1.GetVNCSessionRequest]:
		addAuthHeaders(r, serverToken)
	case *connect.Request[gatewayv1.CloseVNCSessionRequest]:
		addAuthHeaders(r, serverToken)
	case *connect.Request[gatewayv1.CreateSOLSessionRequest]:
		addAuthHeaders(r, serverToken)
	case *connect.Request[gatewayv1.GetSOLSessionRequest]:
		addAuthHeaders(r, serverToken)
	case *connect.Request[gatewayv1.CloseSOLSessionRequest]:
		addAuthHeaders(r, serverToken)
	}
}

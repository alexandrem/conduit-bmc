package agent

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	gatewayv1 "gateway/gen/gateway/v1"
	"local-agent/internal/metrics"
)

// RPC Handler Methods
//
// This file implements the GatewayService RPC interface that allows the gateway
// to call the agent. The agent acts as a service provider for:
// - Power operations (PowerOn, PowerOff, PowerCycle, Reset, GetPowerStatus)
// - Streaming sessions (StreamVNCData, StreamConsoleData)
//
// Methods that return "Unimplemented" are part of the interface but are only
// called ON the gateway (not on the agent), such as:
// - Agent registration/heartbeat (agents call these on gateway)
// - Session management (gateway manages sessions, not agent)
// - Server listing (gateway forwards to manager)

func (a *LocalAgent) HealthCheck(
	ctx context.Context,
	req *connect.Request[gatewayv1.HealthCheckRequest],
) (*connect.Response[gatewayv1.HealthCheckResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement HealthCheck"))
}

func (a *LocalAgent) RegisterAgent(
	ctx context.Context,
	req *connect.Request[gatewayv1.RegisterAgentRequest],
) (*connect.Response[gatewayv1.RegisterAgentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement RegisterAgent"))
}

func (a *LocalAgent) AgentHeartbeat(
	ctx context.Context,
	req *connect.Request[gatewayv1.AgentHeartbeatRequest],
) (*connect.Response[gatewayv1.AgentHeartbeatResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement AgentHeartbeat"))
}

func (a *LocalAgent) PowerOn(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerOperationRequest],
) (*connect.Response[gatewayv1.PowerOperationResponse], error) {
	start := time.Now()

	// Find the server by ID
	server := a.discoveredServers[req.Msg.ServerId]
	if server == nil {
		metrics.BMCOperationsTotal.WithLabelValues("unknown", "power_on", "not_found").Inc()
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %s", req.Msg.ServerId))
	}

	bmcType := string(server.GetPrimaryControlEndpoint().Type)

	// Execute power on operation
	if err := a.bmcClient.PowerOn(ctx, server); err != nil {
		metrics.BMCOperationsTotal.WithLabelValues(bmcType, "power_on", "failure").Inc()
		metrics.BMCOperationDuration.WithLabelValues(bmcType, "power_on").Observe(time.Since(start).Seconds())
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("power on failed: %w", err))
	}

	metrics.BMCOperationsTotal.WithLabelValues(bmcType, "power_on", "success").Inc()
	metrics.BMCOperationDuration.WithLabelValues(bmcType, "power_on").Observe(time.Since(start).Seconds())

	resp := &gatewayv1.PowerOperationResponse{
		Success: true,
		Message: fmt.Sprintf("Power on operation completed for server %s", req.Msg.ServerId),
	}
	return connect.NewResponse(resp), nil
}

func (a *LocalAgent) PowerOff(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerOperationRequest],
) (*connect.Response[gatewayv1.PowerOperationResponse], error) {
	start := time.Now()

	// Find the server by ID
	server := a.discoveredServers[req.Msg.ServerId]
	if server == nil {
		metrics.BMCOperationsTotal.WithLabelValues("unknown", "power_off", "not_found").Inc()
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %s", req.Msg.ServerId))
	}

	bmcType := string(server.GetPrimaryControlEndpoint().Type)

	// Execute power off operation
	if err := a.bmcClient.PowerOff(ctx, server); err != nil {
		metrics.BMCOperationsTotal.WithLabelValues(bmcType, "power_off", "failure").Inc()
		metrics.BMCOperationDuration.WithLabelValues(bmcType, "power_off").Observe(time.Since(start).Seconds())
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("power off failed: %w", err))
	}

	metrics.BMCOperationsTotal.WithLabelValues(bmcType, "power_off", "success").Inc()
	metrics.BMCOperationDuration.WithLabelValues(bmcType, "power_off").Observe(time.Since(start).Seconds())

	resp := &gatewayv1.PowerOperationResponse{
		Success: true,
		Message: fmt.Sprintf("Power off operation completed for server %s", req.Msg.ServerId),
	}
	return connect.NewResponse(resp), nil
}

func (a *LocalAgent) PowerCycle(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerOperationRequest],
) (*connect.Response[gatewayv1.PowerOperationResponse], error) {
	start := time.Now()

	// Find the server by ID
	server := a.discoveredServers[req.Msg.ServerId]
	if server == nil {
		metrics.BMCOperationsTotal.WithLabelValues("unknown", "power_cycle", "not_found").Inc()
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %s", req.Msg.ServerId))
	}

	bmcType := string(server.GetPrimaryControlEndpoint().Type)

	// Execute power cycle operation
	if err := a.bmcClient.PowerCycle(ctx, server); err != nil {
		metrics.BMCOperationsTotal.WithLabelValues(bmcType, "power_cycle", "failure").Inc()
		metrics.BMCOperationDuration.WithLabelValues(bmcType, "power_cycle").Observe(time.Since(start).Seconds())
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("power cycle failed: %w", err))
	}

	metrics.BMCOperationsTotal.WithLabelValues(bmcType, "power_cycle", "success").Inc()
	metrics.BMCOperationDuration.WithLabelValues(bmcType, "power_cycle").Observe(time.Since(start).Seconds())

	resp := &gatewayv1.PowerOperationResponse{
		Success: true,
		Message: fmt.Sprintf("Power cycle operation completed for server %s", req.Msg.ServerId),
	}
	return connect.NewResponse(resp), nil
}

func (a *LocalAgent) Reset(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerOperationRequest],
) (*connect.Response[gatewayv1.PowerOperationResponse], error) {
	start := time.Now()

	// Find the server by ID
	server := a.discoveredServers[req.Msg.ServerId]
	if server == nil {
		metrics.BMCOperationsTotal.WithLabelValues("unknown", "reset", "not_found").Inc()
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %s", req.Msg.ServerId))
	}

	bmcType := string(server.GetPrimaryControlEndpoint().Type)

	// Execute reset operation
	if err := a.bmcClient.Reset(ctx, server); err != nil {
		metrics.BMCOperationsTotal.WithLabelValues(bmcType, "reset", "failure").Inc()
		metrics.BMCOperationDuration.WithLabelValues(bmcType, "reset").Observe(time.Since(start).Seconds())
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("reset failed: %w", err))
	}

	metrics.BMCOperationsTotal.WithLabelValues(bmcType, "reset", "success").Inc()
	metrics.BMCOperationDuration.WithLabelValues(bmcType, "reset").Observe(time.Since(start).Seconds())

	resp := &gatewayv1.PowerOperationResponse{
		Success: true,
		Message: fmt.Sprintf("Reset operation completed for server %s", req.Msg.ServerId),
	}
	return connect.NewResponse(resp), nil
}

func (a *LocalAgent) GetPowerStatus(
	ctx context.Context,
	req *connect.Request[gatewayv1.PowerStatusRequest],
) (*connect.Response[gatewayv1.PowerStatusResponse], error) {
	start := time.Now()

	// Find the server by ID
	server := a.discoveredServers[req.Msg.ServerId]
	if server == nil {
		metrics.BMCOperationsTotal.WithLabelValues("unknown", "get_status", "not_found").Inc()
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server not found: %s", req.Msg.ServerId))
	}

	bmcType := string(server.GetPrimaryControlEndpoint().Type)

	// Get power state
	stateStr, err := a.bmcClient.GetPowerState(ctx, server)
	if err != nil {
		metrics.BMCOperationsTotal.WithLabelValues(bmcType, "get_status", "failure").Inc()
		metrics.BMCOperationDuration.WithLabelValues(bmcType, "get_status").Observe(time.Since(start).Seconds())
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get power state failed: %w", err))
	}

	metrics.BMCOperationsTotal.WithLabelValues(bmcType, "get_status", "success").Inc()
	metrics.BMCOperationDuration.WithLabelValues(bmcType, "get_status").Observe(time.Since(start).Seconds())

	// Convert string state to protobuf enum
	var state gatewayv1.PowerState
	switch stateStr {
	case "on", "On":
		state = gatewayv1.PowerState_POWER_STATE_ON
	case "off", "Off":
		state = gatewayv1.PowerState_POWER_STATE_OFF
	default:
		state = gatewayv1.PowerState_POWER_STATE_UNKNOWN
	}

	resp := &gatewayv1.PowerStatusResponse{
		State:   state,
		Message: fmt.Sprintf("Power state: %s", stateStr),
	}
	return connect.NewResponse(resp), nil
}

func (a *LocalAgent) CreateVNCSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.CreateVNCSessionRequest],
) (*connect.Response[gatewayv1.CreateVNCSessionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement CreateVNCSession"))
}

func (a *LocalAgent) GetVNCSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.GetVNCSessionRequest],
) (*connect.Response[gatewayv1.GetVNCSessionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement GetVNCSession"))
}

func (a *LocalAgent) CloseVNCSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.CloseVNCSessionRequest],
) (*connect.Response[gatewayv1.CloseVNCSessionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement CloseVNCSession"))
}

func (a *LocalAgent) StartVNCProxy(
	ctx context.Context,
	req *connect.Request[gatewayv1.StartVNCProxyRequest],
) (*connect.Response[gatewayv1.StartVNCProxyResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement StartVNCProxy"))
}

func (a *LocalAgent) CreateSOLSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.CreateSOLSessionRequest],
) (*connect.Response[gatewayv1.CreateSOLSessionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement CreateSOLSession"))
}

func (a *LocalAgent) GetSOLSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.GetSOLSessionRequest],
) (*connect.Response[gatewayv1.GetSOLSessionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement GetSOLSession"))
}

func (a *LocalAgent) CloseSOLSession(
	ctx context.Context,
	req *connect.Request[gatewayv1.CloseSOLSessionRequest],
) (*connect.Response[gatewayv1.CloseSOLSessionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("agents do not implement CloseSOLSession"))
}

func (a *LocalAgent) GetBMCInfo(
	ctx context.Context,
	req *connect.Request[gatewayv1.GetBMCInfoRequest],
) (*connect.Response[gatewayv1.GetBMCInfoResponse], error) {
	// Find the server by ID
	server := a.discoveredServers[req.Msg.ServerId]
	if server == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("server %s not found", req.Msg.ServerId))
	}

	// Get BMC info using BMC client
	bmcInfo, err := a.bmcClient.GetBMCInfo(ctx, server)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get BMC info: %w", err))
	}

	return connect.NewResponse(&gatewayv1.GetBMCInfoResponse{
		Info: bmcInfo,
	}), nil
}

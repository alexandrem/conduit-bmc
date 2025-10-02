package gateway

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	gatewayv1 "gateway/gen/gateway/v1"
)

// StreamVNCData handles VNC data streaming from agents (agent->gateway direction)
// This is NOT used in the current architecture where gateway initiates streams to agents
func (h *RegionalGatewayHandler) StreamVNCData(
	ctx context.Context,
	stream *connect.BidiStream[gatewayv1.VNCDataChunk, gatewayv1.VNCDataChunk],
) error {
	return connect.NewError(connect.CodeUnimplemented,
		fmt.Errorf("gateway does not accept incoming VNC streams - gateway initiates streams to agents"))
}

// StreamConsoleData handles console data streaming from agents (agent->gateway direction)
// This is NOT used in the current architecture where gateway initiates streams to agents
func (h *RegionalGatewayHandler) StreamConsoleData(
	ctx context.Context,
	stream *connect.BidiStream[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk],
) error {
	return connect.NewError(connect.CodeUnimplemented,
		fmt.Errorf("gateway does not accept incoming console streams - gateway initiates streams to agents"))
}

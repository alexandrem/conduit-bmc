package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"

	"core/streaming"
	gatewayv1 "gateway/gen/gateway/v1"
	"gateway/gen/gateway/v1/gatewayv1connect"
	gatewaystreaming "gateway/internal/streaming"
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

// StreamConsoleData handles console data streaming from CLI clients
// This proxies the stream between CLI and the appropriate agent
func (h *RegionalGatewayHandler) StreamConsoleData(
	ctx context.Context,
	clientStream *connect.BidiStream[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk],
) error {
	log.Info().Msg("New CLI console streaming connection")

	// Receive handshake from CLI to get session and server info
	helper := streaming.NewHandshakeHelper(&gatewaystreaming.ConsoleChunkFactory{})
	sessionID, serverID, err := helper.ReceiveHandshake(clientStream)
	if err != nil {
		return fmt.Errorf("failed to receive handshake from CLI: %w", err)
	}

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", serverID).
		Msg("Console handshake received from CLI")

	// Get the SOL session to find which agent to connect to
	solSession, exists := h.GetSOLSessionByID(sessionID)
	if !exists {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("SOL session not found: %s", sessionID))
	}

	// Get agent information
	agentInfo := h.agentRegistry.Get(solSession.AgentID)
	if agentInfo == nil {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("agent not available: %s", solSession.AgentID))
	}

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", serverID).
		Str("agent_id", solSession.AgentID).
		Msg("Proxying CLI console stream to agent")

	// Create HTTP client with HTTP/2 support
	httpClient := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}

	// Create agent client
	agentClient := gatewayv1connect.NewGatewayServiceClient(&http.Client{Transport: httpClient}, agentInfo.Endpoint)

	// Create stream to agent
	agentStream := agentClient.StreamConsoleData(ctx)

	// Send handshake to agent
	if err := helper.SendHandshake(agentStream, sessionID, serverID); err != nil {
		return fmt.Errorf("failed to send handshake to agent: %w", err)
	}

	log.Debug().Str("server_id", serverID).Msg("Sent console handshake to agent")

	// Send handshake ack back to CLI
	if err := helper.SendHandshakeAck(clientStream, sessionID, serverID); err != nil {
		return fmt.Errorf("failed to send handshake ack to CLI: %w", err)
	}

	// Proxy bidirectionally between CLI and agent
	return h.proxyConsoleStreams(ctx, clientStream, agentStream, sessionID, serverID)
}

// proxyConsoleStreams proxies console data bidirectionally between CLI and agent
func (h *RegionalGatewayHandler) proxyConsoleStreams(
	ctx context.Context,
	clientStream *connect.BidiStream[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk],
	agentStream *connect.BidiStreamForClient[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk],
	sessionID, serverID string,
) error {
	errChan := make(chan error, 2)

	// Goroutine: Agent -> CLI
	go func() {
		defer log.Debug().Msg("Agent->CLI console proxy goroutine exiting")
		for {
			chunk, err := agentStream.Receive()
			if err != nil {
				errChan <- fmt.Errorf("agent stream receive error: %w", err)
				return
			}

			// Forward to CLI
			if err := clientStream.Send(chunk); err != nil {
				errChan <- fmt.Errorf("client stream send error: %w", err)
				return
			}

			// Check for close signal
			if chunk.CloseStream {
				log.Debug().Msg("Received close signal from agent")
				return
			}
		}
	}()

	// Goroutine: CLI -> Agent
	go func() {
		defer log.Debug().Msg("CLI->Agent console proxy goroutine exiting")
		for {
			chunk, err := clientStream.Receive()
			if err != nil {
				errChan <- fmt.Errorf("client stream receive error: %w", err)
				return
			}

			// Forward to agent
			if err := agentStream.Send(chunk); err != nil {
				errChan <- fmt.Errorf("agent stream send error: %w", err)
				return
			}

			// Check for close signal
			if chunk.CloseStream {
				log.Debug().Msg("Received close signal from CLI")
				return
			}
		}
	}()

	// Wait for either direction to fail
	err := <-errChan
	log.Info().Err(err).Msg("Console proxy terminated")

	// Send close signals
	closeChunk := &gatewayv1.ConsoleDataChunk{
		SessionId:   sessionID,
		ServerId:    serverID,
		CloseStream: true,
	}
	clientStream.Send(closeChunk)
	agentStream.Send(closeChunk)

	return nil
}

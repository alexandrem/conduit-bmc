package agent

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"

	"core/streaming"
	gatewayv1 "gateway/gen/gateway/v1"
	agentstreaming "local-agent/internal/streaming"
	"local-agent/pkg/sol"
	"local-agent/pkg/vnc"
)

// StreamVNCData implements bidirectional streaming for VNC data
// Gateway sends VNC data from browser, agent forwards to BMC VNC endpoint using native TCP
func (a *LocalAgent) StreamVNCData(
	ctx context.Context,
	stream *connect.BidiStream[gatewayv1.VNCDataChunk, gatewayv1.VNCDataChunk],
) error {
	log.Info().Msg("New VNC streaming connection")

	// Receive handshake from gateway
	helper := streaming.NewHandshakeHelper(&agentstreaming.VNCChunkFactory{})
	sessionID, serverID, err := helper.ReceiveHandshake(stream)
	if err != nil {
		return err
	}

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", serverID).
		Msg("VNC handshake received")

	// Look up server in discovered servers
	server, exists := a.discoveredServers[serverID]
	if !exists {
		return fmt.Errorf("server %s not found", serverID)
	}

	// Check if server has VNC endpoint
	if server.VNCEndpoint == nil {
		return fmt.Errorf("VNC not available for server %s", serverID)
	}

	// Create VNC endpoint configuration
	// Transport type is auto-detected from endpoint URL scheme
	vncEndpoint := &vnc.Endpoint{
		Endpoint: server.VNCEndpoint.Endpoint,
		Username: server.VNCEndpoint.Username,
		Password: server.VNCEndpoint.Password,
	}

	// Add TLS configuration if present (for VeNCrypt, RFB-over-TLS, enterprise BMCs)
	if server.VNCEndpoint.TLS != nil {
		log.Debug().
			Bool("tls_enabled", server.VNCEndpoint.TLS.Enabled).
			Bool("insecure_skip_verify", server.VNCEndpoint.TLS.InsecureSkipVerify).
			Msg("VNC endpoint has TLS configuration")

		vncEndpoint.TLS = &vnc.TLSConfig{
			Enabled:            server.VNCEndpoint.TLS.Enabled,
			InsecureSkipVerify: server.VNCEndpoint.TLS.InsecureSkipVerify,
		}
	} else {
		log.Debug().Msg("VNC endpoint has no TLS configuration")
	}

	// Create appropriate VNC transport based on endpoint type
	vncTransport, err := vnc.NewTransport(vncEndpoint)
	if err != nil {
		return fmt.Errorf("failed to create VNC transport: %w", err)
	}
	defer vncTransport.Close()

	log.Debug().
		Str("endpoint", server.VNCEndpoint.Endpoint).
		Msg("Connecting to VNC endpoint (transport auto-detected from URL scheme)")

	// Connect to VNC endpoint
	if err := vnc.ConnectTransport(ctx, vncTransport, vncEndpoint); err != nil {
		return fmt.Errorf("failed to connect to VNC endpoint: %w", err)
	}

	// Determine transport type for logging
	transportType := "unknown"
	switch vncTransport.(type) {
	case *vnc.NativeTransport:
		transportType = "native-tcp"
	case *vnc.WebSocketTransport:
		transportType = "websocket"
	}

	log.Info().
		Str("server_id", serverID).
		Str("endpoint", server.VNCEndpoint.Endpoint).
		Str("transport", transportType).
		Msg("Connected and authenticated with VNC endpoint")

	// Create RFB proxy handler to manage browser-side RFB handshake
	// The browser (noVNC) expects to do a full RFB handshake, but we've already
	// authenticated with the BMC. The RFBProxyHandler terminates the browser's
	// handshake and proxies to the authenticated BMC connection.
	rfbProxy := vnc.NewRFBProxyHandler(vncTransport)

	// Create a stream adapter for the RFB proxy handler
	streamAdapter := &vncStreamAdapter{
		stream:    stream,
		sessionID: sessionID,
		serverID:  serverID,
	}

	// Handle browser's RFB handshake
	log.Debug().Msg("Handling browser RFB handshake via proxy")
	if err := rfbProxy.HandleBrowserHandshake(ctx, streamAdapter); err != nil {
		return fmt.Errorf("RFB proxy handshake failed: %w", err)
	}

	log.Info().Msg("Browser RFB handshake completed, starting framebuffer data proxying")

	// Send handshake acknowledgment back to gateway AFTER RFB handshake completes
	if err := helper.SendHandshakeAck(stream, sessionID, serverID); err != nil {
		return fmt.Errorf("failed to send handshake ack: %w", err)
	}

	// Use TCP streaming proxy to handle bidirectional data flow
	logger := log.With().
		Str("session_id", sessionID).
		Str("server_id", serverID).
		Str("protocol", "vnc").
		Str("transport", transportType).
		Logger()

	proxy := streaming.NewStreamToTCPProxy(
		sessionID,
		serverID,
		logger,
		&agentstreaming.VNCChunkFactory{},
	)

	return proxy.ProxyFromStream(ctx, stream, vncTransport)
}

// vncStreamAdapter adapts the gRPC stream to io.ReadWriter for RFB proxy
type vncStreamAdapter struct {
	stream    *connect.BidiStream[gatewayv1.VNCDataChunk, gatewayv1.VNCDataChunk]
	sessionID string
	serverID  string
	readBuf   []byte
	readPos   int
}

func (v *vncStreamAdapter) Read(p []byte) (int, error) {
	// Return buffered data first
	if v.readPos < len(v.readBuf) {
		n := copy(p, v.readBuf[v.readPos:])
		v.readPos += n
		if v.readPos >= len(v.readBuf) {
			v.readBuf = nil
			v.readPos = 0
		}
		return n, nil
	}

	// Receive next chunk from stream
	chunk, err := v.stream.Receive()
	if err != nil {
		return 0, fmt.Errorf("stream receive error: %w", err)
	}

	// Skip handshake chunks
	if chunk.IsHandshake {
		return v.Read(p) // Recursively read next chunk
	}

	// Handle close signal
	if chunk.CloseStream {
		return 0, fmt.Errorf("stream closed by client")
	}

	// Copy data to output buffer
	n := copy(p, chunk.Data)

	// Buffer remaining data if any
	if n < len(chunk.Data) {
		v.readBuf = chunk.Data
		v.readPos = n
	}

	return n, nil
}

func (v *vncStreamAdapter) Write(p []byte) (int, error) {
	chunk := &gatewayv1.VNCDataChunk{
		SessionId:   v.sessionID,
		ServerId:    v.serverID,
		Data:        p,
		IsHandshake: false,
		CloseStream: false,
	}

	if err := v.stream.Send(chunk); err != nil {
		return 0, fmt.Errorf("stream send error: %w", err)
	}

	return len(p), nil
}

// StreamConsoleData implements bidirectional streaming for SOL/Console data
// Gateway sends console data from browser, agent forwards to BMC SOL endpoint
func (a *LocalAgent) StreamConsoleData(
	ctx context.Context,
	stream *connect.BidiStream[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk],
) error {
	log.Info().Msg("New console streaming connection")

	// Receive handshake from gateway
	helper := streaming.NewHandshakeHelper(&agentstreaming.ConsoleChunkFactory{})
	sessionID, serverID, err := helper.ReceiveHandshake(stream)
	if err != nil {
		return err
	}

	log.Info().
		Str("session_id", sessionID).
		Str("server_id", serverID).
		Msg("Console handshake received")

	// Look up server in discovered servers
	server, exists := a.discoveredServers[serverID]
	if !exists {
		return fmt.Errorf("server %s not found", serverID)
	}

	// Check if server has SOL endpoint
	if server.SOLEndpoint == nil {
		return fmt.Errorf("SOL not available for server %s", serverID)
	}

	log.Debug().
		Str("endpoint", server.SOLEndpoint.Endpoint).
		Str("type", server.SOLEndpoint.Type.String()).
		Msg("Connecting to SOL endpoint")

	// Create SOL client using the factory based on BMC type
	solClient, err := sol.NewClient(server.SOLEndpoint.Type)
	if err != nil {
		return fmt.Errorf("failed to create SOL client: %w", err)
	}

	// Prepare SOL config, inheriting TLS settings from control endpoint
	solConfig := sol.DefaultSOLConfig()
	if server.ControlEndpoint != nil && server.ControlEndpoint.TLS != nil {
		solConfig.InsecureSkipVerify = server.ControlEndpoint.TLS.InsecureSkipVerify
	} else {
		// Default to true for BMCs (they typically use self-signed certs)
		solConfig.InsecureSkipVerify = true
	}

	// Create SOL session
	solSession, err := solClient.CreateSession(ctx, server.SOLEndpoint.Endpoint, server.SOLEndpoint.Username, server.SOLEndpoint.Password, solConfig)
	if err != nil {
		return fmt.Errorf("failed to create SOL session: %w", err)
	}
	defer solSession.Close()

	log.Info().
		Str("server_id", serverID).
		Msg("Connected to SOL endpoint")

	// Send handshake acknowledgment back to gateway
	if err := helper.SendHandshakeAck(stream, sessionID, serverID); err != nil {
		return fmt.Errorf("failed to send handshake ack: %w", err)
	}

	// Proxy SOL data bidirectionally between stream and SOL session
	return a.proxySOLSession(ctx, stream, solSession, sessionID, serverID)
}

// proxySOLSession proxies data between buf Connect stream and SOL session
func (a *LocalAgent) proxySOLSession(
	ctx context.Context,
	stream *connect.BidiStream[gatewayv1.ConsoleDataChunk, gatewayv1.ConsoleDataChunk],
	solSession sol.Session,
	sessionID, serverID string,
) error {
	errChan := make(chan error, 2)

	// Goroutine: SOL -> Stream (read from BMC, send to gateway)
	go func() {
		defer log.Debug().Msg("SOL->Stream goroutine exiting")
		for {
			// Read from SOL session
			data, err := solSession.Read(ctx)
			if err != nil {
				errChan <- fmt.Errorf("SOL read error: %w", err)
				return
			}

			if len(data) > 0 {
				log.Debug().Int("bytes", len(data)).Msg("Forwarding data from SOL to stream")

				// Create chunk and send to stream
				chunk := &gatewayv1.ConsoleDataChunk{
					SessionId:   sessionID,
					ServerId:    serverID,
					Data:        data,
					IsHandshake: false,
					CloseStream: false,
				}

				if err := stream.Send(chunk); err != nil {
					errChan <- fmt.Errorf("stream send error: %w", err)
					return
				}
			}
		}
	}()

	// Goroutine: Stream -> SOL (receive from gateway, write to BMC)
	go func() {
		defer log.Debug().Msg("Stream->SOL goroutine exiting")
		for {
			chunk, err := stream.Receive()
			if err != nil {
				errChan <- fmt.Errorf("stream receive error: %w", err)
				return
			}

			// Check for close signal
			if chunk.CloseStream {
				log.Debug().Msg("Received close signal from stream")
				errChan <- fmt.Errorf("stream closed")
				return
			}

			// Skip handshake chunks
			if chunk.IsHandshake {
				continue
			}

			if len(chunk.Data) > 0 {
				log.Debug().Int("bytes", len(chunk.Data)).Msg("Forwarding data from stream to SOL")

				// Write to SOL session
				if err := solSession.Write(ctx, chunk.Data); err != nil {
					errChan <- fmt.Errorf("SOL write error: %w", err)
					return
				}
			}
		}
	}()

	// Wait for either direction to fail
	err := <-errChan
	log.Info().Err(err).Msg("Console proxy terminated")

	// Send close signal
	closeChunk := &gatewayv1.ConsoleDataChunk{
		SessionId:   sessionID,
		ServerId:    serverID,
		Data:        nil,
		IsHandshake: false,
		CloseStream: true,
	}
	stream.Send(closeChunk)

	return nil
}

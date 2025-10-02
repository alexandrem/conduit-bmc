package streaming

import (
	"context"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// StreamChunk is a generic interface for streaming data chunks
// Protobuf generated types (VNCDataChunk, ConsoleDataChunk) implement this
type StreamChunk interface {
	GetSessionId() string
	GetServerId() string
	GetData() []byte
	GetIsHandshake() bool
	GetCloseStream() bool
}

// ChunkFactory creates new chunk instances
type ChunkFactory[T StreamChunk] interface {
	NewChunk(sessionID, serverID string, data []byte, isHandshake, closeStream bool) T
}

// WebSocketToStreamProxy handles WebSocket -> buf Connect streaming translation
// This is used by the gateway to translate browser WebSocket to agent streaming RPC
type WebSocketToStreamProxy[T StreamChunk] struct {
	wsConn    *websocket.Conn
	sessionID string
	serverID  string
	logger    zerolog.Logger
	factory   ChunkFactory[T]
}

// NewWebSocketToStreamProxy creates a new WebSocket to stream proxy
func NewWebSocketToStreamProxy[T StreamChunk](
	wsConn *websocket.Conn,
	sessionID, serverID string,
	logger zerolog.Logger,
	factory ChunkFactory[T],
) *WebSocketToStreamProxy[T] {
	return &WebSocketToStreamProxy[T]{
		wsConn:    wsConn,
		sessionID: sessionID,
		serverID:  serverID,
		logger:    logger,
		factory:   factory,
	}
}

// ProxyToStream handles bidirectional proxying: WebSocket <-> buf Connect stream
func (p *WebSocketToStreamProxy[T]) ProxyToStream(
	ctx context.Context,
	stream interface {
		Send(T) error
		Receive() (T, error)
		CloseRequest() error
	},
) error {
	errChan := make(chan error, 2)

	// Goroutine: WebSocket -> Stream
	go func() {
		defer p.logger.Debug().Msg("WebSocket->Stream goroutine exiting")
		for {
			messageType, data, err := p.wsConn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("WebSocket read error: %w", err)
				return
			}

			// Only handle binary/text messages
			if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
				continue
			}

			p.logger.Debug().Int("bytes", len(data)).Msg("Proxying data from WebSocket to stream")

			chunk := p.factory.NewChunk(p.sessionID, p.serverID, data, false, false)
			if err := stream.Send(chunk); err != nil {
				errChan <- fmt.Errorf("stream send error: %w", err)
				return
			}
		}
	}()

	// Goroutine: Stream -> WebSocket
	go func() {
		defer p.logger.Debug().Msg("Stream->WebSocket goroutine exiting")
		for {
			chunk, err := stream.Receive()
			if err != nil {
				errChan <- fmt.Errorf("stream receive error: %w", err)
				return
			}

			// Check for close signal
			if chunk.GetCloseStream() {
				p.logger.Debug().Msg("Received close signal from stream")
				errChan <- fmt.Errorf("stream closed")
				return
			}

			// Skip handshake responses
			if chunk.GetIsHandshake() {
				p.logger.Debug().Msg("Received handshake response")
				continue
			}

			data := chunk.GetData()
			if len(data) > 0 {
				p.logger.Debug().Int("bytes", len(data)).Msg("Proxying data from stream to WebSocket")

				if err := p.wsConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					errChan <- fmt.Errorf("WebSocket write error: %w", err)
					return
				}
			}
		}
	}()

	// Wait for either direction to fail
	err := <-errChan
	p.logger.Debug().Err(err).Msg("Proxy terminated")

	// Send close signal
	closeChunk := p.factory.NewChunk(p.sessionID, p.serverID, nil, false, true)
	stream.Send(closeChunk)
	stream.CloseRequest()

	return nil
}

// StreamToWebSocketProxy handles buf Connect streaming -> WebSocket translation
// This is used by the agent to translate gateway streaming RPC to BMC WebSocket
type StreamToWebSocketProxy[T StreamChunk] struct {
	sessionID string
	serverID  string
	logger    zerolog.Logger
	factory   ChunkFactory[T]
}

// NewStreamToWebSocketProxy creates a new stream to WebSocket proxy
func NewStreamToWebSocketProxy[T StreamChunk](
	sessionID, serverID string,
	logger zerolog.Logger,
	factory ChunkFactory[T],
) *StreamToWebSocketProxy[T] {
	return &StreamToWebSocketProxy[T]{
		sessionID: sessionID,
		serverID:  serverID,
		logger:    logger,
		factory:   factory,
	}
}

// ProxyFromStream handles bidirectional proxying: buf Connect stream <-> WebSocket
func (p *StreamToWebSocketProxy[T]) ProxyFromStream(
	ctx context.Context,
	stream interface {
		Send(T) error
		Receive() (T, error)
	},
	wsConn *websocket.Conn,
) error {
	errChan := make(chan error, 2)

	// Goroutine: Stream -> WebSocket
	go func() {
		defer p.logger.Debug().Msg("Stream->WebSocket goroutine exiting")
		for {
			chunk, err := stream.Receive()
			if err != nil {
				errChan <- fmt.Errorf("stream receive error: %w", err)
				return
			}

			// Check for close signal
			if chunk.GetCloseStream() {
				p.logger.Debug().Msg("Received close signal from stream")
				errChan <- fmt.Errorf("stream closed")
				return
			}

			// Skip handshake chunks
			if chunk.GetIsHandshake() {
				continue
			}

			data := chunk.GetData()
			if len(data) > 0 {
				p.logger.Debug().Int("bytes", len(data)).Msg("Forwarding data from stream to WebSocket")

				if err := wsConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
					errChan <- fmt.Errorf("WebSocket write error: %w", err)
					return
				}
			}
		}
	}()

	// Goroutine: WebSocket -> Stream
	go func() {
		defer p.logger.Debug().Msg("WebSocket->Stream goroutine exiting")
		for {
			messageType, data, err := wsConn.ReadMessage()
			if err != nil {
				errChan <- fmt.Errorf("WebSocket read error: %w", err)
				return
			}

			// Only handle binary/text messages
			if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
				continue
			}

			p.logger.Debug().Int("bytes", len(data)).Msg("Forwarding data from WebSocket to stream")

			chunk := p.factory.NewChunk(p.sessionID, p.serverID, data, false, false)
			if err := stream.Send(chunk); err != nil {
				errChan <- fmt.Errorf("stream send error: %w", err)
				return
			}
		}
	}()

	// Wait for either direction to fail
	err := <-errChan
	p.logger.Debug().Err(err).Msg("Proxy terminated")

	// Send close signal
	closeChunk := p.factory.NewChunk(p.sessionID, p.serverID, nil, false, true)
	stream.Send(closeChunk)

	return nil
}

// HandshakeHelper helps with initial stream handshakes
type HandshakeHelper[T StreamChunk] struct {
	factory ChunkFactory[T]
}

// NewHandshakeHelper creates a handshake helper
func NewHandshakeHelper[T StreamChunk](factory ChunkFactory[T]) *HandshakeHelper[T] {
	return &HandshakeHelper[T]{factory: factory}
}

// SendHandshake sends a handshake chunk
func (h *HandshakeHelper[T]) SendHandshake(
	stream interface{ Send(T) error },
	sessionID, serverID string,
) error {
	chunk := h.factory.NewChunk(sessionID, serverID, nil, true, false)
	return stream.Send(chunk)
}

// ReceiveHandshake receives and validates a handshake chunk
func (h *HandshakeHelper[T]) ReceiveHandshake(
	stream interface{ Receive() (T, error) },
) (sessionID, serverID string, err error) {
	chunk, err := stream.Receive()
	if err != nil {
		return "", "", fmt.Errorf("failed to receive handshake: %w", err)
	}

	if !chunk.GetIsHandshake() {
		return "", "", fmt.Errorf("expected handshake chunk, got data chunk")
	}

	return chunk.GetSessionId(), chunk.GetServerId(), nil
}

// SendHandshakeAck sends a handshake acknowledgment
func (h *HandshakeHelper[T]) SendHandshakeAck(
	stream interface{ Send(T) error },
	sessionID, serverID string,
) error {
	ackChunk := h.factory.NewChunk(sessionID, serverID, nil, true, false)
	return stream.Send(ackChunk)
}

package streaming

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/rs/zerolog"
)

// TCPReader abstracts reading from a TCP connection or any net.Conn
type TCPReader interface {
	Read(ctx context.Context) ([]byte, error)
}

// TCPWriter abstracts writing to a TCP connection or any net.Conn
type TCPWriter interface {
	Write(ctx context.Context, data []byte) error
}

// TCPTransport combines reader and writer for bidirectional TCP communication
type TCPTransport interface {
	TCPReader
	TCPWriter
	Close() error
}

// StreamToTCPProxy handles bidirectional proxying between buf Connect stream and TCP connection
// This is used by the agent to translate gateway streaming RPC to native TCP protocols (VNC, etc.)
type StreamToTCPProxy[T StreamChunk] struct {
	sessionID string
	serverID  string
	logger    zerolog.Logger
	factory   ChunkFactory[T]
}

// NewStreamToTCPProxy creates a new stream to TCP proxy
func NewStreamToTCPProxy[T StreamChunk](
	sessionID, serverID string,
	logger zerolog.Logger,
	factory ChunkFactory[T],
) *StreamToTCPProxy[T] {
	return &StreamToTCPProxy[T]{
		sessionID: sessionID,
		serverID:  serverID,
		logger:    logger,
		factory:   factory,
	}
}

// ProxyFromStream handles bidirectional proxying: buf Connect stream <-> TCP connection
func (p *StreamToTCPProxy[T]) ProxyFromStream(
	ctx context.Context,
	stream interface {
		Send(T) error
		Receive() (T, error)
	},
	transport TCPTransport,
) error {
	errChan := make(chan error, 2)

	// Goroutine: Stream -> TCP
	go func() {
		defer p.logger.Debug().Msg("Stream->TCP goroutine exiting")
		for {
			chunk, err := stream.Receive()
			if err != nil {
				if err == io.EOF {
					errChan <- fmt.Errorf("stream closed by client")
				} else {
					errChan <- fmt.Errorf("stream receive error: %w", err)
				}
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
				// p.logger.Debug().Int("bytes", len(data)).Msg("Forwarding data from stream to TCP")

				if err := transport.Write(ctx, data); err != nil {
					errChan <- fmt.Errorf("TCP write error: %w", err)
					return
				}
			}
		}
	}()

	// Goroutine: TCP -> Stream
	go func() {
		defer p.logger.Debug().Msg("TCP->Stream goroutine exiting")
		for {
			data, err := transport.Read(ctx)
			if err != nil {
				if err == io.EOF {
					errChan <- fmt.Errorf("TCP connection closed")
				} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is not fatal, continue reading
					p.logger.Debug().Msg("TCP read timeout, continuing...")
					continue
				} else {
					errChan <- fmt.Errorf("TCP read error: %w", err)
				}
				return
			}

			if len(data) > 0 {
				// p.logger.Debug().Int("bytes", len(data)).Msg("Forwarding data from TCP to stream")

				chunk := p.factory.NewChunk(p.sessionID, p.serverID, data, false, false)
				if err := stream.Send(chunk); err != nil {
					errChan <- fmt.Errorf("stream send error: %w", err)
					return
				}
			}
		}
	}()

	// Wait for either direction to fail
	err := <-errChan
	p.logger.Debug().Err(err).Msg("TCP proxy terminated")

	// Close the transport
	if closeErr := transport.Close(); closeErr != nil {
		p.logger.Debug().Err(closeErr).Msg("Error closing TCP transport")
	}

	// Send close signal to stream
	closeChunk := p.factory.NewChunk(p.sessionID, p.serverID, nil, false, true)
	stream.Send(closeChunk)

	return nil
}

package sol

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// UnifiedClient implements Client using a transport abstraction
type UnifiedClient struct {
	transport Transport
}

// NewUnifiedClient creates a new unified SOL client with the specified transport
func NewUnifiedClient(transport Transport) *UnifiedClient {
	return &UnifiedClient{
		transport: transport,
	}
}

// CreateSession creates a new unified SOL session
func (c *UnifiedClient) CreateSession(ctx context.Context, endpoint, username, password string, config *Config) (Session, error) {
	if config == nil {
		config = DefaultSOLConfig()
	}

	session := &UnifiedSession{
		transport:  c.transport,
		endpoint:   endpoint,
		username:   username,
		password:   password,
		config:     config,
		status:     SessionStatus{Active: false, Connected: false, Message: "created"},
		stopCh:     make(chan struct{}),
		readBuffer: make(chan []byte, 1024),
	}

	return session, nil
}

// SupportsSOL checks if the BMC supports SOL functionality via the transport
func (c *UnifiedClient) SupportsSOL(ctx context.Context, endpoint, username, password string) (bool, error) {
	return c.transport.SupportsSOL(ctx, endpoint, username, password)
}

// UnifiedSession implements Session using transport abstraction
type UnifiedSession struct {
	mu         sync.RWMutex
	transport  Transport
	endpoint   string
	username   string
	password   string
	config     *Config
	status     SessionStatus
	stopCh     chan struct{}
	readBuffer chan []byte
	closed     bool
}

// Read reads console output from the SOL session
func (s *UnifiedSession) Read(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("session is closed")
	}

	if !s.status.Active {
		if err := s.start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start SOL session: %w", err)
		}
	}

	select {
	case data := <-s.readBuffer:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(s.config.TimeoutSeconds) * time.Second):
		return nil, fmt.Errorf("read timeout")
	}
}

// Write sends console input to the SOL session
func (s *UnifiedSession) Write(ctx context.Context, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return fmt.Errorf("session is closed")
	}

	if !s.status.Active {
		return fmt.Errorf("session is not active")
	}

	return s.transport.Write(ctx, data)
}

// Close terminates the SOL session
func (s *UnifiedSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	close(s.stopCh)

	if err := s.transport.Close(); err != nil {
		return err
	}

	s.status = SessionStatus{Active: false, Connected: false, Message: "closed"}
	return nil
}

// Status returns the current session status
func (s *UnifiedSession) Status() SessionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// start initiates the SOL session using the transport
func (s *UnifiedSession) start(ctx context.Context) error {
	if s.status.Active {
		return nil
	}

	// Connect using the transport
	if err := s.transport.Connect(ctx, s.endpoint, s.username, s.password, s.config); err != nil {
		return fmt.Errorf("transport connection failed: %w", err)
	}

	// Start reading from transport in a goroutine
	go s.readFromTransport(ctx)

	s.status = SessionStatus{Active: true, Connected: true, Message: "SOL session active"}
	return nil
}

// readFromTransport reads data from transport and forwards to buffer
func (s *UnifiedSession) readFromTransport(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		if s.status.Active {
			s.status = SessionStatus{Active: false, Connected: false, Message: "transport disconnected"}
		}
		s.mu.Unlock()
	}()

	for {
		select {
		case <-s.stopCh:
			return
		default:
			data, err := s.transport.Read(ctx)
			if err != nil {
				if !s.closed {
					s.mu.Lock()
					s.status = SessionStatus{Active: false, Connected: false, Message: fmt.Sprintf("read error: %v", err)}
					s.mu.Unlock()
				}
				return
			}

			if len(data) > 0 {
				select {
				case s.readBuffer <- data:
				case <-s.stopCh:
					return
				default:
					// Buffer full, drop data
				}
			}
		}
	}
}

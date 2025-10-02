package sol

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
)

// IPMITransport implements Transport using FreeIPMI's ipmiconsole subprocess
type IPMITransport struct {
	mu      sync.RWMutex
	session *IPMISOLSession
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewIPMITransport creates a new IPMI SOL transport
func NewIPMITransport() *IPMITransport {
	return &IPMITransport{}
}

// Connect establishes an IPMI SOL connection using ipmiconsole subprocess
func (t *IPMITransport) Connect(ctx context.Context, endpoint, username, password string, config *Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.session != nil {
		return fmt.Errorf("transport already connected")
	}

	// Create context for session lifecycle
	sessionCtx, cancel := context.WithCancel(ctx)
	t.ctx = sessionCtx
	t.cancel = cancel

	// Determine replay buffer size (default 64KB for session replay)
	replayBufferSize := 65536

	// Create IPMI SOL session
	session, err := NewIPMISOLSession(sessionCtx, endpoint, username, password, replayBufferSize)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create IPMI SOL session: %w", err)
	}

	t.session = session

	log.Info().
		Str("endpoint", endpoint).
		Str("username", username).
		Msg("IPMI SOL transport connected")

	return nil
}

// Read reads console output from the IPMI SOL session
func (t *IPMITransport) Read(ctx context.Context) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.session == nil {
		return nil, fmt.Errorf("transport not connected")
	}

	return t.session.Read()
}

// Write sends console input to the IPMI SOL session
func (t *IPMITransport) Write(ctx context.Context, data []byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.session == nil {
		return fmt.Errorf("transport not connected")
	}

	return t.session.Write(data)
}

// Close terminates the IPMI SOL transport connection
func (t *IPMITransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.session == nil {
		return nil
	}

	err := t.session.Close()
	t.session = nil

	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}

	log.Info().Msg("IPMI SOL transport closed")

	return err
}

// Status returns the current transport status
func (t *IPMITransport) Status() TransportStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.session == nil {
		return TransportStatus{
			Connected: false,
			Protocol:  "ipmi_sol",
			Message:   "not connected",
		}
	}

	isRunning := t.session.IsRunning()

	return TransportStatus{
		Connected: isRunning,
		Protocol:  "ipmi_sol",
		Message:   fmt.Sprintf("running: %v", isRunning),
	}
}

// SupportsSOL checks if IPMI SOL is supported (checks for ipmiconsole binary)
func (t *IPMITransport) SupportsSOL(ctx context.Context, endpoint, username, password string) (bool, error) {
	// Check if ipmiconsole is available
	session, err := NewIPMISOLSession(ctx, endpoint, username, password, 0)
	if err != nil {
		return false, err
	}

	// Clean up immediately
	session.Close()

	return true, nil
}

// GetMetrics returns session metrics (IPMI-specific extension)
func (t *IPMITransport) GetMetrics() *SOLMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.session == nil {
		return nil
	}

	metrics := t.session.GetMetrics()
	return &metrics
}

// GetReplayBuffer returns the replay buffer contents (IPMI-specific extension)
func (t *IPMITransport) GetReplayBuffer() []byte {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.session == nil {
		return nil
	}

	return t.session.GetReplayBuffer()
}

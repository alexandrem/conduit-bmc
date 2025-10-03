package sol

import (
	"context"

	"core/types"
)

// Session represents an active Serial-over-LAN console session
type Session interface {
	// Read reads console output from the BMC
	Read(ctx context.Context) ([]byte, error)

	// Write sends console input to the BMC
	Write(ctx context.Context, data []byte) error

	// Close terminates the SOL session
	Close() error

	// Status returns the current session status
	Status() SessionStatus
}

// SessionStatus represents the status of a SOL session
type SessionStatus struct {
	Active    bool   `json:"active"`
	Connected bool   `json:"connected"`
	Message   string `json:"message"`
}

// Client provides methods for creating and managing SOL sessions
type Client interface {
	// CreateSession creates a new SOL session to the specified BMC
	CreateSession(ctx context.Context, endpoint, username, password string, config *Config) (Session, error)

	// SupportsSOL checks if the BMC supports SOL functionality
	SupportsSOL(ctx context.Context, endpoint, username, password string) (bool, error)
}

// Config contains configuration for SOL sessions
type Config struct {
	BaudRate       int    `json:"baud_rate"`       // Serial baud rate (default: 115200)
	FlowControl    string `json:"flow_control"`    // Flow control settings ("none", "hardware", "software")
	TimeoutSeconds int    `json:"timeout_seconds"` // Session timeout in seconds
}

// DefaultSOLConfig returns a default SOL configuration
func DefaultSOLConfig() *Config {
	return &Config{
		BaudRate:       115200,
		FlowControl:    "none",
		TimeoutSeconds: 300, // 5 minutes
	}
}

// Transport handles protocol-specific communication details
type Transport interface {
	// Connect establishes connection to BMC using protocol-specific method
	Connect(ctx context.Context, endpoint, username, password string, config *Config) error

	// Read reads console output from the BMC transport
	Read(ctx context.Context) ([]byte, error)

	// Write sends console input to the BMC transport
	Write(ctx context.Context, data []byte) error

	// Close terminates the transport connection
	Close() error

	// Status returns transport-specific status information
	Status() TransportStatus

	// SupportsSOL checks if the BMC supports SOL via this transport
	SupportsSOL(ctx context.Context, endpoint, username, password string) (bool, error)
}

// TransportStatus represents transport-specific status
type TransportStatus struct {
	Connected bool   `json:"connected"`
	Protocol  string `json:"protocol"`
	Message   string `json:"message"`
}

// Mock SOL type for development/testing
const (
	TypeMock types.SOLType = "mock_sol"
)

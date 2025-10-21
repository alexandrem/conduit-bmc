package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"core/types"
)

type BMCType string

const (
	BMCTypeIPMI    BMCType = "ipmi"
	BMCTypeRedfish BMCType = "redfish"
)

type Server struct {
	ID                string                   `json:"id" db:"id"`
	CustomerID        string                   `json:"customer_id" db:"customer_id"`
	DatacenterID      string                   `json:"datacenter_id" db:"datacenter_id"`
	ControlEndpoints  []*BMCControlEndpoint    `json:"control_endpoints" db:"control_endpoints"`
	PrimaryProtocol   BMCType                  `json:"primary_protocol" db:"primary_protocol"`
	SOLEndpoint       *SOLEndpoint             `json:"sol_endpoint" db:"sol_endpoint"`
	VNCEndpoint       *VNCEndpoint             `json:"vnc_endpoint" db:"vnc_endpoint"`
	Features          []string                 `json:"features" db:"features"`
	Status            string                   `json:"status" db:"status"`
	Metadata          map[string]string        `json:"metadata" db:"metadata"`
	DiscoveryMetadata *types.DiscoveryMetadata `json:"discovery_metadata,omitempty" db:"discovery_metadata"`
	CreatedAt         time.Time                `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at" db:"updated_at"`
}

// GetPrimaryControlEndpoint returns the control endpoint matching PrimaryProtocol.
// If PrimaryProtocol is set and found, returns that endpoint.
// Otherwise, falls back to the first endpoint in the array.
// Returns nil if no endpoints are available.
func (s *Server) GetPrimaryControlEndpoint() *BMCControlEndpoint {
	if len(s.ControlEndpoints) == 0 {
		return nil
	}

	// If PrimaryProtocol is set, try to find matching endpoint
	if s.PrimaryProtocol != "" {
		for _, endpoint := range s.ControlEndpoints {
			if endpoint.Type == s.PrimaryProtocol {
				return endpoint
			}
		}
	}

	// Fallback to first endpoint
	return s.ControlEndpoints[0]
}

// BMCControlEndpoint represents BMC control API configuration
type BMCControlEndpoint struct {
	Endpoint     string     `json:"endpoint"`
	Type         BMCType    `json:"type"`
	Username     string     `json:"username"`
	Password     string     `json:"password"`
	TLS          *TLSConfig `json:"tls"`
	Capabilities []string   `json:"capabilities"`
}

// SOLEndpoint represents Serial-over-LAN configuration
type SOLEndpoint struct {
	Type     SOLType    `json:"type"`
	Endpoint string     `json:"endpoint"`
	Username string     `json:"username"`
	Password string     `json:"password"`
	Config   *SOLConfig `json:"config"`
}

// VNCEndpoint represents VNC/KVM access configuration
type VNCEndpoint struct {
	Type     VNCType    `json:"type"`
	Endpoint string     `json:"endpoint"`
	Username string     `json:"username"`
	Password string     `json:"password"`
	Config   *VNCConfig `json:"config"`
}

// TLSConfig holds TLS-specific configuration
type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	CACert             string `json:"ca_cert"`
}

// SOLConfig holds SOL-specific configuration
type SOLConfig struct {
	BaudRate       int    `json:"baud_rate"`
	FlowControl    string `json:"flow_control"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// VNCConfig holds VNC-specific configuration
type VNCConfig struct {
	Protocol string `json:"protocol"`
	Path     string `json:"path"`
	Display  int    `json:"display"`
	ReadOnly bool   `json:"read_only"`
}

type SOLType string

const (
	SOLTypeIPMI          SOLType = "ipmi"
	SOLTypeRedfishSerial SOLType = "redfish_serial"
)

type VNCType string

const (
	VNCTypeNative    VNCType = "native"
	VNCTypeWebSocket VNCType = "websocket"
)

type Agent struct {
	ID           string    `json:"id" db:"id"`
	DatacenterID string    `json:"datacenter_id" db:"datacenter_id"`
	Endpoint     string    `json:"endpoint" db:"endpoint"`
	Status       string    `json:"status" db:"status"`
	LastSeen     time.Time `json:"last_seen" db:"last_seen"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type ProxySession struct {
	ID         string    `json:"id" db:"id"`
	CustomerID string    `json:"customer_id" db:"customer_id"`
	ServerID   string    `json:"server_id" db:"server_id"`
	AgentID    string    `json:"agent_id" db:"agent_id"`
	Status     string    `json:"status" db:"status"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	ExpiresAt  time.Time `json:"expires_at" db:"expires_at"`
}

type Customer struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	APIKey    string    `json:"api_key" db:"api_key"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type CreateProxyRequest struct {
	ServerID string `json:"server_id"`
}

type CreateProxyResponse struct {
	SessionID string `json:"session_id"`
	Endpoint  string `json:"endpoint"`
	ExpiresAt string `json:"expires_at"`
}

type ServerInfo struct {
	ID               string                `json:"id"`
	ControlEndpoints []*BMCControlEndpoint `json:"control_endpoints"`
	PrimaryProtocol  BMCType               `json:"primary_protocol"`
	SOLEndpoint      *SOLEndpoint          `json:"sol_endpoint"`
	VNCEndpoint      *VNCEndpoint          `json:"vnc_endpoint"`
	Features         []string              `json:"features"`
	Status           string                `json:"status"`
	DatacenterID     string                `json:"datacenter_id"`
	Metadata         map[string]string     `json:"metadata"`
}

type AuthClaims struct {
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	uuid.UUID  `json:"jti"`
}

// New models for the updated architecture

// RegionalGateway represents a regional gateway that aggregates multiple datacenters
type RegionalGateway struct {
	ID            string    `json:"id" db:"id"`
	Region        string    `json:"region" db:"region"`
	Endpoint      string    `json:"endpoint" db:"endpoint"`
	DatacenterIDs []string  `json:"datacenter_ids" db:"datacenter_ids"`
	Status        string    `json:"status" db:"status"`
	LastSeen      time.Time `json:"last_seen" db:"last_seen"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// ServerLocation maps servers to their regional gateways (for BMC Manager)
type ServerLocation struct {
	ServerID          string                `json:"server_id" db:"server_id"`
	CustomerID        string                `json:"customer_id" db:"customer_id"`
	DatacenterID      string                `json:"datacenter_id" db:"datacenter_id"`
	RegionalGatewayID string                `json:"regional_gateway_id" db:"regional_gateway_id"`
	ControlEndpoints  []*BMCControlEndpoint `json:"control_endpoints" db:"control_endpoints"`
	PrimaryProtocol   BMCType               `json:"primary_protocol" db:"primary_protocol"`
	Features          []string              `json:"features" db:"features"`
	CreatedAt         time.Time             `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at" db:"updated_at"`
}

// ServerCustomerMapping represents the mapping between servers and customers
type ServerCustomerMapping struct {
	ID         string    `json:"id" db:"id"`
	ServerID   string    `json:"server_id" db:"server_id"`
	CustomerID string    `json:"customer_id" db:"customer_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// DelegatedToken represents a delegated token for gateway access
type DelegatedToken struct {
	ID         string    `json:"id" db:"id"`
	CustomerID string    `json:"customer_id" db:"customer_id"`
	ServerID   string    `json:"server_id" db:"server_id"`
	Token      string    `json:"token" db:"token"`
	ExpiresAt  time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// AgentBMCMapping represents the in-memory BMC endpoint mapping in Regional Gateway
// This is not persisted to database (stateless design)
// Gateway only tracks BMC endpoints, not server concepts
type AgentBMCMapping struct {
	BMCEndpoint  string            // The BMC network endpoint (e.g., "192.168.1.100:623")
	AgentID      string            // Agent that provides access to this BMC
	DatacenterID string            // Datacenter containing this BMC
	BMCType      BMCType           // Type of BMC interface (IPMI/Redfish)
	Features     []string          // BMC capabilities
	Status       string            // BMC reachability status
	LastSeen     time.Time         // When this BMC was last verified
	Metadata     map[string]string // Optional metadata (rack location, hardware model, etc.)
}

// GenerateServerIDFromBMCEndpoint creates a server ID from datacenter ID and BMC endpoint.
// This is used by the manager to create synthetic server IDs for BMC endpoints reported by gateways.
//
// The format is: bmc-{datacenter_id}-{sanitized_endpoint}
// where sanitized_endpoint has colons (:) and dots (.) replaced with hyphens (-).
// Slashes (/) are NOT replaced to maintain URL structure visibility.
//
// Examples:
//   - "http://localhost:9001" in "dc-local-dev" -> "bmc-dc-local-dev-http-//localhost-9001"
//   - "http://redfish-01:8000" in "dc-east-1" -> "bmc-dc-east-1-http-//redfish-01-8000"
//   - "192.168.1.100:623" in "dc-west-2" -> "bmc-dc-west-2-192-168-1-100-623"
func GenerateServerIDFromBMCEndpoint(datacenterID, bmcEndpoint string) string {
	// Sanitize endpoint: replace : and . with -, but NOT /
	sanitizedEndpoint := strings.ReplaceAll(
		strings.ReplaceAll(bmcEndpoint, ":", "-"),
		".", "-")

	return fmt.Sprintf("bmc-%s-%s", datacenterID, sanitizedEndpoint)
}

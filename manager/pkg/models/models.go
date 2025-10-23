package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"core/types"
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
	IsAdmin   bool      `json:"is_admin" db:"is_admin"`
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
	ID               string                      `json:"id"`
	ControlEndpoints []*types.BMCControlEndpoint `json:"control_endpoints"`
	PrimaryProtocol  types.BMCType               `json:"primary_protocol"`
	SOLEndpoint      *types.SOLEndpoint          `json:"sol_endpoint"`
	VNCEndpoint      *types.VNCEndpoint          `json:"vnc_endpoint"`
	Features         []string                    `json:"features"`
	Status           string                      `json:"status"`
	DatacenterID     string                      `json:"datacenter_id"`
	Metadata         map[string]string           `json:"metadata"`
}

type AuthClaims struct {
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	IsAdmin    bool   `json:"is_admin"`
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
	ServerID          string                      `json:"server_id" db:"server_id"`
	CustomerID        string                      `json:"customer_id" db:"customer_id"`
	DatacenterID      string                      `json:"datacenter_id" db:"datacenter_id"`
	RegionalGatewayID string                      `json:"regional_gateway_id" db:"regional_gateway_id"`
	ControlEndpoints  []*types.BMCControlEndpoint `json:"control_endpoints" db:"control_endpoints"`
	PrimaryProtocol   types.BMCType               `json:"primary_protocol" db:"primary_protocol"`
	Features          []string                    `json:"features" db:"features"`
	CreatedAt         time.Time                   `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at" db:"updated_at"`
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
	BMCType      types.BMCType     // Type of BMC interface (IPMI/Redfish)
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

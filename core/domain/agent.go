package domain

import (
	"time"

	"core/types"
)

// Agent represents an agent in a datacenter
type Agent struct {
	ID           string    `json:"id" db:"id"`
	DatacenterID string    `json:"datacenter_id" db:"datacenter_id"`
	Endpoint     string    `json:"endpoint" db:"endpoint"`
	Status       string    `json:"status" db:"status"`
	LastSeen     time.Time `json:"last_seen" db:"last_seen"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// AgentBMCMapping represents the mapping between a BMC endpoint and its agent
type AgentBMCMapping struct {
	ServerID          string                   `json:"server_id"` // Logical server ID
	BMCEndpoint       string                   `json:"bmc_endpoint"`
	AgentID           string                   `json:"agent_id"`
	DatacenterID      string                   `json:"datacenter_id"`
	BMCType           types.BMCType            `json:"bmc_type"`
	Features          []string                 `json:"features"`
	Status            string                   `json:"status"`
	LastSeen          time.Time                `json:"last_seen"`
	Metadata          map[string]string        `json:"metadata"`
	Username          string                   `json:"username"`
	Capabilities      []string                 `json:"capabilities"`
	DiscoveryMetadata *types.DiscoveryMetadata `json:"discovery_metadata,omitempty"` // RFD 017
}

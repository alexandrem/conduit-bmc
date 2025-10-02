package domain

import (
	"time"

	"core/types"
)

// Server represents a physical or virtual server with BMC access
type Server struct {
	ID           string         `json:"id" db:"id"`
	CustomerID   string         `json:"customer_id" db:"customer_id"`
	DatacenterID string         `json:"datacenter_id" db:"datacenter_id"`
	BMCType      types.BMCType  `json:"bmc_type" db:"bmc_type"`
	BMCEndpoint  string         `json:"bmc_endpoint" db:"bmc_endpoint"`
	Username     string         `json:"username" db:"username"`
	Capabilities []string       `json:"capabilities" db:"capabilities"`
	Features     []string       `json:"features" db:"features"`
	Status       string         `json:"status" db:"status"`
	CreatedAt    time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at" db:"updated_at"`
}

// ServerLocation maps servers to their regional gateways (for BMC Manager)
type ServerLocation struct {
	ServerID          string        `json:"server_id" db:"server_id"`
	CustomerID        string        `json:"customer_id" db:"customer_id"`
	DatacenterID      string        `json:"datacenter_id" db:"datacenter_id"`
	RegionalGatewayID string        `json:"regional_gateway_id" db:"regional_gateway_id"`
	BMCType           types.BMCType `json:"bmc_type" db:"bmc_type"`
	Features          []string      `json:"features" db:"features"`
	CreatedAt         time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at" db:"updated_at"`
}

// ProxySession represents a proxy session
type ProxySession struct {
	ID         string    `json:"id" db:"id"`
	CustomerID string    `json:"customer_id" db:"customer_id"`
	ServerID   string    `json:"server_id" db:"server_id"`
	AgentID    string    `json:"agent_id" db:"agent_id"`
	Status     string    `json:"status" db:"status"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	ExpiresAt  time.Time `json:"expires_at" db:"expires_at"`
}

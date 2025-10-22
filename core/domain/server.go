package domain

import (
	"time"

	"core/types"
)

// Server represents a physical or virtual server with BMC access.
// This is the canonical server type used across all components.
type Server struct {
	ID                string                      `json:"id" db:"id"`
	CustomerID        string                      `json:"customer_id" db:"customer_id"`
	DatacenterID      string                      `json:"datacenter_id" db:"datacenter_id"`
	ControlEndpoints  []*types.BMCControlEndpoint `json:"control_endpoints" db:"control_endpoints"`
	PrimaryProtocol   types.BMCType               `json:"primary_protocol" db:"primary_protocol"`
	SOLEndpoint       *types.SOLEndpoint          `json:"sol_endpoint" db:"sol_endpoint"`
	VNCEndpoint       *types.VNCEndpoint          `json:"vnc_endpoint" db:"vnc_endpoint"`
	Features          []string                    `json:"features" db:"features"`
	Status            string                      `json:"status" db:"status"`
	Metadata          map[string]string           `json:"metadata" db:"metadata"`
	DiscoveryMetadata *types.DiscoveryMetadata    `json:"discovery_metadata,omitempty" db:"discovery_metadata"`
	CreatedAt         time.Time                   `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at" db:"updated_at"`
}

// GetPrimaryControlEndpoint returns the control endpoint matching PrimaryProtocol.
// If PrimaryProtocol is set and found, returns that endpoint.
// Otherwise, falls back to the first endpoint in the array.
// Returns nil if no endpoints are available.
func (s *Server) GetPrimaryControlEndpoint() *types.BMCControlEndpoint {
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

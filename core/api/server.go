package api

import "core/types"

// ServerInfo represents server information for API responses
type ServerInfo struct {
	ID           string        `json:"id"`
	BMCType      types.BMCType `json:"bmc_type"`
	Features     []string      `json:"features"`
	Status       string        `json:"status"`
	DatacenterID string        `json:"datacenter_id"`
}

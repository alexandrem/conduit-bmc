package auth

import "time"

// ServerContext contains the BMC endpoint information that can be encrypted in JWT tokens
type ServerContext struct {
	ServerID     string    `json:"server_id"`
	CustomerID   string    `json:"customer_id"`
	BMCEndpoint  string    `json:"bmc_endpoint"`
	BMCType      string    `json:"bmc_type"`
	Features     []string  `json:"features"`
	DatacenterID string    `json:"datacenter_id"`
	Permissions  []string  `json:"permissions"`
	IssuedAt     time.Time `json:"iat"`
	ExpiresAt    time.Time `json:"exp"`
}

// HasPermission checks if the server context has a specific permission
func (sc *ServerContext) HasPermission(permission string) bool {
	for _, p := range sc.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

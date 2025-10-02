package domain

import "time"

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

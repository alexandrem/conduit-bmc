package domain

import "time"

// Customer represents a customer/tenant in the system
type Customer struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	APIKey    string    `json:"api_key" db:"api_key"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
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

// ServerCustomerMapping represents the mapping between servers and customers
// This allows flexible assignment of servers to customers without modifying the server record
type ServerCustomerMapping struct {
	ID         string    `json:"id" db:"id"`
	ServerID   string    `json:"server_id" db:"server_id"`
	CustomerID string    `json:"customer_id" db:"customer_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

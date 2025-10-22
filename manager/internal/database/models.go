package database

import (
	"time"

	"github.com/uptrace/bun"

	"core/domain"
	"core/types"
	"manager/pkg/models"
)

// Customer represents a customer in the database using Bun ORM
type Customer struct {
	bun.BaseModel `bun:"table:customers"`

	ID        string    `bun:"id,pk"`
	Email     string    `bun:"email,unique,notnull"`
	APIKey    string    `bun:"api_key,unique,notnull"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`

	// Relations
	Servers []*Server `bun:"rel:has-many,join:id=customer_id"`
}

// ToModel converts database Customer to domain model
func (c *Customer) ToModel() *models.Customer {
	return &models.Customer{
		ID:        c.ID,
		Email:     c.Email,
		APIKey:    c.APIKey,
		CreatedAt: c.CreatedAt,
	}
}

// FromModel converts domain model to database Customer
func CustomerFromModel(m *models.Customer) *Customer {
	return &Customer{
		ID:        m.ID,
		Email:     m.Email,
		APIKey:    m.APIKey,
		CreatedAt: m.CreatedAt,
	}
}

// Server represents a server in the database using Bun ORM
type Server struct {
	bun.BaseModel `bun:"table:servers"`

	ID                string                      `bun:"id,pk"`
	CustomerID        string                      `bun:"customer_id,notnull"`
	DatacenterID      string                      `bun:"datacenter_id,notnull"`
	ControlEndpoints  []*types.BMCControlEndpoint `bun:"control_endpoints,type:json,notnull"`
	PrimaryProtocol   string                      `bun:"primary_protocol,notnull"`
	Features          []string                    `bun:"features,type:json,notnull"`
	Status            string                      `bun:"status,notnull,default:'active'"`
	SOLEndpoint       *types.SOLEndpoint          `bun:"sol_endpoint,type:json"`
	VNCEndpoint       *types.VNCEndpoint          `bun:"vnc_endpoint,type:json"`
	Metadata          map[string]string           `bun:"metadata,type:json"`
	DiscoveryMetadata *types.DiscoveryMetadata    `bun:"discovery_metadata,type:json"`
	CreatedAt         time.Time                   `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time                   `bun:"updated_at,nullzero,notnull,default:current_timestamp"`

	// Relations
	Customer *Customer `bun:"rel:belongs-to,join:customer_id=id"`
}

// ToModel converts database Server to domain model
func (s *Server) ToModel() *domain.Server {
	return &domain.Server{
		ID:                s.ID,
		CustomerID:        s.CustomerID,
		DatacenterID:      s.DatacenterID,
		ControlEndpoints:  s.ControlEndpoints,
		PrimaryProtocol:   types.BMCType(s.PrimaryProtocol),
		Features:          s.Features,
		Status:            s.Status,
		SOLEndpoint:       s.SOLEndpoint,
		VNCEndpoint:       s.VNCEndpoint,
		Metadata:          s.Metadata,
		DiscoveryMetadata: s.DiscoveryMetadata,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}

// FromModel converts domain model to database Server
func ServerFromModel(m *domain.Server) *Server {
	return &Server{
		ID:                m.ID,
		CustomerID:        m.CustomerID,
		DatacenterID:      m.DatacenterID,
		ControlEndpoints:  m.ControlEndpoints,
		PrimaryProtocol:   string(m.PrimaryProtocol),
		Features:          m.Features,
		Status:            m.Status,
		SOLEndpoint:       m.SOLEndpoint,
		VNCEndpoint:       m.VNCEndpoint,
		Metadata:          m.Metadata,
		DiscoveryMetadata: m.DiscoveryMetadata,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

// Agent represents an agent in the database using Bun ORM
type Agent struct {
	bun.BaseModel `bun:"table:agents"`

	ID           string    `bun:"id,pk"`
	DatacenterID string    `bun:"datacenter_id,notnull"`
	Endpoint     string    `bun:"endpoint,notnull"`
	Status       string    `bun:"status,notnull,default:'active'"`
	LastSeen     time.Time `bun:"last_seen,nullzero,default:current_timestamp"`
	CreatedAt    time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// ToModel converts database Agent to domain model
func (a *Agent) ToModel() *models.Agent {
	return &models.Agent{
		ID:           a.ID,
		DatacenterID: a.DatacenterID,
		Endpoint:     a.Endpoint,
		Status:       a.Status,
		LastSeen:     a.LastSeen,
		CreatedAt:    a.CreatedAt,
	}
}

// FromModel converts domain model to database Agent
func AgentFromModel(m *models.Agent) *Agent {
	return &Agent{
		ID:           m.ID,
		DatacenterID: m.DatacenterID,
		Endpoint:     m.Endpoint,
		Status:       m.Status,
		LastSeen:     m.LastSeen,
		CreatedAt:    m.CreatedAt,
	}
}

// RegionalGateway represents a regional gateway in the database using Bun ORM
type RegionalGateway struct {
	bun.BaseModel `bun:"table:regional_gateways"`

	ID            string    `bun:"id,pk"`
	Region        string    `bun:"region,notnull"`
	Endpoint      string    `bun:"endpoint,notnull"`
	DatacenterIDs []string  `bun:"datacenter_ids,type:json,notnull"`
	Status        string    `bun:"status,notnull,default:'active'"`
	LastSeen      time.Time `bun:"last_seen,nullzero,default:current_timestamp"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// ToModel converts database RegionalGateway to domain model
func (g *RegionalGateway) ToModel() *models.RegionalGateway {
	return &models.RegionalGateway{
		ID:            g.ID,
		Region:        g.Region,
		Endpoint:      g.Endpoint,
		DatacenterIDs: g.DatacenterIDs,
		Status:        g.Status,
		LastSeen:      g.LastSeen,
		CreatedAt:     g.CreatedAt,
	}
}

// FromModel converts domain model to database RegionalGateway
func RegionalGatewayFromModel(m *models.RegionalGateway) *RegionalGateway {
	return &RegionalGateway{
		ID:            m.ID,
		Region:        m.Region,
		Endpoint:      m.Endpoint,
		DatacenterIDs: m.DatacenterIDs,
		Status:        m.Status,
		LastSeen:      m.LastSeen,
		CreatedAt:     m.CreatedAt,
	}
}

// ServerLocation represents a server location in the database using Bun ORM
type ServerLocation struct {
	bun.BaseModel `bun:"table:server_locations"`

	ServerID          string                      `bun:"server_id,pk"`
	CustomerID        string                      `bun:"customer_id,notnull"`
	DatacenterID      string                      `bun:"datacenter_id,notnull"`
	RegionalGatewayID string                      `bun:"regional_gateway_id,notnull"`
	ControlEndpoints  []*types.BMCControlEndpoint `bun:"control_endpoints,type:json,notnull"`
	PrimaryProtocol   string                      `bun:"primary_protocol,notnull"`
	Features          []string                    `bun:"features,type:json,notnull"`
	CreatedAt         time.Time                   `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time                   `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// ToModel converts database ServerLocation to domain model
func (sl *ServerLocation) ToModel() *models.ServerLocation {
	return &models.ServerLocation{
		ServerID:          sl.ServerID,
		CustomerID:        sl.CustomerID,
		DatacenterID:      sl.DatacenterID,
		RegionalGatewayID: sl.RegionalGatewayID,
		ControlEndpoints:  sl.ControlEndpoints,
		PrimaryProtocol:   types.BMCType(sl.PrimaryProtocol),
		Features:          sl.Features,
		CreatedAt:         sl.CreatedAt,
		UpdatedAt:         sl.UpdatedAt,
	}
}

// FromModel converts domain model to database ServerLocation
func ServerLocationFromModel(m *models.ServerLocation) *ServerLocation {
	return &ServerLocation{
		ServerID:          m.ServerID,
		CustomerID:        m.CustomerID,
		DatacenterID:      m.DatacenterID,
		RegionalGatewayID: m.RegionalGatewayID,
		ControlEndpoints:  m.ControlEndpoints,
		PrimaryProtocol:   string(m.PrimaryProtocol),
		Features:          m.Features,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

// ProxySession represents a proxy session in the database using Bun ORM
type ProxySession struct {
	bun.BaseModel `bun:"table:proxy_sessions"`

	ID         string    `bun:"id,pk"`
	CustomerID string    `bun:"customer_id,notnull"`
	ServerID   string    `bun:"server_id,notnull"`
	AgentID    string    `bun:"agent_id,notnull"`
	Status     string    `bun:"status,notnull,default:'active'"`
	CreatedAt  time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	ExpiresAt  time.Time `bun:"expires_at,notnull"`

	// Relations
	Customer *Customer `bun:"rel:belongs-to,join:customer_id=id"`
	Server   *Server   `bun:"rel:belongs-to,join:server_id=id"`
	Agent    *Agent    `bun:"rel:belongs-to,join:agent_id=id"`
}

// ToModel converts database ProxySession to domain model
func (ps *ProxySession) ToModel() *models.ProxySession {
	return &models.ProxySession{
		ID:         ps.ID,
		CustomerID: ps.CustomerID,
		ServerID:   ps.ServerID,
		AgentID:    ps.AgentID,
		Status:     ps.Status,
		CreatedAt:  ps.CreatedAt,
		ExpiresAt:  ps.ExpiresAt,
	}
}

// FromModel converts domain model to database ProxySession
func ProxySessionFromModel(m *models.ProxySession) *ProxySession {
	return &ProxySession{
		ID:         m.ID,
		CustomerID: m.CustomerID,
		ServerID:   m.ServerID,
		AgentID:    m.AgentID,
		Status:     m.Status,
		CreatedAt:  m.CreatedAt,
		ExpiresAt:  m.ExpiresAt,
	}
}

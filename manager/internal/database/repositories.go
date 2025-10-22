package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"core/domain"
	managermodels "manager/pkg/models"
)

// ServerRepository provides database operations for servers
type ServerRepository interface {
	Get(ctx context.Context, id string) (*domain.Server, error)
	List(ctx context.Context, customerID string) ([]*domain.Server, error)
	ListAll(ctx context.Context) ([]*domain.Server, error)
	Create(ctx context.Context, server *domain.Server) error
	Update(ctx context.Context, server *domain.Server) error
	Delete(ctx context.Context, id string) error
}

type serverRepository struct {
	db *bun.DB
}

// NewServerRepository creates a new server repository
func NewServerRepository(db *bun.DB) ServerRepository {
	return &serverRepository{db: db}
}

func (r *serverRepository) Get(ctx context.Context, id string) (*domain.Server, error) {
	server := new(Server)
	err := r.db.NewSelect().
		Model(server).
		Where("id = ?", id).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("server not found")
	}
	if err != nil {
		return nil, err
	}

	return server.ToModel(), nil
}

func (r *serverRepository) List(ctx context.Context, customerID string) ([]*domain.Server, error) {
	var servers []*Server
	err := r.db.NewSelect().
		Model(&servers).
		Where("customer_id = ?", customerID).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	result := make([]*domain.Server, len(servers))
	for i, s := range servers {
		result[i] = s.ToModel()
	}
	return result, nil
}

func (r *serverRepository) ListAll(ctx context.Context) ([]*domain.Server, error) {
	var servers []*Server
	err := r.db.NewSelect().
		Model(&servers).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	result := make([]*domain.Server, len(servers))
	for i, s := range servers {
		result[i] = s.ToModel()
	}
	return result, nil
}

func (r *serverRepository) Create(ctx context.Context, server *domain.Server) error {
	dbServer := ServerFromModel(server)
	_, err := r.db.NewInsert().
		Model(dbServer).
		Exec(ctx)
	return err
}

func (r *serverRepository) Update(ctx context.Context, server *domain.Server) error {
	dbServer := ServerFromModel(server)
	_, err := r.db.NewUpdate().
		Model(dbServer).
		WherePK().
		Exec(ctx)
	return err
}

func (r *serverRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.NewDelete().
		Model((*Server)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

// CustomerRepository provides database operations for customers
type CustomerRepository interface {
	Get(ctx context.Context, id string) (*managermodels.Customer, error)
	GetByEmail(ctx context.Context, email string) (*managermodels.Customer, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*managermodels.Customer, error)
	Create(ctx context.Context, customer *managermodels.Customer) error
	Update(ctx context.Context, customer *managermodels.Customer) error
	Delete(ctx context.Context, id string) error
}

type customerRepository struct {
	db *bun.DB
}

// NewCustomerRepository creates a new customer repository
func NewCustomerRepository(db *bun.DB) CustomerRepository {
	return &customerRepository{db: db}
}

func (r *customerRepository) Get(ctx context.Context, id string) (*managermodels.Customer, error) {
	customer := new(Customer)
	err := r.db.NewSelect().
		Model(customer).
		Where("id = ?", id).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("customer not found")
	}
	if err != nil {
		return nil, err
	}

	return customer.ToModel(), nil
}

func (r *customerRepository) GetByEmail(ctx context.Context, email string) (*managermodels.Customer, error) {
	customer := new(Customer)
	err := r.db.NewSelect().
		Model(customer).
		Where("email = ?", email).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("customer not found")
	}
	if err != nil {
		return nil, err
	}

	return customer.ToModel(), nil
}

func (r *customerRepository) GetByAPIKey(ctx context.Context, apiKey string) (*managermodels.Customer, error) {
	customer := new(Customer)
	err := r.db.NewSelect().
		Model(customer).
		Where("api_key = ?", apiKey).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("customer not found")
	}
	if err != nil {
		return nil, err
	}

	return customer.ToModel(), nil
}

func (r *customerRepository) Create(ctx context.Context, customer *managermodels.Customer) error {
	dbCustomer := CustomerFromModel(customer)
	_, err := r.db.NewInsert().
		Model(dbCustomer).
		Exec(ctx)
	return err
}

func (r *customerRepository) Update(ctx context.Context, customer *managermodels.Customer) error {
	dbCustomer := CustomerFromModel(customer)
	_, err := r.db.NewUpdate().
		Model(dbCustomer).
		WherePK().
		Exec(ctx)
	return err
}

func (r *customerRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.NewDelete().
		Model((*Customer)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

// AgentRepository provides database operations for agents
type AgentRepository interface {
	Get(ctx context.Context, id string) (*managermodels.Agent, error)
	GetByDatacenter(ctx context.Context, datacenterID string) (*managermodels.Agent, error)
	List(ctx context.Context) ([]*managermodels.Agent, error)
	Create(ctx context.Context, agent *managermodels.Agent) error
	Update(ctx context.Context, agent *managermodels.Agent) error
	Delete(ctx context.Context, id string) error
}

type agentRepository struct {
	db *bun.DB
}

// NewAgentRepository creates a new agent repository
func NewAgentRepository(db *bun.DB) AgentRepository {
	return &agentRepository{db: db}
}

func (r *agentRepository) Get(ctx context.Context, id string) (*managermodels.Agent, error) {
	agent := new(Agent)
	err := r.db.NewSelect().
		Model(agent).
		Where("id = ?", id).
		Scan(ctx)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, err
	}

	return agent.ToModel(), nil
}

func (r *agentRepository) GetByDatacenter(ctx context.Context, datacenterID string) (*managermodels.Agent, error) {
	agent := new(Agent)
	err := r.db.NewSelect().
		Model(agent).
		Where("datacenter_id = ?", datacenterID).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, err
	}

	return agent.ToModel(), nil
}

func (r *agentRepository) List(ctx context.Context) ([]*managermodels.Agent, error) {
	var agents []*Agent
	err := r.db.NewSelect().
		Model(&agents).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	result := make([]*managermodels.Agent, len(agents))
	for i, a := range agents {
		result[i] = a.ToModel()
	}
	return result, nil
}

func (r *agentRepository) Create(ctx context.Context, agent *managermodels.Agent) error {
	dbAgent := AgentFromModel(agent)
	_, err := r.db.NewInsert().
		Model(dbAgent).
		Exec(ctx)
	return err
}

func (r *agentRepository) Update(ctx context.Context, agent *managermodels.Agent) error {
	dbAgent := AgentFromModel(agent)
	_, err := r.db.NewUpdate().
		Model(dbAgent).
		WherePK().
		Exec(ctx)
	return err
}

func (r *agentRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.NewDelete().
		Model((*Agent)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

// GatewayRepository provides database operations for regional gateways
type GatewayRepository interface {
	Get(ctx context.Context, id string) (*managermodels.RegionalGateway, error)
	List(ctx context.Context) ([]*managermodels.RegionalGateway, error)
	Create(ctx context.Context, gateway *managermodels.RegionalGateway) error
	Update(ctx context.Context, gateway *managermodels.RegionalGateway) error
	Upsert(ctx context.Context, gateway *managermodels.RegionalGateway) error
	Delete(ctx context.Context, id string) error
}

type gatewayRepository struct {
	db *bun.DB
}

// NewGatewayRepository creates a new gateway repository
func NewGatewayRepository(db *bun.DB) GatewayRepository {
	return &gatewayRepository{db: db}
}

func (r *gatewayRepository) Get(ctx context.Context, id string) (*managermodels.RegionalGateway, error) {
	gateway := new(RegionalGateway)
	err := r.db.NewSelect().
		Model(gateway).
		Where("id = ?", id).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("gateway not found")
	}
	if err != nil {
		return nil, err
	}

	return gateway.ToModel(), nil
}

func (r *gatewayRepository) List(ctx context.Context) ([]*managermodels.RegionalGateway, error) {
	var gateways []*RegionalGateway
	err := r.db.NewSelect().
		Model(&gateways).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	result := make([]*managermodels.RegionalGateway, len(gateways))
	for i, g := range gateways {
		result[i] = g.ToModel()
	}
	return result, nil
}

func (r *gatewayRepository) Create(ctx context.Context, gateway *managermodels.RegionalGateway) error {
	dbGateway := RegionalGatewayFromModel(gateway)
	_, err := r.db.NewInsert().
		Model(dbGateway).
		Exec(ctx)
	return err
}

func (r *gatewayRepository) Update(ctx context.Context, gateway *managermodels.RegionalGateway) error {
	dbGateway := RegionalGatewayFromModel(gateway)
	_, err := r.db.NewUpdate().
		Model(dbGateway).
		WherePK().
		Exec(ctx)
	return err
}

func (r *gatewayRepository) Upsert(ctx context.Context, gateway *managermodels.RegionalGateway) error {
	dbGateway := RegionalGatewayFromModel(gateway)
	_, err := r.db.NewInsert().
		Model(dbGateway).
		On("CONFLICT (id) DO UPDATE").
		Set("region = EXCLUDED.region").
		Set("endpoint = EXCLUDED.endpoint").
		Set("datacenter_ids = EXCLUDED.datacenter_ids").
		Set("status = EXCLUDED.status").
		Set("last_seen = EXCLUDED.last_seen").
		Exec(ctx)
	return err
}

func (r *gatewayRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.NewDelete().
		Model((*RegionalGateway)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

// ServerLocationRepository provides database operations for server locations
type ServerLocationRepository interface {
	Get(ctx context.Context, serverID string) (*managermodels.ServerLocation, error)
	List(ctx context.Context) ([]*managermodels.ServerLocation, error)
	Create(ctx context.Context, location *managermodels.ServerLocation) error
	Update(ctx context.Context, location *managermodels.ServerLocation) error
	Upsert(ctx context.Context, location *managermodels.ServerLocation) error
	Delete(ctx context.Context, serverID string) error
}

type serverLocationRepository struct {
	db *bun.DB
}

// NewServerLocationRepository creates a new server location repository
func NewServerLocationRepository(db *bun.DB) ServerLocationRepository {
	return &serverLocationRepository{db: db}
}

func (r *serverLocationRepository) Get(ctx context.Context, serverID string) (*managermodels.ServerLocation, error) {
	location := new(ServerLocation)
	err := r.db.NewSelect().
		Model(location).
		Where("server_id = ?", serverID).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("server location not found")
	}
	if err != nil {
		return nil, err
	}

	return location.ToModel(), nil
}

func (r *serverLocationRepository) List(ctx context.Context) ([]*managermodels.ServerLocation, error) {
	var locations []*ServerLocation
	err := r.db.NewSelect().
		Model(&locations).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	result := make([]*managermodels.ServerLocation, len(locations))
	for i, l := range locations {
		result[i] = l.ToModel()
	}
	return result, nil
}

func (r *serverLocationRepository) Create(ctx context.Context, location *managermodels.ServerLocation) error {
	dbLocation := ServerLocationFromModel(location)
	_, err := r.db.NewInsert().
		Model(dbLocation).
		Exec(ctx)
	return err
}

func (r *serverLocationRepository) Update(ctx context.Context, location *managermodels.ServerLocation) error {
	dbLocation := ServerLocationFromModel(location)
	_, err := r.db.NewUpdate().
		Model(dbLocation).
		WherePK().
		Exec(ctx)
	return err
}

func (r *serverLocationRepository) Upsert(ctx context.Context, location *managermodels.ServerLocation) error {
	dbLocation := ServerLocationFromModel(location)
	_, err := r.db.NewInsert().
		Model(dbLocation).
		On("CONFLICT (server_id) DO UPDATE").
		Exec(ctx)
	return err
}

func (r *serverLocationRepository) Delete(ctx context.Context, serverID string) error {
	_, err := r.db.NewDelete().
		Model((*ServerLocation)(nil)).
		Where("server_id = ?", serverID).
		Exec(ctx)
	return err
}

// ProxySessionRepository provides database operations for proxy sessions
type ProxySessionRepository interface {
	Get(ctx context.Context, id string) (*managermodels.ProxySession, error)
	ListByCustomer(ctx context.Context, customerID string) ([]*managermodels.ProxySession, error)
	ListActive(ctx context.Context) ([]*managermodels.ProxySession, error)
	Create(ctx context.Context, session *managermodels.ProxySession) error
	Update(ctx context.Context, session *managermodels.ProxySession) error
	Delete(ctx context.Context, id string) error
}

type proxySessionRepository struct {
	db *bun.DB
}

// NewProxySessionRepository creates a new proxy session repository
func NewProxySessionRepository(db *bun.DB) ProxySessionRepository {
	return &proxySessionRepository{db: db}
}

func (r *proxySessionRepository) Get(ctx context.Context, id string) (*managermodels.ProxySession, error) {
	session := new(ProxySession)
	err := r.db.NewSelect().
		Model(session).
		Where("id = ?", id).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("proxy session not found")
	}
	if err != nil {
		return nil, err
	}

	return session.ToModel(), nil
}

func (r *proxySessionRepository) ListByCustomer(ctx context.Context, customerID string) ([]*managermodels.ProxySession, error) {
	var sessions []*ProxySession
	err := r.db.NewSelect().
		Model(&sessions).
		Where("customer_id = ?", customerID).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	result := make([]*managermodels.ProxySession, len(sessions))
	for i, s := range sessions {
		result[i] = s.ToModel()
	}
	return result, nil
}

func (r *proxySessionRepository) ListActive(ctx context.Context) ([]*managermodels.ProxySession, error) {
	var sessions []*ProxySession
	err := r.db.NewSelect().
		Model(&sessions).
		Where("status = ?", "active").
		Where("expires_at > datetime('now')").
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	result := make([]*managermodels.ProxySession, len(sessions))
	for i, s := range sessions {
		result[i] = s.ToModel()
	}
	return result, nil
}

func (r *proxySessionRepository) Create(ctx context.Context, session *managermodels.ProxySession) error {
	dbSession := ProxySessionFromModel(session)
	_, err := r.db.NewInsert().
		Model(dbSession).
		Exec(ctx)
	return err
}

func (r *proxySessionRepository) Update(ctx context.Context, session *managermodels.ProxySession) error {
	dbSession := ProxySessionFromModel(session)
	_, err := r.db.NewUpdate().
		Model(dbSession).
		WherePK().
		Exec(ctx)
	return err
}

func (r *proxySessionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.NewDelete().
		Model((*ProxySession)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

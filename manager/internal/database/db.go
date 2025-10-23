package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"
)

// BunDB wraps bun.DB and provides repository access
type BunDB struct {
	db *bun.DB

	// Repositories
	Servers   ServerRepository
	Customers CustomerRepository
	Agents    AgentRepository
	Gateways  GatewayRepository
	Locations ServerLocationRepository
	Sessions  ProxySessionRepository
	Admin     AdminRepository
}

// Option is a functional option for configuring the database
type Option func(*BunDB)

// WithDebug enables query logging for debugging
func WithDebug(enabled bool) Option {
	return func(db *BunDB) {
		if enabled {
			db.db.AddQueryHook(bundebug.NewQueryHook(
				bundebug.WithVerbose(true),
			))
			log.Info().Msg("Bun query logging enabled")
		}
	}
}

// New creates a new Bun-based database connection
func New(dbPath string, opts ...Option) (*BunDB, error) {
	// Open SQLite connection using sqliteshim (compatible with modernc.org/sqlite)
	sqldb, err := sql.Open(sqliteshim.ShimName, dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create Bun DB with SQLite dialect
	db := bun.NewDB(sqldb, sqlitedialect.New())

	bunDB := &BunDB{
		db: db,
	}

	// Apply options
	for _, opt := range opts {
		opt(bunDB)
	}

	// Initialize repositories
	bunDB.Servers = NewServerRepository(db)
	bunDB.Customers = NewCustomerRepository(db)
	bunDB.Agents = NewAgentRepository(db)
	bunDB.Gateways = NewGatewayRepository(db)
	bunDB.Locations = NewServerLocationRepository(db)
	bunDB.Sessions = NewProxySessionRepository(db)
	bunDB.Admin = NewAdminRepository(db)

	// Run migrations
	if err := bunDB.Migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Info().Str("path", dbPath).Msg("Bun database initialized successfully")
	return bunDB, nil
}

// Close closes the database connection
func (db *BunDB) Close() error {
	return db.db.Close()
}

// DB returns the underlying bun.DB instance for advanced operations
func (db *BunDB) DB() *bun.DB {
	return db.db
}

// Migrate runs database migrations
func (db *BunDB) Migrate(ctx context.Context) error {
	log.Info().Msg("Running database migrations")

	// Create tables if they don't exist
	models := []interface{}{
		(*Customer)(nil),
		(*Agent)(nil),
		(*Server)(nil),
		(*ProxySession)(nil),
		(*RegionalGateway)(nil),
		(*ServerLocation)(nil),
	}

	for _, model := range models {
		if _, err := db.db.NewCreateTable().
			Model(model).
			IfNotExists().
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Create indexes for foreign keys and common queries
	indexes := []string{
		// Server indexes
		"CREATE INDEX IF NOT EXISTS idx_servers_customer_id ON servers(customer_id)",
		"CREATE INDEX IF NOT EXISTS idx_servers_datacenter_id ON servers(datacenter_id)",
		"CREATE INDEX IF NOT EXISTS idx_servers_status ON servers(status)",

		// ServerLocation indexes
		"CREATE INDEX IF NOT EXISTS idx_server_locations_gateway_id ON server_locations(regional_gateway_id)",
		"CREATE INDEX IF NOT EXISTS idx_server_locations_datacenter_id ON server_locations(datacenter_id)",
		"CREATE INDEX IF NOT EXISTS idx_server_locations_customer_id ON server_locations(customer_id)",

		// ProxySession indexes
		"CREATE INDEX IF NOT EXISTS idx_proxy_sessions_customer_id ON proxy_sessions(customer_id)",
		"CREATE INDEX IF NOT EXISTS idx_proxy_sessions_server_id ON proxy_sessions(server_id)",
		"CREATE INDEX IF NOT EXISTS idx_proxy_sessions_status ON proxy_sessions(status)",
		"CREATE INDEX IF NOT EXISTS idx_proxy_sessions_expires_at ON proxy_sessions(expires_at)",

		// Agent indexes
		"CREATE INDEX IF NOT EXISTS idx_agents_datacenter_id ON agents(datacenter_id)",
		"CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)",

		// Customer indexes
		"CREATE INDEX IF NOT EXISTS idx_customers_email ON customers(email)",
		"CREATE INDEX IF NOT EXISTS idx_customers_api_key ON customers(api_key)",
		"CREATE INDEX IF NOT EXISTS idx_customers_is_admin ON customers(is_admin) WHERE is_admin = true",

		// Gateway indexes
		"CREATE INDEX IF NOT EXISTS idx_regional_gateways_region ON regional_gateways(region)",
		"CREATE INDEX IF NOT EXISTS idx_regional_gateways_status ON regional_gateways(status)",
	}

	for _, idx := range indexes {
		if _, err := db.db.ExecContext(ctx, idx); err != nil {
			log.Warn().Err(err).Str("index", idx).Msg("Failed to create index (may already exist)")
			// Don't fail on index errors - they might already exist
		}
	}

	log.Info().Msg("Database migrations completed successfully")
	return nil
}

// BeginTx starts a new transaction
func (db *BunDB) BeginTx(ctx context.Context) (bun.Tx, error) {
	return db.db.BeginTx(ctx, nil)
}

// Clean removes all data from all tables (useful for development/testing)
// WARNING: This will delete ALL data in the database!
func (db *BunDB) Clean(ctx context.Context) error {
	log.Warn().Msg("Cleaning all data from database")

	// Delete in order to respect foreign key constraints
	tables := []string{
		"proxy_sessions",
		"server_locations",
		"servers",
		"agents",
		"regional_gateways",
		"customers",
	}

	for _, table := range tables {
		_, err := db.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			log.Error().Err(err).Str("table", table).Msg("Failed to clean table")
			// Continue with other tables even if one fails
		} else {
			log.Debug().Str("table", table).Msg("Cleaned table")
		}
	}

	log.Info().Msg("Database cleaned successfully")
	return nil
}

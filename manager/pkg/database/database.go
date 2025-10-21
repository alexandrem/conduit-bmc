package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"core/types"
	"manager/pkg/models"
	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS customers (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		api_key TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY,
		datacenter_id TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS servers (
		id TEXT PRIMARY KEY,
		customer_id TEXT NOT NULL,
		datacenter_id TEXT NOT NULL,
		bmc_type TEXT NOT NULL,
		bmc_endpoint TEXT NOT NULL,
		username TEXT,
		capabilities TEXT,
		features TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		sol_endpoint TEXT,
		vnc_endpoint TEXT,
		control_endpoint TEXT,
		metadata TEXT,
		discovery_metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (customer_id) REFERENCES customers (id),
		FOREIGN KEY (datacenter_id) REFERENCES agents (datacenter_id)
	);

	CREATE TABLE IF NOT EXISTS proxy_sessions (
		id TEXT PRIMARY KEY,
		customer_id TEXT NOT NULL,
		server_id TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY (customer_id) REFERENCES customers (id),
		FOREIGN KEY (server_id) REFERENCES servers (id),
		FOREIGN KEY (agent_id) REFERENCES agents (id)
	);

	-- New tables for updated architecture
	CREATE TABLE IF NOT EXISTS regional_gateways (
		id TEXT PRIMARY KEY,
		region TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		datacenter_ids TEXT NOT NULL, -- JSON array of datacenter IDs
		status TEXT NOT NULL DEFAULT 'active',
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS server_locations (
		server_id TEXT PRIMARY KEY,
		customer_id TEXT NOT NULL,
		datacenter_id TEXT NOT NULL,
		regional_gateway_id TEXT NOT NULL,
		bmc_type TEXT NOT NULL,
		features TEXT NOT NULL, -- JSON array of features
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (customer_id) REFERENCES customers (id),
		FOREIGN KEY (regional_gateway_id) REFERENCES regional_gateways (id)
	);

	CREATE TABLE IF NOT EXISTS delegated_tokens (
		id TEXT PRIMARY KEY,
		customer_id TEXT NOT NULL,
		server_id TEXT,
		token TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (customer_id) REFERENCES customers (id)
	);

	CREATE TABLE IF NOT EXISTS server_customer_mappings (
		id TEXT PRIMARY KEY,
		server_id TEXT NOT NULL,
		customer_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(server_id, customer_id),
		FOREIGN KEY (server_id) REFERENCES servers (id),
		FOREIGN KEY (customer_id) REFERENCES customers (id)
	);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Add missing columns to existing servers table
	alterStatements := []string{
		"ALTER TABLE servers ADD COLUMN username TEXT",
		"ALTER TABLE servers ADD COLUMN capabilities TEXT",
	}

	for _, stmt := range alterStatements {
		_, err := db.conn.Exec(stmt)
		// Ignore errors if columns already exist
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}

	return nil
}

func (db *DB) GetCustomerByAPIKey(apiKey string) (*models.Customer, error) {
	var customer models.Customer
	err := db.conn.QueryRow(
		"SELECT id, email, api_key, created_at FROM customers WHERE api_key = ?",
		apiKey,
	).Scan(&customer.ID, &customer.Email, &customer.APIKey, &customer.CreatedAt)

	if err != nil {
		return nil, err
	}
	return &customer, nil
}

// Temporary struct for old database schema compatibility
type legacyServerRow struct {
	ID           string    `db:"id"`
	CustomerID   string    `db:"customer_id"`
	DatacenterID string    `db:"datacenter_id"`
	BMCType      string    `db:"bmc_type"`
	BMCEndpoint  string    `db:"bmc_endpoint"`
	Username     string    `db:"username"`
	Capabilities string    `db:"capabilities"`
	Features     string    `db:"features"`
	Status       string    `db:"status"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

func (db *DB) GetServerByID(serverID string) (*models.Server, error) {
	var row legacyServerRow
	var solEndpointJSON, vncEndpointJSON, controlEndpointJSON, metadataJSON, discoveryMetadataJSON sql.NullString

	err := db.conn.QueryRow(
		"SELECT id, customer_id, datacenter_id, bmc_type, bmc_endpoint, username, capabilities, features, status, sol_endpoint, vnc_endpoint, control_endpoint, metadata, discovery_metadata, created_at, updated_at FROM servers WHERE id = ?",
		serverID,
	).Scan(&row.ID, &row.CustomerID, &row.DatacenterID, &row.BMCType, &row.BMCEndpoint, &row.Username, &row.Capabilities, &row.Features, &row.Status, &solEndpointJSON, &vncEndpointJSON, &controlEndpointJSON, &metadataJSON, &discoveryMetadataJSON, &row.CreatedAt, &row.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrServerNotFound
		}
		return nil, err
	}

	// Convert legacy format to new Server structure
	server := &models.Server{
		ID:           row.ID,
		CustomerID:   row.CustomerID,
		DatacenterID: row.DatacenterID,
		Status:       row.Status,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
		Features:     []string{}, // TODO: Parse features JSON
		Metadata:     make(map[string]string),
	}

	// Deserialize SOL endpoint if present
	if solEndpointJSON.Valid && solEndpointJSON.String != "" {
		server.SOLEndpoint = &types.SOLEndpoint{}
		if err := json.Unmarshal([]byte(solEndpointJSON.String), server.SOLEndpoint); err != nil {
			log.Warn().Err(err).Str("server_id", serverID).Msg("Failed to unmarshal SOL endpoint")
		}
	}

	// Deserialize VNC endpoint if present
	if vncEndpointJSON.Valid && vncEndpointJSON.String != "" {
		server.VNCEndpoint = &types.VNCEndpoint{}
		if err := json.Unmarshal([]byte(vncEndpointJSON.String), server.VNCEndpoint); err != nil {
			log.Warn().Err(err).Str("server_id", serverID).Msg("Failed to unmarshal VNC endpoint")
		}
	}

	// Deserialize control endpoints if present (prefer JSON over legacy fields)
	if controlEndpointJSON.Valid && controlEndpointJSON.String != "" {
		server.ControlEndpoints = []*types.BMCControlEndpoint{}
		if err := json.Unmarshal([]byte(controlEndpointJSON.String), &server.ControlEndpoints); err != nil {
			log.Warn().Err(err).Str("server_id", serverID).Msg("Failed to unmarshal control endpoints")
			server.ControlEndpoints = nil // Fall back to legacy parsing below
		}
	}

	// Convert legacy BMC fields to control endpoint (fallback if JSON not available)
	if len(server.ControlEndpoints) == 0 && row.BMCEndpoint != "" {
		bmcType := types.BMCTypeIPMI // Default
		if row.BMCType == "redfish" {
			bmcType = types.BMCTypeRedfish
		}

		// Parse capabilities from string
		capabilities := []string{}
		if row.Capabilities != "" {
			capabilities = strings.Split(row.Capabilities, ",")
		}

		server.ControlEndpoints = []*types.BMCControlEndpoint{{
			Endpoint:     row.BMCEndpoint,
			Type:         bmcType,
			Username:     row.Username,
			Password:     "", // Password not stored per security requirement
			Capabilities: capabilities,
		}}
		server.PrimaryProtocol = bmcType
	}

	// Deserialize metadata if present
	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &server.Metadata); err != nil {
			log.Warn().Err(err).Str("server_id", serverID).Msg("Failed to unmarshal metadata")
		}
	}

	// Deserialize discovery metadata if present
	if discoveryMetadataJSON.Valid && discoveryMetadataJSON.String != "" {
		server.DiscoveryMetadata = &types.DiscoveryMetadata{}
		if err := json.Unmarshal([]byte(discoveryMetadataJSON.String), server.DiscoveryMetadata); err != nil {
			log.Warn().Err(err).Str("server_id", serverID).Msg("Failed to unmarshal discovery metadata")
			server.DiscoveryMetadata = nil
		}
	}

	// Parse features if needed
	if row.Features != "" {
		// TODO: Implement proper JSON parsing
		server.Features = []string{row.Features} // Simplified for now
	}

	return server, nil
}

func (db *DB) GetAgentByDatacenter(datacenterID string) (*models.Agent, error) {
	var agent models.Agent
	err := db.conn.QueryRow(
		"SELECT id, datacenter_id, endpoint, status, last_seen, created_at FROM agents WHERE datacenter_id = ? AND status = 'active'",
		datacenterID,
	).Scan(&agent.ID, &agent.DatacenterID, &agent.Endpoint, &agent.Status, &agent.LastSeen, &agent.CreatedAt)

	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (db *DB) CreateProxySession(session *models.ProxySession) error {
	_, err := db.conn.Exec(
		"INSERT INTO proxy_sessions (id, customer_id, server_id, agent_id, status, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		session.ID, session.CustomerID, session.ServerID, session.AgentID, session.Status, session.CreatedAt, session.ExpiresAt,
	)
	return err
}

func (db *DB) GetProxySession(sessionID string) (*models.ProxySession, error) {
	var session models.ProxySession
	err := db.conn.QueryRow(
		"SELECT id, customer_id, server_id, agent_id, status, created_at, expires_at FROM proxy_sessions WHERE id = ?",
		sessionID,
	).Scan(&session.ID, &session.CustomerID, &session.ServerID, &session.AgentID, &session.Status, &session.CreatedAt, &session.ExpiresAt)

	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (db *DB) UpdateProxySessionStatus(sessionID, status string) error {
	_, err := db.conn.Exec(
		"UPDATE proxy_sessions SET status = ? WHERE id = ?",
		status, sessionID,
	)
	return err
}

func (db *DB) GetServersByCustomer(customerID string) ([]models.Server, error) {
	rows, err := db.conn.Query(
		"SELECT id, customer_id, datacenter_id, bmc_type, bmc_endpoint, username, capabilities, features, status, sol_endpoint, vnc_endpoint, control_endpoint, metadata, created_at, updated_at FROM servers WHERE customer_id = ?",
		customerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		var row legacyServerRow
		var solEndpointJSON, vncEndpointJSON, controlEndpointJSON, metadataJSON sql.NullString

		err := rows.Scan(&row.ID, &row.CustomerID, &row.DatacenterID, &row.BMCType, &row.BMCEndpoint, &row.Username, &row.Capabilities, &row.Features, &row.Status, &solEndpointJSON, &vncEndpointJSON, &controlEndpointJSON, &metadataJSON, &row.CreatedAt, &row.UpdatedAt)
		if err != nil {
			return nil, err
		}

		// Convert legacy format to new Server structure
		server := models.Server{
			ID:           row.ID,
			CustomerID:   row.CustomerID,
			DatacenterID: row.DatacenterID,
			Status:       row.Status,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
			Features:     []string{}, // TODO: Parse features JSON
			Metadata:     make(map[string]string),
		}

		// Deserialize SOL endpoint if present
		if solEndpointJSON.Valid && solEndpointJSON.String != "" {
			server.SOLEndpoint = &types.SOLEndpoint{}
			if err := json.Unmarshal([]byte(solEndpointJSON.String), server.SOLEndpoint); err != nil {
				log.Warn().Err(err).Str("server_id", row.ID).Msg("Failed to unmarshal SOL endpoint")
			}
		}

		// Deserialize VNC endpoint if present
		if vncEndpointJSON.Valid && vncEndpointJSON.String != "" {
			server.VNCEndpoint = &types.VNCEndpoint{}
			if err := json.Unmarshal([]byte(vncEndpointJSON.String), server.VNCEndpoint); err != nil {
				log.Warn().Err(err).Str("server_id", row.ID).Msg("Failed to unmarshal VNC endpoint")
			}
		}

		// Deserialize control endpoints if present (prefer JSON over legacy fields)
		if controlEndpointJSON.Valid && controlEndpointJSON.String != "" {
			server.ControlEndpoints = []*types.BMCControlEndpoint{}
			if err := json.Unmarshal([]byte(controlEndpointJSON.String), &server.ControlEndpoints); err != nil {
				log.Warn().Err(err).Str("server_id", row.ID).Msg("Failed to unmarshal control endpoints")
				server.ControlEndpoints = nil // Fall back to legacy parsing below
			}
		}

		// Convert legacy BMC fields to control endpoint (fallback if JSON not available)
		if len(server.ControlEndpoints) == 0 && row.BMCEndpoint != "" {
			bmcType := types.BMCTypeIPMI // Default
			if row.BMCType == "redfish" {
				bmcType = types.BMCTypeRedfish
			}

			server.ControlEndpoints = []*types.BMCControlEndpoint{{
				Endpoint: row.BMCEndpoint,
				Type:     bmcType,
				Username: "", // Will be filled by agent registration
				Password: "", // Will be filled by agent registration
			}}
			server.PrimaryProtocol = bmcType
		}

		// Deserialize metadata if present
		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &server.Metadata); err != nil {
				log.Warn().Err(err).Str("server_id", row.ID).Msg("Failed to unmarshal metadata")
			}
		}

		// Parse features if needed
		if row.Features != "" {
			// TODO: Implement proper JSON parsing
			server.Features = []string{row.Features} // Simplified for now
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// New methods for updated architecture

func (db *DB) GetCustomerByEmail(email string) (*models.Customer, error) {
	var customer models.Customer
	err := db.conn.QueryRow(
		"SELECT id, email, api_key, created_at FROM customers WHERE email = ?",
		email,
	).Scan(&customer.ID, &customer.Email, &customer.APIKey, &customer.CreatedAt)

	if err != nil {
		return nil, err
	}
	return &customer, nil
}

// Regional Gateway methods
func (db *DB) CreateRegionalGateway(gateway *models.RegionalGateway) error {
	// Convert datacenter IDs slice to JSON string (simplified)
	datacenterIDsStr := ""
	for i, id := range gateway.DatacenterIDs {
		if i > 0 {
			datacenterIDsStr += ","
		}
		datacenterIDsStr += id
	}

	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO regional_gateways (id, region, endpoint, datacenter_ids, status, last_seen, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		gateway.ID, gateway.Region, gateway.Endpoint, datacenterIDsStr, gateway.Status, gateway.LastSeen, gateway.CreatedAt,
	)
	return err
}

func (db *DB) GetRegionalGateway(gatewayID string) (*models.RegionalGateway, error) {
	var gateway models.RegionalGateway
	var datacenterIDsStr string

	err := db.conn.QueryRow(
		"SELECT id, region, endpoint, datacenter_ids, status, last_seen, created_at FROM regional_gateways WHERE id = ?",
		gatewayID,
	).Scan(&gateway.ID, &gateway.Region, &gateway.Endpoint, &datacenterIDsStr, &gateway.Status, &gateway.LastSeen, &gateway.CreatedAt)

	if err != nil {
		return nil, err
	}

	// Parse datacenter IDs from string (simplified)
	if datacenterIDsStr != "" {
		gateway.DatacenterIDs = []string{datacenterIDsStr} // Simplified parsing
	}

	return &gateway, nil
}

func (db *DB) ListRegionalGateways(region string) ([]models.RegionalGateway, error) {
	query := "SELECT id, region, endpoint, datacenter_ids, status, last_seen, created_at FROM regional_gateways"
	args := []interface{}{}

	if region != "" {
		query += " WHERE region = ?"
		args = append(args, region)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gateways []models.RegionalGateway
	for rows.Next() {
		var gateway models.RegionalGateway
		var datacenterIDsStr string

		err := rows.Scan(&gateway.ID, &gateway.Region, &gateway.Endpoint, &datacenterIDsStr, &gateway.Status, &gateway.LastSeen, &gateway.CreatedAt)
		if err != nil {
			return nil, err
		}

		// Parse datacenter IDs from string (simplified)
		if datacenterIDsStr != "" {
			gateway.DatacenterIDs = []string{datacenterIDsStr} // Simplified parsing
		}

		gateways = append(gateways, gateway)
	}

	return gateways, nil
}

// Server Location methods
func (db *DB) CreateServerLocation(location *models.ServerLocation) error {
	// Convert features slice to string (simplified)
	featuresStr := ""
	for i, feature := range location.Features {
		if i > 0 {
			featuresStr += ","
		}
		featuresStr += feature
	}

	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO server_locations (server_id, customer_id, datacenter_id, regional_gateway_id, bmc_type, features, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		location.ServerID, location.CustomerID, location.DatacenterID, location.RegionalGatewayID, string(location.PrimaryProtocol), featuresStr, location.CreatedAt, location.UpdatedAt,
	)
	return err
}

func (db *DB) GetServerLocation(serverID string) (*models.ServerLocation, error) {
	var location models.ServerLocation
	var featuresStr string
	var bmcType string

	err := db.conn.QueryRow(
		"SELECT server_id, customer_id, datacenter_id, regional_gateway_id, bmc_type, features, created_at, updated_at FROM server_locations WHERE server_id = ?",
		serverID,
	).Scan(&location.ServerID, &location.CustomerID, &location.DatacenterID, &location.RegionalGatewayID, &bmcType, &featuresStr, &location.CreatedAt, &location.UpdatedAt)

	location.PrimaryProtocol = types.BMCType(bmcType)

	if err != nil {
		return nil, err
	}

	// Parse features from string (simplified)
	if featuresStr != "" {
		location.Features = []string{featuresStr} // Simplified parsing
	}

	return &location, nil
}

// ListAllServers returns all server locations in the system (for admin/monitoring)
func (db *DB) ListAllServers() ([]models.ServerLocation, error) {
	rows, err := db.conn.Query(
		"SELECT server_id, customer_id, datacenter_id, regional_gateway_id, bmc_type, features, created_at, updated_at FROM server_locations ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []models.ServerLocation
	for rows.Next() {
		var location models.ServerLocation
		var featuresStr string
		var bmcType string

		err := rows.Scan(&location.ServerID, &location.CustomerID, &location.DatacenterID, &location.RegionalGatewayID, &bmcType, &featuresStr, &location.CreatedAt, &location.UpdatedAt)
		if err != nil {
			return nil, err
		}

		location.PrimaryProtocol = types.BMCType(bmcType)

		// Parse features from string (simplified)
		if featuresStr != "" {
			location.Features = []string{featuresStr} // Simplified parsing
		}

		servers = append(servers, location)
	}

	return servers, nil
}

// Error constants
var (
	ErrServerNotFound = fmt.Errorf("server not found")
)

// GetServerByIDAndCustomer retrieves a server by ID and ensures it belongs to the customer
func (db *DB) GetServerByIDAndCustomer(serverID, customerID string) (*models.Server, error) {
	var row legacyServerRow

	err := db.conn.QueryRow(
		"SELECT id, customer_id, datacenter_id, bmc_type, bmc_endpoint, username, capabilities, features, status, created_at, updated_at FROM servers WHERE id = ? AND customer_id = ?",
		serverID, customerID,
	).Scan(&row.ID, &row.CustomerID, &row.DatacenterID, &row.BMCType, &row.BMCEndpoint, &row.Username, &row.Capabilities, &row.Features, &row.Status, &row.CreatedAt, &row.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrServerNotFound
		}
		return nil, err
	}

	// Convert legacy format to new Server structure
	server := &models.Server{
		ID:           row.ID,
		CustomerID:   row.CustomerID,
		DatacenterID: row.DatacenterID,
		Status:       row.Status,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
		Features:     []string{}, // TODO: Parse features JSON
		Metadata:     make(map[string]string),
	}

	// Convert legacy BMC fields to control endpoints
	if row.BMCEndpoint != "" {
		bmcType := types.BMCTypeIPMI // Default
		if row.BMCType == "redfish" {
			bmcType = types.BMCTypeRedfish
		}

		// Parse capabilities from string
		capabilities := []string{}
		if row.Capabilities != "" {
			capabilities = strings.Split(row.Capabilities, ",")
		}

		server.ControlEndpoints = []*types.BMCControlEndpoint{{
			Endpoint:     row.BMCEndpoint,
			Type:         bmcType,
			Username:     row.Username,
			Password:     "", // Password not stored per security requirement
			Capabilities: capabilities,
		}}
		server.PrimaryProtocol = bmcType
	}

	// Parse features if needed
	if row.Features != "" {
		// TODO: Implement proper JSON parsing
		server.Features = []string{row.Features} // Simplified for now
	}

	return server, nil
}

// ListServersByCustomer retrieves servers for a customer with pagination
func (db *DB) ListServersByCustomer(customerID string, pageSize int32, pageToken string) ([]models.Server, string, error) {
	// For simplicity, we're not implementing full pagination with tokens
	// In a production system, you'd parse the pageToken to determine offset

	rows, err := db.conn.Query(
		"SELECT id, customer_id, datacenter_id, bmc_type, bmc_endpoint, username, capabilities, features, status, created_at, updated_at FROM servers WHERE customer_id = ? LIMIT ?",
		customerID, pageSize,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		var row legacyServerRow

		err := rows.Scan(&row.ID, &row.CustomerID, &row.DatacenterID, &row.BMCType, &row.BMCEndpoint, &row.Username, &row.Capabilities, &row.Features, &row.Status, &row.CreatedAt, &row.UpdatedAt)
		if err != nil {
			return nil, "", err
		}

		// Convert legacy format to new Server structure
		server := models.Server{
			ID:           row.ID,
			CustomerID:   row.CustomerID,
			DatacenterID: row.DatacenterID,
			Status:       row.Status,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
			Features:     []string{}, // TODO: Parse features JSON
			Metadata:     make(map[string]string),
		}

		// Convert legacy BMC fields to control endpoints
		if row.BMCEndpoint != "" {
			bmcType := types.BMCTypeIPMI // Default
			if row.BMCType == "redfish" {
				bmcType = types.BMCTypeRedfish
			}

			server.ControlEndpoints = []*types.BMCControlEndpoint{{
				Endpoint: row.BMCEndpoint,
				Type:     bmcType,
				Username: "", // Will be filled by agent registration
				Password: "", // Will be filled by agent registration
			}}
			server.PrimaryProtocol = bmcType
		}

		// Parse features if needed
		if row.Features != "" {
			// TODO: Implement proper JSON parsing
			server.Features = []string{row.Features} // Simplified for now
		}

		servers = append(servers, server)
	}

	// For simplicity, we're not implementing actual pagination tokens
	// In a production system, you'd generate a token based on the last item
	nextPageToken := ""
	if len(servers) == int(pageSize) {
		nextPageToken = "next_page" // Placeholder token
	}

	return servers, nextPageToken, nil
}

// CreateServer creates a new server record or updates existing one
func (db *DB) CreateServer(server *models.Server) error {
	// Convert features slice to string (simplified)
	featuresStr := ""
	for i, feature := range server.Features {
		if i > 0 {
			featuresStr += ","
		}
		featuresStr += feature
	}

	// Convert new Server structure to legacy format for database storage
	bmcType := string(server.PrimaryProtocol)
	if bmcType == "" {
		bmcType = "ipmi" // Default fallback
	}
	bmcEndpoint := ""
	username := ""
	capabilities := ""

	if len(server.ControlEndpoints) > 0 {
		bmcEndpoint = server.GetPrimaryControlEndpoint().Endpoint
		if server.GetPrimaryControlEndpoint().Type != "" {
			bmcType = string(server.GetPrimaryControlEndpoint().Type)
		}
		username = server.GetPrimaryControlEndpoint().Username

		// Convert capabilities slice to string
		for i, capability := range server.GetPrimaryControlEndpoint().Capabilities {
			if i > 0 {
				capabilities += ","
			}
			capabilities += capability
		}
	}

	// Serialize endpoints and metadata to JSON
	var solEndpointJSON, vncEndpointJSON, controlEndpointsJSON, metadataJSON []byte
	var err error

	if server.SOLEndpoint != nil {
		solEndpointJSON, err = json.Marshal(server.SOLEndpoint)
		if err != nil {
			return fmt.Errorf("failed to marshal SOL endpoint: %w", err)
		}
	}

	if server.VNCEndpoint != nil {
		vncEndpointJSON, err = json.Marshal(server.VNCEndpoint)
		if err != nil {
			return fmt.Errorf("failed to marshal VNC endpoint: %w", err)
		}
	}

	if len(server.ControlEndpoints) > 0 {
		controlEndpointsJSON, err = json.Marshal(server.ControlEndpoints)
		if err != nil {
			return fmt.Errorf("failed to marshal control endpoints: %w", err)
		}
	}

	if server.Metadata != nil && len(server.Metadata) > 0 {
		metadataJSON, err = json.Marshal(server.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	var discoveryMetadataJSON []byte
	if server.DiscoveryMetadata != nil {
		discoveryMetadataJSON, err = json.Marshal(server.DiscoveryMetadata)
		if err != nil {
			return fmt.Errorf("failed to marshal discovery metadata: %w", err)
		}
	}

	// Use INSERT ... ON CONFLICT for better upsert control
	_, err = db.conn.Exec(`
		INSERT INTO servers (id, customer_id, datacenter_id, bmc_type, bmc_endpoint, username, capabilities, features, status, sol_endpoint, vnc_endpoint, control_endpoint, metadata, discovery_metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			customer_id = excluded.customer_id,
			datacenter_id = excluded.datacenter_id,
			bmc_type = excluded.bmc_type,
			bmc_endpoint = excluded.bmc_endpoint,
			username = excluded.username,
			capabilities = excluded.capabilities,
			features = excluded.features,
			status = excluded.status,
			sol_endpoint = excluded.sol_endpoint,
			vnc_endpoint = excluded.vnc_endpoint,
			control_endpoint = excluded.control_endpoint,
			metadata = excluded.metadata,
			discovery_metadata = excluded.discovery_metadata,
			updated_at = excluded.updated_at
	`, server.ID, server.CustomerID, server.DatacenterID, bmcType, bmcEndpoint, username, capabilities, featuresStr, server.Status, solEndpointJSON, vncEndpointJSON, controlEndpointsJSON, metadataJSON, discoveryMetadataJSON, server.CreatedAt, server.UpdatedAt)

	return err
}

// ServerLocationWithBMC contains server location info plus BMC endpoint
type ServerLocationWithBMC struct {
	ServerID          string
	CustomerID        string
	DatacenterID      string
	RegionalGatewayID string
	BMCType           types.BMCType
	Features          []string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	BMCEndpoint       string // Added from servers table
}

// ListAllServersWithBMCEndpoints returns all server locations joined with BMC endpoints for system status
func (db *DB) ListAllServersWithBMCEndpoints() ([]ServerLocationWithBMC, error) {
	rows, err := db.conn.Query(`
		SELECT
			sl.server_id,
			sl.customer_id,
			sl.datacenter_id,
			sl.regional_gateway_id,
			sl.bmc_type,
			sl.features,
			sl.created_at,
			sl.updated_at,
			COALESCE(s.bmc_endpoint, '') as bmc_endpoint
		FROM server_locations sl
		LEFT JOIN servers s ON sl.server_id = s.id
		ORDER BY sl.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []ServerLocationWithBMC
	for rows.Next() {
		var server ServerLocationWithBMC
		var featuresStr string

		err := rows.Scan(
			&server.ServerID,
			&server.CustomerID,
			&server.DatacenterID,
			&server.RegionalGatewayID,
			&server.BMCType,
			&featuresStr,
			&server.CreatedAt,
			&server.UpdatedAt,
			&server.BMCEndpoint,
		)
		if err != nil {
			return nil, err
		}

		// Parse features from string (simplified)
		if featuresStr != "" {
			server.Features = []string{featuresStr} // Simplified parsing
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// Server-Customer Mapping Methods

// CreateServerCustomerMapping creates a mapping between a server and customer
func (db *DB) CreateServerCustomerMapping(mapping *models.ServerCustomerMapping) error {
	_, err := db.conn.Exec(`
		INSERT INTO server_customer_mappings (id, server_id, customer_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(server_id, customer_id) DO UPDATE SET
			updated_at = excluded.updated_at
	`, mapping.ID, mapping.ServerID, mapping.CustomerID, mapping.CreatedAt, mapping.UpdatedAt)
	return err
}

// GetServersForCustomer returns all servers accessible by a customer via the mapping table
func (db *DB) GetServersForCustomer(customerID string) ([]models.Server, error) {
	rows, err := db.conn.Query(`
		SELECT s.id, s.customer_id, s.datacenter_id, s.bmc_type, s.bmc_endpoint, s.features, s.status, s.created_at, s.updated_at
		FROM servers s
		INNER JOIN server_customer_mappings scm ON s.id = scm.server_id
		WHERE scm.customer_id = ?
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		var row legacyServerRow
		err := rows.Scan(
			&row.ID,
			&row.CustomerID,
			&row.DatacenterID,
			&row.BMCType,
			&row.BMCEndpoint,
			&row.Features,
			&row.Status,
			&row.CreatedAt,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert legacy format to new Server structure
		server := models.Server{
			ID:           row.ID,
			CustomerID:   row.CustomerID,
			DatacenterID: row.DatacenterID,
			Status:       row.Status,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
			Features:     []string{}, // TODO: Parse features JSON
			Metadata:     make(map[string]string),
		}

		// Convert legacy BMC fields to control endpoints
		if row.BMCEndpoint != "" {
			bmcType := types.BMCTypeIPMI // Default
			if row.BMCType == "redfish" {
				bmcType = types.BMCTypeRedfish
			}

			server.ControlEndpoints = []*types.BMCControlEndpoint{{
				Endpoint: row.BMCEndpoint,
				Type:     bmcType,
				Username: "", // Will be filled by agent registration
				Password: "", // Will be filled by agent registration
			}}
			server.PrimaryProtocol = bmcType
		}

		// Parse features if needed
		if row.Features != "" {
			// TODO: Implement proper JSON parsing
			server.Features = []string{row.Features} // Simplified for now
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// GetAllServers returns all servers in the system (for temporary use)
func (db *DB) GetAllServers() ([]models.Server, error) {
	rows, err := db.conn.Query(`
		SELECT id, customer_id, datacenter_id, bmc_type, bmc_endpoint, features, status, sol_endpoint, vnc_endpoint, control_endpoint, metadata, created_at, updated_at
		FROM servers
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []models.Server
	for rows.Next() {
		var row legacyServerRow
		var solEndpointJSON, vncEndpointJSON, controlEndpointJSON, metadataJSON sql.NullString

		err := rows.Scan(
			&row.ID,
			&row.CustomerID,
			&row.DatacenterID,
			&row.BMCType,
			&row.BMCEndpoint,
			&row.Features,
			&row.Status,
			&solEndpointJSON,
			&vncEndpointJSON,
			&controlEndpointJSON,
			&metadataJSON,
			&row.CreatedAt,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert legacy format to new Server structure
		server := models.Server{
			ID:           row.ID,
			CustomerID:   row.CustomerID,
			DatacenterID: row.DatacenterID,
			Status:       row.Status,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
			Features:     []string{}, // TODO: Parse features JSON
			Metadata:     make(map[string]string),
		}

		// Deserialize SOL endpoint if present
		if solEndpointJSON.Valid && solEndpointJSON.String != "" {
			server.SOLEndpoint = &types.SOLEndpoint{}
			if err := json.Unmarshal([]byte(solEndpointJSON.String), server.SOLEndpoint); err != nil {
				log.Warn().Err(err).Str("server_id", row.ID).Msg("Failed to unmarshal SOL endpoint")
			}
		}

		// Deserialize VNC endpoint if present
		if vncEndpointJSON.Valid && vncEndpointJSON.String != "" {
			server.VNCEndpoint = &types.VNCEndpoint{}
			if err := json.Unmarshal([]byte(vncEndpointJSON.String), server.VNCEndpoint); err != nil {
				log.Warn().Err(err).Str("server_id", row.ID).Msg("Failed to unmarshal VNC endpoint")
			}
		}

		// Deserialize control endpoints if present (prefer JSON over legacy fields)
		if controlEndpointJSON.Valid && controlEndpointJSON.String != "" {
			server.ControlEndpoints = []*types.BMCControlEndpoint{}
			if err := json.Unmarshal([]byte(controlEndpointJSON.String), &server.ControlEndpoints); err != nil {
				log.Warn().Err(err).Str("server_id", row.ID).Msg("Failed to unmarshal control endpoints")
				server.ControlEndpoints = nil // Fall back to legacy parsing below
			}
		}

		// Convert legacy BMC fields to control endpoint (fallback if JSON not available)
		if len(server.ControlEndpoints) == 0 && row.BMCEndpoint != "" {
			bmcType := types.BMCTypeIPMI // Default
			if row.BMCType == "redfish" {
				bmcType = types.BMCTypeRedfish
			}

			server.ControlEndpoints = []*types.BMCControlEndpoint{{
				Endpoint: row.BMCEndpoint,
				Type:     bmcType,
				Username: "", // Will be filled by agent registration
				Password: "", // Will be filled by agent registration
			}}
			server.PrimaryProtocol = bmcType
		}

		// Deserialize metadata if present
		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &server.Metadata); err != nil {
				log.Warn().Err(err).Str("server_id", row.ID).Msg("Failed to unmarshal metadata")
			}
		}

		// Parse features if needed
		if row.Features != "" {
			// TODO: Implement proper JSON parsing
			server.Features = []string{row.Features} // Simplified for now
		}

		servers = append(servers, server)
	}

	return servers, nil
}

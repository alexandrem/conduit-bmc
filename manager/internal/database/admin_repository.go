package database

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"google.golang.org/protobuf/types/known/timestamppb"

	managerv1 "manager/gen/manager/v1"
)

// AdminRepository provides database operations for admin dashboard
type AdminRepository interface {
	// Dashboard metrics
	GetDashboardMetrics(ctx context.Context) (*managerv1.GetDashboardMetricsResponse, error)

	// Server operations
	ListAllServersWithFilters(ctx context.Context, filters *AdminServerFilters) ([]*managerv1.ServerDetails, error)
	GetTotalServersCount(ctx context.Context, filters *AdminServerFilters) (int32, error)

	// Customer operations
	ListAllCustomersWithCounts(ctx context.Context, pageSize int32, offset int32) ([]*managerv1.CustomerSummary, error)
	GetTotalCustomersCount(ctx context.Context) (int32, error)

	// Gateway operations
	GetGatewayHealth(ctx context.Context) ([]*managerv1.GatewayHealth, error)

	// Region operations
	GetRegions(ctx context.Context) ([]string, error)
}

// AdminServerFilters holds filter parameters for admin server listing
type AdminServerFilters struct {
	CustomerFilter string
	RegionFilter   []string
	GatewayFilter  string
	StatusFilter   string
	PageSize       int32
	Offset         int32
}

type adminRepository struct {
	db *bun.DB
}

// NewAdminRepository creates a new admin repository
func NewAdminRepository(db *bun.DB) AdminRepository {
	return &adminRepository{db: db}
}

// GetDashboardMetrics returns aggregated metrics for the admin dashboard
func (r *adminRepository) GetDashboardMetrics(ctx context.Context) (*managerv1.GetDashboardMetricsResponse, error) {
	metrics := &managerv1.GetDashboardMetricsResponse{}

	// Count total servers via server_location (this represents actual registered servers)
	totalServers, err := r.db.NewSelect().
		Model((*ServerLocation)(nil)).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	metrics.TotalBmcs = int32(totalServers)

	// Count online servers (any status except 'offline' or 'unknown') by joining with servers table
	// This includes statuses like 'active', 'configured', 'online', etc.
	onlineServers, err := r.db.NewSelect().
		Model((*ServerLocation)(nil)).
		Join("INNER JOIN servers AS s ON s.id = server_location.server_id").
		Where("s.status != ? AND s.status != ?", "offline", "unknown").
		Count(ctx)
	if err != nil {
		return nil, err
	}
	metrics.OnlineBmcs = int32(onlineServers)
	metrics.OfflineBmcs = metrics.TotalBmcs - metrics.OnlineBmcs

	// Count total gateways
	totalGateways, err := r.db.NewSelect().
		Model((*RegionalGateway)(nil)).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	metrics.TotalGateways = int32(totalGateways)

	// Count active gateways (last_seen within 1 minute)
	activeThreshold := time.Now().UTC().Add(-1 * time.Minute)
	activeGateways, err := r.db.NewSelect().
		Model((*RegionalGateway)(nil)).
		Where("status = ?", "active").
		Where("last_seen > ?", activeThreshold).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	metrics.ActiveGateways = int32(activeGateways)

	// Count total customers
	totalCustomers, err := r.db.NewSelect().
		Model((*Customer)(nil)).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	metrics.TotalCustomers = int32(totalCustomers)

	// Active sessions - future implementation
	metrics.ActiveSessions = 0

	return metrics, nil
}

// ListAllServersWithFilters returns servers with optional filtering
func (r *adminRepository) ListAllServersWithFilters(ctx context.Context, filters *AdminServerFilters) ([]*managerv1.ServerDetails, error) {
	query := r.db.NewSelect().
		Model((*ServerLocation)(nil)).
		Relation("Server").
		Order("server_location.created_at DESC")

	// Apply filters
	if filters.CustomerFilter != "" {
		query = query.Where("server_location.customer_id = ?", filters.CustomerFilter)
	}

	if filters.GatewayFilter != "" {
		query = query.Where("server_location.regional_gateway_id = ?", filters.GatewayFilter)
	}

	if filters.StatusFilter != "" {
		query = query.Join("INNER JOIN servers AS s ON s.id = server_location.server_id").
			Where("s.status = ?", filters.StatusFilter)
	}

	if len(filters.RegionFilter) > 0 {
		query = query.Join("INNER JOIN regional_gateways AS rg ON rg.id = server_location.regional_gateway_id").
			Where("rg.region IN (?)", bun.In(filters.RegionFilter))
	}

	// Apply pagination
	if filters.PageSize > 0 {
		query = query.Limit(int(filters.PageSize))
	}
	if filters.Offset > 0 {
		query = query.Offset(int(filters.Offset))
	}

	var locations []*ServerLocation
	err := query.Scan(ctx, &locations)
	if err != nil {
		return nil, err
	}

	// Load related servers and gateways
	var serverIDs []string
	var gatewayIDs []string
	for _, loc := range locations {
		serverIDs = append(serverIDs, loc.ServerID)
		gatewayIDs = append(gatewayIDs, loc.RegionalGatewayID)
	}

	// Fetch servers
	servers := make(map[string]*Server)
	if len(serverIDs) > 0 {
		var serverList []*Server
		err = r.db.NewSelect().
			Model(&serverList).
			Where("id IN (?)", bun.In(serverIDs)).
			Scan(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range serverList {
			servers[s.ID] = s
		}
	}

	// Fetch gateways
	gateways := make(map[string]*RegionalGateway)
	if len(gatewayIDs) > 0 {
		var gatewayList []*RegionalGateway
		err = r.db.NewSelect().
			Model(&gatewayList).
			Where("id IN (?)", bun.In(gatewayIDs)).
			Scan(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range gatewayList {
			gateways[g.ID] = g
		}
	}

	// Convert to ServerDetails
	result := make([]*managerv1.ServerDetails, 0, len(locations))
	for _, loc := range locations {
		server, ok := servers[loc.ServerID]
		if !ok {
			continue
		}

		details := &managerv1.ServerDetails{
			ServerId:     loc.ServerID,
			CustomerId:   loc.CustomerID,
			DatacenterId: loc.DatacenterID,
			GatewayId:    loc.RegionalGatewayID,
			Status:       server.Status,
			HasVnc:       server.VNCEndpoint != nil,
			HasSol:       server.SOLEndpoint != nil,
		}

		// Set primary endpoint and protocol
		if len(loc.ControlEndpoints) > 0 {
			details.PrimaryEndpoint = loc.ControlEndpoints[0].Endpoint
		}
		details.PrimaryProtocol = string(loc.PrimaryProtocol)

		// Timestamps
		if !server.UpdatedAt.IsZero() {
			details.LastSeen = timestampProto(server.UpdatedAt)
		}
		details.CreatedAt = timestampProto(loc.CreatedAt)

		result = append(result, details)
	}

	return result, nil
}

// GetTotalServersCount returns the total count of servers matching filters
func (r *adminRepository) GetTotalServersCount(ctx context.Context, filters *AdminServerFilters) (int32, error) {
	query := r.db.NewSelect().
		Model((*ServerLocation)(nil))

	// Apply the same filters as ListAllServersWithFilters
	if filters.CustomerFilter != "" {
		query = query.Where("customer_id = ?", filters.CustomerFilter)
	}

	if filters.GatewayFilter != "" {
		query = query.Where("regional_gateway_id = ?", filters.GatewayFilter)
	}

	if filters.StatusFilter != "" {
		query = query.Join("INNER JOIN servers AS s ON s.id = server_location.server_id").
			Where("s.status = ?", filters.StatusFilter)
	}

	if len(filters.RegionFilter) > 0 {
		query = query.Join("INNER JOIN regional_gateways AS rg ON rg.id = server_location.regional_gateway_id").
			Where("rg.region IN (?)", bun.In(filters.RegionFilter))
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, err
	}

	return int32(count), nil
}

// ListAllCustomersWithCounts returns customers with their server counts
func (r *adminRepository) ListAllCustomersWithCounts(ctx context.Context, pageSize int32, offset int32) ([]*managerv1.CustomerSummary, error) {
	var customers []*Customer
	query := r.db.NewSelect().
		Model(&customers).
		Order("created_at DESC")

	if pageSize > 0 {
		query = query.Limit(int(pageSize))
	}
	if offset > 0 {
		query = query.Offset(int(offset))
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*managerv1.CustomerSummary, 0, len(customers))
	for _, c := range customers {
		// Count total servers for this customer
		serverCount, err := r.db.NewSelect().
			Model((*Server)(nil)).
			Where("customer_id = ?", c.ID).
			Count(ctx)
		if err != nil {
			return nil, err
		}

		// Count online servers (any status except 'offline' or 'unknown')
		onlineCount, err := r.db.NewSelect().
			Model((*Server)(nil)).
			Where("customer_id = ?", c.ID).
			Where("status != ? AND status != ?", "offline", "unknown").
			Count(ctx)
		if err != nil {
			return nil, err
		}

		summary := &managerv1.CustomerSummary{
			CustomerId:        c.ID,
			Email:             c.Email,
			ServerCount:       int32(serverCount),
			OnlineServerCount: int32(onlineCount),
			IsAdmin:           c.IsAdmin,
			CreatedAt:         timestampProto(c.CreatedAt),
		}

		result = append(result, summary)
	}

	return result, nil
}

// GetTotalCustomersCount returns the total number of customers
func (r *adminRepository) GetTotalCustomersCount(ctx context.Context) (int32, error) {
	count, err := r.db.NewSelect().
		Model((*Customer)(nil)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

// GetGatewayHealth returns health information for all gateways
func (r *adminRepository) GetGatewayHealth(ctx context.Context) ([]*managerv1.GatewayHealth, error) {
	var gateways []*RegionalGateway
	err := r.db.NewSelect().
		Model(&gateways).
		Order("region ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*managerv1.GatewayHealth, 0, len(gateways))
	for _, g := range gateways {
		// Count servers for this gateway
		serverCount, err := r.db.NewSelect().
			Model((*ServerLocation)(nil)).
			Where("regional_gateway_id = ?", g.ID).
			Count(ctx)
		if err != nil {
			return nil, err
		}

		// Determine gateway status based on last_seen and status field
		status := "offline"
		activeThreshold := time.Now().UTC().Add(-1 * time.Minute)
		if g.Status == "active" && g.LastSeen.After(activeThreshold) {
			status = "active"
		} else if g.Status == "active" {
			status = "degraded"
		}

		health := &managerv1.GatewayHealth{
			GatewayId:     g.ID,
			Region:        g.Region,
			Endpoint:      g.Endpoint,
			Status:        status,
			LastSeen:      timestampProto(g.LastSeen),
			ServerCount:   int32(serverCount),
			DatacenterIds: g.DatacenterIDs,
		}

		result = append(result, health)
	}

	return result, nil
}

// GetRegions returns a list of unique regions from gateways
func (r *adminRepository) GetRegions(ctx context.Context) ([]string, error) {
	var regions []string
	err := r.db.NewSelect().
		Model((*RegionalGateway)(nil)).
		ColumnExpr("DISTINCT region").
		Order("region ASC").
		Scan(ctx, &regions)
	if err != nil {
		return nil, err
	}
	return regions, nil
}

// Helper function to convert time.Time to protobuf Timestamp
func timestampProto(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

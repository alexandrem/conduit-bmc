package metrics

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"manager/internal/database"
)

// Collector periodically updates gauge metrics from database state
type Collector struct {
	db       *database.BunDB
	interval time.Duration
	stopCh   chan struct{}
}

// NewCollector creates a new metrics collector
func NewCollector(db *database.BunDB, interval time.Duration) *Collector {
	if interval == 0 {
		interval = 30 * time.Second // Default collection interval
	}

	return &Collector{
		db:       db,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic metrics collection
func (c *Collector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect initial metrics immediately
	c.collectMetrics(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.collectMetrics(ctx)
		}
	}
}

// Stop stops the metrics collector
func (c *Collector) Stop() {
	close(c.stopCh)
}

// collectMetrics updates all gauge metrics from current system state
func (c *Collector) collectMetrics(ctx context.Context) {
	// Collect gateway metrics
	if err := c.collectGatewayMetrics(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to collect gateway metrics")
	}

	// Collect server metrics
	if err := c.collectServerMetrics(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to collect server metrics")
	}

	// Collect database connection metrics
	if err := c.collectDatabaseMetrics(); err != nil {
		log.Warn().Err(err).Msg("Failed to collect database metrics")
	}
}

// collectGatewayMetrics updates gateway-related metrics
func (c *Collector) collectGatewayMetrics(ctx context.Context) error {
	gateways, err := c.db.Gateways.List(ctx)
	if err != nil {
		return err
	}

	// Reset gateway metrics (to handle deleted gateways)
	GatewaysTotal.Reset()

	// Count gateways by status
	cutoffTime := time.Now().Add(-2 * time.Minute)      // Active if seen within 2 minutes
	datacenterCounts := make(map[string]map[string]int) // datacenter -> status -> count

	for _, gateway := range gateways {
		status := "offline"
		if gateway.LastSeen.After(cutoffTime) {
			status = "online"
		}

		// Gateway spans multiple datacenters, count for each
		for _, dcID := range gateway.DatacenterIDs {
			if datacenterCounts[dcID] == nil {
				datacenterCounts[dcID] = make(map[string]int)
			}
			datacenterCounts[dcID][status]++
		}
	}

	// Update metrics
	for datacenter, statuses := range datacenterCounts {
		for status, count := range statuses {
			GatewaysTotal.WithLabelValues(datacenter, status).Set(float64(count))
		}
	}

	return nil
}

// collectServerMetrics updates server-related metrics
func (c *Collector) collectServerMetrics(ctx context.Context) error {
	locations, err := c.db.Locations.List(ctx)
	if err != nil {
		return err
	}

	// Reset server metrics
	ServersTotal.Reset()

	// Count servers by customer, datacenter, and status
	type serverKey struct {
		customerID   string
		datacenterID string
		status       string
	}
	serverCounts := make(map[serverKey]int)

	for _, loc := range locations {
		key := serverKey{
			customerID:   loc.CustomerID,
			datacenterID: loc.DatacenterID,
			status:       "active", // We don't have explicit status in locations, default to active
		}
		serverCounts[key]++
	}

	// Update metrics
	for key, count := range serverCounts {
		ServersTotal.WithLabelValues(key.customerID, key.datacenterID, key.status).Set(float64(count))
	}

	return nil
}

// collectDatabaseMetrics updates database-related metrics
func (c *Collector) collectDatabaseMetrics() error {
	stats := c.db.DB().Stats()

	DBConnections.Set(float64(stats.OpenConnections))

	return nil
}

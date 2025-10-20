package metrics

import (
	"context"
	"time"

	"gateway/internal/gateway"
)

// Collector periodically updates gauge metrics from gateway state
type Collector struct {
	handler  *gateway.RegionalGatewayHandler
	interval time.Duration
	stopCh   chan struct{}
}

// NewCollector creates a new metrics collector
func NewCollector(handler *gateway.RegionalGatewayHandler, interval time.Duration) *Collector {
	if interval == 0 {
		interval = 15 * time.Second // Default collection interval (more frequent for gateway)
	}

	return &Collector{
		handler:  handler,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic metrics collection
func (c *Collector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect initial metrics immediately
	c.collectMetrics()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.collectMetrics()
		}
	}
}

// Stop stops the metrics collector
func (c *Collector) Stop() {
	close(c.stopCh)
}

// collectMetrics updates all gauge metrics from current gateway state
func (c *Collector) collectMetrics() {
	c.collectAgentMetrics()
	c.collectSessionMetrics()
}

// collectAgentMetrics updates agent-related metrics
func (c *Collector) collectAgentMetrics() {
	registry := c.handler.GetAgentRegistry()
	agents := registry.List()

	// Reset agent metrics
	AgentsTotal.Reset()
	AgentLastHeartbeat.Reset()

	// Count agents by datacenter and status
	type agentKey struct {
		datacenter string
		status     string
	}
	agentCounts := make(map[agentKey]int)

	cutoffTime := time.Now().Add(-2 * time.Minute) // Active if seen within 2 minutes

	for _, a := range agents {
		status := "offline"
		if a.LastSeen.After(cutoffTime) {
			status = "online"
		}

		key := agentKey{
			datacenter: a.DatacenterID,
			status:     status,
		}
		agentCounts[key]++

		// Update last heartbeat metric
		lastSeen := time.Since(a.LastSeen).Seconds()
		AgentLastHeartbeat.WithLabelValues(a.ID).Set(lastSeen)
	}

	// Update agent total metrics
	for key, count := range agentCounts {
		AgentsTotal.WithLabelValues(key.datacenter, key.status).Set(float64(count))
	}
}

// collectSessionMetrics updates session-related metrics
func (c *Collector) collectSessionMetrics() {
	sessionCount := c.handler.GetConsoleSessionCount()

	// Reset session metrics
	SessionsTotal.Reset()

	// For now, we set total sessions without type/customer breakdown
	// TODO: Extend gateway handler to provide sessions by type
	SessionsTotal.WithLabelValues("sol", "all").Set(float64(sessionCount))
}

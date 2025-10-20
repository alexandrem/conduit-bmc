package metrics

import (
	"context"
	"time"
)

// AgentState provides access to agent state for metrics collection
type AgentState interface {
	GetServerCount() int
	IsRegistered() bool
}

// Collector periodically updates gauge metrics from agent state
type Collector struct {
	agent    AgentState
	interval time.Duration
	stopCh   chan struct{}
}

// NewCollector creates a new metrics collector
func NewCollector(agent AgentState, interval time.Duration) *Collector {
	if interval == 0 {
		interval = 15 * time.Second // Default collection interval
	}

	return &Collector{
		agent:    agent,
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

// collectMetrics updates all gauge metrics from current agent state
func (c *Collector) collectMetrics() {
	c.collectDiscoveryMetrics()
	c.collectConnectionMetrics()
	c.collectSessionMetrics()
}

// collectDiscoveryMetrics updates discovery-related metrics
func (c *Collector) collectDiscoveryMetrics() {
	serverCount := c.agent.GetServerCount()

	// Reset discovery metrics
	ServersDiscovered.Reset()

	// For now, we report total servers without BMC type breakdown
	// TODO: Extend agent to track servers by BMC type
	ServersDiscovered.WithLabelValues("all").Set(float64(serverCount))
}

// collectConnectionMetrics updates gateway connection metrics
func (c *Collector) collectConnectionMetrics() {
	registered := c.agent.IsRegistered()

	status := 0.0
	if registered {
		status = 1.0
	}

	GatewayConnectionStatus.Set(status)
}

// collectSessionMetrics updates session-related metrics
func (c *Collector) collectSessionMetrics() {
	// Reset session metrics
	SOLSessionsTotal.Reset()
	VNCSessionsTotal.Reset()

	// TODO: Implement session tracking
	// Need to extend agent to expose active session counts
	// For now, these will report 0
}

package agent

import (
	"sync"
	"time"
)

// Info represents a registered Local Agent
type Info struct {
	ID           string
	DatacenterID string
	Endpoint     string
	LastSeen     time.Time
	Status       string
}

// Registry manages the in-memory registry of Local Agents
// This is rebuilt on Regional Gateway restart when agents re-register
type Registry struct {
	agents map[string]*Info
	mu     sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]*Info),
	}
}

// Register adds or updates an agent in the registry
func (r *Registry) Register(info *Info) {
	r.mu.Lock()
	defer r.mu.Unlock()

	info.Status = "active"
	r.agents[info.ID] = info
}

// Get retrieves agent information by ID
func (r *Registry) Get(agentID string) *Info {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.agents[agentID]
}

// UpdateLastSeen updates the last seen timestamp for an agent
func (r *Registry) UpdateLastSeen(agentID string, timestamp time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if agent, exists := r.agents[agentID]; exists {
		agent.LastSeen = timestamp
	}
}

// List returns all registered agents
func (r *Registry) List() []*Info {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agents := make([]*Info, 0, len(r.agents))
	for _, agent := range r.agents {
		// Create a copy to avoid race conditions
		agentCopy := *agent
		agents = append(agents, &agentCopy)
	}

	return agents
}

// Count returns the number of registered agents
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.agents)
}

// Remove removes an agent from the registry
func (r *Registry) Remove(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.agents, agentID)
}

// Cleanup removes stale agents that haven't sent heartbeats
func (r *Registry) Cleanup(staleThreshold time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for agentID, agent := range r.agents {
		if now.Sub(agent.LastSeen) > staleThreshold {
			agent.Status = "stale"
			// Optionally remove completely stale agents
			_ = agentID // Acknowledge variable use
			// delete(r.agents, agentID)
		}
	}
}

// GetByDatacenter returns all agents in a specific datacenter
func (r *Registry) GetByDatacenter(datacenterID string) []*Info {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var agents []*Info
	for _, agent := range r.agents {
		if agent.DatacenterID == datacenterID {
			// Create a copy to avoid race conditions
			agentCopy := *agent
			agents = append(agents, &agentCopy)
		}
	}

	return agents
}

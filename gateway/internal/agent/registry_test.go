package agent

import (
	"fmt"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	if registry.agents == nil {
		t.Fatal("Registry agents map is nil")
	}

	if registry.Count() != 0 {
		t.Errorf("Expected count 0, got %d", registry.Count())
	}
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	agent := &Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://localhost:8080",
		LastSeen:     time.Now(),
	}

	registry.Register(agent)

	if registry.Count() != 1 {
		t.Errorf("Expected count 1, got %d", registry.Count())
	}

	retrieved := registry.Get("agent-1")
	if retrieved == nil {
		t.Fatal("Failed to retrieve registered agent")
	}

	if retrieved.ID != "agent-1" {
		t.Errorf("Expected ID 'agent-1', got '%s'", retrieved.ID)
	}

	if retrieved.Status != "active" {
		t.Errorf("Expected status 'active', got '%s'", retrieved.Status)
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()

	// Test getting non-existent agent
	agent := registry.Get("non-existent")
	if agent != nil {
		t.Error("Expected nil for non-existent agent")
	}

	// Register and get agent
	info := &Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://localhost:8080",
		LastSeen:     time.Now(),
	}

	registry.Register(info)
	retrieved := registry.Get("agent-1")

	if retrieved == nil {
		t.Fatal("Failed to retrieve registered agent")
	}

	if retrieved.ID != info.ID {
		t.Errorf("Expected ID '%s', got '%s'", info.ID, retrieved.ID)
	}
}

func TestRegistry_UpdateLastSeen(t *testing.T) {
	registry := NewRegistry()

	agent := &Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://localhost:8080",
		LastSeen:     time.Now().Add(-time.Hour),
	}

	registry.Register(agent)

	newTime := time.Now()
	registry.UpdateLastSeen("agent-1", newTime)

	retrieved := registry.Get("agent-1")
	if retrieved == nil {
		t.Fatal("Failed to retrieve agent")
	}

	if !retrieved.LastSeen.Equal(newTime) {
		t.Errorf("LastSeen not updated correctly")
	}

	// Test updating non-existent agent
	registry.UpdateLastSeen("non-existent", newTime)
	// Should not panic or cause issues
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	// Test empty list
	agents := registry.List()
	if len(agents) != 0 {
		t.Errorf("Expected empty list, got %d agents", len(agents))
	}

	// Add agents
	agent1 := &Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://localhost:8080",
		LastSeen:     time.Now(),
	}

	agent2 := &Info{
		ID:           "agent-2",
		DatacenterID: "dc-2",
		Endpoint:     "http://localhost:8081",
		LastSeen:     time.Now(),
	}

	registry.Register(agent1)
	registry.Register(agent2)

	agents = registry.List()
	if len(agents) != 2 {
		t.Errorf("Expected 2 agents, got %d", len(agents))
	}

	// Verify copies are returned (not original pointers)
	for _, agent := range agents {
		original := registry.Get(agent.ID)
		if agent == original {
			t.Error("List returned original pointer instead of copy")
		}
	}
}

func TestRegistry_Count(t *testing.T) {
	registry := NewRegistry()

	if registry.Count() != 0 {
		t.Errorf("Expected count 0, got %d", registry.Count())
	}

	registry.Register(&Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://localhost:8080",
		LastSeen:     time.Now(),
	})

	if registry.Count() != 1 {
		t.Errorf("Expected count 1, got %d", registry.Count())
	}

	registry.Register(&Info{
		ID:           "agent-2",
		DatacenterID: "dc-2",
		Endpoint:     "http://localhost:8081",
		LastSeen:     time.Now(),
	})

	if registry.Count() != 2 {
		t.Errorf("Expected count 2, got %d", registry.Count())
	}
}

func TestRegistry_Remove(t *testing.T) {
	registry := NewRegistry()

	agent := &Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://localhost:8080",
		LastSeen:     time.Now(),
	}

	registry.Register(agent)

	if registry.Count() != 1 {
		t.Errorf("Expected count 1, got %d", registry.Count())
	}

	registry.Remove("agent-1")

	if registry.Count() != 0 {
		t.Errorf("Expected count 0, got %d", registry.Count())
	}

	retrieved := registry.Get("agent-1")
	if retrieved != nil {
		t.Error("Agent should have been removed")
	}

	// Test removing non-existent agent
	registry.Remove("non-existent")
	// Should not panic
}

func TestRegistry_Cleanup(t *testing.T) {
	registry := NewRegistry()

	now := time.Now()

	// Fresh agent
	freshAgent := &Info{
		ID:           "fresh-agent",
		DatacenterID: "dc-1",
		Endpoint:     "http://localhost:8080",
		LastSeen:     now.Add(-time.Minute),
	}

	// Stale agent
	staleAgent := &Info{
		ID:           "stale-agent",
		DatacenterID: "dc-2",
		Endpoint:     "http://localhost:8081",
		LastSeen:     now.Add(-2 * time.Hour),
	}

	registry.Register(freshAgent)
	registry.Register(staleAgent)

	// Cleanup with 1 hour threshold
	registry.Cleanup(time.Hour)

	// Check fresh agent is still active
	fresh := registry.Get("fresh-agent")
	if fresh == nil || fresh.Status != "active" {
		t.Error("Fresh agent should remain active")
	}

	// Check stale agent is marked as stale
	stale := registry.Get("stale-agent")
	if stale == nil || stale.Status != "stale" {
		t.Error("Stale agent should be marked as stale")
	}
}

func TestRegistry_GetByDatacenter(t *testing.T) {
	registry := NewRegistry()

	agent1 := &Info{
		ID:           "agent-1",
		DatacenterID: "dc-1",
		Endpoint:     "http://localhost:8080",
		LastSeen:     time.Now(),
	}

	agent2 := &Info{
		ID:           "agent-2",
		DatacenterID: "dc-1",
		Endpoint:     "http://localhost:8081",
		LastSeen:     time.Now(),
	}

	agent3 := &Info{
		ID:           "agent-3",
		DatacenterID: "dc-2",
		Endpoint:     "http://localhost:8082",
		LastSeen:     time.Now(),
	}

	registry.Register(agent1)
	registry.Register(agent2)
	registry.Register(agent3)

	// Test datacenter 1
	dc1Agents := registry.GetByDatacenter("dc-1")
	if len(dc1Agents) != 2 {
		t.Errorf("Expected 2 agents in dc-1, got %d", len(dc1Agents))
	}

	// Test datacenter 2
	dc2Agents := registry.GetByDatacenter("dc-2")
	if len(dc2Agents) != 1 {
		t.Errorf("Expected 1 agent in dc-2, got %d", len(dc2Agents))
	}

	// Test non-existent datacenter
	dcNoneAgents := registry.GetByDatacenter("dc-none")
	if len(dcNoneAgents) != 0 {
		t.Errorf("Expected 0 agents in dc-none, got %d", len(dcNoneAgents))
	}

	// Verify copies are returned
	for _, agent := range dc1Agents {
		original := registry.Get(agent.ID)
		if agent == original {
			t.Error("GetByDatacenter returned original pointer instead of copy")
		}
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewRegistry()

	// Test concurrent registration and access
	done := make(chan bool, 2)

	// Goroutine 1: Register agents
	go func() {
		for i := 0; i < 100; i++ {
			agent := &Info{
				ID:           fmt.Sprintf("agent-%d", i),
				DatacenterID: "dc-1",
				Endpoint:     fmt.Sprintf("http://localhost:%d", 8080+i),
				LastSeen:     time.Now(),
			}
			registry.Register(agent)
		}
		done <- true
	}()

	// Goroutine 2: Read agents
	go func() {
		for i := 0; i < 100; i++ {
			registry.Count()
			registry.List()
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	if registry.Count() != 100 {
		t.Errorf("Expected 100 agents, got %d", registry.Count())
	}
}

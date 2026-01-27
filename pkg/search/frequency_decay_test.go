package search

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/dan-solli/gognee/pkg/store"
)

// TestCalculateHeatMultiplier tests the heat multiplier calculation (M2: Plan 021)
func TestCalculateHeatMultiplier(t *testing.T) {
	mockMemoryStore := &MockMemoryStore{}
	mockGraphStore := &MockGraphStore{}
	mockSearcher := &MockSearcher{}

	// Create searcher with access frequency enabled, reference count = 10
	d := NewDecayingSearcher(mockSearcher, mockGraphStore, mockMemoryStore, true, 30, "access", true, 10)

	tests := []struct {
		name        string
		accessCount int
		wantMin     float64
		wantMax     float64
	}{
		{"zero accesses", 0, 0.0, 0.0},
		{"one access", 1, 0.25, 0.35},       // log(2) / log(11) ≈ 0.289
		{"five accesses", 5, 0.65, 0.75},    // log(6) / log(11) ≈ 0.747
		{"ten accesses", 10, 0.95, 1.0},     // log(11) / log(11) = 1.0
		{"twenty accesses", 20, 1.0, 1.0},   // Capped at 1.0
		{"hundred accesses", 100, 1.0, 1.0}, // Capped at 1.0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.calculateHeatMultiplier(tt.accessCount)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateHeatMultiplier(%d) = %.3f, want between %.3f and %.3f",
					tt.accessCount, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestFrequencyDecay_Disabled tests that frequency decay is disabled by default
func TestFrequencyDecay_Disabled(t *testing.T) {
	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour) // 30 days ago (1 half-life)

	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{NodeID: "node1", Score: 1.0},
		},
	}

	mockGraphStore := &MockGraphStore{
		Nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Old", CreatedAt: old, LastAccessedAt: &old},
		},
	}

	mockMemoryStore := &MockMemoryStore{
		Memories: map[string]*store.MemoryRecord{
			"mem1": {ID: "mem1", AccessCount: 100}, // High access count
		},
		NodeToMemories: map[string][]string{
			"node1": {"mem1"},
		},
	}

	// Frequency decay DISABLED (accessFrequencyEnabled=false)
	d := NewDecayingSearcher(mockSearcher, mockGraphStore, mockMemoryStore, true, 30, "access", false, 10)

	ctx := context.Background()
	results, err := d.Search(ctx, "test", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Expected: time decay only (30 days = 1 half-life = 0.5 multiplier)
	// NO frequency boost because it's disabled
	expectedScore := 0.5
	if math.Abs(results[0].Score-expectedScore) > 0.02 {
		t.Errorf("Score with frequency disabled: got %.3f, want ~%.2f (time decay only)",
			results[0].Score, expectedScore)
	}
}

// TestFrequencyDecay_Enabled tests frequency-based heat multiplier
func TestFrequencyDecay_Enabled(t *testing.T) {
	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour) // 30 days ago (1 half-life)

	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{NodeID: "node1", Score: 1.0}, // Zero accesses
			{NodeID: "node2", Score: 1.0}, // 10 accesses (reference count)
		},
	}

	mockGraphStore := &MockGraphStore{
		Nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Unused", CreatedAt: old, LastAccessedAt: &old},
			"node2": {ID: "node2", Name: "Frequently Used", CreatedAt: old, LastAccessedAt: &old},
		},
	}

	mockMemoryStore := &MockMemoryStore{
		Memories: map[string]*store.MemoryRecord{
			"mem1": {ID: "mem1", AccessCount: 0},  // Zero accesses
			"mem2": {ID: "mem2", AccessCount: 10}, // Reference count accesses
		},
		NodeToMemories: map[string][]string{
			"node1": {"mem1"},
			"node2": {"mem2"},
		},
	}

	// Frequency decay ENABLED, reference count = 10
	d := NewDecayingSearcher(mockSearcher, mockGraphStore, mockMemoryStore, true, 30, "access", true, 10)

	ctx := context.Background()
	results, err := d.Search(ctx, "test", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Find node1 and node2 in results
	var node1Score, node2Score float64
	for _, r := range results {
		if r.NodeID == "node1" {
			node1Score = r.Score
		} else if r.NodeID == "node2" {
			node2Score = r.Score
		}
	}

	// Node1 (zero accesses):
	// time_decay = 0.5 (1 half-life)
	// heat_multiplier = 0.0 (log(0+1) / log(10+1) = 0)
	// final = 1.0 × 0.5 × (0.5 + 0.5×0.0) = 0.5 × 0.5 = 0.25
	expectedNode1 := 0.25
	if math.Abs(node1Score-expectedNode1) > 0.02 {
		t.Errorf("Node1 (zero accesses) score: got %.3f, want ~%.2f", node1Score, expectedNode1)
	}

	// Node2 (10 accesses = reference count):
	// time_decay = 0.5 (1 half-life)
	// heat_multiplier = 1.0 (log(10+1) / log(10+1) = 1.0)
	// final = 1.0 × 0.5 × (0.5 + 0.5×1.0) = 0.5 × 1.0 = 0.5
	expectedNode2 := 0.5
	if math.Abs(node2Score-expectedNode2) > 0.02 {
		t.Errorf("Node2 (reference accesses) score: got %.3f, want ~%.2f", node2Score, expectedNode2)
	}

	// Node2 should score HIGHER than Node1 (frequency boost)
	if node2Score <= node1Score {
		t.Errorf("High-access node should score higher: node2=%.3f vs node1=%.3f", node2Score, node1Score)
	}
}

// TestFrequencyDecay_MultipleMemories tests handling multiple memories per node
func TestFrequencyDecay_MultipleMemories(t *testing.T) {
	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour)

	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{NodeID: "node1", Score: 1.0},
		},
	}

	mockGraphStore := &MockGraphStore{
		Nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Multi-memory node", CreatedAt: old, LastAccessedAt: &old},
		},
	}

	// Node1 belongs to TWO memories: one with 5 accesses, one with 20 accesses
	// Should use the MAX (20 accesses)
	mockMemoryStore := &MockMemoryStore{
		Memories: map[string]*store.MemoryRecord{
			"mem1": {ID: "mem1", AccessCount: 5},
			"mem2": {ID: "mem2", AccessCount: 20}, // MAX (should be used)
		},
		NodeToMemories: map[string][]string{
			"node1": {"mem1", "mem2"},
		},
	}

	d := NewDecayingSearcher(mockSearcher, mockGraphStore, mockMemoryStore, true, 30, "access", true, 10)

	ctx := context.Background()
	results, err := d.Search(ctx, "test", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should use 20 accesses (the maximum)
	// heat_multiplier = min(1.0, log(21) / log(11)) = 1.0 (capped)
	// final = 1.0 × 0.5 × (0.5 + 0.5×1.0) = 0.5
	expectedScore := 0.5
	if math.Abs(results[0].Score-expectedScore) > 0.02 {
		t.Errorf("Multi-memory node score: got %.3f, want ~%.2f (should use max access count)",
			results[0].Score, expectedScore)
	}
}

// TestFrequencyDecay_NoMemory tests handling nodes with no associated memory
func TestFrequencyDecay_NoMemory(t *testing.T) {
	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour)

	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{NodeID: "node1", Score: 1.0},
		},
	}

	mockGraphStore := &MockGraphStore{
		Nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Orphan node", CreatedAt: old, LastAccessedAt: &old},
		},
	}

	// No memory for node1
	mockMemoryStore := &MockMemoryStore{
		Memories:       map[string]*store.MemoryRecord{},
		NodeToMemories: map[string][]string{},
	}

	d := NewDecayingSearcher(mockSearcher, mockGraphStore, mockMemoryStore, true, 30, "access", true, 10)

	ctx := context.Background()
	results, err := d.Search(ctx, "test", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// No memory found → fallback to time decay only
	// time_decay = 0.5, no frequency adjustment
	expectedScore := 0.5
	if math.Abs(results[0].Score-expectedScore) > 0.02 {
		t.Errorf("No-memory node score: got %.3f, want ~%.2f (time decay only)",
			results[0].Score, expectedScore)
	}
}

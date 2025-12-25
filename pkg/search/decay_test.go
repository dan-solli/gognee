package search

import (
	"context"
	"testing"
	"time"

	"github.com/dan-solli/gognee/pkg/store"
)

// MockSearcher implements Searcher for testing decay wrapper
type MockSearcher struct {
	Results []SearchResult
	Error   error
}

func (m *MockSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Results, nil
}

// MockGraphStore implements store.GraphStore for testing (minimal implementation)
type MockGraphStore struct {
	Nodes map[string]*store.Node
}

func (m *MockGraphStore) GetNode(ctx context.Context, id string) (*store.Node, error) {
	return m.Nodes[id], nil
}

// Stub methods to satisfy interface
func (m *MockGraphStore) AddNode(ctx context.Context, node *store.Node) error { return nil }
func (m *MockGraphStore) FindNodesByName(ctx context.Context, name string) ([]*store.Node, error) {
	return nil, nil
}
func (m *MockGraphStore) FindNodeByName(ctx context.Context, name string) (*store.Node, error) {
	return nil, nil
}
func (m *MockGraphStore) AddEdge(ctx context.Context, edge *store.Edge) error { return nil }
func (m *MockGraphStore) GetEdges(ctx context.Context, nodeID string) ([]*store.Edge, error) {
	return nil, nil
}
func (m *MockGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int) ([]*store.Node, error) {
	return nil, nil
}
func (m *MockGraphStore) NodeCount(ctx context.Context) (int64, error) { return 0, nil }
func (m *MockGraphStore) EdgeCount(ctx context.Context) (int64, error) { return 0, nil }
func (m *MockGraphStore) Close() error                                 { return nil }

func TestDecayingSearcher_DecayDisabled(t *testing.T) {
	now := time.Now()
	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{NodeID: "node1", Score: 0.9},
			{NodeID: "node2", Score: 0.5},
		},
	}

	mockGraphStore := &MockGraphStore{
		Nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Node 1", CreatedAt: now},
			"node2": {ID: "node2", Name: "Node 2", CreatedAt: now},
		},
	}

	// Decay disabled
	decaySearcher := NewDecayingSearcher(mockSearcher, mockGraphStore, false, 30, "access")

	ctx := context.Background()
	results, err := decaySearcher.Search(ctx, "test query", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Scores should be unchanged when decay is disabled
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
	if results[0].Score != 0.9 {
		t.Errorf("Result 0 score: got %.2f, want 0.9", results[0].Score)
	}
	if results[1].Score != 0.5 {
		t.Errorf("Result 1 score: got %.2f, want 0.5", results[1].Score)
	}
}

func TestDecayingSearcher_WithAccessBasedDecay(t *testing.T) {
	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour) // 30 days ago (1 half-life)

	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{NodeID: "node1", Score: 1.0}, // Recent, should have minimal decay
			{NodeID: "node2", Score: 1.0}, // Old, should have 0.5 multiplier
		},
	}

	mockGraphStore := &MockGraphStore{
		Nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Recent", CreatedAt: now, LastAccessedAt: &now},
			"node2": {ID: "node2", Name: "Old", CreatedAt: old, LastAccessedAt: &old},
		},
	}

	// Decay enabled with 30-day half-life, access-based
	decaySearcher := NewDecayingSearcher(mockSearcher, mockGraphStore, true, 30, "access")

	ctx := context.Background()
	results, err := decaySearcher.Search(ctx, "test query", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Node1 (recent) should have score close to 1.0
	if results[0].NodeID == "node1" && results[0].Score < 0.99 {
		t.Errorf("Recent node score: got %.6f, want ~1.0", results[0].Score)
	}

	// Node2 (30 days old) should have score ~0.5 (half-life decay)
	if results[1].NodeID == "node2" {
		expectedScore := 0.5 // 1.0 * 0.5 (decay multiplier at 1 half-life)
		if results[1].Score < 0.48 || results[1].Score > 0.52 {
			t.Errorf("Old node score: got %.6f, want ~%.2f", results[1].Score, expectedScore)
		}
	}
}

func TestDecayingSearcher_WithCreationBasedDecay(t *testing.T) {
	now := time.Now()
	old := now.Add(-60 * 24 * time.Hour) // 60 days ago (2 half-lives)

	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{NodeID: "node1", Score: 1.0},
		},
	}

	mockGraphStore := &MockGraphStore{
		Nodes: map[string]*store.Node{
			"node1": {
				ID:        "node1",
				Name:      "Old but accessed",
				CreatedAt: old,
				// No LastAccessedAt - should fall back to creation time
			},
		},
	}

	// Decay enabled with 30-day half-life, creation-based
	decaySearcher := NewDecayingSearcher(mockSearcher, mockGraphStore, true, 30, "creation")

	ctx := context.Background()
	results, err := decaySearcher.Search(ctx, "test query", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// 60 days / 30 half-life = 2 half-lives = 0.5^2 = 0.25
	expectedScore := 0.25
	if results[0].Score < 0.23 || results[0].Score > 0.27 {
		t.Errorf("Old node score: got %.6f, want ~%.2f", results[0].Score, expectedScore)
	}
}

func TestDecayingSearcher_FallbackToCreationTime(t *testing.T) {
	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour)

	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{NodeID: "node1", Score: 1.0},
		},
	}

	mockGraphStore := &MockGraphStore{
		Nodes: map[string]*store.Node{
			"node1": {
				ID:        "node1",
				Name:      "Never accessed",
				CreatedAt: old,
				// LastAccessedAt is nil - should fall back to CreatedAt
			},
		},
	}

	// Decay enabled, access-based but node never accessed
	decaySearcher := NewDecayingSearcher(mockSearcher, mockGraphStore, true, 30, "access")

	ctx := context.Background()
	results, err := decaySearcher.Search(ctx, "test query", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Should fall back to creation time (30 days = 1 half-life = 0.5 multiplier)
	expectedScore := 0.5
	if results[0].Score < 0.48 || results[0].Score > 0.52 {
		t.Errorf("Never-accessed node score: got %.6f, want ~%.2f", results[0].Score, expectedScore)
	}
}

func TestDecayingSearcher_MinimumThreshold(t *testing.T) {
	now := time.Now()
	veryOld := now.Add(-300 * 24 * time.Hour) // 300 days ago (10 half-lives)

	mockSearcher := &MockSearcher{
		Results: []SearchResult{
			{NodeID: "node1", Score: 1.0},
		},
	}

	mockGraphStore := &MockGraphStore{
		Nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Very old", CreatedAt: veryOld, LastAccessedAt: &veryOld},
		},
	}

	decaySearcher := NewDecayingSearcher(mockSearcher, mockGraphStore, true, 30, "access")

	ctx := context.Background()
	results, err := decaySearcher.Search(ctx, "test query", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Very old nodes may be filtered if score drops below threshold
	// For now, just verify it doesn't crash and score is very low
	if len(results) > 0 && results[0].Score > 0.01 {
		t.Errorf("Very old node score: got %.6f, want < 0.01", results[0].Score)
	}
}

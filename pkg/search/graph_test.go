package search

import (
	"context"
	"errors"
	"testing"

	"github.com/dan-solli/gognee/pkg/store"
)

// Tests for GraphSearcher

func TestGraphSearcher_SingleSeedDepth1(t *testing.T) {
	ctx := context.Background()

	graphStore := &mockGraphStore{
		nodes: map[string]*store.Node{
			"seed1":     {ID: "seed1", Name: "Seed", Type: "Test"},
			"neighbor1": {ID: "neighbor1", Name: "Neighbor1", Type: "Test"},
			"neighbor2": {ID: "neighbor2", Name: "Neighbor2", Type: "Test"},
		},
	}

	neighborMap := map[string][]*store.Node{
		"seed1": {
			graphStore.nodes["neighbor1"],
			graphStore.nodes["neighbor2"],
		},
	}

	// Create a custom mock with GetNeighbors
	customGraphStore := &testGraphStore{
		nodes:     graphStore.nodes,
		neighbors: neighborMap,
	}

	searcher := NewGraphSearcher(customGraphStore)

	results, err := searcher.Search(ctx, "", SearchOptions{
		SeedNodeIDs: []string{"seed1"},
		GraphDepth:  1,
		TopK:        10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should include seed (depth=0, score=1.0) + neighbors (depth=1, score=0.5)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Check seed node
	found := false
	for _, r := range results {
		if r.NodeID == "seed1" {
			found = true
			if r.Score != 1.0 {
				t.Errorf("Seed score should be 1.0, got %f", r.Score)
			}
			if r.GraphDepth != 0 {
				t.Errorf("Seed depth should be 0, got %d", r.GraphDepth)
			}
			if r.Source != "graph" {
				t.Errorf("Expected source 'graph', got %s", r.Source)
			}
		}
	}
	if !found {
		t.Error("Seed node not found in results")
	}

	// Check neighbor scoring
	for _, r := range results {
		if r.NodeID == "neighbor1" || r.NodeID == "neighbor2" {
			expectedScore := 1.0 / (1 + 1) // depth=1
			if r.Score != expectedScore {
				t.Errorf("Neighbor score should be %f, got %f", expectedScore, r.Score)
			}
			if r.GraphDepth != 1 {
				t.Errorf("Neighbor depth should be 1, got %d", r.GraphDepth)
			}
		}
	}
}

func TestGraphSearcher_MultipleSeeds(t *testing.T) {
	ctx := context.Background()

	customGraphStore := &testGraphStore{
		nodes: map[string]*store.Node{
			"seed1":     {ID: "seed1", Name: "Seed1", Type: "Test"},
			"seed2":     {ID: "seed2", Name: "Seed2", Type: "Test"},
			"neighbor1": {ID: "neighbor1", Name: "Neighbor1", Type: "Test"},
		},
		neighbors: map[string][]*store.Node{
			"seed1": {{ID: "neighbor1", Name: "Neighbor1", Type: "Test"}},
			"seed2": {{ID: "neighbor1", Name: "Neighbor1", Type: "Test"}},
		},
	}

	searcher := NewGraphSearcher(customGraphStore)

	results, err := searcher.Search(ctx, "", SearchOptions{
		SeedNodeIDs: []string{"seed1", "seed2"},
		GraphDepth:  1,
		TopK:        10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should include 2 seeds + 1 neighbor (deduplicated)
	if len(results) != 3 {
		t.Errorf("Expected 3 results (deduplicated), got %d", len(results))
	}

	// Neighbor1 should have best score from either seed path
	for _, r := range results {
		if r.NodeID == "neighbor1" {
			expectedScore := 0.5 // 1/(1+1)
			if r.Score != expectedScore {
				t.Errorf("Neighbor score should be %f, got %f", expectedScore, r.Score)
			}
		}
	}
}

func TestGraphSearcher_Depth2(t *testing.T) {
	ctx := context.Background()

	customGraphStore := &testGraphStore{
		nodes: map[string]*store.Node{
			"seed1": {ID: "seed1", Name: "Seed", Type: "Test"},
			"n1":    {ID: "n1", Name: "N1", Type: "Test"},
			"n2":    {ID: "n2", Name: "N2", Type: "Test"},
		},
		neighbors: map[string][]*store.Node{
			"seed1": {{ID: "n1", Name: "N1", Type: "Test"}},
			"n1":    {{ID: "n2", Name: "N2", Type: "Test"}},
		},
	}

	searcher := NewGraphSearcher(customGraphStore)

	results, err := searcher.Search(ctx, "", SearchOptions{
		SeedNodeIDs: []string{"seed1"},
		GraphDepth:  2,
		TopK:        10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should include seed + n1 (depth 1) + n2 (depth 2)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	for _, r := range results {
		if r.NodeID == "n2" {
			expectedScore := 1.0 / (1 + 2) // depth=2
			if r.Score != expectedScore {
				t.Errorf("Depth-2 node score should be %f, got %f", expectedScore, r.Score)
			}
			if r.GraphDepth != 2 {
				t.Errorf("Expected GraphDepth 2, got %d", r.GraphDepth)
			}
		}
	}
}

func TestGraphSearcher_Deduplication(t *testing.T) {
	ctx := context.Background()

	// Graph where two paths lead to same node at different depths
	customGraphStore := &testGraphStore{
		nodes: map[string]*store.Node{
			"seed":   {ID: "seed", Name: "Seed", Type: "Test"},
			"middle": {ID: "middle", Name: "Middle", Type: "Test"},
			"target": {ID: "target", Name: "Target", Type: "Test"},
		},
		neighbors: map[string][]*store.Node{
			"seed": {
				{ID: "middle", Name: "Middle", Type: "Test"},
				{ID: "target", Name: "Target", Type: "Test"}, // Direct path
			},
			"middle": {
				{ID: "target", Name: "Target", Type: "Test"}, // Indirect path
			},
		},
	}

	searcher := NewGraphSearcher(customGraphStore)

	results, err := searcher.Search(ctx, "", SearchOptions{
		SeedNodeIDs: []string{"seed"},
		GraphDepth:  2,
		TopK:        10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should deduplicate target node
	targetCount := 0
	var targetResult SearchResult
	for _, r := range results {
		if r.NodeID == "target" {
			targetCount++
			targetResult = r
		}
	}

	if targetCount != 1 {
		t.Errorf("Target should appear once (deduplicated), appeared %d times", targetCount)
	}

	// Should keep the best score (shortest path = depth 1)
	expectedScore := 1.0 / (1 + 1) // depth=1
	if targetResult.Score != expectedScore {
		t.Errorf("Expected best score %f for deduplicated node, got %f", expectedScore, targetResult.Score)
	}
	if targetResult.GraphDepth != 1 {
		t.Errorf("Expected GraphDepth 1 (shortest path), got %d", targetResult.GraphDepth)
	}
}

func TestGraphSearcher_ScoreDecay(t *testing.T) {
	ctx := context.Background()

	customGraphStore := &testGraphStore{
		nodes: map[string]*store.Node{
			"seed": {ID: "seed", Name: "Seed", Type: "Test"},
			"d0":   {ID: "d0", Name: "D0", Type: "Test"},
			"d1":   {ID: "d1", Name: "D1", Type: "Test"},
			"d2":   {ID: "d2", Name: "D2", Type: "Test"},
		},
		neighbors: map[string][]*store.Node{
			"seed": {{ID: "d1", Name: "D1", Type: "Test"}},
			"d1":   {{ID: "d2", Name: "D2", Type: "Test"}},
		},
	}

	searcher := NewGraphSearcher(customGraphStore)

	results, err := searcher.Search(ctx, "", SearchOptions{
		SeedNodeIDs: []string{"seed"},
		GraphDepth:  2,
		TopK:        10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify score decreases with depth
	scoreMap := make(map[string]float64)
	for _, r := range results {
		scoreMap[r.NodeID] = r.Score
	}

	if scoreMap["seed"] <= scoreMap["d1"] {
		t.Errorf("Seed score (%f) should be > depth-1 score (%f)", scoreMap["seed"], scoreMap["d1"])
	}
	if scoreMap["d1"] <= scoreMap["d2"] {
		t.Errorf("Depth-1 score (%f) should be > depth-2 score (%f)", scoreMap["d1"], scoreMap["d2"])
	}
}

func TestGraphSearcher_EmptySeeds(t *testing.T) {
	ctx := context.Background()

	graphStore := &testGraphStore{
		nodes: map[string]*store.Node{},
	}

	searcher := NewGraphSearcher(graphStore)

	_, err := searcher.Search(ctx, "", SearchOptions{
		SeedNodeIDs: []string{},
		GraphDepth:  1,
		TopK:        10,
	})

	if err == nil {
		t.Error("Expected error for empty seeds, got nil")
	}
}

// Helper mock with GetNeighbors support

type testGraphStore struct {
	nodes     map[string]*store.Node
	neighbors map[string][]*store.Node
}

func (t *testGraphStore) AddNode(ctx context.Context, node *store.Node) error {
	if t.nodes == nil {
		t.nodes = make(map[string]*store.Node)
	}
	t.nodes[node.ID] = node
	return nil
}

func (t *testGraphStore) GetNode(ctx context.Context, id string) (*store.Node, error) {
	if t.nodes == nil {
		return nil, nil
	}
	return t.nodes[id], nil
}

func (t *testGraphStore) FindNodesByName(ctx context.Context, name string) ([]*store.Node, error) {
	return nil, nil
}

func (t *testGraphStore) FindNodeByName(ctx context.Context, name string) (*store.Node, error) {
	return nil, store.ErrNodeNotFound
}

func (t *testGraphStore) AddEdge(ctx context.Context, edge *store.Edge) error {
	return nil
}

func (t *testGraphStore) GetEdges(ctx context.Context, nodeID string) ([]*store.Edge, error) {
	return nil, nil
}

func (t *testGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int) ([]*store.Node, error) {
	if depth == 1 {
		return t.neighbors[nodeID], nil
	}

	// For depth > 1, recursive traversal
	visited := make(map[string]bool)
	var result []*store.Node

	var traverse func(id string, currentDepth int)
	traverse = func(id string, currentDepth int) {
		if currentDepth > depth {
			return
		}
		neighbors := t.neighbors[id]
		for _, n := range neighbors {
			if !visited[n.ID] {
				visited[n.ID] = true
				result = append(result, n)
				traverse(n.ID, currentDepth+1)
			}
		}
	}

	traverse(nodeID, 1)
	return result, nil
}

func (t *testGraphStore) NodeCount(ctx context.Context) (int64, error) {
	return int64(len(t.nodes)), nil
}

func (t *testGraphStore) EdgeCount(ctx context.Context) (int64, error) {
	return 0, nil
}

func (t *testGraphStore) Close() error {
	return nil
}

var errNoSeeds = errors.New("graph search requires seed node IDs")

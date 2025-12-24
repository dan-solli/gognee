package search

import (
	"context"
	"testing"

	"github.com/dan-solli/gognee/pkg/store"
)

// Tests for HybridSearcher

func TestHybridSearcher_VectorPlusGraph(t *testing.T) {
	ctx := context.Background()

	// Setup: 3 nodes, vector search finds node1 and node2, node3 is neighbor of node1
	graphStore := &testGraphStore{
		nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "React", Type: "Tech"},
			"node2": {ID: "node2", Name: "TypeScript", Type: "Tech"},
			"node3": {ID: "node3", Name: "JSX", Type: "Tech"},
		},
		neighbors: map[string][]*store.Node{
			"node1": {{ID: "node3", Name: "JSX", Type: "Tech"}},
		},
	}

	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			return []store.SearchResult{
				{ID: "node1", Score: 0.8},
				{ID: "node2", Score: 0.6},
			}, nil
		},
	}

	embClient := &mockEmbeddingClient{}

	searcher := NewHybridSearcher(embClient, vectorStore, graphStore)

	results, err := searcher.Search(ctx, "frontend tech", SearchOptions{
		TopK:       10,
		GraphDepth: 1,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should have: node1 (vector), node2 (vector), node3 (graph expansion from node1)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Check sources
	sourceMap := make(map[string]string)
	scoreMap := make(map[string]float64)
	for _, r := range results {
		sourceMap[r.NodeID] = r.Source
		scoreMap[r.NodeID] = r.Score
	}

	if sourceMap["node1"] != "vector" {
		t.Errorf("node1 should be source 'vector', got %s", sourceMap["node1"])
	}
	if sourceMap["node2"] != "vector" {
		t.Errorf("node2 should be source 'vector', got %s", sourceMap["node2"])
	}
	if sourceMap["node3"] != "graph" {
		t.Errorf("node3 should be source 'graph', got %s", sourceMap["node3"])
	}

	// node3 score should be graph-only: 1/(1+1) = 0.5
	expectedNode3Score := 0.5
	if scoreMap["node3"] != expectedNode3Score {
		t.Errorf("node3 score should be %f (graph only), got %f", expectedNode3Score, scoreMap["node3"])
	}
}

func TestHybridSearcher_NodeFoundByBoth(t *testing.T) {
	ctx := context.Background()

	// Setup: node2 is both a vector hit AND a neighbor of node1
	graphStore := &testGraphStore{
		nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "React", Type: "Tech"},
			"node2": {ID: "node2", Name: "TypeScript", Type: "Tech"},
		},
		neighbors: map[string][]*store.Node{
			"node1": {{ID: "node2", Name: "TypeScript", Type: "Tech"}},
		},
	}

	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			return []store.SearchResult{
				{ID: "node1", Score: 0.8},
				{ID: "node2", Score: 0.6}, // Also a neighbor of node1
			}, nil
		},
	}

	embClient := &mockEmbeddingClient{}
	searcher := NewHybridSearcher(embClient, vectorStore, graphStore)

	results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10, GraphDepth: 1})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Find node2
	var node2Result *SearchResult
	for _, r := range results {
		if r.NodeID == "node2" {
			node2Result = &r
			break
		}
	}

	if node2Result == nil {
		t.Fatal("node2 not found in results")
	}

	// node2 should have Source="hybrid" (found by both)
	if node2Result.Source != "hybrid" {
		t.Errorf("node2 should have source 'hybrid', got %s", node2Result.Source)
	}

	// node2 score should be boosted: vector_score (0.6) + graph_score (0.5) = 1.1
	expectedScore := 0.6 + 0.5
	if node2Result.Score != expectedScore {
		t.Errorf("node2 score should be %f (vector + graph), got %f", expectedScore, node2Result.Score)
	}
}

func TestHybridSearcher_VectorOnlyNode(t *testing.T) {
	ctx := context.Background()

	graphStore := &testGraphStore{
		nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Isolated", Type: "Tech"},
		},
		neighbors: map[string][]*store.Node{}, // No neighbors
	}

	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			return []store.SearchResult{
				{ID: "node1", Score: 0.7},
			}, nil
		},
	}

	embClient := &mockEmbeddingClient{}
	searcher := NewHybridSearcher(embClient, vectorStore, graphStore)

	results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10, GraphDepth: 1})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// node1 should be vector-only
	if results[0].Source != "vector" {
		t.Errorf("Expected source 'vector', got %s", results[0].Source)
	}
	if results[0].Score != 0.7 {
		t.Errorf("Expected score 0.7 (vector only), got %f", results[0].Score)
	}
}

func TestHybridSearcher_GraphOnlyNode(t *testing.T) {
	ctx := context.Background()

	graphStore := &testGraphStore{
		nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Primary", Type: "Tech"},
			"node2": {ID: "node2", Name: "Secondary", Type: "Tech"},
		},
		neighbors: map[string][]*store.Node{
			"node1": {{ID: "node2", Name: "Secondary", Type: "Tech"}},
		},
	}

	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			// Only node1 from vector search
			return []store.SearchResult{
				{ID: "node1", Score: 0.8},
			}, nil
		},
	}

	embClient := &mockEmbeddingClient{}
	searcher := NewHybridSearcher(embClient, vectorStore, graphStore)

	results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10, GraphDepth: 1})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Find node2
	var node2Result *SearchResult
	for _, r := range results {
		if r.NodeID == "node2" {
			node2Result = &r
			break
		}
	}

	if node2Result == nil {
		t.Fatal("node2 not found (should be discovered via graph)")
	}

	// node2 should be graph-only
	if node2Result.Source != "graph" {
		t.Errorf("node2 should have source 'graph', got %s", node2Result.Source)
	}

	expectedScore := 1.0 / (1 + 1) // depth=1
	if node2Result.Score != expectedScore {
		t.Errorf("node2 score should be %f (graph only), got %f", expectedScore, node2Result.Score)
	}
}

func TestHybridSearcher_TopKLimiting(t *testing.T) {
	ctx := context.Background()

	nodes := make(map[string]*store.Node)
	neighbors := make(map[string][]*store.Node)

	// Create nodes first
	for i := 1; i <= 5; i++ {
		id := "node" + string(rune('0'+i))
		nodes[id] = &store.Node{ID: id, Name: "Node" + string(rune('0'+i)), Type: "Test"}
	}

	// Then setup neighbors
	for i := 1; i < 5; i++ {
		id := "node" + string(rune('0'+i))
		nextID := "node" + string(rune('0'+i+1))
		neighbors[id] = []*store.Node{nodes[nextID]}
	}

	graphStore := &testGraphStore{
		nodes:     nodes,
		neighbors: neighbors,
	}

	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			// Return a few results
			results := make([]store.SearchResult, 0, 3)
			for i := 1; i <= 3; i++ {
				id := "node" + string(rune('0'+i))
				results = append(results, store.SearchResult{
					ID:    id,
					Score: 1.0 - float64(i)*0.1,
				})
			}
			return results, nil
		},
	}

	embClient := &mockEmbeddingClient{}
	searcher := NewHybridSearcher(embClient, vectorStore, graphStore)

	results, err := searcher.Search(ctx, "test", SearchOptions{
		TopK:       3, // Limit to 3
		GraphDepth: 1,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should be limited to TopK=3
	if len(results) > 3 {
		t.Errorf("Expected at most 3 results (TopK), got %d", len(results))
	}
}

func TestHybridSearcher_GraphDepthExpansion(t *testing.T) {
	ctx := context.Background()

	graphStore := &testGraphStore{
		nodes: map[string]*store.Node{
			"n1": {ID: "n1", Name: "N1", Type: "Test"},
			"n2": {ID: "n2", Name: "N2", Type: "Test"},
			"n3": {ID: "n3", Name: "N3", Type: "Test"},
		},
		neighbors: map[string][]*store.Node{
			"n1": {{ID: "n2", Name: "N2", Type: "Test"}},
			"n2": {{ID: "n3", Name: "N3", Type: "Test"}},
		},
	}

	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			return []store.SearchResult{
				{ID: "n1", Score: 0.8},
			}, nil
		},
	}

	embClient := &mockEmbeddingClient{}
	searcher := NewHybridSearcher(embClient, vectorStore, graphStore)

	// Search with depth=2
	results, err := searcher.Search(ctx, "test", SearchOptions{
		TopK:       10,
		GraphDepth: 2,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find n1 (vector), n2 (depth 1), n3 (depth 2)
	if len(results) != 3 {
		t.Errorf("Expected 3 results with depth=2, got %d", len(results))
	}

	foundN3 := false
	for _, r := range results {
		if r.NodeID == "n3" {
			foundN3 = true
			if r.GraphDepth != 2 {
				t.Errorf("n3 should be at GraphDepth=2, got %d", r.GraphDepth)
			}
		}
	}

	if !foundN3 {
		t.Error("n3 should be discovered at depth=2")
	}
}

package search

import (
	"context"
	"testing"

	"github.com/dan-solli/gognee/pkg/store"
)

// Mock implementations for testing

type mockEmbeddingClient struct {
	embedOneFunc func(ctx context.Context, text string) ([]float32, error)
}

func (m *mockEmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.EmbedOne(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

func (m *mockEmbeddingClient) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	if m.embedOneFunc != nil {
		return m.embedOneFunc(ctx, text)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

type mockVectorStore struct {
	searchFunc func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error)
}

func (m *mockVectorStore) Add(ctx context.Context, id string, embedding []float32) error {
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, query, topK)
	}
	return []store.SearchResult{}, nil
}

func (m *mockVectorStore) Delete(ctx context.Context, id string) error {
	return nil
}

type mockGraphStore struct {
	nodes map[string]*store.Node
}

func (m *mockGraphStore) AddNode(ctx context.Context, node *store.Node) error {
	if m.nodes == nil {
		m.nodes = make(map[string]*store.Node)
	}
	m.nodes[node.ID] = node
	return nil
}

func (m *mockGraphStore) GetNode(ctx context.Context, id string) (*store.Node, error) {
	if m.nodes == nil {
		return nil, nil
	}
	return m.nodes[id], nil
}

func (m *mockGraphStore) FindNodesByName(ctx context.Context, name string) ([]*store.Node, error) {
	return nil, nil
}

func (m *mockGraphStore) FindNodeByName(ctx context.Context, name string) (*store.Node, error) {
	return nil, store.ErrNodeNotFound
}

func (m *mockGraphStore) AddEdge(ctx context.Context, edge *store.Edge) error {
	return nil
}

func (m *mockGraphStore) GetEdges(ctx context.Context, nodeID string) ([]*store.Edge, error) {
	return nil, nil
}

func (m *mockGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int) ([]*store.Node, error) {
	return nil, nil
}

func (m *mockGraphStore) Close() error {
	return nil
}

// Tests for VectorSearcher

func TestVectorSearcher_BasicSearch(t *testing.T) {
	ctx := context.Background()

	// Setup mock stores with test data
	graphStore := &mockGraphStore{
		nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "React", Type: "Technology"},
			"node2": {ID: "node2", Name: "TypeScript", Type: "Technology"},
		},
	}

	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			return []store.SearchResult{
				{ID: "node1", Score: 0.9},
				{ID: "node2", Score: 0.7},
			}, nil
		},
	}

	embClient := &mockEmbeddingClient{
		embedOneFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.5, 0.5, 0.5}, nil
		},
	}

	searcher := NewVectorSearcher(embClient, vectorStore, graphStore)

	results, err := searcher.Search(ctx, "frontend technologies", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Verify first result
	if results[0].NodeID != "node1" {
		t.Errorf("Expected node1 first, got %s", results[0].NodeID)
	}
	if results[0].Score != 0.9 {
		t.Errorf("Expected score 0.9, got %f", results[0].Score)
	}
	if results[0].Source != "vector" {
		t.Errorf("Expected source 'vector', got %s", results[0].Source)
	}
	if results[0].GraphDepth != 0 {
		t.Errorf("Expected GraphDepth 0, got %d", results[0].GraphDepth)
	}
	if results[0].Node == nil || results[0].Node.Name != "React" {
		t.Errorf("Expected node enriched with data")
	}
}

func TestVectorSearcher_HandlesStaleIndex(t *testing.T) {
	ctx := context.Background()

	// Graph store missing one of the vector results
	graphStore := &mockGraphStore{
		nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "React", Type: "Technology"},
		},
	}

	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			return []store.SearchResult{
				{ID: "node1", Score: 0.9},
				{ID: "node2", Score: 0.7}, // This node doesn't exist in graph store
			}, nil
		},
	}

	embClient := &mockEmbeddingClient{}
	searcher := NewVectorSearcher(embClient, vectorStore, graphStore)

	results, err := searcher.Search(ctx, "test query", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should skip the missing node
	if len(results) != 1 {
		t.Errorf("Expected 1 result (stale entry skipped), got %d", len(results))
	}
	if results[0].NodeID != "node1" {
		t.Errorf("Expected node1, got %s", results[0].NodeID)
	}
}

func TestVectorSearcher_EmptyResults(t *testing.T) {
	ctx := context.Background()

	graphStore := &mockGraphStore{}
	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			return []store.SearchResult{}, nil
		},
	}
	embClient := &mockEmbeddingClient{}

	searcher := NewVectorSearcher(embClient, vectorStore, graphStore)

	results, err := searcher.Search(ctx, "test query", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestVectorSearcher_ScoreOrdering(t *testing.T) {
	ctx := context.Background()

	graphStore := &mockGraphStore{
		nodes: map[string]*store.Node{
			"node1": {ID: "node1", Name: "Low", Type: "Test"},
			"node2": {ID: "node2", Name: "High", Type: "Test"},
			"node3": {ID: "node3", Name: "Medium", Type: "Test"},
		},
	}

	vectorStore := &mockVectorStore{
		searchFunc: func(ctx context.Context, query []float32, topK int) ([]store.SearchResult, error) {
			// Return out of order - searcher should rely on vector store's ordering
			return []store.SearchResult{
				{ID: "node2", Score: 0.9},
				{ID: "node3", Score: 0.6},
				{ID: "node1", Score: 0.3},
			}, nil
		},
	}

	embClient := &mockEmbeddingClient{}
	searcher := NewVectorSearcher(embClient, vectorStore, graphStore)

	results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Vector store is responsible for sorting; searcher preserves order
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}
	if results[0].NodeID != "node2" || results[0].Score != 0.9 {
		t.Errorf("Expected first result to be node2 with score 0.9, got %s with %f", results[0].NodeID, results[0].Score)
	}
	if results[1].NodeID != "node3" || results[1].Score != 0.6 {
		t.Errorf("Expected second result to be node3 with score 0.6, got %s with %f", results[1].NodeID, results[1].Score)
	}
	if results[2].NodeID != "node1" || results[2].Score != 0.3 {
		t.Errorf("Expected third result to be node1 with score 0.3, got %s with %f", results[2].NodeID, results[2].Score)
	}
}

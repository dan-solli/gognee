package search

import (
	"context"
	"testing"

	"github.com/dan-solli/gognee/pkg/store"
)

// BenchmarkHybridSearch_GraphExpansion measures the performance of hybrid search
// with realistic graph topology and depths. Validates v1.4.0 recursive CTE optimization.
func BenchmarkHybridSearch_GraphExpansion(b *testing.B) {
	ctx := context.Background()

	// Setup in-memory stores
	graphStore, err := store.NewSQLiteGraphStore(":memory:")
	if err != nil {
		b.Fatalf("Failed to create graph store: %v", err)
	}
	defer graphStore.Close()

	vectorStore := store.NewMemoryVectorStore()
	embedClient := &mockEmbeddingClient{}

	// Build a realistic graph topology:
	// Central hub node connected to 20 entities, each with 5 connections (total ~100 nodes)
	// This simulates a knowledge graph with moderate connectivity

	hub := &store.Node{
		ID:          "hub",
		Name:        "Central Concept",
		Type:        "Concept",
		Description: "A central organizing concept",
		Embedding:   make([]float32, 1536),
	}
	if err := graphStore.AddNode(ctx, hub); err != nil {
		b.Fatalf("Failed to add hub node: %v", err)
	}
	if err := vectorStore.Add(ctx, hub.ID, hub.Embedding); err != nil {
		b.Fatalf("Failed to index hub node: %v", err)
	}

	// Create 20 primary nodes connected to hub
	for i := 0; i < 20; i++ {
		primary := &store.Node{
			ID:          generateNodeID(i),
			Name:        generateNodeName(i),
			Type:        "Entity",
			Description: "A primary entity",
			Embedding:   make([]float32, 1536),
		}
		if err := graphStore.AddNode(ctx, primary); err != nil {
			b.Fatalf("Failed to add primary node %d: %v", i, err)
		}
		if err := vectorStore.Add(ctx, primary.ID, primary.Embedding); err != nil {
			b.Fatalf("Failed to index primary node %d: %v", i, err)
		}

		// Connect to hub
		edge := &store.Edge{
			ID:       generateEdgeID(hub.ID, primary.ID),
			SourceID: hub.ID,
			Relation: "RELATES_TO",
			TargetID: primary.ID,
			Weight:   1.0,
		}
		if err := graphStore.AddEdge(ctx, edge); err != nil {
			b.Fatalf("Failed to add edge to primary %d: %v", i, err)
		}

		// Create 5 secondary nodes for each primary
		for j := 0; j < 5; j++ {
			secondaryID := generateNodeID(i*100 + j)
			secondary := &store.Node{
				ID:          secondaryID,
				Name:        generateNodeName(i*100 + j),
				Type:        "Detail",
				Description: "A detailed entity",
				Embedding:   make([]float32, 1536),
			}
			if err := graphStore.AddNode(ctx, secondary); err != nil {
				b.Fatalf("Failed to add secondary node %d-%d: %v", i, j, err)
			}
			if err := vectorStore.Add(ctx, secondary.ID, secondary.Embedding); err != nil {
				b.Fatalf("Failed to index secondary node %d-%d: %v", i, j, err)
			}

			// Connect to primary
			secondaryEdge := &store.Edge{
				ID:       generateEdgeID(primary.ID, secondaryID),
				SourceID: primary.ID,
				Relation: "HAS_DETAIL",
				TargetID: secondaryID,
				Weight:   1.0,
			}
			if err := graphStore.AddEdge(ctx, secondaryEdge); err != nil {
				b.Fatalf("Failed to add edge to secondary %d-%d: %v", i, j, err)
			}
		}
	}

	// Create searcher
	hybridSearcher := NewHybridSearcher(embedClient, vectorStore, graphStore)

	// Reset timer after setup
	b.ResetTimer()

	// Benchmark search operations with depth=2, TopK=10
	for i := 0; i < b.N; i++ {
		_, err := hybridSearcher.Search(ctx, "test query", SearchOptions{
			TopK:       10,
			GraphDepth: 2,
		})
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// BenchmarkHybridSearch_ShallowGraph measures search on a shallow graph (depth 1).
func BenchmarkHybridSearch_ShallowGraph(b *testing.B) {
	ctx := context.Background()

	graphStore, err := store.NewSQLiteGraphStore(":memory:")
	if err != nil {
		b.Fatalf("Failed to create graph store: %v", err)
	}
	defer graphStore.Close()

	vectorStore := store.NewMemoryVectorStore()
	embedClient := &mockEmbeddingClient{}

	// Create a shallow graph: 10 disconnected nodes
	for i := 0; i < 10; i++ {
		node := &store.Node{
			ID:          generateNodeID(i),
			Name:        generateNodeName(i),
			Type:        "Entity",
			Description: "An isolated entity",
			Embedding:   make([]float32, 1536),
		}
		if err := graphStore.AddNode(ctx, node); err != nil {
			b.Fatalf("Failed to add node %d: %v", i, err)
		}
		if err := vectorStore.Add(ctx, node.ID, node.Embedding); err != nil {
			b.Fatalf("Failed to index node %d: %v", i, err)
		}
	}

	hybridSearcher := NewHybridSearcher(embedClient, vectorStore, graphStore)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := hybridSearcher.Search(ctx, "test query", SearchOptions{
			TopK:       10,
			GraphDepth: 1,
		})
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

// Helper functions for test data generation
func generateNodeID(i int) string {
	return string(rune('A'+(i%26))) + string(rune('0'+(i/26)))
}

func generateNodeName(i int) string {
	return "Node_" + generateNodeID(i)
}

func generateEdgeID(sourceID, targetID string) string {
	return sourceID + "-" + targetID
}

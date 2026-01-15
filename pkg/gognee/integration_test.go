//go:build integration
// +build integration

package gognee

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegration_DecayAndPrune tests the end-to-end decay and prune workflow
func TestIntegration_DecayAndPrune(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	// Create gognee with decay enabled
	g, err := New(Config{
		DBPath:            ":memory:",
		OpenAIKey:         apiKey,
		DecayEnabled:      true,
		DecayHalfLifeDays: 1, // Very short for testing
		DecayBasis:        "creation",
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Simulate old documents by manually adding nodes with old timestamps
	now := time.Now()
	oldTime := now.Add(-2 * 24 * time.Hour) // 2 days old (2 half-lives)

	// Add an old node
	oldNode := &Node{
		ID:          "old-node-1",
		Name:        "Old Concept",
		Type:        "Concept",
		Description: "An old concept",
		CreatedAt:   oldTime,
	}
	if err := g.graphStore.AddNode(ctx, oldNode); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Add a recent node
	recentNode := &Node{
		ID:          "recent-node-1",
		Name:        "Recent Concept",
		Type:        "Concept",
		Description: "A recent concept",
		CreatedAt:   now,
	}
	if err := g.graphStore.AddNode(ctx, recentNode); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Add embeddings to vector store
	oldEmbedding := []float32{0.1, 0.2, 0.3, 0.4}
	recentEmbedding := []float32{0.2, 0.3, 0.4, 0.5}

	if err := g.vectorStore.Add(ctx, "old-node-1", oldEmbedding); err != nil {
		t.Fatalf("VectorStore.Add failed: %v", err)
	}
	if err := g.vectorStore.Add(ctx, "recent-node-1", recentEmbedding); err != nil {
		t.Fatalf("VectorStore.Add failed: %v", err)
	}

	// Search - old node should have lower score due to decay
	resp, err := g.Search(ctx, "concept", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	results := resp.Results
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Verify old node has lower score than recent node (assuming similar vector scores)
	var oldScore, recentScore float64
	for _, r := range results {
		if r.NodeID == "old-node-1" {
			oldScore = r.Score
		} else if r.NodeID == "recent-node-1" {
			recentScore = r.Score
		}
	}

	if oldScore >= recentScore {
		t.Logf("Old node score: %.6f, Recent node score: %.6f", oldScore, recentScore)
		t.Error("Expected old node to have lower score than recent node due to decay")
	}

	// Test pruning - remove nodes older than 1 day
	pruneResult, err := g.Prune(ctx, PruneOptions{
		MaxAgeDays: 1,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	if pruneResult.NodesPruned != 1 {
		t.Errorf("Expected 1 node pruned, got %d", pruneResult.NodesPruned)
	}

	// Verify old node is gone
	node, err := g.graphStore.GetNode(ctx, "old-node-1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node != nil {
		t.Error("Expected old node to be pruned")
	}

	// Verify recent node still exists
	node, err = g.graphStore.GetNode(ctx, "recent-node-1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node == nil {
		t.Error("Expected recent node to still exist")
	}

	// Search again - should only return recent node
	resp, err = g.Search(ctx, "concept", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search after prune failed: %v", err)
	}

	results = resp.Results
	if len(results) != 1 {
		t.Errorf("Expected 1 result after prune, got %d", len(results))
	}
	if len(results) > 0 && results[0].NodeID != "recent-node-1" {
		t.Errorf("Expected recent node, got %s", results[0].NodeID)
	}
}

// TestIntegration_AccessReinforcement tests that accessed nodes resist decay
func TestIntegration_AccessReinforcement(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	g, err := New(Config{
		DBPath:            ":memory:",
		OpenAIKey:         apiKey,
		DecayEnabled:      true,
		DecayHalfLifeDays: 1,
		DecayBasis:        "access", // Access-based decay
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Add an old node
	now := time.Now()
	oldTime := now.Add(-2 * 24 * time.Hour)

	oldNode := &Node{
		ID:          "old-but-accessed",
		Name:        "Old But Accessed",
		Type:        "Concept",
		Description: "Old but frequently accessed",
		CreatedAt:   oldTime,
	}
	if err := g.graphStore.AddNode(ctx, oldNode); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	embedding := []float32{0.1, 0.2, 0.3, 0.4}
	if err := g.vectorStore.Add(ctx, "old-but-accessed", embedding); err != nil {
		t.Fatalf("VectorStore.Add failed: %v", err)
	}

	// Access the node via Search (this should update last_accessed_at)
	_, err = g.Search(ctx, "concept", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify last_accessed_at was updated
	node, err := g.graphStore.GetNode(ctx, "old-but-accessed")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node.LastAccessedAt == nil {
		t.Fatal("Expected last_accessed_at to be set after search")
	}

	// With access-based decay, the node should now have minimal decay
	// even though it's 2 days old, because it was just accessed
	age := now.Sub(*node.LastAccessedAt)
	if age.Hours() > 1 {
		t.Errorf("Expected recently accessed node, but last_accessed_at is %v old", age)
	}
}

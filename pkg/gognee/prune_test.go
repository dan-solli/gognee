package gognee

import (
	"context"
	"testing"
	"time"

	"github.com/dan-solli/gognee/pkg/store"
)

// TestPrune_DryRun tests that DryRun doesn't actually delete nodes
func TestPrune_DryRun(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Add some old nodes
	old := time.Now().Add(-60 * 24 * time.Hour) // 60 days old
	nodes := []*store.Node{
		{ID: "old1", Name: "Old Node 1", CreatedAt: old},
		{ID: "old2", Name: "Old Node 2", CreatedAt: old},
		{ID: "recent", Name: "Recent Node", CreatedAt: time.Now()},
	}

	for _, node := range nodes {
		if err := g.graphStore.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	// Dry run prune with MaxAgeDays=30
	result, err := g.Prune(ctx, PruneOptions{
		MaxAgeDays: 30,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	// Should report 2 nodes would be pruned
	if result.NodesPruned != 2 {
		t.Errorf("NodesPruned: got %d, want 2", result.NodesPruned)
	}

	// Verify nodes are still there
	count, err := g.graphStore.NodeCount(ctx)
	if err != nil {
		t.Fatalf("NodeCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("NodeCount after dry run: got %d, want 3", count)
	}
}

// TestPrune_MaxAgeDays tests pruning by age
func TestPrune_MaxAgeDays(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Add nodes with different ages
	now := time.Now()
	nodes := []*store.Node{
		{ID: "very-old", Name: "Very Old", CreatedAt: now.Add(-90 * 24 * time.Hour)},
		{ID: "old", Name: "Old", CreatedAt: now.Add(-60 * 24 * time.Hour)},
		{ID: "medium", Name: "Medium", CreatedAt: now.Add(-20 * 24 * time.Hour)},
		{ID: "recent", Name: "Recent", CreatedAt: now},
	}

	for _, node := range nodes {
		if err := g.graphStore.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	// Prune nodes older than 30 days
	result, err := g.Prune(ctx, PruneOptions{
		MaxAgeDays: 30,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	// Should prune 2 nodes (very-old and old)
	if result.NodesPruned != 2 {
		t.Errorf("NodesPruned: got %d, want 2", result.NodesPruned)
	}

	// Verify remaining nodes
	count, err := g.graphStore.NodeCount(ctx)
	if err != nil {
		t.Fatalf("NodeCount failed: %v", err)
	}
	if count != 2 {
		t.Errorf("NodeCount after prune: got %d, want 2", count)
	}

	// Verify old nodes are gone
	node, err := g.graphStore.GetNode(ctx, "very-old")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node != nil {
		t.Error("Expected very-old node to be pruned")
	}

	// Verify recent nodes remain
	node, err = g.graphStore.GetNode(ctx, "recent")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node == nil {
		t.Error("Expected recent node to remain")
	}
}

// TestPrune_CascadeEdges tests that edges are deleted when nodes are pruned
func TestPrune_CascadeEdges(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Add nodes
	now := time.Now()
	old := now.Add(-60 * 24 * time.Hour)
	nodes := []*store.Node{
		{ID: "old-node", Name: "Old Node", CreatedAt: old},
		{ID: "recent-node", Name: "Recent Node", CreatedAt: now},
	}

	for _, node := range nodes {
		if err := g.graphStore.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	// Add edges connecting old and recent nodes
	edges := []*store.Edge{
		{ID: "edge1", SourceID: "old-node", Relation: "RELATES_TO", TargetID: "recent-node"},
		{ID: "edge2", SourceID: "recent-node", Relation: "LINKS_TO", TargetID: "old-node"},
	}

	for _, edge := range edges {
		if err := g.graphStore.AddEdge(ctx, edge); err != nil {
			t.Fatalf("AddEdge failed: %v", err)
		}
	}

	// Prune old node
	result, err := g.Prune(ctx, PruneOptions{
		MaxAgeDays: 30,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	// Should prune 1 node and 2 edges
	if result.NodesPruned != 1 {
		t.Errorf("NodesPruned: got %d, want 1", result.NodesPruned)
	}
	if result.EdgesPruned != 2 {
		t.Errorf("EdgesPruned: got %d, want 2", result.EdgesPruned)
	}

	// Verify edges are gone
	edgeCount, err := g.graphStore.EdgeCount(ctx)
	if err != nil {
		t.Fatalf("EdgeCount failed: %v", err)
	}
	if edgeCount != 0 {
		t.Errorf("EdgeCount after prune: got %d, want 0", edgeCount)
	}
}

// TestPrune_EmptyDatabase tests pruning on empty database
func TestPrune_EmptyDatabase(t *testing.T) {
	g, err := New(Config{DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	result, err := g.Prune(ctx, PruneOptions{
		MaxAgeDays: 30,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	if result.NodesPruned != 0 {
		t.Errorf("NodesPruned: got %d, want 0", result.NodesPruned)
	}
	if result.NodesEvaluated != 0 {
		t.Errorf("NodesEvaluated: got %d, want 0", result.NodesEvaluated)
	}
}

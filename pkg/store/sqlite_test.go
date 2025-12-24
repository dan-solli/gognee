package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAddNodeAndGetNode tests basic node CRUD operations.
func TestAddNodeAndGetNode(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create a test node
	node := &Node{
		ID:          "test-id-1",
		Name:        "Test Node",
		Type:        "Concept",
		Description: "A test concept",
		Embedding:   []float32{0.1, 0.2, 0.3},
		CreatedAt:   time.Now(),
		Metadata:    map[string]interface{}{"key": "value"},
	}

	// Add node
	err := store.AddNode(ctx, node)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Get node back
	retrieved, err := store.GetNode(ctx, "test-id-1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected node, got nil")
	}

	// Verify fields
	if retrieved.ID != node.ID {
		t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, node.ID)
	}
	if retrieved.Name != node.Name {
		t.Errorf("Name mismatch: got %s, want %s", retrieved.Name, node.Name)
	}
	if retrieved.Type != node.Type {
		t.Errorf("Type mismatch: got %s, want %s", retrieved.Type, node.Type)
	}
	if retrieved.Description != node.Description {
		t.Errorf("Description mismatch: got %s, want %s", retrieved.Description, node.Description)
	}

	// Verify embedding
	if len(retrieved.Embedding) != len(node.Embedding) {
		t.Fatalf("Embedding length mismatch: got %d, want %d", len(retrieved.Embedding), len(node.Embedding))
	}
	for i := range node.Embedding {
		if retrieved.Embedding[i] != node.Embedding[i] {
			t.Errorf("Embedding[%d] mismatch: got %f, want %f", i, retrieved.Embedding[i], node.Embedding[i])
		}
	}

	// Verify metadata
	if retrieved.Metadata["key"] != "value" {
		t.Errorf("Metadata mismatch: got %v", retrieved.Metadata)
	}
}

// TestGetNode_NotFound tests that GetNode returns nil for non-existent nodes.
func TestGetNode_NotFound(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Try to get non-existent node
	node, err := store.GetNode(ctx, "non-existent")
	if err != nil {
		t.Fatalf("GetNode returned error for non-existent node: %v", err)
	}

	if node != nil {
		t.Errorf("Expected nil node, got %v", node)
	}
}

// TestAddNode_Upsert tests that AddNode updates existing nodes.
func TestAddNode_Upsert(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Add initial node
	node := &Node{
		ID:          "test-id-1",
		Name:        "Original Name",
		Type:        "Concept",
		Description: "Original description",
	}

	err := store.AddNode(ctx, node)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Update the node
	node.Name = "Updated Name"
	node.Description = "Updated description"

	err = store.AddNode(ctx, node)
	if err != nil {
		t.Fatalf("AddNode (update) failed: %v", err)
	}

	// Verify update
	retrieved, err := store.GetNode(ctx, "test-id-1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("Name not updated: got %s, want Updated Name", retrieved.Name)
	}
	if retrieved.Description != "Updated description" {
		t.Errorf("Description not updated: got %s, want Updated description", retrieved.Description)
	}
}

// TestFindNodesByName_CaseInsensitive tests case-insensitive name matching.
func TestFindNodesByName_CaseInsensitive(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Add nodes with different cases
	nodes := []*Node{
		{ID: "1", Name: "Test Node", Type: "Concept"},
		{ID: "2", Name: "test node", Type: "Concept"},
		{ID: "3", Name: "TEST NODE", Type: "Concept"},
		{ID: "4", Name: "Different", Type: "Concept"},
	}

	for _, node := range nodes {
		if err := store.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	// Search with different case
	results, err := store.FindNodesByName(ctx, "test node")
	if err != nil {
		t.Fatalf("FindNodesByName failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Verify ordering is deterministic
	if results[0].ID != "1" && results[0].ID != "2" && results[0].ID != "3" {
		t.Errorf("Unexpected node in results: %s", results[0].ID)
	}
}

// TestFindNodeByName_SingleMatch tests the convenience method with one match.
func TestFindNodeByName_SingleMatch(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Add single node
	node := &Node{ID: "1", Name: "Unique Node", Type: "Concept"}
	if err := store.AddNode(ctx, node); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Find it
	result, err := store.FindNodeByName(ctx, "unique node")
	if err != nil {
		t.Fatalf("FindNodeByName failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected node, got nil")
	}

	if result.ID != "1" {
		t.Errorf("Wrong node returned: got %s, want 1", result.ID)
	}
}

// TestFindNodeByName_NotFound tests the error when no nodes match.
func TestFindNodeByName_NotFound(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Try to find non-existent node
	_, err := store.FindNodeByName(ctx, "nonexistent")
	if err != ErrNodeNotFound {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

// TestFindNodeByName_Ambiguous tests the error when multiple nodes match.
func TestFindNodeByName_Ambiguous(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Add duplicate names
	nodes := []*Node{
		{ID: "1", Name: "Duplicate", Type: "Concept"},
		{ID: "2", Name: "Duplicate", Type: "Concept"},
	}

	for _, node := range nodes {
		if err := store.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	// Try to find with single-match method
	_, err := store.FindNodeByName(ctx, "Duplicate")
	if err != ErrAmbiguousNode {
		t.Errorf("Expected ErrAmbiguousNode, got %v", err)
	}
}

// TestAddEdgeAndGetEdges tests edge CRUD operations.
func TestAddEdgeAndGetEdges(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Add nodes first
	nodes := []*Node{
		{ID: "node1", Name: "Node 1", Type: "Concept"},
		{ID: "node2", Name: "Node 2", Type: "Concept"},
	}

	for _, node := range nodes {
		if err := store.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	// Add edge
	edge := &Edge{
		ID:        "edge1",
		SourceID:  "node1",
		Relation:  "RELATES_TO",
		TargetID:  "node2",
		Weight:    1.5,
		CreatedAt: time.Now(),
	}

	err := store.AddEdge(ctx, edge)
	if err != nil {
		t.Fatalf("AddEdge failed: %v", err)
	}

	// Get edges for source node
	edges, err := store.GetEdges(ctx, "node1")
	if err != nil {
		t.Fatalf("GetEdges failed: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("Expected 1 edge, got %d", len(edges))
	}

	if edges[0].ID != "edge1" {
		t.Errorf("Edge ID mismatch: got %s, want edge1", edges[0].ID)
	}
	if edges[0].SourceID != "node1" {
		t.Errorf("SourceID mismatch: got %s, want node1", edges[0].SourceID)
	}
	if edges[0].TargetID != "node2" {
		t.Errorf("TargetID mismatch: got %s, want node2", edges[0].TargetID)
	}
	if edges[0].Relation != "RELATES_TO" {
		t.Errorf("Relation mismatch: got %s, want RELATES_TO", edges[0].Relation)
	}
	if edges[0].Weight != 1.5 {
		t.Errorf("Weight mismatch: got %f, want 1.5", edges[0].Weight)
	}
}

// TestGetEdges_DirectionAgnostic tests that GetEdges returns both incoming and outgoing edges.
func TestGetEdges_DirectionAgnostic(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Add nodes
	nodes := []*Node{
		{ID: "center", Name: "Center", Type: "Concept"},
		{ID: "source", Name: "Source", Type: "Concept"},
		{ID: "target", Name: "Target", Type: "Concept"},
	}

	for _, node := range nodes {
		if err := store.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	// Add edges: one incoming, one outgoing from "center"
	edges := []*Edge{
		{ID: "edge1", SourceID: "source", Relation: "TO", TargetID: "center"},
		{ID: "edge2", SourceID: "center", Relation: "FROM", TargetID: "target"},
	}

	for _, edge := range edges {
		if err := store.AddEdge(ctx, edge); err != nil {
			t.Fatalf("AddEdge failed: %v", err)
		}
	}

	// Get all edges for center node
	result, err := store.GetEdges(ctx, "center")
	if err != nil {
		t.Fatalf("GetEdges failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 edges, got %d", len(result))
	}

	// Verify both edges are present
	edgeIDs := make(map[string]bool)
	for _, e := range result {
		edgeIDs[e.ID] = true
	}

	if !edgeIDs["edge1"] || !edgeIDs["edge2"] {
		t.Error("Expected both edge1 and edge2 in results")
	}
}

// TestGetEdges_Empty tests that GetEdges returns empty slice when no edges exist.
func TestGetEdges_Empty(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Add node with no edges
	node := &Node{ID: "lonely", Name: "Lonely Node", Type: "Concept"}
	if err := store.AddNode(ctx, node); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Get edges
	edges, err := store.GetEdges(ctx, "lonely")
	if err != nil {
		t.Fatalf("GetEdges failed: %v", err)
	}

	if len(edges) != 0 {
		t.Errorf("Expected 0 edges, got %d", len(edges))
	}
}

// TestGetNeighbors_Depth1 tests basic neighbor discovery.
func TestGetNeighbors_Depth1(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create a simple graph: A -- B -- C
	nodes := []*Node{
		{ID: "A", Name: "Node A", Type: "Concept"},
		{ID: "B", Name: "Node B", Type: "Concept"},
		{ID: "C", Name: "Node C", Type: "Concept"},
	}

	for _, node := range nodes {
		if err := store.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	edges := []*Edge{
		{ID: "e1", SourceID: "A", Relation: "CONNECTS", TargetID: "B"},
		{ID: "e2", SourceID: "B", Relation: "CONNECTS", TargetID: "C"},
	}

	for _, edge := range edges {
		if err := store.AddEdge(ctx, edge); err != nil {
			t.Fatalf("AddEdge failed: %v", err)
		}
	}

	// Get neighbors of B at depth 1
	neighbors, err := store.GetNeighbors(ctx, "B", 1)
	if err != nil {
		t.Fatalf("GetNeighbors failed: %v", err)
	}

	if len(neighbors) != 2 {
		t.Fatalf("Expected 2 neighbors, got %d", len(neighbors))
	}

	// Verify A and C are neighbors
	neighborIDs := make(map[string]bool)
	for _, n := range neighbors {
		neighborIDs[n.ID] = true
	}

	if !neighborIDs["A"] || !neighborIDs["C"] {
		t.Error("Expected A and C as neighbors of B")
	}
}

// TestGetNeighbors_Depth2 tests multi-hop traversal.
func TestGetNeighbors_Depth2(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create a chain: A -- B -- C -- D
	nodes := []*Node{
		{ID: "A", Name: "Node A", Type: "Concept"},
		{ID: "B", Name: "Node B", Type: "Concept"},
		{ID: "C", Name: "Node C", Type: "Concept"},
		{ID: "D", Name: "Node D", Type: "Concept"},
	}

	for _, node := range nodes {
		if err := store.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	edges := []*Edge{
		{ID: "e1", SourceID: "A", Relation: "CONNECTS", TargetID: "B"},
		{ID: "e2", SourceID: "B", Relation: "CONNECTS", TargetID: "C"},
		{ID: "e3", SourceID: "C", Relation: "CONNECTS", TargetID: "D"},
	}

	for _, edge := range edges {
		if err := store.AddEdge(ctx, edge); err != nil {
			t.Fatalf("AddEdge failed: %v", err)
		}
	}

	// Get neighbors of A at depth 2 (should reach B and C)
	neighbors, err := store.GetNeighbors(ctx, "A", 2)
	if err != nil {
		t.Fatalf("GetNeighbors failed: %v", err)
	}

	if len(neighbors) != 2 {
		t.Fatalf("Expected 2 neighbors at depth 2, got %d", len(neighbors))
	}

	// Verify B and C are found
	neighborIDs := make(map[string]bool)
	for _, n := range neighbors {
		neighborIDs[n.ID] = true
	}

	if !neighborIDs["B"] || !neighborIDs["C"] {
		t.Error("Expected B and C as neighbors of A at depth 2")
	}

	// D should NOT be included (depth 3)
	if neighborIDs["D"] {
		t.Error("D should not be included at depth 2")
	}
}

// TestGetNeighbors_NoDuplicates tests that neighbors are deduplicated.
func TestGetNeighbors_NoDuplicates(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Create a triangle: A -- B, A -- C, B -- C
	nodes := []*Node{
		{ID: "A", Name: "Node A", Type: "Concept"},
		{ID: "B", Name: "Node B", Type: "Concept"},
		{ID: "C", Name: "Node C", Type: "Concept"},
	}

	for _, node := range nodes {
		if err := store.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	edges := []*Edge{
		{ID: "e1", SourceID: "A", Relation: "CONNECTS", TargetID: "B"},
		{ID: "e2", SourceID: "A", Relation: "CONNECTS", TargetID: "C"},
		{ID: "e3", SourceID: "B", Relation: "CONNECTS", TargetID: "C"},
	}

	for _, edge := range edges {
		if err := store.AddEdge(ctx, edge); err != nil {
			t.Fatalf("AddEdge failed: %v", err)
		}
	}

	// Get neighbors of A at depth 2 (should reach B and C via multiple paths)
	neighbors, err := store.GetNeighbors(ctx, "A", 2)
	if err != nil {
		t.Fatalf("GetNeighbors failed: %v", err)
	}

	// Should have exactly 2 unique neighbors
	if len(neighbors) != 2 {
		t.Fatalf("Expected 2 unique neighbors, got %d", len(neighbors))
	}

	// Verify no duplicates in result
	seen := make(map[string]bool)
	for _, n := range neighbors {
		if seen[n.ID] {
			t.Errorf("Duplicate neighbor found: %s", n.ID)
		}
		seen[n.ID] = true
	}
}

// setupTestStore creates an in-memory SQLite store for testing.
func setupTestStore(t *testing.T) *SQLiteGraphStore {
	store, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	return store
}

// TestPersistence tests that data persists across store close/reopen.
func TestPersistence(t *testing.T) {
	// Create temp file for database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create store and add data
	store, err := NewSQLiteGraphStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	ctx := context.Background()

	node := &Node{
		ID:          "persist-test",
		Name:        "Persistent Node",
		Type:        "Concept",
		Description: "Should persist",
	}

	if err := store.AddNode(ctx, node); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Close store
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopen store
	store2, err := NewSQLiteGraphStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen store: %v", err)
	}
	defer store2.Close()

	// Verify data persisted
	retrieved, err := store2.GetNode(ctx, "persist-test")
	if err != nil {
		t.Fatalf("GetNode failed after reopen: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Node not found after reopen")
	}

	if retrieved.Name != "Persistent Node" {
		t.Errorf("Node data not persisted correctly: got %s", retrieved.Name)
	}
}

// TestEdgeDefaultWeight tests that edges get default weight of 1.0.
func TestEdgeDefaultWeight(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Add nodes
	nodes := []*Node{
		{ID: "node1", Name: "Node 1", Type: "Concept"},
		{ID: "node2", Name: "Node 2", Type: "Concept"},
	}

	for _, node := range nodes {
		if err := store.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}
	}

	// Add edge without specifying weight
	edge := &Edge{
		ID:       "edge1",
		SourceID: "node1",
		Relation: "RELATES_TO",
		TargetID: "node2",
		// Weight not specified
	}

	err := store.AddEdge(ctx, edge)
	if err != nil {
		t.Fatalf("AddEdge failed: %v", err)
	}

	// Retrieve and verify weight is 1.0
	edges, err := store.GetEdges(ctx, "node1")
	if err != nil {
		t.Fatalf("GetEdges failed: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("Expected 1 edge, got %d", len(edges))
	}

	if edges[0].Weight != 1.0 {
		t.Errorf("Expected default weight 1.0, got %f", edges[0].Weight)
	}
}

// TestNodeWithoutID tests that AddNode generates ID if not provided.
func TestNodeWithoutID(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	node := &Node{
		Name: "No ID Node",
		Type: "Concept",
		// ID not provided
	}

	err := store.AddNode(ctx, node)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Verify ID was generated
	if node.ID == "" {
		t.Error("Expected ID to be generated")
	}

	// Verify we can retrieve it
	retrieved, err := store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Node not found after creation with auto-generated ID")
	}
}

// TestEmptyMetadata tests nodes with nil metadata.
func TestEmptyMetadata(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	node := &Node{
		ID:       "no-meta",
		Name:     "No Metadata",
		Type:     "Concept",
		Metadata: nil,
	}

	err := store.AddNode(ctx, node)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	retrieved, err := store.GetNode(ctx, "no-meta")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if retrieved.Metadata != nil {
		t.Errorf("Expected nil metadata, got %v", retrieved.Metadata)
	}
}

// TestEmptyEmbedding tests nodes with no embedding.
func TestEmptyEmbedding(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := context.Background()

	node := &Node{
		ID:        "no-embed",
		Name:      "No Embedding",
		Type:      "Concept",
		Embedding: nil,
	}

	err := store.AddNode(ctx, node)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	retrieved, err := store.GetNode(ctx, "no-embed")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if len(retrieved.Embedding) != 0 {
		t.Errorf("Expected empty embedding, got length %d", len(retrieved.Embedding))
	}
}

// TestDatabasePath tests creating store with file path.
func TestDatabasePath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteGraphStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store with file path: %v", err)
	}
	defer store.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestNodeCount(t *testing.T) {
	store, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Initial count should be 0
	count, err := store.NodeCount(ctx)
	if err != nil {
		t.Fatalf("NodeCount failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("Initial NodeCount: got %d, want 0", count)
	}

	// Add nodes and verify count increases
	for i := 0; i < 3; i++ {
		node := &Node{
			ID:        fmt.Sprintf("node-%d", i),
			Name:      fmt.Sprintf("Node %d", i),
			Type:      "Test",
			CreatedAt: time.Now(),
		}
		if err := store.AddNode(ctx, node); err != nil {
			t.Fatalf("AddNode failed: %v", err)
		}

		count, err := store.NodeCount(ctx)
		if err != nil {
			t.Fatalf("NodeCount failed: %v", err)
		}
		if count != int64(i+1) {
			t.Fatalf("NodeCount after adding node %d: got %d, want %d", i, count, i+1)
		}
	}

	// Upsert (replace) should not increase count
	node := &Node{
		ID:        "node-0",
		Name:      "Updated Node 0",
		Type:      "Test",
		CreatedAt: time.Now(),
	}
	if err := store.AddNode(ctx, node); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	count2, err2 := store.NodeCount(ctx)
	if err2 != nil {
		t.Fatalf("NodeCount failed: %v", err2)
	}
	if count2 != 3 {
		t.Fatalf("NodeCount after upsert: got %d, want 3", count2)
	}
}

func TestEdgeCount(t *testing.T) {
	store, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create nodes
	node1 := &Node{
		ID:        "node-1",
		Name:      "Node 1",
		Type:      "Test",
		CreatedAt: time.Now(),
	}
	node2 := &Node{
		ID:        "node-2",
		Name:      "Node 2",
		Type:      "Test",
		CreatedAt: time.Now(),
	}
	if err := store.AddNode(ctx, node1); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}
	if err := store.AddNode(ctx, node2); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Initial count should be 0
	count, err := store.EdgeCount(ctx)
	if err != nil {
		t.Fatalf("EdgeCount failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("Initial EdgeCount: got %d, want 0", count)
	}

	// Add edges and verify count increases
	for i := 0; i < 3; i++ {
		edge := &Edge{
			ID:        fmt.Sprintf("edge-%d", i),
			SourceID:  "node-1",
			TargetID:  "node-2",
			Relation:  "TEST",
			Weight:    1.0,
			CreatedAt: time.Now(),
		}
		if err := store.AddEdge(ctx, edge); err != nil {
			t.Fatalf("AddEdge failed: %v", err)
		}

		count, err := store.EdgeCount(ctx)
		if err != nil {
			t.Fatalf("EdgeCount failed: %v", err)
		}
		if count != int64(i+1) {
			t.Fatalf("EdgeCount after adding edge %d: got %d, want %d", i, count, i+1)
		}
	}

	// Upsert should not increase count
	edge := &Edge{
		ID:        "edge-0",
		SourceID:  "node-1",
		TargetID:  "node-2",
		Relation:  "UPDATED",
		Weight:    2.0,
		CreatedAt: time.Now(),
	}
	if err := store.AddEdge(ctx, edge); err != nil {
		t.Fatalf("AddEdge failed: %v", err)
	}

	count2, err2 := store.EdgeCount(ctx)
	if err2 != nil {
		t.Fatalf("EdgeCount failed: %v", err2)
	}
	if count2 != 3 {
		t.Fatalf("EdgeCount after upsert: got %d, want 3", count2)
	}
}

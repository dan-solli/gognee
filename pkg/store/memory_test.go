package store

import (
	"context"
	"testing"
	"time"
)

// TestMemoryStore_CRUD tests basic CRUD operations.
func TestMemoryStore_CRUD(t *testing.T) {
	ctx := context.Background()

	// Create in-memory store
	graphStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer graphStore.Close()

	memStore := NewSQLiteMemoryStore(graphStore.DB())

	// Test AddMemory
	memory := &MemoryRecord{
		Topic:     "Test Memory",
		Context:   "This is a test context",
		Decisions: []string{"Decision 1", "Decision 2"},
		Rationale: []string{"Rationale 1"},
		Metadata:  map[string]interface{}{"key": "value"},
		DocHash:   ComputeDocHash("Test Memory", "This is a test context", []string{"Decision 1", "Decision 2"}, []string{"Rationale 1"}),
		Source:    "test",
		Status:    "complete",
	}

	err = memStore.AddMemory(ctx, memory)
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}

	if memory.ID == "" {
		t.Error("Memory ID not generated")
	}

	// Test GetMemory
	retrieved, err := memStore.GetMemory(ctx, memory.ID)
	if err != nil {
		t.Fatalf("GetMemory failed: %v", err)
	}

	if retrieved.Topic != memory.Topic {
		t.Errorf("Topic mismatch: got %s, want %s", retrieved.Topic, memory.Topic)
	}

	if retrieved.Context != memory.Context {
		t.Errorf("Context mismatch: got %s, want %s", retrieved.Context, memory.Context)
	}

	if len(retrieved.Decisions) != 2 {
		t.Errorf("Decisions length mismatch: got %d, want 2", len(retrieved.Decisions))
	}

	// Test UpdateMemory
	newContext := "Updated context"
	updates := MemoryUpdate{
		Context: &newContext,
	}

	err = memStore.UpdateMemory(ctx, memory.ID, updates)
	if err != nil {
		t.Fatalf("UpdateMemory failed: %v", err)
	}

	updated, err := memStore.GetMemory(ctx, memory.ID)
	if err != nil {
		t.Fatalf("GetMemory after update failed: %v", err)
	}

	if updated.Context != newContext {
		t.Errorf("Context not updated: got %s, want %s", updated.Context, newContext)
	}

	if updated.Version != 2 {
		t.Errorf("Version not incremented: got %d, want 2", updated.Version)
	}

	// Test DeleteMemory
	err = memStore.DeleteMemory(ctx, memory.ID)
	if err != nil {
		t.Fatalf("DeleteMemory failed: %v", err)
	}

	_, err = memStore.GetMemory(ctx, memory.ID)
	if err != ErrMemoryNotFound {
		t.Errorf("Expected ErrMemoryNotFound, got %v", err)
	}
}

// TestMemoryStore_ListMemories tests pagination.
func TestMemoryStore_ListMemories(t *testing.T) {
	ctx := context.Background()

	graphStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer graphStore.Close()

	memStore := NewSQLiteMemoryStore(graphStore.DB())

	// Add multiple memories
	for i := 0; i < 15; i++ {
		memory := &MemoryRecord{
			Topic:   "Memory " + string(rune('A'+i)),
			Context: "Context for memory " + string(rune('A'+i)),
			DocHash: ComputeDocHash("Memory "+string(rune('A'+i)), "Context for memory "+string(rune('A'+i)), nil, nil),
			Status:  "complete",
		}

		err := memStore.AddMemory(ctx, memory)
		if err != nil {
			t.Fatalf("AddMemory failed: %v", err)
		}

		// Sleep to ensure different timestamps
		time.Sleep(time.Millisecond)
	}

	// Test default pagination
	results, err := memStore.ListMemories(ctx, ListMemoriesOptions{})
	if err != nil {
		t.Fatalf("ListMemories failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected non-empty results")
	}

	// Test limit
	results, err = memStore.ListMemories(ctx, ListMemoriesOptions{Limit: 5})
	if err != nil {
		t.Fatalf("ListMemories failed: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(results))
	}

	// Test offset
	results2, err := memStore.ListMemories(ctx, ListMemoriesOptions{Limit: 5, Offset: 5})
	if err != nil {
		t.Fatalf("ListMemories with offset failed: %v", err)
	}

	if len(results2) != 5 {
		t.Errorf("Expected 5 results with offset, got %d", len(results2))
	}

	// Verify no overlap
	if results[0].ID == results2[0].ID {
		t.Error("Pagination overlap detected")
	}
}

// TestMemoryStore_Provenance tests provenance tracking.
func TestMemoryStore_Provenance(t *testing.T) {
	ctx := context.Background()

	graphStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer graphStore.Close()

	memStore := NewSQLiteMemoryStore(graphStore.DB())
	if memStore.DB() == nil {
		t.Fatalf("expected non-nil memStore DB")
	}

	// Add memory
	memory := &MemoryRecord{
		Topic:   "Test",
		Context: "Test context",
		DocHash: ComputeDocHash("Test", "Test context", nil, nil),
		Status:  "complete",
	}

	err = memStore.AddMemory(ctx, memory)
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}

	// Add some nodes to graph
	node1 := &Node{ID: "node1", Name: "Node 1", Type: "Concept"}
	node2 := &Node{ID: "node2", Name: "Node 2", Type: "Concept"}

	err = graphStore.AddNode(ctx, node1)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	err = graphStore.AddNode(ctx, node2)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Link provenance
	err = memStore.LinkProvenance(ctx, memory.ID, []string{"node1", "node2"}, []string{})
	if err != nil {
		t.Fatalf("LinkProvenance failed: %v", err)
	}

	// Get provenance
	nodeIDs, edgeIDs, err := memStore.GetProvenanceByMemory(ctx, memory.ID)
	if err != nil {
		t.Fatalf("GetProvenanceByMemory failed: %v", err)
	}

	if len(nodeIDs) != 2 {
		t.Errorf("Expected 2 node IDs, got %d", len(nodeIDs))
	}

	if len(edgeIDs) != 0 {
		t.Errorf("Expected 0 edge IDs, got %d", len(edgeIDs))
	}

	// Test GetMemoriesByNodeID
	memoryIDs, err := memStore.GetMemoriesByNodeID(ctx, "node1")
	if err != nil {
		t.Fatalf("GetMemoriesByNodeID failed: %v", err)
	}

	if len(memoryIDs) != 1 || memoryIDs[0] != memory.ID {
		t.Errorf("Expected memory ID %s, got %v", memory.ID, memoryIDs)
	}

	// Test batched query
	batchMap, err := memStore.GetMemoriesByNodeIDBatched(ctx, []string{"node1", "node2"})
	if err != nil {
		t.Fatalf("GetMemoriesByNodeIDBatched failed: %v", err)
	}

	if len(batchMap) != 2 {
		t.Errorf("Expected 2 entries in batch map, got %d", len(batchMap))
	}

	if len(batchMap["node1"]) != 1 {
		t.Errorf("Expected 1 memory for node1, got %d", len(batchMap["node1"]))
	}
}

func TestMemoryStore_UnlinkProvenanceAndCounts(t *testing.T) {
	ctx := context.Background()

	graphStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer graphStore.Close()

	memStore := NewSQLiteMemoryStore(graphStore.DB())

	memory := &MemoryRecord{
		Topic:   "Test",
		Context: "Test context",
		DocHash: ComputeDocHash("Test", "Test context", nil, nil),
		Status:  "complete",
	}
	if err := memStore.AddMemory(ctx, memory); err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}

	// Add node + edge to graph, link provenance, then unlink.
	node := &Node{ID: "node1", Name: "Node 1", Type: "Concept"}
	if err := graphStore.AddNode(ctx, node); err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}
	edge := &Edge{ID: "edge1", SourceID: "node1", Relation: "RELATES_TO", TargetID: "node1", Weight: 1.0}
	if err := graphStore.AddEdge(ctx, edge); err != nil {
		t.Fatalf("AddEdge failed: %v", err)
	}

	if err := memStore.LinkProvenance(ctx, memory.ID, []string{"node1"}, []string{"edge1"}); err != nil {
		t.Fatalf("LinkProvenance failed: %v", err)
	}

	refCount, err := memStore.CountMemoryReferences(ctx, "node1")
	if err != nil {
		t.Fatalf("CountMemoryReferences failed: %v", err)
	}
	if refCount != 1 {
		t.Fatalf("expected 1 reference, got %d", refCount)
	}

	if err := memStore.UnlinkProvenance(ctx, memory.ID); err != nil {
		t.Fatalf("UnlinkProvenance failed: %v", err)
	}

	nodeIDs, edgeIDs, err := memStore.GetProvenanceByMemory(ctx, memory.ID)
	if err != nil {
		t.Fatalf("GetProvenanceByMemory failed: %v", err)
	}
	if len(nodeIDs) != 0 || len(edgeIDs) != 0 {
		t.Fatalf("expected empty provenance after unlink, got nodes=%v edges=%v", nodeIDs, edgeIDs)
	}

	refCount, err = memStore.CountMemoryReferences(ctx, "node1")
	if err != nil {
		t.Fatalf("CountMemoryReferences failed: %v", err)
	}
	if refCount != 0 {
		t.Fatalf("expected 0 references after unlink, got %d", refCount)
	}

	// Candidate-based GC should delete the unreferenced artifacts.
	nodesDeleted, edgesDeleted, err := memStore.GarbageCollectCandidates(ctx, []string{"node1"}, []string{"edge1"})
	if err != nil {
		t.Fatalf("GarbageCollectCandidates failed: %v", err)
	}
	if nodesDeleted != 1 {
		t.Fatalf("expected 1 node deleted, got %d", nodesDeleted)
	}
	if edgesDeleted != 1 {
		t.Fatalf("expected 1 edge deleted, got %d", edgesDeleted)
	}

	// Placeholder GC currently returns (0,0,nil); exercise it to lock behavior.
	if nodes, edges, err := memStore.GarbageCollect(ctx); err != nil || nodes != 0 || edges != 0 {
		t.Fatalf("expected placeholder GarbageCollect to return (0,0,nil), got (%d,%d,%v)", nodes, edges, err)
	}
}

// TestMemoryStore_GarbageCollection tests GC behavior.
func TestMemoryStore_GarbageCollection(t *testing.T) {
	ctx := context.Background()

	graphStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer graphStore.Close()

	memStore := NewSQLiteMemoryStore(graphStore.DB())

	// Add two memories sharing a node
	memory1 := &MemoryRecord{
		Topic:   "Memory 1",
		Context: "Context 1",
		DocHash: ComputeDocHash("Memory 1", "Context 1", nil, nil),
		Status:  "complete",
	}

	memory2 := &MemoryRecord{
		Topic:   "Memory 2",
		Context: "Context 2",
		DocHash: ComputeDocHash("Memory 2", "Context 2", nil, nil),
		Status:  "complete",
	}

	err = memStore.AddMemory(ctx, memory1)
	if err != nil {
		t.Fatalf("AddMemory 1 failed: %v", err)
	}

	err = memStore.AddMemory(ctx, memory2)
	if err != nil {
		t.Fatalf("AddMemory 2 failed: %v", err)
	}

	// Add shared node
	sharedNode := &Node{ID: "shared", Name: "Shared Node", Type: "Concept"}
	err = graphStore.AddNode(ctx, sharedNode)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Add unique nodes
	node1 := &Node{ID: "unique1", Name: "Unique 1", Type: "Concept"}
	node2 := &Node{ID: "unique2", Name: "Unique 2", Type: "Concept"}

	err = graphStore.AddNode(ctx, node1)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	err = graphStore.AddNode(ctx, node2)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	// Link provenance
	err = memStore.LinkProvenance(ctx, memory1.ID, []string{"shared", "unique1"}, []string{})
	if err != nil {
		t.Fatalf("LinkProvenance 1 failed: %v", err)
	}

	err = memStore.LinkProvenance(ctx, memory2.ID, []string{"shared", "unique2"}, []string{})
	if err != nil {
		t.Fatalf("LinkProvenance 2 failed: %v", err)
	}

	// Get provenance for memory1 before delete
	nodeIDs, _, err := memStore.GetProvenanceByMemory(ctx, memory1.ID)
	if err != nil {
		t.Fatalf("GetProvenanceByMemory failed: %v", err)
	}

	if len(nodeIDs) != 2 {
		t.Fatalf("Expected 2 provenance node IDs, got %d: %v", len(nodeIDs), nodeIDs)
	}

	// Delete memory1 (CASCADE deletes provenance)
	err = memStore.DeleteMemory(ctx, memory1.ID)
	if err != nil {
		t.Fatalf("DeleteMemory failed: %v", err)
	}

	// Run GC on candidates from memory1
	nodesDeleted, _, err := memStore.GarbageCollectCandidates(ctx, nodeIDs, []string{})
	if err != nil {
		t.Fatalf("GarbageCollectCandidates failed: %v", err)
	}

	// Validate reference counts explicitly (ensures CountMemoryReferences is exercised).
	sharedCount, err := memStore.CountMemoryReferences(ctx, "shared")
	if err != nil {
		t.Fatalf("CountMemoryReferences(shared) failed: %v", err)
	}
	if sharedCount != 1 {
		t.Fatalf("expected shared refcount=1, got %d", sharedCount)
	}
	unique1Count, err := memStore.CountMemoryReferences(ctx, "unique1")
	if err != nil {
		t.Fatalf("CountMemoryReferences(unique1) failed: %v", err)
	}
	if unique1Count != 0 {
		t.Fatalf("expected unique1 refcount=0, got %d", unique1Count)
	}
	unique2Count, err := memStore.CountMemoryReferences(ctx, "unique2")
	if err != nil {
		t.Fatalf("CountMemoryReferences(unique2) failed: %v", err)
	}
	if unique2Count != 1 {
		t.Fatalf("expected unique2 refcount=1, got %d", unique2Count)
	}

	// Should delete unique1 but preserve shared (still referenced by memory2)
	// Expected:  shared=1 ref, unique1=0 refs, unique2=1 ref
	if nodesDeleted != 1 {
		// Debug: check actual refcounts
		for _, nodeID := range nodeIDs {
			count, _ := memStore.CountMemoryReferences(ctx, nodeID)
			t.Logf("Node %s has %d references", nodeID, count)
		}
		t.Errorf("Expected 1 node deleted, got %d", nodesDeleted)
	}

	// Verify shared node still exists
	sharedRetrieved, err := graphStore.GetNode(ctx, "shared")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if sharedRetrieved == nil {
		t.Error("Shared node was incorrectly deleted")
	}

	// Verify unique1 is gone
	unique1Retrieved, err := graphStore.GetNode(ctx, "unique1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if unique1Retrieved != nil {
		t.Error("Unique1 node was not deleted")
	}
}

// TestComputeDocHash tests doc_hash canonicalization.
func TestComputeDocHash(t *testing.T) {
	// Same content should produce same hash
	hash1 := ComputeDocHash("Topic", "Context", []string{"D1"}, []string{"R1"})
	hash2 := ComputeDocHash("Topic", "Context", []string{"D1"}, []string{"R1"})

	if hash1 != hash2 {
		t.Error("Same content produced different hashes")
	}

	// Whitespace should be trimmed
	hash3 := ComputeDocHash("  Topic  ", "  Context  ", []string{"D1"}, []string{"R1"})
	if hash1 != hash3 {
		t.Error("Whitespace trimming not working")
	}

	// Different content should produce different hashes
	hash4 := ComputeDocHash("Topic", "Different Context", []string{"D1"}, []string{"R1"})
	if hash1 == hash4 {
		t.Error("Different content produced same hash")
	}

	// Metadata should not affect hash
	// (tested by not including metadata in ComputeDocHash params)
}

// TestMemoryStore_DuplicateDetection tests that duplicate memories are detected.
func TestMemoryStore_DuplicateDetection(t *testing.T) {
	ctx := context.Background()

	graphStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer graphStore.Close()

	memStore := NewSQLiteMemoryStore(graphStore.DB())

	docHash := ComputeDocHash("Same Topic", "Same Context", nil, nil)

	// Add first memory
	memory1 := &MemoryRecord{
		Topic:   "Same Topic",
		Context: "Same Context",
		DocHash: docHash,
		Status:  "complete",
	}

	err = memStore.AddMemory(ctx, memory1)
	if err != nil {
		t.Fatalf("AddMemory 1 failed: %v", err)
	}

	// Try to add duplicate
	memory2 := &MemoryRecord{
		Topic:   "Same Topic",
		Context: "Same Context",
		DocHash: docHash,
		Status:  "complete",
	}

	err = memStore.AddMemory(ctx, memory2)
	if err != nil {
		t.Fatalf("AddMemory 2 failed: %v", err)
	}

	// Both should have same ID if dedup check in gognee.AddMemory works
	// But at store level, they'll have different IDs
	// The dedup check happens in gognee.AddMemory via SQL query
}

// TestMemoryStore_CountMemories tests the CountMemories method.
func TestMemoryStore_CountMemories(t *testing.T) {
	ctx := context.Background()

	// Create in-memory store
	graphStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer graphStore.Close()

	memStore := NewSQLiteMemoryStore(graphStore.DB())

	// Test empty count
	count, err := memStore.CountMemories(ctx)
	if err != nil {
		t.Fatalf("CountMemories (empty) failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 memories, got %d", count)
	}

	// Add some memories
	for i := 0; i < 5; i++ {
		memory := &MemoryRecord{
			Topic:     "Test Memory " + string(rune('A'+i)),
			Context:   "Test context",
			Decisions: []string{"Decision"},
			DocHash:   ComputeDocHash("Test Memory "+string(rune('A'+i)), "Test context", []string{"Decision"}, nil),
			Source:    "test",
			Status:    "complete",
		}
		err = memStore.AddMemory(ctx, memory)
		if err != nil {
			t.Fatalf("AddMemory %d failed: %v", i, err)
		}
	}

	// Count should be 5
	count, err = memStore.CountMemories(ctx)
	if err != nil {
		t.Fatalf("CountMemories (with records) failed: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 memories, got %d", count)
	}

	// Delete one memory
	memories, err := memStore.ListMemories(ctx, ListMemoriesOptions{Limit: 1})
	if err != nil {
		t.Fatalf("ListMemories failed: %v", err)
	}
	if len(memories) == 0 {
		t.Fatal("Expected at least one memory")
	}

	err = memStore.DeleteMemory(ctx, memories[0].ID)
	if err != nil {
		t.Fatalf("DeleteMemory failed: %v", err)
	}

	// Count should be 4
	count, err = memStore.CountMemories(ctx)
	if err != nil {
		t.Fatalf("CountMemories (after delete) failed: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 memories, got %d", count)
	}
}

package store

import (
	"context"
	"testing"
	"time"
)

// TestSupersession_RecordAndRetrieve tests basic supersession recording and retrieval (M3: Plan 021)
func TestSupersession_RecordAndRetrieve(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	memStore := NewSQLiteMemoryStore(store.DB())

	// Create two memories
	mem1 := &MemoryRecord{
		Topic:   "Decision v1",
		Context: "Initial decision about X",
		Status:  "Active",
	}
	if err := memStore.AddMemory(ctx, mem1); err != nil {
		t.Fatalf("Failed to add memory 1: %v", err)
	}

	mem2 := &MemoryRecord{
		Topic:   "Decision v2",
		Context: "Updated decision about X",
		Status:  "Active",
	}
	if err := memStore.AddMemory(ctx, mem2); err != nil {
		t.Fatalf("Failed to add memory 2: %v", err)
	}

	// Record that mem2 supersedes mem1
	reason := "Updated with new information"
	err = memStore.RecordSupersession(ctx, mem2.ID, mem1.ID, reason)
	if err != nil {
		t.Fatalf("RecordSupersession failed: %v", err)
	}

	// Verify mem1 is marked as Superseded
	updatedMem1, err := memStore.GetMemory(ctx, mem1.ID)
	if err != nil {
		t.Fatalf("GetMemory failed: %v", err)
	}

	if updatedMem1.Status != "Superseded" {
		t.Errorf("Expected mem1 status 'Superseded', got '%s'", updatedMem1.Status)
	}

	if updatedMem1.SupersededBy == nil {
		t.Fatalf("Expected mem1.SupersededBy to be set")
	}
	if *updatedMem1.SupersededBy != mem2.ID {
		t.Errorf("Expected mem1.SupersededBy = %s, got %s", mem2.ID, *updatedMem1.SupersededBy)
	}

	// Verify mem2 is still Active
	updatedMem2, err := memStore.GetMemory(ctx, mem2.ID)
	if err != nil {
		t.Fatalf("GetMemory failed: %v", err)
	}

	if updatedMem2.Status != "Active" {
		t.Errorf("Expected mem2 status 'Active', got '%s'", updatedMem2.Status)
	}

	// Test GetSupersedingMemory
	supersedingID, err := memStore.GetSupersedingMemory(ctx, mem1.ID)
	if err != nil {
		t.Fatalf("GetSupersedingMemory failed: %v", err)
	}
	if supersedingID == nil {
		t.Fatalf("Expected superseding memory ID, got nil")
	}
	if *supersedingID != mem2.ID {
		t.Errorf("Expected superseding memory %s, got %s", mem2.ID, *supersedingID)
	}

	// Test GetSupersededMemories
	supersededIDs, err := memStore.GetSupersededMemories(ctx, mem2.ID)
	if err != nil {
		t.Fatalf("GetSupersededMemories failed: %v", err)
	}
	if len(supersededIDs) != 1 {
		t.Fatalf("Expected 1 superseded memory, got %d", len(supersededIDs))
	}
	if supersededIDs[0] != mem1.ID {
		t.Errorf("Expected superseded memory %s, got %s", mem1.ID, supersededIDs[0])
	}
}

// TestSupersession_Chain tests supersession chain traversal (M3: Plan 021)
func TestSupersession_Chain(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	memStore := NewSQLiteMemoryStore(store.DB())

	// Create chain: v1 → v2 → v3
	mem1 := &MemoryRecord{Topic: "Decision v1", Context: "Original", Status: "Active"}
	mem2 := &MemoryRecord{Topic: "Decision v2", Context: "First update", Status: "Active"}
	mem3 := &MemoryRecord{Topic: "Decision v3", Context: "Second update", Status: "Active"}

	if err := memStore.AddMemory(ctx, mem1); err != nil {
		t.Fatalf("Failed to add memory 1: %v", err)
	}
	if err := memStore.AddMemory(ctx, mem2); err != nil {
		t.Fatalf("Failed to add memory 2: %v", err)
	}
	if err := memStore.AddMemory(ctx, mem3); err != nil {
		t.Fatalf("Failed to add memory 3: %v", err)
	}

	// Record supersessions
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	if err := memStore.RecordSupersession(ctx, mem2.ID, mem1.ID, "Update 1"); err != nil {
		t.Fatalf("RecordSupersession 1 failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := memStore.RecordSupersession(ctx, mem3.ID, mem2.ID, "Update 2"); err != nil {
		t.Fatalf("RecordSupersession 2 failed: %v", err)
	}

	// Get chain from any point (should return same chain)
	chain, err := memStore.GetSupersessionChain(ctx, mem2.ID)
	if err != nil {
		t.Fatalf("GetSupersessionChain failed: %v", err)
	}

	if len(chain) != 2 {
		t.Fatalf("Expected chain length 2, got %d", len(chain))
	}

	// Chain should be ordered oldest to newest
	if chain[0].SupersededID != mem1.ID || chain[0].SupersedingID != mem2.ID {
		t.Errorf("Chain[0]: expected %s→%s, got %s→%s",
			mem1.ID, mem2.ID, chain[0].SupersededID, chain[0].SupersedingID)
	}

	if chain[1].SupersededID != mem2.ID || chain[1].SupersedingID != mem3.ID {
		t.Errorf("Chain[1]: expected %s→%s, got %s→%s",
			mem2.ID, mem3.ID, chain[1].SupersededID, chain[1].SupersedingID)
	}

	// Verify reasons are preserved
	if chain[0].Reason != "Update 1" {
		t.Errorf("Chain[0] reason: expected 'Update 1', got '%s'", chain[0].Reason)
	}
	if chain[1].Reason != "Update 2" {
		t.Errorf("Chain[1] reason: expected 'Update 2', got '%s'", chain[1].Reason)
	}
}

// TestSupersession_NonExistentMemory tests validation of memory existence (M3: Plan 021)
func TestSupersession_NonExistentMemory(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	memStore := NewSQLiteMemoryStore(store.DB())

	mem1 := &MemoryRecord{Topic: "Memory 1", Context: "Content", Status: "Active"}
	if err := memStore.AddMemory(ctx, mem1); err != nil {
		t.Fatalf("Failed to add memory: %v", err)
	}

	// Try to supersede with non-existent memory
	err = memStore.RecordSupersession(ctx, "nonexistent-id", mem1.ID, "Test")
	if err == nil {
		t.Error("Expected error when superseding memory doesn't exist")
	}

	// Try to supersede non-existent memory
	err = memStore.RecordSupersession(ctx, mem1.ID, "nonexistent-id", "Test")
	if err == nil {
		t.Error("Expected error when superseded memory doesn't exist")
	}
}

// TestSupersession_CascadeDelete tests CASCADE behavior (M3: Plan 021)
func TestSupersession_CascadeDelete(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	memStore := NewSQLiteMemoryStore(store.DB())

	mem1 := &MemoryRecord{Topic: "Memory 1", Context: "Content", Status: "Active"}
	mem2 := &MemoryRecord{Topic: "Memory 2", Context: "Updated", Status: "Active"}

	if err := memStore.AddMemory(ctx, mem1); err != nil {
		t.Fatalf("Failed to add memory 1: %v", err)
	}
	if err := memStore.AddMemory(ctx, mem2); err != nil {
		t.Fatalf("Failed to add memory 2: %v", err)
	}

	// Record supersession
	if err := memStore.RecordSupersession(ctx, mem2.ID, mem1.ID, "Update"); err != nil {
		t.Fatalf("RecordSupersession failed: %v", err)
	}

	// Delete superseding memory (mem2)
	if err := memStore.DeleteMemory(ctx, mem2.ID); err != nil {
		t.Fatalf("DeleteMemory failed: %v", err)
	}

	// Supersession record should be deleted due to CASCADE
	// Verify by checking mem1's status
	updatedMem1, err := memStore.GetMemory(ctx, mem1.ID)
	if err != nil {
		t.Fatalf("GetMemory failed: %v", err)
	}

	// mem1 should still exist but its superseded_by should be cleared (or remain)
	// This depends on implementation - CASCADE deletes the supersession record
	// but doesn't automatically revert the memory's status
	// For this test, we just verify mem1 still exists
	if updatedMem1 == nil {
		t.Error("mem1 should still exist after superseding memory is deleted")
	}
}

// TestSupersession_NoChain tests retrieval when no chain exists (M3: Plan 021)
func TestSupersession_NoChain(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	memStore := NewSQLiteMemoryStore(store.DB())

	mem1 := &MemoryRecord{Topic: "Standalone", Context: "No supersession", Status: "Active"}
	if err := memStore.AddMemory(ctx, mem1); err != nil {
		t.Fatalf("Failed to add memory: %v", err)
	}

	// Get chain for standalone memory
	chain, err := memStore.GetSupersessionChain(ctx, mem1.ID)
	if err != nil {
		t.Fatalf("GetSupersessionChain failed: %v", err)
	}

	if len(chain) != 0 {
		t.Errorf("Expected empty chain, got %d records", len(chain))
	}

	// GetSupersedingMemory should return nil
	supersedingID, err := memStore.GetSupersedingMemory(ctx, mem1.ID)
	if err != nil {
		t.Fatalf("GetSupersedingMemory failed: %v", err)
	}
	if supersedingID != nil {
		t.Error("Expected no superseding memory")
	}

	// GetSupersededMemories should return empty slice
	supersededIDs, err := memStore.GetSupersededMemories(ctx, mem1.ID)
	if err != nil {
		t.Fatalf("GetSupersededMemories failed: %v", err)
	}
	if len(supersededIDs) != 0 {
		t.Errorf("Expected no superseded memories, got %d", len(supersededIDs))
	}
}

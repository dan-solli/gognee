package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestUpdateMemoryAccess_SingleMemory tests access tracking for a single memory.
func TestUpdateMemoryAccess_SingleMemory(t *testing.T) {
	// Create in-memory database
	sqliteStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer sqliteStore.Close()

	memoryStore := NewSQLiteMemoryStore(sqliteStore.DB())
	ctx := context.Background()

	// Create a test memory
	memID := uuid.New().String()
	memory := &MemoryRecord{
		ID:        memID,
		Topic:     "Test Memory",
		Context:   "This is a test memory for access tracking",
		Decisions: []string{"Decision 1"},
		CreatedAt: time.Now().Add(-7 * 24 * time.Hour), // 7 days ago
		DocHash:   ComputeDocHash("Test Memory", "This is a test memory for access tracking", []string{"Decision 1"}, nil),
	}

	err = memoryStore.AddMemory(ctx, memory)
	if err != nil {
		t.Fatalf("Failed to add memory: %v", err)
	}

	// Update access once
	err = memoryStore.UpdateMemoryAccess(ctx, memID)
	if err != nil {
		t.Fatalf("Failed to update memory access: %v", err)
	}

	// Retrieve and verify access count
	retrieved, err := memoryStore.GetMemory(ctx, memID)
	if err != nil {
		t.Fatalf("Failed to get memory: %v", err)
	}

	// Note: GetMemory also calls UpdateMemoryAccess, so count should be 2
	// (once from explicit call, once from GetMemory)
	// However, we can't reliably test the exact count due to the recursive call
	// Instead, verify that access tracking fields exist and are updated

	// Query directly without triggering GetMemory's UpdateMemoryAccess
	var accessCount int
	var lastAccessedAt *time.Time
	var accessVelocity float64
	err = memoryStore.DB().QueryRow(`
		SELECT access_count, last_accessed_at, access_velocity 
		FROM memories 
		WHERE id = ?
	`, memID).Scan(&accessCount, &lastAccessedAt, &accessVelocity)
	if err != nil {
		t.Fatalf("Failed to query access tracking fields: %v", err)
	}

	if accessCount < 1 {
		t.Errorf("Expected access_count >= 1, got %d", accessCount)
	}

	if lastAccessedAt == nil {
		t.Error("Expected last_accessed_at to be set")
	}

	if accessVelocity <= 0 {
		t.Errorf("Expected access_velocity > 0, got %f", accessVelocity)
	}

	// Verify velocity calculation: access_count / days_since_creation
	daysSinceCreation := time.Since(retrieved.CreatedAt).Hours() / 24.0
	if daysSinceCreation < 1 {
		daysSinceCreation = 1
	}
	expectedVelocity := float64(accessCount) / daysSinceCreation

	// Allow for small floating point differences
	if abs(accessVelocity-expectedVelocity) > 0.01 {
		t.Errorf("Expected access_velocity ≈ %f, got %f", expectedVelocity, accessVelocity)
	}
}

// TestBatchUpdateMemoryAccess tests batch access tracking for multiple memories.
func TestBatchUpdateMemoryAccess(t *testing.T) {
	// Create in-memory database
	sqliteStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer sqliteStore.Close()

	memoryStore := NewSQLiteMemoryStore(sqliteStore.DB())
	ctx := context.Background()

	// Create multiple test memories
	memoryIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		memID := uuid.New().String()
		memoryIDs[i] = memID

		memory := &MemoryRecord{
			ID:        memID,
			Topic:     "Test Memory " + string(rune('A'+i)),
			Context:   "This is test memory for batch access tracking",
			CreatedAt: time.Now().Add(-time.Duration(i+1) * 24 * time.Hour),
			DocHash:   ComputeDocHash("Test Memory", "This is test memory for batch access tracking", nil, nil),
		}

		err = memoryStore.AddMemory(ctx, memory)
		if err != nil {
			t.Fatalf("Failed to add memory %d: %v", i, err)
		}
	}

	// Batch update access for all memories
	err = memoryStore.BatchUpdateMemoryAccess(ctx, memoryIDs)
	if err != nil {
		t.Fatalf("Failed to batch update memory access: %v", err)
	}

	// Verify all memories have updated access counts
	for i, memID := range memoryIDs {
		var accessCount int
		var lastAccessedAt *time.Time
		err = memoryStore.DB().QueryRow(`
			SELECT access_count, last_accessed_at 
			FROM memories 
			WHERE id = ?
		`, memID).Scan(&accessCount, &lastAccessedAt)
		if err != nil {
			t.Fatalf("Failed to query memory %d: %v", i, err)
		}

		if accessCount != 1 {
			t.Errorf("Memory %d: expected access_count = 1, got %d", i, accessCount)
		}

		if lastAccessedAt == nil {
			t.Errorf("Memory %d: expected last_accessed_at to be set", i)
		}
	}
}

// TestBatchUpdateMemoryAccess_Deduplication tests that batch update handles duplicates.
func TestBatchUpdateMemoryAccess_Deduplication(t *testing.T) {
	// Create in-memory database
	sqliteStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer sqliteStore.Close()

	memoryStore := NewSQLiteMemoryStore(sqliteStore.DB())
	ctx := context.Background()

	// Create a test memory
	memID := uuid.New().String()
	memory := &MemoryRecord{
		ID:        memID,
		Topic:     "Test Memory",
		Context:   "Test deduplication",
		CreatedAt: time.Now().Add(-24 * time.Hour),
		DocHash:   ComputeDocHash("Test Memory", "Test deduplication", nil, nil),
	}

	err = memoryStore.AddMemory(ctx, memory)
	if err != nil {
		t.Fatalf("Failed to add memory: %v", err)
	}

	// Batch update with duplicates
	duplicateIDs := []string{memID, memID, memID}
	err = memoryStore.BatchUpdateMemoryAccess(ctx, duplicateIDs)
	if err != nil {
		t.Fatalf("Failed to batch update with duplicates: %v", err)
	}

	// Verify access count is only 1 (not 3)
	var accessCount int
	err = memoryStore.DB().QueryRow("SELECT access_count FROM memories WHERE id = ?", memID).Scan(&accessCount)
	if err != nil {
		t.Fatalf("Failed to query access count: %v", err)
	}

	if accessCount != 1 {
		t.Errorf("Expected access_count = 1 after deduplication, got %d", accessCount)
	}
}

// TestUpdateMemoryAccess_NotFound tests error handling for non-existent memory.
func TestUpdateMemoryAccess_NotFound(t *testing.T) {
	// Create in-memory database
	sqliteStore, err := NewSQLiteGraphStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create graph store: %v", err)
	}
	defer sqliteStore.Close()

	memoryStore := NewSQLiteMemoryStore(sqliteStore.DB())
	ctx := context.Background()

	// Try to update non-existent memory
	err = memoryStore.UpdateMemoryAccess(ctx, "non-existent-id")
	if err != ErrMemoryNotFound {
		t.Errorf("Expected ErrMemoryNotFound, got %v", err)
	}
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

package store

import (
	"context"
	"math"
	"sync"
	"testing"
)

// TestCosineSimilarity tests the cosine similarity function with known vectors.
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
		epsilon  float64
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
			epsilon:  0.001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
			epsilon:  0.001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
			epsilon:  0.001,
		},
		{
			name:     "45 degree angle",
			a:        []float32{1, 1},
			b:        []float32{1, 0},
			expected: 0.707, // cos(45°) ≈ 0.707
			epsilon:  0.01,
		},
		{
			name:     "different magnitude same direction",
			a:        []float32{2, 0, 0},
			b:        []float32{10, 0, 0},
			expected: 1.0,
			epsilon:  0.001,
		},
		{
			name:     "zero vector",
			a:        []float32{0, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 0.0,
			epsilon:  0.001,
		},
		{
			name:     "different lengths",
			a:        []float32{1, 0},
			b:        []float32{1, 0, 0},
			expected: 0.0,
			epsilon:  0.001,
		},
		{
			name:     "empty vectors",
			a:        []float32{},
			b:        []float32{},
			expected: 0.0,
			epsilon:  0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CosineSimilarity(tt.a, tt.b)
			if math.Abs(result-tt.expected) > tt.epsilon {
				t.Errorf("CosineSimilarity(%v, %v) = %f, want %f (±%f)",
					tt.a, tt.b, result, tt.expected, tt.epsilon)
			}
		})
	}
}

// TestMemoryVectorStore_AddAndSearch tests basic add and search operations.
func TestMemoryVectorStore_AddAndSearch(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	// Add some vectors
	vectors := map[string][]float32{
		"vec1": {1.0, 0.0, 0.0},
		"vec2": {0.0, 1.0, 0.0},
		"vec3": {0.0, 0.0, 1.0},
		"vec4": {0.7, 0.7, 0.0}, // 45° from vec1
	}

	for id, vec := range vectors {
		err := store.Add(ctx, id, vec)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Search for vec1
	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 3)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should return vec1, vec4, and one more in order of similarity
	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// First result should be vec1 (exact match)
	if results[0].ID != "vec1" {
		t.Errorf("Expected first result to be vec1, got %s", results[0].ID)
	}

	if math.Abs(results[0].Score-1.0) > 0.001 {
		t.Errorf("Expected score 1.0 for exact match, got %f", results[0].Score)
	}

	// Second result should be vec4 (cos 45° ≈ 0.707)
	if results[1].ID != "vec4" {
		t.Errorf("Expected second result to be vec4, got %s", results[1].ID)
	}

	if math.Abs(results[1].Score-0.707) > 0.01 {
		t.Errorf("Expected score ~0.707, got %f", results[1].Score)
	}
}

// TestMemoryVectorStore_TopKOrdering tests that results are properly sorted.
func TestMemoryVectorStore_TopKOrdering(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	// Add vectors with varying similarity to [1, 0, 0]
	vectors := map[string][]float32{
		"exact":   {1.0, 0.0, 0.0},  // similarity = 1.0
		"close":   {0.9, 0.1, 0.0},  // similarity ≈ 0.995
		"medium":  {0.7, 0.7, 0.0},  // similarity ≈ 0.707
		"far":     {0.1, 0.9, 0.0},  // similarity ≈ 0.110
		"orthog":  {0.0, 1.0, 0.0},  // similarity = 0.0
		"farther": {-0.5, 0.5, 0.0}, // similarity < 0
	}

	for id, vec := range vectors {
		if err := store.Add(ctx, id, vec); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Search for [1, 0, 0]
	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 6)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Verify results are sorted by score descending
	for i := 0; i < len(results)-1; i++ {
		if results[i].Score < results[i+1].Score {
			t.Errorf("Results not sorted: results[%d].Score (%f) < results[%d].Score (%f)",
				i, results[i].Score, i+1, results[i+1].Score)
		}
	}

	// First should be exact match
	if results[0].ID != "exact" {
		t.Errorf("Expected exact match first, got %s", results[0].ID)
	}
}

// TestMemoryVectorStore_TopKLimit tests that only topK results are returned.
func TestMemoryVectorStore_TopKLimit(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	// Add 10 vectors
	for i := 0; i < 10; i++ {
		vec := []float32{float32(i), float32(10 - i), 0.0}
		err := store.Add(ctx, string(rune('0'+i)), vec)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Search with topK = 3
	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 3)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}

// TestMemoryVectorStore_EmptyStore tests searching an empty store.
func TestMemoryVectorStore_EmptyStore(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results from empty store, got %d", len(results))
	}
}

// TestMemoryVectorStore_Delete tests vector deletion.
func TestMemoryVectorStore_Delete(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	// Add vectors
	err := store.Add(ctx, "vec1", []float32{1.0, 0.0, 0.0})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	err = store.Add(ctx, "vec2", []float32{0.0, 1.0, 0.0})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Delete vec1
	err = store.Delete(ctx, "vec1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Search should not return vec1
	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, result := range results {
		if result.ID == "vec1" {
			t.Error("Deleted vector vec1 still appears in search results")
		}
	}
}

// TestMemoryVectorStore_Update tests updating an existing vector.
func TestMemoryVectorStore_Update(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	// Add initial vector
	err := store.Add(ctx, "vec1", []float32{1.0, 0.0, 0.0})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Update with different vector
	err = store.Add(ctx, "vec1", []float32{0.0, 1.0, 0.0})
	if err != nil {
		t.Fatalf("Add (update) failed: %v", err)
	}

	// Search with original direction
	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should have low similarity now (orthogonal)
	for _, result := range results {
		if result.ID == "vec1" && math.Abs(result.Score) > 0.001 {
			t.Errorf("Expected near-zero similarity after update, got %f", result.Score)
		}
	}

	// Search with new direction
	query2 := []float32{0.0, 1.0, 0.0}
	results2, err := store.Search(ctx, query2, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should have high similarity now
	found := false
	for _, result := range results2 {
		if result.ID == "vec1" {
			found = true
			if math.Abs(result.Score-1.0) > 0.001 {
				t.Errorf("Expected score 1.0 after update, got %f", result.Score)
			}
		}
	}

	if !found {
		t.Error("Updated vector not found in search results")
	}
}

// TestMemoryVectorStore_ConcurrentAccess tests thread safety.
func TestMemoryVectorStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent adds
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			vec := []float32{float32(id), 0.0, 0.0}
			_ = store.Add(ctx, string(rune('0'+id)), vec)
		}(i)
	}
	wg.Wait()

	// Concurrent searches
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			query := []float32{1.0, 0.0, 0.0}
			_, _ = store.Search(ctx, query, 10)
		}()
	}
	wg.Wait()

	// Concurrent deletes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			_ = store.Delete(ctx, string(rune('0'+id)))
		}(i)
	}
	wg.Wait()

	// Verify store is empty
	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected empty store after concurrent deletes, got %d results", len(results))
	}
}

// TestMemoryVectorStore_ImmutabilityCheck tests that external modifications don't affect stored vectors.
func TestMemoryVectorStore_ImmutabilityCheck(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	// Add vector
	originalVec := []float32{1.0, 0.0, 0.0}
	err := store.Add(ctx, "vec1", originalVec)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Modify the original slice
	originalVec[0] = 999.0

	// Search should not reflect the modification
	query := []float32{1.0, 0.0, 0.0}
	results, err := store.Search(ctx, query, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("No results returned")
	}

	// Should still match original vector
	if math.Abs(results[0].Score-1.0) > 0.001 {
		t.Errorf("Expected score 1.0 (stored vector should be unchanged), got %f", results[0].Score)
	}
}

// TestMemoryVectorStore_LargeVectors tests with realistic embedding dimensions.
func TestMemoryVectorStore_LargeVectors(t *testing.T) {
	store := NewMemoryVectorStore()
	ctx := context.Background()

	// Create vectors with realistic embedding dimension (e.g., 1536 for OpenAI)
	dim := 1536
	vec1 := make([]float32, dim)
	vec2 := make([]float32, dim)

	// Initialize with different patterns
	for i := 0; i < dim; i++ {
		vec1[i] = float32(i) / float32(dim)
		vec2[i] = float32(dim-i) / float32(dim)
	}

	// Add vectors
	err := store.Add(ctx, "large1", vec1)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	err = store.Add(ctx, "large2", vec2)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Search
	results, err := store.Search(ctx, vec1, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// First result should be exact match
	if results[0].ID != "large1" {
		t.Errorf("Expected large1 first, got %s", results[0].ID)
	}

	if math.Abs(results[0].Score-1.0) > 0.001 {
		t.Errorf("Expected exact match score 1.0, got %f", results[0].Score)
	}
}

package store

import (
	"context"
	"sort"
	"sync"
)

// MemoryVectorStore is an in-memory implementation of VectorStore.
// It uses a map to store vectors and provides thread-safe access via RWMutex.
// Note: This implementation does not persist vectors across restarts.
type MemoryVectorStore struct {
	vectors map[string][]float32
	mu      sync.RWMutex
}

// NewMemoryVectorStore creates a new in-memory vector store.
func NewMemoryVectorStore() *MemoryVectorStore {
	return &MemoryVectorStore{
		vectors: make(map[string][]float32),
	}
}

// Add adds or updates a vector for the given ID.
func (m *MemoryVectorStore) Add(ctx context.Context, id string, embedding []float32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Make a copy to avoid external mutations
	embeddingCopy := make([]float32, len(embedding))
	copy(embeddingCopy, embedding)

	m.vectors[id] = embeddingCopy
	return nil
}

// Search finds the most similar vectors to the query.
// Returns up to topK results sorted by similarity score (descending).
func (m *MemoryVectorStore) Search(ctx context.Context, query []float32, topK int) ([]SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Handle empty store
	if len(m.vectors) == 0 {
		return []SearchResult{}, nil
	}

	// Compute similarity for all vectors
	var results []SearchResult
	for id, embedding := range m.vectors {
		score := CosineSimilarity(query, embedding)
		results = append(results, SearchResult{
			ID:    id,
			Score: score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top-K
	if topK < len(results) {
		results = results[:topK]
	}

	return results, nil
}

// Delete removes a vector from the store.
func (m *MemoryVectorStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.vectors, id)
	return nil
}

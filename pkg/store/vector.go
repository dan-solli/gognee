package store

import (
	"context"
	"math"
)

// SearchResult represents a vector search result with similarity score.
type SearchResult struct {
	ID    string  // Node ID
	Score float64 // Cosine similarity score (0-1, higher is more similar)
}

// VectorStore defines the interface for vector storage and similarity search.
type VectorStore interface {
	// Add adds or updates a vector for the given ID.
	Add(ctx context.Context, id string, embedding []float32) error

	// Search finds the most similar vectors to the query.
	// Returns up to topK results sorted by similarity score (descending).
	Search(ctx context.Context, query []float32, topK int) ([]SearchResult, error)

	// Delete removes a vector from the store.
	Delete(ctx context.Context, id string) error
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical direction,
// 0 means orthogonal, and -1 means opposite direction.
// For normalized vectors (embeddings), the result is typically between 0 and 1.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	if len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64

	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

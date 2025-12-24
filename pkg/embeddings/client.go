package embeddings

import "context"

// EmbeddingClient defines the interface for generating text embeddings
type EmbeddingClient interface {
	// Embed generates embeddings for multiple texts
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// EmbedOne generates an embedding for a single text
	EmbedOne(ctx context.Context, text string) ([]float32, error)
}

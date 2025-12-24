package search

import (
	"context"

	"github.com/dan-solli/gognee/pkg/embeddings"
	"github.com/dan-solli/gognee/pkg/store"
)

// VectorSearcher performs vector similarity search.
type VectorSearcher struct {
	embeddings  embeddings.EmbeddingClient
	vectorStore store.VectorStore
	graphStore  store.GraphStore
}

// NewVectorSearcher creates a new vector searcher.
func NewVectorSearcher(
	embClient embeddings.EmbeddingClient,
	vectorStore store.VectorStore,
	graphStore store.GraphStore,
) *VectorSearcher {
	return &VectorSearcher{
		embeddings:  embClient,
		vectorStore: vectorStore,
		graphStore:  graphStore,
	}
}

// Search performs vector similarity search.
// It embeds the query, searches the vector store, and enriches results with full node data.
func (v *VectorSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	applyDefaults(&opts)

	// Embed the query
	embedding, err := v.embeddings.EmbedOne(ctx, query)
	if err != nil {
		return nil, err
	}

	// Vector search
	vectorResults, err := v.vectorStore.Search(ctx, embedding, opts.TopK)
	if err != nil {
		return nil, err
	}

	// Enrich with full node data
	results := make([]SearchResult, 0, len(vectorResults))
	for _, vr := range vectorResults {
		node, err := v.graphStore.GetNode(ctx, vr.ID)
		if err != nil {
			return nil, err
		}

		// Skip if node not found (stale vector index)
		if node == nil {
			continue
		}

		results = append(results, SearchResult{
			NodeID:     vr.ID,
			Node:       node,
			Score:      vr.Score,
			Source:     "vector",
			GraphDepth: 0,
		})
	}

	return results, nil
}

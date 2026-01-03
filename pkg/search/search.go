// Package search provides search implementations for gognee's knowledge graph.
package search

import (
	"context"

	"github.com/dan-solli/gognee/pkg/store"
)

// SearchType specifies the type of search to perform.
type SearchType string

const (
	// SearchTypeVector performs vector similarity search only.
	SearchTypeVector SearchType = "vector"

	// SearchTypeGraph performs graph traversal search only (requires seed nodes).
	SearchTypeGraph SearchType = "graph"

	// SearchTypeHybrid combines vector similarity and graph traversal.
	SearchTypeHybrid SearchType = "hybrid"
)

// SearchResult represents a single search result with scoring metadata.
type SearchResult struct {
	NodeID string      // Unique identifier of the node
	Node   *store.Node // Full node data (nil if node was deleted)
	Score  float64     // Combined relevance score (higher is better)
	Source string      // Origin: "vector", "graph", or "hybrid"
	// GraphDepth indicates the minimum graph distance from the search origin.
	// 0 for direct vector hits, >0 for nodes discovered via graph expansion.
	GraphDepth int
	// MemoryIDs lists memory IDs that reference this node (v1.0.0+).
	// Sorted by memory updated_at DESC (most recent first).
	// Empty for legacy nodes (created via Add/Cognify without provenance).
	MemoryIDs []string
}

// SearchOptions configures search behavior.
type SearchOptions struct {
	Type       SearchType // Type of search to perform
	TopK       int        // Maximum number of results to return (default: 10)
	GraphDepth int        // Maximum graph traversal depth (default: 1)
	// SeedNodeIDs specifies starting nodes for graph search.
	// Required for SearchTypeGraph; ignored for SearchTypeVector.
	// For SearchTypeHybrid, seeds augment vector results.
	SeedNodeIDs []string
	// IncludeMemoryIDs enables memory provenance enrichment (v1.0.0+).
	// Default: true. Set to false to skip provenance lookup for performance.
	IncludeMemoryIDs *bool
}

// Searcher defines the interface for knowledge graph search.
type Searcher interface {
	// Search performs a search based on the query and options.
	// For vector/hybrid search, query is the text to embed and search.
	// For graph search, query is ignored (uses opts.SeedNodeIDs instead).
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
}

// ApplyDefaults sets default values for unspecified search options.
func ApplyDefaults(opts *SearchOptions) {
	if opts.TopK <= 0 {
		opts.TopK = 10
	}
	if opts.GraphDepth <= 0 {
		opts.GraphDepth = 1
	}
}

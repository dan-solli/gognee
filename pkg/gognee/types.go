package gognee

import (
	"github.com/dan-solli/gognee/pkg/search"
	"github.com/dan-solli/gognee/pkg/store"
)

// Type re-exports for caller convenience

// SearchResult is re-exported from search package
type SearchResult = search.SearchResult

// SearchOptions is re-exported from search package
type SearchOptions = search.SearchOptions

// SearchType is re-exported from search package
type SearchType = search.SearchType

// SearchType constants re-exported from search package
const (
	SearchTypeVector = search.SearchTypeVector
	SearchTypeGraph  = search.SearchTypeGraph
	SearchTypeHybrid = search.SearchTypeHybrid
)

// Node is re-exported from store package
type Node = store.Node

// Edge is re-exported from store package
type Edge = store.Edge

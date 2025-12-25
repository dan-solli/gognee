// Package store provides storage implementations for gognee's knowledge graph.
package store

import (
	"context"
	"errors"
	"time"
)

// Node represents a knowledge graph entity with embeddings and metadata.
type Node struct {
	ID             string                 // Unique identifier (UUID)
	Name           string                 // Entity name
	Type           string                 // Entity type (Person, Concept, System, etc.)
	Description    string                 // Brief description
	Embedding      []float32              // Vector embedding for semantic search
	CreatedAt      time.Time              // Timestamp of creation
	LastAccessedAt *time.Time             // Timestamp of last access (for decay tracking)
	Metadata       map[string]interface{} // Additional metadata as JSON
}

// Edge represents a relationship between two nodes in the knowledge graph.
type Edge struct {
	ID        string    // Unique identifier (UUID)
	SourceID  string    // Source node ID
	Relation  string    // Relationship type (USES, DEPENDS_ON, etc.)
	TargetID  string    // Target node ID
	Weight    float64   // Relationship weight (default 1.0, reserved for future ranking)
	CreatedAt time.Time // Timestamp of creation
}

// GraphStore defines the interface for graph storage operations.
// Implementations must provide persistent storage for nodes and edges,
// supporting both direct access and graph traversal operations.
type GraphStore interface {
	// AddNode adds or updates a node in the graph.
	// Uses upsert semantics (INSERT OR REPLACE by ID).
	AddNode(ctx context.Context, node *Node) error

	// GetNode retrieves a node by its ID.
	// Returns (nil, nil) if the node is not found (no error).
	GetNode(ctx context.Context, id string) (*Node, error)

	// FindNodesByName searches for nodes by name using case-insensitive matching.
	// Returns all matching nodes ordered deterministically (by created_at, then id).
	// Callers must handle ambiguity when multiple matches are returned.
	FindNodesByName(ctx context.Context, name string) ([]*Node, error)

	// FindNodeByName is a convenience method that returns a single node if and only if
	// exactly one node matches the name (case-insensitive).
	// Returns an error if zero matches (not found) or multiple matches (ambiguous).
	FindNodeByName(ctx context.Context, name string) (*Node, error)

	// AddEdge adds or updates an edge in the graph.
	// Uses upsert semantics (INSERT OR REPLACE by ID).
	// If Edge.ID is empty, a new UUID will be generated.
	AddEdge(ctx context.Context, edge *Edge) error

	// GetEdges retrieves all edges incident to a node (both incoming and outgoing).
	// This is direction-agnostic: returns edges where the node is either source or target.
	// Returns an empty slice if no edges are found.
	// Cognee-aligned: treats adjacency as undirected for discovery.
	GetEdges(ctx context.Context, nodeID string) ([]*Edge, error)

	// GetNeighbors retrieves all nodes adjacent to a given node, up to the specified depth.
	// Depth=1 returns direct neighbors only (Cognee-aligned default).
	// Depth>1 recursively traverses the graph (gognee extension).
	// Traversal is direction-agnostic (treats edges as undirected).
	// Returns unique nodes only (no duplicates).
	GetNeighbors(ctx context.Context, nodeID string, depth int) ([]*Node, error)

	// NodeCount returns the total number of nodes in the graph.
	NodeCount(ctx context.Context) (int64, error)

	// EdgeCount returns the total number of edges in the graph.
	EdgeCount(ctx context.Context) (int64, error)

	// Close releases any resources held by the store (e.g., database connections).
	Close() error
}

// ErrNodeNotFound indicates that no node was found for the given criteria.
var ErrNodeNotFound = errors.New("node not found")

// ErrAmbiguousNode indicates that multiple nodes matched the given name.
var ErrAmbiguousNode = errors.New("multiple nodes match the given name")

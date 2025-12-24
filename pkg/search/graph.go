package search

import (
	"context"
	"errors"
	"sort"

	"github.com/dan-solli/gognee/pkg/store"
)

// ErrNoSeeds is returned when graph search is attempted without seed nodes.
var ErrNoSeeds = errors.New("graph search requires seed node IDs in SearchOptions.SeedNodeIDs")

// GraphSearcher performs graph traversal search from seed nodes.
type GraphSearcher struct {
	graphStore store.GraphStore
}

// NewGraphSearcher creates a new graph searcher.
func NewGraphSearcher(graphStore store.GraphStore) *GraphSearcher {
	return &GraphSearcher{
		graphStore: graphStore,
	}
}

// Search performs graph traversal from seed nodes.
// The query parameter is ignored (graph search uses opts.SeedNodeIDs).
func (g *GraphSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	ApplyDefaults(&opts)

	if len(opts.SeedNodeIDs) == 0 {
		return nil, ErrNoSeeds
	}

	// Track nodes and their best scores
	nodeScores := make(map[string]nodeScore)
	visited := make(map[string]bool)

	// BFS traversal from all seeds
	type queueItem struct {
		nodeID string
		depth  int
	}
	queue := make([]queueItem, 0)

	// Initialize with seeds
	for _, seedID := range opts.SeedNodeIDs {
		seedNode, err := g.graphStore.GetNode(ctx, seedID)
		if err != nil {
			return nil, err
		}
		if seedNode != nil {
			updateNodeScore(nodeScores, seedID, seedNode, 0)
			queue = append(queue, queueItem{seedID, 0})
			visited[seedID] = true
		}
	}

	// BFS traversal
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Stop if we've reached max depth
		if current.depth >= opts.GraphDepth {
			continue
		}

		// Get direct neighbors (depth=1 from current node)
		neighbors, err := g.graphStore.GetNeighbors(ctx, current.nodeID, 1)
		if err != nil {
			return nil, err
		}

		nextDepth := current.depth + 1
		for _, neighbor := range neighbors {
			updateNodeScore(nodeScores, neighbor.ID, neighbor, nextDepth)

			// Add to queue if not visited
			if !visited[neighbor.ID] {
				visited[neighbor.ID] = true
				queue = append(queue, queueItem{neighbor.ID, nextDepth})
			}
		}
	}

	// Convert to results and sort
	results := make([]SearchResult, 0, len(nodeScores))
	for nodeID, ns := range nodeScores {
		results = append(results, SearchResult{
			NodeID:     nodeID,
			Node:       ns.node,
			Score:      ns.score,
			Source:     "graph",
			GraphDepth: ns.depth,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply TopK limit
	if len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	return results, nil
}

type nodeScore struct {
	node  *store.Node
	score float64
	depth int
}

func updateNodeScore(scores map[string]nodeScore, nodeID string, node *store.Node, depth int) {
	score := 1.0 / float64(1+depth)

	if existing, found := scores[nodeID]; found {
		// Keep best score (shortest path)
		if score > existing.score {
			scores[nodeID] = nodeScore{
				node:  node,
				score: score,
				depth: depth,
			}
		}
	} else {
		scores[nodeID] = nodeScore{
			node:  node,
			score: score,
			depth: depth,
		}
	}
}

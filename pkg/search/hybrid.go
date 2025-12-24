package search

import (
	"context"
	"math"
	"sort"

	"github.com/dan-solli/gognee/pkg/embeddings"
	"github.com/dan-solli/gognee/pkg/store"
)

// HybridSearcher combines vector similarity and graph traversal search.
type HybridSearcher struct {
	embeddings  embeddings.EmbeddingClient
	vectorStore store.VectorStore
	graphStore  store.GraphStore
}

// NewHybridSearcher creates a new hybrid searcher.
func NewHybridSearcher(
	embClient embeddings.EmbeddingClient,
	vectorStore store.VectorStore,
	graphStore store.GraphStore,
) *HybridSearcher {
	return &HybridSearcher{
		embeddings:  embClient,
		vectorStore: vectorStore,
		graphStore:  graphStore,
	}
}

// Search performs hybrid search combining vector similarity and graph expansion.
// Score formula: combined_score = vector_score + graph_score
// where vector_score = 0 if not found by vector, graph_score = 0 if not found by graph.
func (h *HybridSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	ApplyDefaults(&opts)

	// Step 1: Embed the query
	embedding, err := h.embeddings.EmbedOne(ctx, query)
	if err != nil {
		return nil, err
	}

	// Step 2: Vector search for initial results
	// Fetch more than TopK to ensure adequate expansion base
	initialFetch := int(math.Max(float64(opts.TopK*2), 20))
	vectorResults, err := h.vectorStore.Search(ctx, embedding, initialFetch)
	if err != nil {
		return nil, err
	}

	// Track combined scores and metadata
	type nodeInfo struct {
		node        *store.Node
		vectorScore float64
		graphScore  float64
		graphDepth  int
		foundBy     map[string]bool // "vector" and/or "graph"
	}
	nodes := make(map[string]*nodeInfo)

	// Step 3: Process vector results and expand via graph
	for _, vr := range vectorResults {
		node, err := h.graphStore.GetNode(ctx, vr.ID)
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue // Skip stale entries
		}

		// Record vector score
		if _, exists := nodes[vr.ID]; !exists {
			nodes[vr.ID] = &nodeInfo{
				node:       node,
				foundBy:    make(map[string]bool),
				graphDepth: 0, // Direct vector hit
			}
		}
		nodes[vr.ID].vectorScore = vr.Score
		nodes[vr.ID].foundBy["vector"] = true

		// Step 4: Graph expansion from this vector result
		if opts.GraphDepth > 0 {
			neighbors, err := h.expandFromNode(ctx, vr.ID, opts.GraphDepth)
			if err != nil {
				return nil, err
			}

			for neighborID, depthInfo := range neighbors {
				// Skip if it's the same node
				if neighborID == vr.ID {
					continue
				}

				neighborNode, err := h.graphStore.GetNode(ctx, neighborID)
				if err != nil {
					return nil, err
				}
				if neighborNode == nil {
					continue
				}

				// Calculate graph score: 1 / (1 + depth)
				graphScore := 1.0 / float64(1+depthInfo.depth)

				if existing, exists := nodes[neighborID]; !exists {
					nodes[neighborID] = &nodeInfo{
						node:       neighborNode,
						graphScore: graphScore,
						graphDepth: depthInfo.depth,
						foundBy:    map[string]bool{"graph": true},
					}
				} else {
					// Node already exists (maybe from vector or another expansion)
					// Update graph score if this path is better
					if graphScore > existing.graphScore {
						existing.graphScore = graphScore
						existing.graphDepth = depthInfo.depth
					}
					existing.foundBy["graph"] = true
				}
			}
		}
	}

	// Step 5: Deduplicate, merge scores, and build results
	results := make([]SearchResult, 0, len(nodes))
	for nodeID, info := range nodes {
		// Combined score = vector_score + graph_score
		combinedScore := info.vectorScore + info.graphScore

		// Determine source
		source := ""
		if info.foundBy["vector"] && info.foundBy["graph"] {
			source = "hybrid"
		} else if info.foundBy["vector"] {
			source = "vector"
		} else {
			source = "graph"
		}

		results = append(results, SearchResult{
			NodeID:     nodeID,
			Node:       info.node,
			Score:      combinedScore,
			Source:     source,
			GraphDepth: info.graphDepth,
		})
	}

	// Step 6: Sort by combined score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Step 7: Return top-K results
	if len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	return results, nil
}

type depthInfo struct {
	depth int
}

// expandFromNode performs BFS graph traversal from a starting node.
func (h *HybridSearcher) expandFromNode(ctx context.Context, startNodeID string, maxDepth int) (map[string]depthInfo, error) {
	result := make(map[string]depthInfo)
	visited := make(map[string]bool)

	type queueItem struct {
		nodeID string
		depth  int
	}
	queue := []queueItem{{startNodeID, 0}}
	visited[startNodeID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= maxDepth {
			continue
		}

		neighbors, err := h.graphStore.GetNeighbors(ctx, current.nodeID, 1)
		if err != nil {
			return nil, err
		}

		nextDepth := current.depth + 1
		for _, neighbor := range neighbors {
			// Record depth info (keep shortest path)
			if existing, exists := result[neighbor.ID]; !exists || nextDepth < existing.depth {
				result[neighbor.ID] = depthInfo{depth: nextDepth}
			}

			if !visited[neighbor.ID] {
				visited[neighbor.ID] = true
				queue = append(queue, queueItem{neighbor.ID, nextDepth})
			}
		}
	}

	return result, nil
}

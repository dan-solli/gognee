package search

import (
	"context"
	"math"
	"time"

	"github.com/dan-solli/gognee/pkg/store"
)

// DecayingSearcher is a decorator that applies time-based decay to search results.
// It wraps any Searcher implementation and modifies scores based on node age.
type DecayingSearcher struct {
	underlying   Searcher
	graphStore   store.GraphStore
	enabled      bool
	halfLifeDays int
	basis        string // "access" or "creation"
}

// NewDecayingSearcher creates a new decaying searcher wrapper.
//
// Parameters:
//   - underlying: The base searcher to wrap
//   - graphStore: Graph store for retrieving node timestamps
//   - enabled: Whether decay is enabled
//   - halfLifeDays: Number of days for half-life decay
//   - basis: "access" (use last_accessed_at) or "creation" (use created_at)
func NewDecayingSearcher(
	underlying Searcher,
	graphStore store.GraphStore,
	enabled bool,
	halfLifeDays int,
	basis string,
) *DecayingSearcher {
	return &DecayingSearcher{
		underlying:   underlying,
		graphStore:   graphStore,
		enabled:      enabled,
		halfLifeDays: halfLifeDays,
		basis:        basis,
	}
}

// Search performs search with decay applied to scores.
func (d *DecayingSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	// Get underlying search results
	results, err := d.underlying.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	// If decay is disabled, return results as-is
	if !d.enabled {
		return results, nil
	}

	// Apply decay to each result
	now := time.Now()
	decayedResults := make([]SearchResult, 0, len(results))

	for _, result := range results {
		// Fetch node to get timestamps
		node, err := d.graphStore.GetNode(ctx, result.NodeID)
		if err != nil {
			// On error, skip decay for this node but include it
			decayedResults = append(decayedResults, result)
			continue
		}
		if node == nil {
			// Node was deleted, skip it
			continue
		}

		// Determine age based on decay basis
		var age time.Duration
		if d.basis == "access" && node.LastAccessedAt != nil {
			// Use last access time
			age = now.Sub(*node.LastAccessedAt)
		} else {
			// Fall back to creation time (if access-based but never accessed, or creation-based)
			age = now.Sub(node.CreatedAt)
		}

		// Calculate decay multiplier
		decayMultiplier := d.calculateDecay(age)

		// Apply decay to score
		result.Score = result.Score * decayMultiplier

		// Optional: filter out results below minimum threshold
		// For now, keep all results (even very low scores)
		if result.Score < 0.001 {
			// Skip nodes with extremely low scores
			continue
		}

		decayedResults = append(decayedResults, result)
	}

	return decayedResults, nil
}

// calculateDecay computes the exponential decay multiplier.
// Formula: 0.5^(age_days / half_life_days)
func (d *DecayingSearcher) calculateDecay(age time.Duration) float64 {
	if age < 0 {
		return 1.0
	}
	if d.halfLifeDays <= 0 {
		return 1.0
	}

	ageDays := age.Hours() / 24.0
	exponent := ageDays / float64(d.halfLifeDays)
	return math.Pow(0.5, exponent)
}

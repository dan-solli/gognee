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
	underlying             Searcher
	graphStore             store.GraphStore
	memoryStore            store.MemoryStore
	enabled                bool
	halfLifeDays           int
	basis                  string // "access" or "creation"
	accessFrequencyEnabled bool   // M2: Enable access frequency decay (Plan 021)
	referenceAccessCount   int    // M2: Reference access count for heat calculation (Plan 021)
}

// NewDecayingSearcher creates a new decaying searcher wrapper.
//
// Parameters:
//   - underlying: The base searcher to wrap
//   - graphStore: Graph store for retrieving node timestamps
//   - memoryStore: Memory store for retrieving access counts (M2: Plan 021)
//   - enabled: Whether decay is enabled
//   - halfLifeDays: Number of days for half-life decay
//   - basis: "access" (use last_accessed_at) or "creation" (use created_at)
//   - accessFrequencyEnabled: Whether to apply access frequency heat multiplier (M2: Plan 021)
//   - referenceAccessCount: Reference count for full heat protection (M2: Plan 021)
func NewDecayingSearcher(
	underlying Searcher,
	graphStore store.GraphStore,
	memoryStore store.MemoryStore,
	enabled bool,
	halfLifeDays int,
	basis string,
	accessFrequencyEnabled bool,
	referenceAccessCount int,
) *DecayingSearcher {
	return &DecayingSearcher{
		underlying:             underlying,
		graphStore:             graphStore,
		memoryStore:            memoryStore,
		enabled:                enabled,
		halfLifeDays:           halfLifeDays,
		basis:                  basis,
		accessFrequencyEnabled: accessFrequencyEnabled,
		referenceAccessCount:   referenceAccessCount,
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
		// M7: Check for retention policy override (Plan 021)
		decayMultiplier := 1.0
		retentionHalfLife := d.halfLifeDays // Default

		if d.accessFrequencyEnabled && d.memoryStore != nil {
			// Fetch memory IDs for this node to check retention policy
			memoryIDs, err := d.memoryStore.GetMemoriesByNodeID(ctx, result.NodeID)
			if err == nil && len(memoryIDs) > 0 {
				// Use the most protective retention policy if multiple memories
				maxHalfLife := 0
				maxAccessCount := 0
				isPermanent := false
				hasExplicitRetentionPolicy := false

				for _, memID := range memoryIDs {
					mem, err := d.memoryStore.GetMemory(ctx, memID)
					if err == nil && mem != nil {
						// M9: Check if pinned (acts like permanent)
						if mem.Pinned {
							isPermanent = true
							break
						}

						// M7: Get retention policy half-life only if explicitly different from standard
						retentionPolicy := mem.RetentionPolicy
						if retentionPolicy == "" {
							retentionPolicy = "standard"
						}

						// Only override if retention policy is explicitly set and non-standard
						if retentionPolicy != "standard" {
							hasExplicitRetentionPolicy = true
							policyHalfLife := 0
							switch retentionPolicy {
							case "permanent":
								isPermanent = true
								break
							case "decision":
								policyHalfLife = 365
							case "ephemeral":
								policyHalfLife = 7
							case "session":
								policyHalfLife = 1
							}

							if policyHalfLife > maxHalfLife {
								maxHalfLife = policyHalfLife
							}
						}

						// Track max access count for heat multiplier
						if mem.AccessCount > maxAccessCount {
							maxAccessCount = mem.AccessCount
						}
					}
				}

				// M7: Permanent memories never decay
				if isPermanent {
					decayMultiplier = 1.0
				} else {
					// Use explicit retention policy half-life if set, otherwise use configured default
					if hasExplicitRetentionPolicy && maxHalfLife > 0 {
						retentionHalfLife = maxHalfLife
						decayMultiplier = d.calculateDecayWithHalfLife(age, retentionHalfLife)
					} else {
						// Use configured default half-life
						decayMultiplier = d.calculateDecay(age)
					}
				}

				// M2: Apply access frequency heat multiplier
				heatMultiplier := d.calculateHeatMultiplier(maxAccessCount)

				// Apply combined formula
				frequencyFactor := 0.5 + 0.5*heatMultiplier
				result.Score = result.Score * decayMultiplier * frequencyFactor
			} else {
				// No memory found - use default time decay
				decayMultiplier = d.calculateDecay(age)
				result.Score = result.Score * decayMultiplier
			}
		} else {
			// Access frequency disabled - apply time decay only
			decayMultiplier = d.calculateDecay(age)
			result.Score = result.Score * decayMultiplier
		}

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

// calculateDecayWithHalfLife computes decay with a custom half-life (M7: Plan 021).
func (d *DecayingSearcher) calculateDecayWithHalfLife(age time.Duration, halfLifeDays int) float64 {
	if age < 0 {
		return 1.0
	}
	if halfLifeDays <= 0 {
		return 1.0
	}

	ageDays := age.Hours() / 24.0
	exponent := ageDays / float64(halfLifeDays)
	return math.Pow(0.5, exponent)
}

// calculateHeatMultiplier computes the access frequency heat multiplier (M2: Plan 021).
// Formula: min(1.0, log(access_count + 1) / log(reference_count + 1))
// This provides a range from 0.0 (zero accesses) to 1.0 (reference_count or more accesses).
// The final score adjustment applies: final_score = raw_score × time_decay × (0.5 + 0.5 × heat_multiplier)
// This ensures:
//   - Minimum 0.5× for zero-access memories (some decay still applies)
//   - Full 1.0× preservation for high-access memories (no decay)
func (d *DecayingSearcher) calculateHeatMultiplier(accessCount int) float64 {
	if accessCount < 0 {
		accessCount = 0
	}
	if d.referenceAccessCount <= 0 {
		return 0.0 // No reference count means no heat protection
	}

	// Logarithmic scaling: log(access_count + 1) / log(reference_count + 1)
	logAccessCount := math.Log(float64(accessCount) + 1.0)
	logReferenceCount := math.Log(float64(d.referenceAccessCount) + 1.0)

	heatMultiplier := logAccessCount / logReferenceCount
	if heatMultiplier > 1.0 {
		heatMultiplier = 1.0 // Cap at 1.0
	}

	return heatMultiplier
}

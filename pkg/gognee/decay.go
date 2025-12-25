package gognee

import (
	"math"
	"time"
)

// calculateDecay computes the exponential decay multiplier for a node based on its age.
// Uses the formula: score_multiplier = 0.5^(age_days / half_life_days)
//
// Parameters:
//   - nodeAge: The age of the node (time since creation or last access)
//   - halfLifeDays: The number of days after which the score is halved
//
// Returns:
//   - A multiplier between 0 and 1 to apply to the node's search score
//   - Returns 1.0 for negative ages (defensive)
//   - Returns 1.0 for zero half-life (defensive)
func calculateDecay(nodeAge time.Duration, halfLifeDays int) float64 {
	// Handle edge cases
	if nodeAge < 0 {
		return 1.0 // No decay for negative age (shouldn't happen but be defensive)
	}
	if halfLifeDays <= 0 {
		return 1.0 // No decay for zero/negative half-life
	}

	// Convert nodeAge to days (float64 for precision)
	ageDays := nodeAge.Hours() / 24.0

	// Apply exponential decay formula: 0.5^(age_days / half_life_days)
	exponent := ageDays / float64(halfLifeDays)
	multiplier := math.Pow(0.5, exponent)

	return multiplier
}

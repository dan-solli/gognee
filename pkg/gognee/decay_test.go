package gognee

import (
	"math"
	"testing"
	"time"
)

// TestCalculateDecay_ZeroAge tests that brand new nodes have no decay
func TestCalculateDecay_ZeroAge(t *testing.T) {
	multiplier := calculateDecay(0, 30)
	if multiplier != 1.0 {
		t.Errorf("calculateDecay(0 days, 30 half-life): got %.6f, want 1.0", multiplier)
	}
}

// TestCalculateDecay_HalfLife tests that nodes at exactly half-life age have 0.5 multiplier
func TestCalculateDecay_HalfLife(t *testing.T) {
	multiplier := calculateDecay(30*24*time.Hour, 30)
	if math.Abs(multiplier-0.5) > 0.001 {
		t.Errorf("calculateDecay(30 days, 30 half-life): got %.6f, want 0.5", multiplier)
	}
}

// TestCalculateDecay_DoubleHalfLife tests that nodes at 2x half-life have 0.25 multiplier
func TestCalculateDecay_DoubleHalfLife(t *testing.T) {
	multiplier := calculateDecay(60*24*time.Hour, 30)
	expected := 0.25 // 0.5^2
	if math.Abs(multiplier-expected) > 0.001 {
		t.Errorf("calculateDecay(60 days, 30 half-life): got %.6f, want %.6f", multiplier, expected)
	}
}

// TestCalculateDecay_VeryOld tests that very old nodes approach zero
func TestCalculateDecay_VeryOld(t *testing.T) {
	multiplier := calculateDecay(300*24*time.Hour, 30)
	// 300 days / 30 half-life = 10 half-lives = 0.5^10 ≈ 0.00098
	if multiplier > 0.01 {
		t.Errorf("calculateDecay(300 days, 30 half-life): got %.6f, want < 0.01", multiplier)
	}
	if multiplier < 0 {
		t.Errorf("calculateDecay(300 days, 30 half-life): got %.6f, want >= 0", multiplier)
	}
}

// TestCalculateDecay_DifferentHalfLives tests different half-life values
func TestCalculateDecay_DifferentHalfLives(t *testing.T) {
	tests := []struct {
		age          time.Duration
		halfLife     int
		expectApprox float64
	}{
		{7 * 24 * time.Hour, 7, 0.5},    // 1 week at 1-week half-life
		{14 * 24 * time.Hour, 7, 0.25},  // 2 weeks at 1-week half-life
		{90 * 24 * time.Hour, 90, 0.5},  // 90 days at 90-day half-life
		{1 * 24 * time.Hour, 30, 0.977}, // 1 day at 30-day half-life (minimal decay)
	}

	for _, tt := range tests {
		multiplier := calculateDecay(tt.age, tt.halfLife)
		if math.Abs(multiplier-tt.expectApprox) > 0.01 {
			t.Errorf("calculateDecay(%v, %d days): got %.6f, want ~%.6f",
				tt.age, tt.halfLife, multiplier, tt.expectApprox)
		}
	}
}

// TestCalculateDecay_NegativeAge tests handling of negative age (should return 1.0)
func TestCalculateDecay_NegativeAge(t *testing.T) {
	multiplier := calculateDecay(-10*24*time.Hour, 30)
	if multiplier != 1.0 {
		t.Errorf("calculateDecay(negative age): got %.6f, want 1.0", multiplier)
	}
}

// TestCalculateDecay_ZeroHalfLife tests handling of zero half-life (edge case)
func TestCalculateDecay_ZeroHalfLife(t *testing.T) {
	multiplier := calculateDecay(10*24*time.Hour, 0)
	// With zero half-life, any age should result in minimal decay
	// Return 1.0 to avoid division by zero
	if multiplier != 1.0 {
		t.Errorf("calculateDecay(10 days, 0 half-life): got %.6f, want 1.0", multiplier)
	}
}

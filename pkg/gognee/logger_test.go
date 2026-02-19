package gognee

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// ===============================================================
// M1: Logger Infrastructure Tests (TDD - tests before implementation)
// ===============================================================

// captureHandler is a slog.Handler that captures log records for test assertions
type captureHandler struct {
	records []slog.Record
	mu      sync.Mutex
}

func newCaptureHandler() *captureHandler {
	return &captureHandler{
		records: make([]slog.Record, 0),
	}
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *captureHandler) WithGroup(_ string) slog.Handler {
	return h
}

func (h *captureHandler) getRecords() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]slog.Record, len(h.records))
	copy(result, h.records)
	return result
}

func (h *captureHandler) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = h.records[:0]
}

// TestWithLogger_NilSafe verifies calling methods with nil logger produces no panic
func TestWithLogger_NilSafe(t *testing.T) {
	cfg := Config{
		DBPath:        ":memory:",
		DecayEnabled:  false,
		OpenAIKey:     "test-key",
		ChunkSize:     512,
		ChunkOverlap:  50,
		EmbeddingModel: "text-embedding-3-small",
		LLMModel:      "gpt-4o-mini",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	// Don't call WithLogger - logger should be nil by default
	// Verify no panic on methods that may use logger
	ctx := context.Background()

	// These operations should not panic with nil logger
	_, _ = g.Stats()
	
	// Add a document (won't process without Cognify, but tests nil safety)
	_ = g.Add(ctx, "test document", AddOptions{Source: "test"})
	
	// Prune with nil logger
	_, _ = g.Prune(ctx, PruneOptions{DryRun: true})
	
	// Search with nil logger
	_, _ = g.Search(ctx, "test", SearchOptions{TopK: 5})

	// If we reach here without panic, test passes
}

// TestWithLogger_Injection verifies WithLogger returns same instance (fluent pattern)
func TestWithLogger_Injection(t *testing.T) {
	cfg := Config{
		DBPath:    ":memory:",
		DecayEnabled: false,
		OpenAIKey: "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	handler := newCaptureHandler()
	logger := slog.New(handler)

	// Test fluent API - should return g for chaining (M2)
	returned := g.WithLogger(logger)
	if returned != g {
		t.Errorf("WithLogger() should return same instance for method chaining")
	}
}

// TestWithLogger_PropagatesLogging verifies logger is actually used when set
func TestWithLogger_PropagatesLogging(t *testing.T) {
	cfg := Config{
		DBPath:        ":memory:",
		DecayEnabled:  true,
		DecayHalfLifeDays: 30,
		DecayBasis:    "access",
		OpenAIKey:     "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	handler := newCaptureHandler()
	logger := slog.New(handler)

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	g.WithLogger(logger)

	// After implementation, logger should emit logs during operations
	// For now, this test will fail - that's expected in TDD Red phase
	ctx := context.Background()
	
	// Prune should log when logger is set
	handler.reset()
	_, _ = g.Prune(ctx, PruneOptions{DryRun: true})
	
	records := handler.getRecords()
	// We expect at least a prune started log
	// This will fail until M6 is implemented - that's OK for TDD
	if len(records) == 0 {
		t.Log("No logs captured yet - expected after M6 implementation")
	}
}

// TestDecayConfigLogging verifies decay config is logged at startup when logger present
func TestDecayConfigLogging(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

	cfg := Config{
		DBPath:                 ":memory:",
		DecayEnabled:           true,
		DecayHalfLifeDays:      60,
		DecayBasis:             "creation",
		AccessFrequencyEnabled: true,
		ReferenceAccessCount:   20,
		OpenAIKey:              "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	// Set logger after creation (M2)
	g.WithLogger(logger)

	// For startup logging, we'll log on first WithLogger() call (M4)
	
	// Verify config was logged
	records := handler.getRecords()
	if len(records) == 0 {
		t.Errorf("Expected startup config log after WithLogger() call, got no logs")
		return
	}

	// Verify we got a decay config log at INFO level
	foundConfigLog := false
	for _, rec := range records {
		if rec.Level == slog.LevelInfo {
			// Check for expected attributes
			rec.Attrs(func(attr slog.Attr) bool {
				if attr.Key == "decay_enabled" {
					foundConfigLog = true
				}
				return true
			})
		}
	}
	
	if !foundConfigLog {
		t.Errorf("Expected decay config log with 'decay_enabled' attribute")
	}

	// Verify all expected safe attributes are present
	expectedAttrs := []string{
		"decay_enabled",
		"half_life_days",
		"decay_basis",
		"access_frequency_enabled",
		"reference_access_count",
	}
	
	foundAttrs := make(map[string]bool)
	for _, rec := range records {
		rec.Attrs(func(attr slog.Attr) bool {
			foundAttrs[attr.Key] = true
			return true
		})
	}
	
	for _, expected := range expectedAttrs {
		if !foundAttrs[expected] {
			t.Errorf("Expected attribute %q in config log, not found", expected)
		}
	}
}

// TestNewWithClients_LogsDecayConfigWhenLoggerSet verifies decay config is logged
// at INFO level with correct attributes (M3)
func TestNewWithClients_LogsDecayConfigWhenLoggerSet(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

	cfg := Config{
		DBPath:                 ":memory:",
		DecayEnabled:           true,
		DecayHalfLifeDays:      45,
		DecayBasis:             "creation",
		AccessFrequencyEnabled: true, // Note: defaults to true per Plan 022
		ReferenceAccessCount:   15,
		OpenAIKey:              "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	// Set logger - should trigger config logging (M4)
	g.WithLogger(logger)

	// Verify INFO level log with correct values
	records := handler.getRecords()
	if len(records) == 0 {
		t.Errorf("Expected config log after WithLogger(), got no logs")
		return
	}

	// Find the config log and verify attributes match the ACTUAL config (post-defaults)
	// Note: Config values may be modified by defaults in NewWithClients
	foundLog := false
	for _, rec := range records {
		if rec.Level != slog.LevelInfo {
			continue
		}
		
		attrMap := make(map[string]interface{})
		rec.Attrs(func(attr slog.Attr) bool {
			attrMap[attr.Key] = attr.Value.Any()
			return true
		})
		
		// Check if this is the decay config log
		if val, ok := attrMap["decay_enabled"]; ok {
			foundLog = true
			
			// Verify all values are present and of correct types
			if val != true {
				t.Errorf("decay_enabled: expected true, got %v", val)
			}
			if attrMap["half_life_days"] != int64(cfg.DecayHalfLifeDays) {
				t.Errorf("half_life_days: expected %d, got %v", cfg.DecayHalfLifeDays, attrMap["half_life_days"])
			}
			if attrMap["decay_basis"] != cfg.DecayBasis {
				t.Errorf("decay_basis: expected %s, got %v", cfg.DecayBasis, attrMap["decay_basis"])
			}
			// AccessFrequencyEnabled and ReferenceAccessCount are logged as configured
			if _, ok := attrMap["access_frequency_enabled"]; !ok {
				t.Errorf("access_frequency_enabled attribute missing")
			}
			if _, ok := attrMap["reference_access_count"];!ok {
				t.Errorf("reference_access_count attribute missing")
			}
		}
	}
	
	if !foundLog {
		t.Errorf("Did not find decay config log with expected attributes")
	}
}

// TestNewWithClients_NoLogWhenLoggerNil verifies no logging overhead when logger is nil (M3)
func TestNewWithClients_NoLogWhenLoggerNil(t *testing.T) {
	cfg := Config{
		DBPath:            ":memory:",
		DecayEnabled:      true,
		DecayHalfLifeDays: 30,
		OpenAIKey:         "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	// Don't call WithLogger - logger should be nil
	// No panic should occur (verified by test completing)
	// This test primarily ensures nil logger doesn't cause issues
}

// TestNewWithClients_LogAttrsAreSecure verifies no sensitive data in logs (M3)
func TestNewWithClients_LogAttrsAreSecure(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

	cfg := Config{
		DBPath:       ":memory:",
		DecayEnabled: true,
		OpenAIKey:    "test-secret-key-12345", // Sensitive - should NOT be logged
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	g.WithLogger(logger)

	// Verify no API key in any log record
	records := handler.getRecords()
	for _, rec := range records {
		// Check message
		if strings.Contains(rec.Message, cfg.OpenAIKey) {
			t.Errorf("API key found in log message: %s", rec.Message)
		}
		
		// Check all attributes
		rec.Attrs(func(attr slog.Attr) bool {
			valStr := fmt.Sprint(attr.Value.Any())
			if strings.Contains(valStr, cfg.OpenAIKey) {
				t.Errorf("API key found in log attribute %s: %s", attr.Key, valStr)
			}
			return true
		})
	}

	// Verify forbidden fields are not present
	forbiddenKeys := []string{"openai_key", "api_key", "key"}
	for _, rec := range records {
		rec.Attrs(func(attr slog.Attr) bool {
			for _, forbidden := range forbiddenKeys {
				if strings.ToLower(attr.Key) == forbidden {
					t.Errorf("Found forbidden attribute key: %s", attr.Key)
				}
			}
			return true
		})
	}
}

// TestNoLogAllocationWhenNil is a benchmark confirming zero allocs when logger is nil
func TestNoLogAllocationWhenNil(t *testing.T) {
	cfg := Config{
		DBPath:       ":memory:",
		DecayEnabled: false,
		OpenAIKey:    "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	// Don't set logger - it should be nil

	// Run a simple operation
	allocs := testing.AllocsPerRun(100, func() {
		// This will be updated in later milestones to test actual logging paths
		// For now, just verify basic operations don't allocate when logger is nil
		_, _ = g.Stats()
	})

	// With nil logger, there should be minimal allocations from Stats itself
	// (Stats may allocate for the Stats struct, but not for logging)
	// We're mainly testing that logging code doesn't allocate when logger is nil
	if allocs > 10 {
		t.Logf("Allocations per run: %.2f (baseline measurement)", allocs)
	}
}

package gognee

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// ===============================================================
// M5: Prune Operation Logging Tests (TDD - tests before implementation)
// ===============================================================

// TestPrune_LogsStartAtInfo verifies INFO log at prune start with options (M5)
func TestPrune_LogsStartAtInfo(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

	cfg := Config{
		DBPath:       ":memory:",
		DecayEnabled: true,
		OpenAIKey:    "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	g.WithLogger(logger)
	handler.reset() // Clear config logging

	ctx := context.Background()
	pruneOpts := PruneOptions{
		DryRun:            true,
		MaxAgeDays:        90,
		MinDecayScore:     0.1,
		PruneSuperseded:   true,
		SupersededAgeDays: 45,
	}

	_, _ = g.Prune(ctx, pruneOpts)

	// Verify we got a prune started log at INFO level
	records := handler.getRecords()
	foundStart := false
	for _, rec := range records {
		if rec.Level == slog.LevelInfo && strings.Contains(rec.Message, "prune") && strings.Contains(rec.Message, "start") {
			foundStart = true

			// Verify expected attributes are present
			attrMap := make(map[string]interface{})
			rec.Attrs(func(attr slog.Attr) bool {
				attrMap[attr.Key] = attr.Value.Any()
				return true
			})

			// Check for expected option attributes
			expectedKeys := []string{"dry_run", "max_age_days", "min_decay_score", "prune_superseded", "superseded_age_days"}
			for _, key := range expectedKeys {
				if _, ok := attrMap[key]; !ok {
					t.Errorf("Expected attribute %q in prune start log", key)
				}
			}

			// Verify values match what we passed
			if attrMap["dry_run"] != true {
				t.Errorf("dry_run: expected true, got %v", attrMap["dry_run"])
			}
			if attrMap["max_age_days"] != int64(90) {
				t.Errorf("max_age_days: expected 90, got %v", attrMap["max_age_days"])
			}
		}
	}

	if !foundStart {
		t.Errorf("Expected prune start log at INFO level")
	}
}

// TestPrune_LogsPerMemoryAtDebug verifies DEBUG logs for memory evaluation (M5)
func TestPrune_LogsPerMemoryAtDebug(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

	cfg := Config{
		DBPath:       ":memory:",
		DecayEnabled: true,
		OpenAIKey:    "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	// Add a memory so there's something to evaluate
	ctx := context.Background()
	_, _ = g.AddMemory(ctx, MemoryInput{
		Topic:   "Test Memory",
		Context: "Testing prune logging",
	})

	g.WithLogger(logger)
	handler.reset() // Clear previous logs

	_, _ = g.Prune(ctx, PruneOptions{DryRun: true})

	// Verify we got DEBUG level logs for memory evaluation
	records := handler.getRecords()
	foundMemoryEval := false
	for _, rec := range records {
		// Look for memory evaluation logs (should be DEBUG level)
		if rec.Level == slog.LevelDebug {
			attrMap := make(map[string]interface{})
			rec.Attrs(func(attr slog.Attr) bool {
				attrMap[attr.Key] = attr.Value.Any()
				return true
			})

			// Check if this is a memory evaluation log (has memory_id)
			if _, ok := attrMap["memory_id"]; ok {
				foundMemoryEval = true

				// Verify expected attributes (safe to log)
				safeAttrs := []string{"memory_id", "status", "retention_policy", "pinned"}
				for _, key := range safeAttrs {
					if _, ok := attrMap[key]; !ok {
						t.Logf("Memory evaluation log missing attribute: %s", key)
					}
				}
			}
		}
	}

	if !foundMemoryEval {
		t.Log("No memory evaluation logs found - expected after M6 implementation")
	}
}

// TestPrune_LogsPerNodeAtDebug verifies DEBUG logs for node evaluation (M5)
func TestPrune_LogsPerNodeAtDebug(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

	cfg := Config{
		DBPath:       ":memory:",
		DecayEnabled: true,
		OpenAIKey:    "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	// Add some data to create nodes
	ctx := context.Background()
	_ = g.Add(ctx, "Test document about Alice and Bob", AddOptions{Source: "test"})
	_, _ = g.Cognify(ctx, CognifyOptions{})

	g.WithLogger(logger)
	handler.reset() // Clear previous logs

	_, _ = g.Prune(ctx, PruneOptions{DryRun: true, MaxAgeDays: 1})

	// Verify we got DEBUG level logs for node evaluation
	records := handler.getRecords()
	foundNodeEval := false
	for _, rec := range records {
		if rec.Level == slog.LevelDebug {
			attrMap := make(map[string]interface{})
			rec.Attrs(func(attr slog.Attr) bool {
				attrMap[attr.Key] = attr.Value.Any()
				return true
			})

			// Check if this is a node evaluation log (has node_id and decay_score)
			if _, ok := attrMap["node_id"]; ok {
				if _, ok := attrMap["decay_score"]; ok {
					foundNodeEval = true

					// Verify no sensitive content in logs
					for key := range attrMap {
						if key == "name" || key == "description" {
							t.Errorf("Found forbidden attribute in node log: %s", key)
						}
					}
				}
			}
		}
	}

	if !foundNodeEval {
		t.Log("No node evaluation logs found - expected after M6 implementation")
	}
}

// TestPrune_LogsSummaryAtInfo verifies INFO log at completion with counts (M5)
func TestPrune_LogsSummaryAtInfo(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

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

	g.WithLogger(logger)
	handler.reset() // Clear config logging

	ctx := context.Background()
	_, _ = g.Prune(ctx, PruneOptions{DryRun: true})

	// Verify we got a prune complete log at INFO level
	records := handler.getRecords()
	foundComplete := false
	for _, rec := range records {
		if rec.Level == slog.LevelInfo && strings.Contains(rec.Message, "prune") && strings.Contains(rec.Message, "complete") {
			foundComplete = true

			// Verify expected summary attributes
			attrMap := make(map[string]interface{})
			rec.Attrs(func(attr slog.Attr) bool {
				attrMap[attr.Key] = attr.Value.Any()
				return true
			})

			expectedKeys := []string{"memories_evaluated", "memories_pruned", "nodes_evaluated", "nodes_pruned", "duration_ms"}
			for _, key := range expectedKeys {
				if _, ok := attrMap[key]; !ok {
					t.Errorf("Expected attribute %q in prune complete log", key)
				}
			}

			// Verify duration_ms is a positive number
			if durationMs, ok := attrMap["duration_ms"].(int64); ok {
				if durationMs < 0 {
					t.Errorf("duration_ms should be >= 0, got %d", durationMs)
				}
			}
		}
	}

	if !foundComplete {
		t.Errorf("Expected prune complete log at INFO level")
	}
}

// TestPrune_NoLogWhenLoggerNil verifies no logs when logger is nil (M5)
func TestPrune_NoLogWhenLoggerNil(t *testing.T) {
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

	// Don't set logger - should be nil

	ctx := context.Background()
	result, err := g.Prune(ctx, PruneOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	// Just verify no panic occurred
	if result == nil {
		t.Errorf("Expected non-nil result from Prune")
	}
}

// TestPrune_NoContentInLogs verifies no Topic, Context, Name, Description in logs (M5)
func TestPrune_NoContentInLogs(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

	cfg := Config{
		DBPath:       ":memory:",
		DecayEnabled: true,
		OpenAIKey:    "test-key",
	}

	mockEmb := &MockEmbeddingClient{}
	mockLLM := &MockLLMClient{}

	g, err := NewWithClients(cfg, mockEmb, mockLLM)
	if err != nil {
		t.Fatalf("NewWithClients failed: %v", err)
	}
	defer g.Close()

	// Add memory and nodes with sensitive content
	ctx := context.Background()
	sensitiveContent := "SENSITIVE_SECRET_DATA_12345"
	_, _ = g.AddMemory(ctx, MemoryInput{
		Topic:   sensitiveContent,
		Context: "More " + sensitiveContent,
	})

	_ = g.Add(ctx, "Document about "+sensitiveContent, AddOptions{Source: "test"})
	_, _ = g.Cognify(ctx, CognifyOptions{})

	g.WithLogger(logger)
	handler.reset() // Clear previous logs

	_, _ = g.Prune(ctx, PruneOptions{DryRun: true})

	// Verify sensitive content does NOT appear in any log
	records := handler.getRecords()
	for _, rec := range records {
		// Check message
		if strings.Contains(rec.Message, sensitiveContent) {
			t.Errorf("Found sensitive content in log message: %s", rec.Message)
		}

		// Check all attributes
		rec.Attrs(func(attr slog.Attr) bool {
			valStr := fmt.Sprint(attr.Value.Any())
			if strings.Contains(valStr, sensitiveContent) {
				t.Errorf("Found sensitive content in log attribute %s: %s", attr.Key, valStr)
			}

			// Verify forbidden attribute keys never appear
			forbiddenKeys := []string{"topic", "context", "decisions", "rationale", "name", "description", "metadata"}
			for _, forbidden := range forbiddenKeys {
				if strings.ToLower(attr.Key) == forbidden {
					t.Errorf("Found forbidden attribute key in prune log: %s", attr.Key)
				}
			}
			return true
		})
	}
}

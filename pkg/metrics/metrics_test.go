package metrics

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsCollector_RecordOperation(t *testing.T) {
	collector := NewCollector()
	ctx := context.Background()

	// Record some operations
	collector.RecordOperation(ctx, "cognify", "success", 1000)
	collector.RecordOperation(ctx, "cognify", "success", 1500)
	collector.RecordOperation(ctx, "cognify", "error", 500)
	collector.RecordOperation(ctx, "search", "success", 200)

	// Verify counters
	if got := testutil.CollectAndCount(collector.operationsTotal); got != 3 {
		t.Errorf("expected 3 metric series (cognify/success, cognify/error, search/success), got %d", got)
	}

	// Check specific counter value
	cognifySuccess := testutil.ToFloat64(collector.operationsTotal.WithLabelValues("cognify", "success"))
	if cognifySuccess != 2 {
		t.Errorf("expected 2 cognify/success operations, got %f", cognifySuccess)
	}

	cognifyError := testutil.ToFloat64(collector.operationsTotal.WithLabelValues("cognify", "error"))
	if cognifyError != 1 {
		t.Errorf("expected 1 cognify/error operation, got %f", cognifyError)
	}
}

func TestMetricsCollector_RecordStage(t *testing.T) {
	collector := NewCollector()
	ctx := context.Background()

	// Record stage durations (in milliseconds)
	collector.RecordStage(ctx, "cognify", "chunk", 100)
	collector.RecordStage(ctx, "cognify", "embed", 2500)
	collector.RecordStage(ctx, "cognify", "embed", 3000)

	// Verify histogram has entries
	if got := testutil.CollectAndCount(collector.operationDuration); got != 2 {
		t.Errorf("expected 2 histogram series, got %d", got)
	}

	// Note: detailed histogram bucket verification would require more complex parsing
	// For now, we verify the histogram is being updated
	embedHistogram := collector.operationDuration.WithLabelValues("cognify", "embed")
	if embedHistogram == nil {
		t.Error("expected embed histogram to exist")
	}
}

func TestMetricsCollector_RecordError(t *testing.T) {
	collector := NewCollector()
	ctx := context.Background()

	collector.RecordError(ctx, "cognify", "network")
	collector.RecordError(ctx, "cognify", "network")
	collector.RecordError(ctx, "cognify", "llm")
	collector.RecordError(ctx, "search", "timeout")

	networkErrors := testutil.ToFloat64(collector.errorsTotal.WithLabelValues("cognify", "network"))
	if networkErrors != 2 {
		t.Errorf("expected 2 network errors, got %f", networkErrors)
	}

	llmErrors := testutil.ToFloat64(collector.errorsTotal.WithLabelValues("cognify", "llm"))
	if llmErrors != 1 {
		t.Errorf("expected 1 llm error, got %f", llmErrors)
	}
}

func TestMetricsCollector_SetStorageCount(t *testing.T) {
	collector := NewCollector()
	ctx := context.Background()

	collector.SetStorageCount(ctx, "memories", 42)
	collector.SetStorageCount(ctx, "nodes", 150)
	collector.SetStorageCount(ctx, "edges", 300)

	memories := testutil.ToFloat64(collector.storageCount.WithLabelValues("memories"))
	if memories != 42 {
		t.Errorf("expected 42 memories, got %f", memories)
	}

	// Update existing gauge
	collector.SetStorageCount(ctx, "memories", 50)
	memories = testutil.ToFloat64(collector.storageCount.WithLabelValues("memories"))
	if memories != 50 {
		t.Errorf("expected 50 memories after update, got %f", memories)
	}
}

func TestMetricsCollector_Registry(t *testing.T) {
	collector := NewCollector()
	ctx := context.Background()

	// Generate some metrics first so they appear in the registry
	collector.RecordOperation(ctx, "test", "success", 100)
	collector.RecordStage(ctx, "test", "stage1", 50)
	collector.RecordError(ctx, "test", "error1")
	collector.SetStorageCount(ctx, "memories", 10)

	registry := collector.Registry()
	if registry == nil {
		t.Fatal("expected non-nil registry")
	}

	// Verify metrics are registered
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// We registered 4 metrics: operations_total, operation_duration, errors_total, storage_count
	expectedFamilies := 4
	if len(metricFamilies) != expectedFamilies {
		t.Errorf("expected %d metric families, got %d", expectedFamilies, len(metricFamilies))
	}
}

// TestMetricsCollector_NoPayloadLeakage verifies metrics contain no sensitive data
func TestMetricsCollector_NoPayloadLeakage(t *testing.T) {
	collector := NewCollector()
	ctx := context.Background()

	// Simulate operations with context that might contain sensitive data
	// (in real usage, context would never contain payload, but this tests the interface contract)
	collector.RecordOperation(ctx, "cognify", "success", 1000)
	collector.RecordStage(ctx, "cognify", "embed", 500)
	collector.RecordError(ctx, "cognify", "llm")

	// Gather all metrics
	metricFamilies, err := collector.Registry().Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Verify no sensitive keywords appear in any label values
	forbiddenTerms := []string{"topic", "context", "decision", "rationale", "api_key", "API", "Bearer"}
	for _, mf := range metricFamilies {
		for _, m := range mf.GetMetric() {
			for _, label := range m.GetLabel() {
				value := label.GetValue()
				for _, term := range forbiddenTerms {
					if value == term {
						t.Errorf("found forbidden term %q in metric label", term)
					}
				}
			}
		}
	}
}

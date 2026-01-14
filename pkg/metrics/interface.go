package metrics

import "context"

// Collector is the interface for metrics collection.
// Implementations include the Prometheus-backed collector (when built with -tags metrics)
// and the no-op collector (default build without metrics tag).
type Collector interface {
	RecordOperation(ctx context.Context, operation string, status string, durationMs int64)
	RecordStage(ctx context.Context, operation string, stage string, durationMs int64)
	RecordError(ctx context.Context, operation string, errorType string)
	SetStorageCount(ctx context.Context, storageType string, count int64)
}

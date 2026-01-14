//go:build !metrics

package metrics

import "context"

// NoopCollector is a no-op implementation when metrics are disabled.
// This file is only compiled when the 'metrics' build tag is NOT present.
type NoopCollector struct{}

// NewNoopCollector creates a no-op collector
func NewNoopCollector() *NoopCollector {
	return &NoopCollector{}
}

// RecordOperation does nothing when metrics are disabled
func (n *NoopCollector) RecordOperation(ctx context.Context, operation string, status string, durationMs int64) {
}

// RecordStage does nothing when metrics are disabled
func (n *NoopCollector) RecordStage(ctx context.Context, operation string, stage string, durationMs int64) {
}

// RecordError does nothing when metrics are disabled
func (n *NoopCollector) RecordError(ctx context.Context, operation string, errorType string) {
}

// SetStorageCount does nothing when metrics are disabled
func (n *NoopCollector) SetStorageCount(ctx context.Context, storageType string, count int64) {
}

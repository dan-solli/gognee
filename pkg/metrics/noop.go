//go:build !metrics

package metrics

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// NoopCollector is a no-op implementation when metrics are disabled.
// This file is only compiled when the 'metrics' build tag is NOT present.
type NoopCollector struct{}

// MetricsCollector is a type alias to NoopCollector when metrics are disabled.
// This allows code to reference *MetricsCollector without build tags.
type MetricsCollector = NoopCollector

// NewNoopCollector creates a no-op collector
func NewNoopCollector() *NoopCollector {
	return &NoopCollector{}
}

// NewCollector returns nil when metrics are disabled.
// Callers must nil-check before use.
func NewCollector() *MetricsCollector {
	return nil
}

// Registry returns nil when metrics are disabled
func (n *NoopCollector) Registry() *prometheus.Registry {
	return nil
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

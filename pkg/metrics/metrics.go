package metrics

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsCollector provides Prometheus metrics collection for gognee operations
type MetricsCollector struct {
	operationsTotal     *prometheus.CounterVec
	operationDuration   *prometheus.HistogramVec
	errorsTotal         *prometheus.CounterVec
	storageCount        *prometheus.GaugeVec
	registry            *prometheus.Registry
}

// NewCollector creates a new Prometheus metrics collector
func NewCollector() *MetricsCollector {
	registry := prometheus.NewRegistry()

	operationsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gognee_operations_total",
			Help: "Total number of gognee operations by type and status",
		},
		[]string{"operation", "status"},
	)

	operationDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gognee_operation_duration_seconds",
			Help:    "Duration of gognee operations by type and stage",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
		},
		[]string{"operation", "stage"},
	)

	errorsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gognee_errors_total",
			Help: "Total number of errors by operation and error type",
		},
		[]string{"operation", "error_type"},
	)

	storageCount := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gognee_storage_count",
			Help: "Current count of stored items by type",
		},
		[]string{"type"},
	)

	registry.MustRegister(operationsTotal)
	registry.MustRegister(operationDuration)
	registry.MustRegister(errorsTotal)
	registry.MustRegister(storageCount)

	return &MetricsCollector{
		operationsTotal:   operationsTotal,
		operationDuration: operationDuration,
		errorsTotal:       errorsTotal,
		storageCount:      storageCount,
		registry:          registry,
	}
}

// RecordOperation records the completion of an operation
func (m *MetricsCollector) RecordOperation(ctx context.Context, operation string, status string, durationMs int64) {
	m.operationsTotal.WithLabelValues(operation, status).Inc()
}

// RecordStage records the duration of a specific stage within an operation
func (m *MetricsCollector) RecordStage(ctx context.Context, operation string, stage string, durationMs int64) {
	m.operationDuration.WithLabelValues(operation, stage).Observe(float64(durationMs) / 1000.0)
}

// RecordError records an error occurrence
func (m *MetricsCollector) RecordError(ctx context.Context, operation string, errorType string) {
	m.errorsTotal.WithLabelValues(operation, errorType).Inc()
}

// SetStorageCount sets the current count for a storage type
func (m *MetricsCollector) SetStorageCount(ctx context.Context, storageType string, count int64) {
	m.storageCount.WithLabelValues(storageType).Set(float64(count))
}

// Registry returns the Prometheus registry for HTTP exposure
func (m *MetricsCollector) Registry() *prometheus.Registry {
	return m.registry
}

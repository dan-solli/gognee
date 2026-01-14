//go:build !tracing

package trace

import "context"

// NoopExporter is a zero-overhead exporter that does nothing.
// Used when tracing is disabled at build time.
type NoopExporter struct{}

// NewFileExporter returns a no-op exporter when tracing is disabled.
// This function signature matches the tracing-enabled version for API compatibility.
func NewFileExporter(filePath string, opts ...FileExporterOption) (Exporter, error) {
	return &NoopExporter{}, nil
}

// Export does nothing.
func (n *NoopExporter) Export(ctx context.Context, record *TraceRecord) error {
	return nil
}

// Close does nothing.
func (n *NoopExporter) Close() error {
	return nil
}

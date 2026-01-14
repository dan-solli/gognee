package trace

import (
	"context"
	"time"
)

// Exporter defines the interface for exporting operation traces.
// Implementations must be safe for concurrent use.
type Exporter interface {
	// Export writes a trace record to the configured destination.
	// Returns error if export fails.
	Export(ctx context.Context, record *TraceRecord) error

	// Close flushes any buffered records and releases resources.
	// Should be called during graceful shutdown.
	Close() error
}

// TraceRecord represents a sanitized operation trace ready for export.
// This structure contains NO sensitive data (no user payloads, API keys, memory content).
type TraceRecord struct {
	// Timestamp is the operation start time
	Timestamp time.Time `json:"timestamp"`

	// OperationID uniquely identifies this operation (for correlation)
	OperationID string `json:"operationId"`

	// Operation is the operation type: "cognify", "search", "add_memory"
	Operation string `json:"operation"`

	// DurationMs is the total operation duration in milliseconds
	DurationMs int64 `json:"durationMs"`

	// Status is "success" or "error"
	Status string `json:"status"`

	// Spans contains per-stage timing and status
	Spans []SpanRecord `json:"spans"`

	// ErrorType classifies the error (if Status == "error")
	// Values: network, timeout, llm, database, validation, unknown
	ErrorType string `json:"errorType,omitempty"`

	// IDs contains operation-specific identifiers (no content)
	IDs map[string]interface{} `json:"ids,omitempty"`
}

// SpanRecord represents a single stage within an operation.
type SpanRecord struct {
	// Name is the stage name (chunk, embed, extract, write-graph, write-vector, search-vector, search-expand)
	Name string `json:"name"`

	// DurationMs is the stage duration in milliseconds
	DurationMs int64 `json:"durationMs"`

	// OK indicates success (true) or failure (false)
	OK bool `json:"ok"`

	// ErrorType classifies the error (if OK == false)
	ErrorType string `json:"errorType,omitempty"`

	// Counters provides stage-specific metrics (e.g., chunkCount, nodeUpserts)
	Counters map[string]int64 `json:"counters,omitempty"`
}

// FileExporterOption configures a FileExporter.
// This type is available in both tracing and non-tracing builds to maintain API compatibility.
type FileExporterOption func(interface{})

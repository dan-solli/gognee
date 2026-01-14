package gognee

import (
	"context"
	"time"

	tracepkg "github.com/dan-solli/gognee/pkg/trace"
)

// OperationTrace captures timing and performance data for a Cognify or Search operation.
// This structure is stable and versioned to support downstream consumers.
type OperationTrace struct {
	// Spans contains timing data for each stage of the operation
	Spans []Span `json:"spans"`

	// TotalDurationMs is the total elapsed time for the operation in milliseconds
	TotalDurationMs int64 `json:"totalDurationMs"`
}

// Span represents a single timed stage within an operation.
// Stage names are stable and documented:
//   - "chunk": Text chunking
//   - "embed": Embedding generation
//   - "extract": Entity/relationship extraction
//   - "write-graph": Graph database writes
//   - "write-vector": Vector store writes
//   - "search-vector": Vector similarity search
//   - "search-expand": Graph traversal/expansion
type Span struct {
	// Name identifies the operation stage (see Span documentation for stable names)
	Name string `json:"name"`

	// DurationMs is the elapsed time for this span in milliseconds
	DurationMs int64 `json:"durationMs"`

	// OK indicates whether the span completed successfully
	OK bool `json:"ok"`

	// Error contains error message if OK is false (optional)
	Error string `json:"error,omitempty"`

	// ErrorType classifies the error for metrics (added in Plan 016 M3)
	ErrorType string `json:"errorType,omitempty"`

	// Counters provides additional metrics for the span (optional)
	// Example keys: "chunkCount", "nodeUpserts", "edgeUpserts", "resultsReturned"
	Counters map[string]int64 `json:"counters,omitempty"`
}

// newTrace creates a new OperationTrace with empty spans
func newTrace() *OperationTrace {
	return &OperationTrace{
		Spans: make([]Span, 0),
	}
}

// addSpan appends a completed span to the trace
func (t *OperationTrace) addSpan(span Span) {
	t.Spans = append(t.Spans, span)
	t.TotalDurationMs += span.DurationMs
}

// spanTimer is a helper for measuring span duration
type spanTimer struct {
	name    string
	start   int64 // Unix time in milliseconds
	trace   *OperationTrace
	enabled bool
}

// newSpanTimer creates a timer for a named span
func newSpanTimer(name string, trace *OperationTrace, enabled bool) *spanTimer {
	if !enabled || trace == nil {
		return &spanTimer{enabled: false}
	}
	return &spanTimer{
		name:    name,
		start:   timeNowMs(),
		trace:   trace,
		enabled: true,
	}
}

// finish completes the span and records it to the trace
func (st *spanTimer) finish(ok bool, err error, counters map[string]int64) {
	if !st.enabled {
		return
	}

	duration := timeNowMs() - st.start
	span := Span{
		Name:       st.name,
		DurationMs: duration,
		OK:         ok,
		Counters:   counters,
	}
	if err != nil {
		span.Error = err.Error()
		span.ErrorType = ClassifyError(err) // Classify error for metrics (Plan 016 M3)
	}
	st.trace.addSpan(span)
}

// timeNowMs returns current Unix time in milliseconds
func timeNowMs() int64 {
	return time.Now().UnixMilli()
}

// exportTrace writes the trace to the configured exporter if available.
// This is called after operation completion (Cognify, Search, AddMemory).
func (g *Gognee) exportTrace(ctx context.Context, operationID, operation string, trace *OperationTrace, startTime time.Time, err error, ids map[string]interface{}) {
	if g.traceExporter == nil || trace == nil {
		return // Tracing not enabled
	}

	status := "success"
	errorType := ""
	if err != nil {
		status = "error"
		errorType = ClassifyError(err)
	}

	// Convert OperationTrace spans to tracepkg.SpanRecord
	spans := make([]tracepkg.SpanRecord, len(trace.Spans))
	for i, span := range trace.Spans {
		spans[i] = tracepkg.SpanRecord{
			Name:       span.Name,
			DurationMs: span.DurationMs,
			OK:         span.OK,
			ErrorType:  span.ErrorType,
			Counters:   span.Counters,
		}
	}

	record := &tracepkg.TraceRecord{
		Timestamp:   startTime,
		OperationID: operationID,
		Operation:   operation,
		DurationMs:  trace.TotalDurationMs,
		Status:      status,
		Spans:       spans,
		ErrorType:   errorType,
		IDs:         ids,
	}

	// Export in background to avoid blocking operation completion
	go func() {
		exportCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if exportErr := g.traceExporter.Export(exportCtx, record); exportErr != nil {
			// Log error but don't fail the operation
			// TODO: Consider adding error callback or metrics for export failures
			_ = exportErr
		}
	}()
}

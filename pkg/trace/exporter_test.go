package trace

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileExporter_BasicExport(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "traces.jsonl")

	exporter, err := NewFileExporter(tracePath)
	if err != nil {
		t.Fatalf("NewFileExporter failed: %v", err)
	}
	defer exporter.Close()

	record := &TraceRecord{
		Timestamp:   time.Date(2026, 1, 14, 10, 30, 0, 0, time.UTC),
		OperationID: "test-op-1",
		Operation:   "cognify",
		DurationMs:  1234,
		Status:      "success",
		Spans: []SpanRecord{
			{Name: "chunk", DurationMs: 100, OK: true},
			{Name: "embed", DurationMs: 500, OK: true},
		},
	}

	if err := exporter.Export(context.Background(), record); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Close to flush
	if err := exporter.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("Read trace file failed: %v", err)
	}

	var readRecord TraceRecord
	if err := json.Unmarshal(data, &readRecord); err != nil {
		t.Fatalf("Unmarshal trace record failed: %v", err)
	}

	if readRecord.OperationID != "test-op-1" {
		t.Errorf("Expected operationId 'test-op-1', got '%s'", readRecord.OperationID)
	}
	if readRecord.Operation != "cognify" {
		t.Errorf("Expected operation 'cognify', got '%s'", readRecord.Operation)
	}
	if len(readRecord.Spans) != 2 {
		t.Errorf("Expected 2 spans, got %d", len(readRecord.Spans))
	}
}

func TestNewFileExporter_EmptyPathIsNoop(t *testing.T) {
	exporter, err := NewFileExporter("")
	if err != nil {
		t.Fatalf("NewFileExporter(\"\") failed: %v", err)
	}
	if exporter == nil {
		t.Fatal("Expected non-nil exporter")
	}

	record := &TraceRecord{
		Timestamp:   time.Now(),
		OperationID: "noop-op",
		Operation:   "smoke",
		DurationMs:  1,
		Status:      "success",
		Spans: []SpanRecord{
			{Name: "noop", DurationMs: 1, OK: true},
		},
	}

	if err := exporter.Export(context.Background(), record); err != nil {
		t.Fatalf("Export on noop exporter should succeed, got: %v", err)
	}
	if err := exporter.Close(); err != nil {
		t.Fatalf("Close on noop exporter should succeed, got: %v", err)
	}
}

func TestFileExporter_MultipleRecords(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "traces.jsonl")

	exporter, err := NewFileExporter(tracePath)
	if err != nil {
		t.Fatalf("NewFileExporter failed: %v", err)
	}
	defer exporter.Close()

	// Write 3 records
	for i := 1; i <= 3; i++ {
		record := &TraceRecord{
			Timestamp:   time.Now(),
			OperationID: "op-" + string(rune('0'+i)),
			Operation:   "search",
			DurationMs:  int64(i * 100),
			Status:      "success",
		}
		if err := exporter.Export(context.Background(), record); err != nil {
			t.Fatalf("Export %d failed: %v", i, err)
		}
	}

	if err := exporter.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read all lines
	file, err := os.Open(tracePath)
	if err != nil {
		t.Fatalf("Open trace file failed: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		var record TraceRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Errorf("Unmarshal line %d failed: %v", lineCount, err)
		}
	}

	if lineCount != 3 {
		t.Errorf("Expected 3 lines, got %d", lineCount)
	}
}

func TestFileExporter_Rotation(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "traces.jsonl")

	// Create exporter with small max size (1KB)
	exporter, err := NewFileExporter(tracePath, WithMaxSize(1024), WithMaxRotatedFiles(3))
	if err != nil {
		t.Fatalf("NewFileExporter failed: %v", err)
	}
	defer exporter.Close()

	// Write records until rotation happens
	// Each record ~200 bytes, so 6 records should trigger rotation
	for i := 0; i < 10; i++ {
		record := &TraceRecord{
			Timestamp:   time.Now(),
			OperationID: "op-" + strings.Repeat("x", 50), // Pad to increase size
			Operation:   "cognify",
			DurationMs:  1000,
			Status:      "success",
			Spans: []SpanRecord{
				{Name: "chunk", DurationMs: 100, OK: true, Counters: map[string]int64{"count": 1}},
				{Name: "embed", DurationMs: 200, OK: true},
			},
		}
		if err := exporter.Export(context.Background(), record); err != nil {
			t.Fatalf("Export %d failed: %v", i, err)
		}
	}

	if err := exporter.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Check that rotated files exist
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	fileCount := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "traces.jsonl") {
			fileCount++
		}
	}

	// Should have at least 2 files (current + rotated)
	if fileCount < 2 {
		t.Errorf("Expected at least 2 trace files, got %d", fileCount)
	}

	// Should not exceed maxRotatedFiles + 1 (current)
	if fileCount > 4 {
		t.Errorf("Expected at most 4 trace files (current + 3 rotated), got %d", fileCount)
	}
}

func TestFileExporter_NoSensitiveData(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "traces.jsonl")

	exporter, err := NewFileExporter(tracePath)
	if err != nil {
		t.Fatalf("NewFileExporter failed: %v", err)
	}
	defer exporter.Close()

	// Create record with IDs but no sensitive content
	record := &TraceRecord{
		Timestamp:   time.Now(),
		OperationID: "test-op",
		Operation:   "cognify",
		DurationMs:  1000,
		Status:      "success",
		Spans: []SpanRecord{
			{Name: "embed", DurationMs: 500, OK: true},
		},
		IDs: map[string]interface{}{
			"memoryId": "uuid-123",
			"nodeIds":  []string{"node-1", "node-2"},
		},
	}

	if err := exporter.Export(context.Background(), record); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if err := exporter.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Read file and verify no sensitive fields
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("Read trace file failed: %v", err)
	}

	content := string(data)

	// Verify prohibited fields are NOT present
	prohibitedFields := []string{"topic", "context", "decisions", "rationale", "apiKey"}
	for _, field := range prohibitedFields {
		if strings.Contains(content, field) {
			t.Errorf("Trace contains prohibited field '%s': %s", field, content)
		}
	}

	// Verify allowed fields ARE present
	allowedFields := []string{"operationId", "operation", "durationMs", "status", "spans"}
	for _, field := range allowedFields {
		if !strings.Contains(content, field) {
			t.Errorf("Trace missing expected field '%s'", field)
		}
	}
}

func TestFileExporter_ErrorRecording(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "traces.jsonl")

	exporter, err := NewFileExporter(tracePath)
	if err != nil {
		t.Fatalf("NewFileExporter failed: %v", err)
	}
	defer exporter.Close()

	record := &TraceRecord{
		Timestamp:   time.Now(),
		OperationID: "error-op",
		Operation:   "search",
		DurationMs:  500,
		Status:      "error",
		ErrorType:   "timeout",
		Spans: []SpanRecord{
			{Name: "search-vector", DurationMs: 500, OK: false, ErrorType: "timeout"},
		},
	}

	if err := exporter.Export(context.Background(), record); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if err := exporter.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify error fields
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("Read trace file failed: %v", err)
	}

	var readRecord TraceRecord
	if err := json.Unmarshal(data, &readRecord); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if readRecord.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", readRecord.Status)
	}
	if readRecord.ErrorType != "timeout" {
		t.Errorf("Expected errorType 'timeout', got '%s'", readRecord.ErrorType)
	}
	if readRecord.Spans[0].OK {
		t.Error("Expected span OK=false")
	}
}

func TestFileExporter_CloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "traces.jsonl")

	exporter, err := NewFileExporter(tracePath)
	if err != nil {
		t.Fatalf("NewFileExporter failed: %v", err)
	}

	// Close multiple times should not error
	if err := exporter.Close(); err != nil {
		t.Errorf("First Close failed: %v", err)
	}
	if err := exporter.Close(); err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}

func TestFileExporter_DirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	tracePath := filepath.Join(dir, "nested", "subdir", "traces.jsonl")

	// Should create nested directories automatically
	exporter, err := NewFileExporter(tracePath)
	if err != nil {
		t.Fatalf("NewFileExporter failed: %v", err)
	}
	defer exporter.Close()

	// Verify directory exists
	if _, err := os.Stat(filepath.Dir(tracePath)); os.IsNotExist(err) {
		t.Error("Expected nested directory to be created")
	}
}

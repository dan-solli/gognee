//go:build tracing

package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// FileExporter exports traces to a JSON Lines file with automatic rotation.
type FileExporter struct {
	filePath        string
	maxSizeBytes    int64
	maxRotatedFiles int
	file            *os.File
	encoder         *json.Encoder
	mu              sync.Mutex
	closed          bool
}

// WithMaxSize sets the maximum file size before rotation (default: 10MB).
func WithMaxSize(bytes int64) FileExporterOption {
	return func(iface interface{}) {
		if fe, ok := iface.(*FileExporter); ok {
			fe.maxSizeBytes = bytes
		}
	}
}

// WithMaxRotatedFiles sets how many rotated files to keep (default: 5).
func WithMaxRotatedFiles(count int) FileExporterOption {
	return func(iface interface{}) {
		if fe, ok := iface.(*FileExporter); ok {
			fe.maxRotatedFiles = count
		}
	}
}

// NewFileExporter creates a file-based trace exporter.
// The file is opened immediately and rotation is checked on each Export.
func NewFileExporter(filePath string, opts ...FileExporterOption) (Exporter, error) {
	fe := &FileExporter{
		filePath:        filePath,
		maxSizeBytes:    10 * 1024 * 1024, // 10MB default
		maxRotatedFiles: 5,
	}

	// Apply options
	for _, opt := range opts {
		opt(fe)
	}

	// Create parent directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create trace directory: %w", err)
	}

	// Open file for append
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open trace file: %w", err)
	}

	fe.file = file
	fe.encoder = json.NewEncoder(file)

	return fe, nil
}

// Export writes a trace record as a JSON Lines entry.
// Checks for rotation after write.
func (fe *FileExporter) Export(ctx context.Context, record *TraceRecord) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if fe.closed {
		return fmt.Errorf("exporter closed")
	}

	// Write JSON line
	if err := fe.encoder.Encode(record); err != nil {
		return fmt.Errorf("encode trace record: %w", err)
	}

	// Check if rotation needed
	if err := fe.rotateIfNeeded(); err != nil {
		return fmt.Errorf("rotate trace file: %w", err)
	}

	return nil
}

// Close flushes and closes the trace file.
func (fe *FileExporter) Close() error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if fe.closed {
		return nil
	}

	fe.closed = true

	if fe.file != nil {
		if err := fe.file.Sync(); err != nil {
			fe.file.Close()
			return fmt.Errorf("sync trace file: %w", err)
		}
		return fe.file.Close()
	}

	return nil
}

// rotateIfNeeded checks file size and rotates if threshold exceeded.
// Must be called with lock held.
func (fe *FileExporter) rotateIfNeeded() error {
	info, err := fe.file.Stat()
	if err != nil {
		return fmt.Errorf("stat trace file: %w", err)
	}

	if info.Size() < fe.maxSizeBytes {
		return nil // No rotation needed
	}

	// Close current file
	if err := fe.file.Close(); err != nil {
		return fmt.Errorf("close trace file for rotation: %w", err)
	}

	// Rotate: move current file to .1, shift existing rotated files
	if err := fe.rotateFiles(); err != nil {
		return fmt.Errorf("rotate files: %w", err)
	}

	// Open new file
	file, err := os.OpenFile(fe.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open new trace file after rotation: %w", err)
	}

	fe.file = file
	fe.encoder = json.NewEncoder(file)

	return nil
}

// rotateFiles shifts existing rotated files and creates new rotation.
// Must be called with lock held.
func (fe *FileExporter) rotateFiles() error {
	// Delete oldest rotated file if at limit
	oldestPath := fmt.Sprintf("%s.%d", fe.filePath, fe.maxRotatedFiles)
	if _, err := os.Stat(oldestPath); err == nil {
		if err := os.Remove(oldestPath); err != nil {
			return fmt.Errorf("remove oldest rotated file: %w", err)
		}
	}

	// Shift existing rotated files: .N-1 -> .N
	for i := fe.maxRotatedFiles - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", fe.filePath, i)
		newPath := fmt.Sprintf("%s.%d", fe.filePath, i+1)

		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("shift rotated file %s -> %s: %w", oldPath, newPath, err)
			}
		}
	}

	// Move current file to .1
	rotatedPath := fmt.Sprintf("%s.%d", fe.filePath, 1)
	if err := os.Rename(fe.filePath, rotatedPath); err != nil {
		return fmt.Errorf("rotate current file to .1: %w", err)
	}

	return nil
}

// listRotatedFiles returns paths of all rotated files, sorted by number.
func (fe *FileExporter) listRotatedFiles() ([]string, error) {
	dir := filepath.Dir(fe.filePath)
	base := filepath.Base(fe.filePath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read trace directory: %w", err)
	}

	var rotatedFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Match pattern: base.1, base.2, etc.
		name := entry.Name()
		if len(name) > len(base)+2 && name[:len(base)+1] == base+"." {
			rotatedFiles = append(rotatedFiles, filepath.Join(dir, name))
		}
	}

	sort.Strings(rotatedFiles)
	return rotatedFiles, nil
}

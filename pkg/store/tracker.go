package store

import (
	"context"
	"fmt"
)

// DocumentTracker provides operations for tracking processed documents.
// Separate from GraphStore to maintain interface cohesion.
// This enables document-level deduplication in incremental Cognify operations.
type DocumentTracker interface {
	// IsDocumentProcessed checks if a document with the given hash has been processed.
	// hash: SHA-256 hash of the document text (content-based identity)
	// Returns true if the document has been processed, false otherwise.
	IsDocumentProcessed(ctx context.Context, hash string) (bool, error)

	// MarkDocumentProcessed records that a document has been successfully processed.
	// hash: SHA-256 hash of document text (content-based identity)
	// source: Optional source identifier (metadata only, does not affect identity)
	// chunkCount: Number of chunks generated from this document
	// Uses INSERT OR REPLACE to support upsert semantics.
	MarkDocumentProcessed(ctx context.Context, hash, source string, chunkCount int) error

	// GetProcessedDocumentCount returns the total number of processed documents tracked.
	GetProcessedDocumentCount(ctx context.Context) (int64, error)

	// ClearProcessedDocuments removes all document tracking records.
	// This does NOT delete nodes or edges - only clears the processed_documents table.
	// Useful for forcing a full reprocess without losing the knowledge graph.
	ClearProcessedDocuments(ctx context.Context) error
}

// Compile-time interface check
var _ DocumentTracker = (*SQLiteGraphStore)(nil)

// IsDocumentProcessed checks if a document with the given hash has been processed.
func (s *SQLiteGraphStore) IsDocumentProcessed(ctx context.Context, hash string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM processed_documents WHERE hash = ?", hash).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check document processed status: %w", err)
	}
	return count > 0, nil
}

// MarkDocumentProcessed records that a document has been successfully processed.
func (s *SQLiteGraphStore) MarkDocumentProcessed(ctx context.Context, hash, source string, chunkCount int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO processed_documents (hash, source, processed_at, chunk_count)
		 VALUES (?, ?, CURRENT_TIMESTAMP, ?)`,
		hash, source, chunkCount)
	if err != nil {
		return fmt.Errorf("failed to mark document as processed: %w", err)
	}
	return nil
}

// GetProcessedDocumentCount returns the total number of processed documents tracked.
func (s *SQLiteGraphStore) GetProcessedDocumentCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM processed_documents").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get processed document count: %w", err)
	}
	return count, nil
}

// ClearProcessedDocuments removes all document tracking records without affecting the knowledge graph.
func (s *SQLiteGraphStore) ClearProcessedDocuments(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM processed_documents")
	if err != nil {
		return fmt.Errorf("failed to clear processed documents: %w", err)
	}
	return nil
}

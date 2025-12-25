//go:build plan009

package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "plan009-tracker.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, dbPath
}

func TestPlan009_ProcessedDocumentsSchemaCreated(t *testing.T) {
	// This test is intentionally behind the plan009 build tag.
	// It defines the expected schema behavior for Plan 009.
	ctx := context.Background()

	// Create store on a new DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "schema.db")

	st, err := NewSQLiteGraphStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteGraphStore: %v", err)
	}
	defer st.Close()

	// Verify processed_documents exists
	row := st.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='processed_documents'")
	var name string
	if err := row.Scan(&name); err != nil {
		t.Fatalf("processed_documents table missing (Plan 009): %v", err)
	}
	if name != "processed_documents" {
		t.Fatalf("expected processed_documents, got %q", name)
	}
}

func TestPlan009_DocumentTrackerCRUD(t *testing.T) {
	ctx := context.Background()

	// New store
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "crud.db")
	st, err := NewSQLiteGraphStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteGraphStore: %v", err)
	}
	defer st.Close()

	tracker, ok := any(st).(DocumentTracker)
	if !ok {
		t.Fatalf("SQLiteGraphStore must implement DocumentTracker (Plan 009)")
	}

	hash := "deadbeef"

	processed, err := tracker.IsDocumentProcessed(ctx, hash)
	if err != nil {
		t.Fatalf("IsDocumentProcessed: %v", err)
	}
	if processed {
		t.Fatalf("expected unprocessed=false")
	}

	if err := tracker.MarkDocumentProcessed(ctx, hash, "source-a", 2); err != nil {
		t.Fatalf("MarkDocumentProcessed: %v", err)
	}

	processed, err = tracker.IsDocumentProcessed(ctx, hash)
	if err != nil {
		t.Fatalf("IsDocumentProcessed (after mark): %v", err)
	}
	if !processed {
		t.Fatalf("expected processed=true")
	}

	count, err := tracker.GetProcessedDocumentCount(ctx)
	if err != nil {
		t.Fatalf("GetProcessedDocumentCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}

	// Upsert should not increase count
	if err := tracker.MarkDocumentProcessed(ctx, hash, "source-b", 3); err != nil {
		t.Fatalf("MarkDocumentProcessed (upsert): %v", err)
	}
	count, err = tracker.GetProcessedDocumentCount(ctx)
	if err != nil {
		t.Fatalf("GetProcessedDocumentCount (after upsert): %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1 after upsert, got %d", count)
	}

	if err := tracker.ClearProcessedDocuments(ctx); err != nil {
		t.Fatalf("ClearProcessedDocuments: %v", err)
	}
	count, err = tracker.GetProcessedDocumentCount(ctx)
	if err != nil {
		t.Fatalf("GetProcessedDocumentCount (after clear): %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count=0 after clear, got %d", count)
	}

	// Sanity: clear should not delete the DB file
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected db file to exist, stat error: %v", err)
	}
}

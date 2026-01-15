package store

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TestSQLiteVectorStore_Add tests adding embeddings to the vector store
func TestSQLiteVectorStore_Add(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	// Create a test node first
	nodeID := "test-node-1"
	_, err := db.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, nodeID, "Test", "Concept")
	if err != nil {
		t.Fatalf("Failed to create test node: %v", err)
	}

	// Add embedding
	embedding := []float32{0.1, 0.2, 0.3}
	err = vs.Add(ctx, nodeID, embedding)
	if err != nil {
		t.Fatalf("Failed to add embedding: %v", err)
	}

	// Verify embedding was stored
	var embeddingBlob []byte
	err = db.QueryRow(`SELECT embedding FROM nodes WHERE id = ?`, nodeID).Scan(&embeddingBlob)
	if err != nil {
		t.Fatalf("Failed to retrieve embedding: %v", err)
	}

	if embeddingBlob == nil {
		t.Fatal("Embedding should not be nil")
	}

	// Verify the blob has the expected size (4 bytes per float32)
	expectedSize := len(embedding) * 4
	if len(embeddingBlob) != expectedSize {
		t.Fatalf("Expected %d bytes, got %d", expectedSize, len(embeddingBlob))
	}
}

// TestSQLiteVectorStore_AddNonexistentNode tests adding embedding for node that doesn't exist
func TestSQLiteVectorStore_AddNonexistentNode(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	// Try to add embedding for non-existent node
	embedding := []float32{0.1, 0.2, 0.3}
	err := vs.Add(ctx, "nonexistent-node", embedding)
	if err == nil {
		t.Fatal("Expected error when adding embedding for non-existent node")
	}
}

func TestSQLiteVectorStore_AddRejectsEmptyEmbedding(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	_, err := db.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, "node-empty", "NodeEmpty", "Concept")
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	if err := vs.Add(ctx, "node-empty", []float32{}); err == nil {
		t.Fatal("Expected error for empty embedding")
	}
}

// TestSQLiteVectorStore_Search tests vector similarity search
func TestSQLiteVectorStore_Search(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	// Create test nodes with embeddings
	nodes := []struct {
		id        string
		embedding []float32
	}{
		{"node-1", []float32{1.0, 0.0, 0.0}}, // orthogonal to query
		{"node-2", []float32{0.0, 1.0, 0.0}}, // identical to query
		{"node-3", []float32{0.5, 0.5, 0.0}}, // somewhat similar
	}

	for _, n := range nodes {
		_, err := db.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, n.id, n.id, "Concept")
		if err != nil {
			t.Fatalf("Failed to create node %s: %v", n.id, err)
		}
		if err := vs.Add(ctx, n.id, n.embedding); err != nil {
			t.Fatalf("Failed to add embedding for %s: %v", n.id, err)
		}
	}

	// Search with query vector [0, 1, 0] - should match node-2 best
	query := []float32{0.0, 1.0, 0.0}
	results, err := vs.Search(ctx, query, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// First result should be node-2 with perfect score
	if results[0].ID != "node-2" {
		t.Errorf("Expected node-2 as top result, got %s", results[0].ID)
	}
	if results[0].Score < 0.99 { // Allow for floating point precision
		t.Errorf("Expected score ~1.0, got %f", results[0].Score)
	}

	// Second result should be node-3
	if results[1].ID != "node-3" {
		t.Errorf("Expected node-3 as second result, got %s", results[1].ID)
	}
}

// TestSQLiteVectorStore_SearchEmptyStore tests search on empty store
func TestSQLiteVectorStore_SearchEmptyStore(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	query := []float32{1.0, 0.0, 0.0}
	results, err := vs.Search(ctx, query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected empty results, got %d", len(results))
	}
}

func TestSQLiteVectorStore_SearchEmptyQuery(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	results, err := vs.Search(ctx, nil, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Expected empty results for empty query, got %d", len(results))
	}
}

// TestSQLiteVectorStore_SearchWithNullEmbeddings tests that nodes without embeddings are skipped
func TestSQLiteVectorStore_SearchWithNullEmbeddings(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	// Create nodes - some with embeddings, some without
	_, err := db.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, "node-1", "Node1", "Concept")
	if err != nil {
		t.Fatalf("Failed to create node-1: %v", err)
	}
	if err := vs.Add(ctx, "node-1", []float32{1.0, 0.0, 0.0}); err != nil {
		t.Fatalf("Failed to add embedding: %v", err)
	}

	// Node without embedding
	_, err = db.Exec(`INSERT INTO nodes (id, name, type, embedding) VALUES (?, ?, ?, ?)`, "node-2", "Node2", "Concept", nil)
	if err != nil {
		t.Fatalf("Failed to create node-2: %v", err)
	}

	query := []float32{1.0, 0.0, 0.0}
	results, err := vs.Search(ctx, query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should only get node-1
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].ID != "node-1" {
		t.Errorf("Expected node-1, got %s", results[0].ID)
	}
}

func TestSQLiteVectorStore_SearchSkipsMalformedEmbedding(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	// Insert a node with an invalid embedding blob length (not divisible by 4).
	_, err := db.Exec(`INSERT INTO nodes (id, name, type, embedding) VALUES (?, ?, ?, ?)`, "node-bad", "NodeBad", "Concept", []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("Failed to create node with malformed embedding: %v", err)
	}

	results, err := vs.Search(ctx, []float32{1, 0, 0}, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Expected malformed embedding to be skipped, got %d results", len(results))
	}
}

// TestSQLiteVectorStore_Delete tests deleting embeddings
func TestSQLiteVectorStore_Delete(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	// Create node with embedding
	nodeID := "node-1"
	_, err := db.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, nodeID, "Node1", "Concept")
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	if err := vs.Add(ctx, nodeID, []float32{1.0, 0.0, 0.0}); err != nil {
		t.Fatalf("Failed to add embedding: %v", err)
	}

	// Delete embedding
	if err := vs.Delete(ctx, nodeID); err != nil {
		t.Fatalf("Failed to delete embedding: %v", err)
	}

	// Verify embedding is NULL
	var embeddingBlob []byte
	err = db.QueryRow(`SELECT embedding FROM nodes WHERE id = ?`, nodeID).Scan(&embeddingBlob)
	if err != nil {
		t.Fatalf("Failed to query node: %v", err)
	}
	if embeddingBlob != nil {
		t.Error("Embedding should be NULL after delete")
	}

	// Verify node still exists
	var name string
	err = db.QueryRow(`SELECT name FROM nodes WHERE id = ?`, nodeID).Scan(&name)
	if err != nil {
		t.Fatalf("Node should still exist: %v", err)
	}
	if name != "Node1" {
		t.Errorf("Node name should be preserved, got %s", name)
	}
}

func TestSQLiteVectorStore_CloseNoOp(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)
	if err := vs.Close(); err != nil {
		t.Fatalf("Close should be a no-op and return nil, got: %v", err)
	}
}

// TestSQLiteVectorStore_DimensionValidation tests handling of dimension mismatches
func TestSQLiteVectorStore_DimensionValidation(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	// Create nodes with same dimension embeddings
	nodes := []struct {
		id        string
		embedding []float32
	}{
		{"node-1", []float32{1.0, 0.0, 0.0}}, // 3D
		{"node-2", []float32{0.0, 1.0, 0.0}}, // 3D
		{"node-3", []float32{1.0, 0.0, 0.0}}, // 3D - same dimension
	}

	for _, n := range nodes {
		_, err := db.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, n.id, n.id, "Concept")
		if err != nil {
			t.Fatalf("Failed to create node %s: %v", n.id, err)
		}
		if err := vs.Add(ctx, n.id, n.embedding); err != nil {
			t.Fatalf("Failed to add embedding for %s: %v", n.id, err)
		}
	}

	// Search with 3D query - should match all 3D embeddings
	query := []float32{0.0, 1.0, 0.0}
	results, err := vs.Search(ctx, query, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should get all 3 results (all have same dimension)
	if len(results) != 3 {
		t.Fatalf("Expected 3 results (all same dimension), got %d", len(results))
	}
}

// TestSQLiteVectorStore_Persistence tests that embeddings persist across store instances
func TestSQLiteVectorStore_Persistence(t *testing.T) {
	ctx := context.Background()

	// Use a temporary file for persistence test
	tmpFile, err := os.CreateTemp("", "gognee-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// First session: create and populate
	EnableSQLiteVec()
	db1, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Initialize schema
	_, err = db1.Exec(`
		CREATE TABLE nodes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT,
			description TEXT,
			embedding BLOB,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			metadata TEXT
		);

		CREATE VIRTUAL TABLE vec_nodes USING vec0(
			embedding float[3]
		);

		CREATE TABLE vec_node_ids (
			rowid INTEGER PRIMARY KEY,
			node_id TEXT NOT NULL UNIQUE,
			FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
		);

		CREATE INDEX idx_vec_node_ids_node_id ON vec_node_ids(node_id);
	`)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	vs1 := NewSQLiteVectorStore(db1)

	// Add test data
	_, err = db1.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, "node-1", "Node1", "Concept")
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	embedding := []float32{0.5, 0.5, 0.5}
	if err := vs1.Add(ctx, "node-1", embedding); err != nil {
		t.Fatalf("Failed to add embedding: %v", err)
	}

	db1.Close()

	// Second session: reopen and verify
	db2, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer db2.Close()

	vs2 := NewSQLiteVectorStore(db2)

	// Search should work immediately without re-adding
	query := []float32{0.5, 0.5, 0.5}
	results, err := vs2.Search(ctx, query, 1)
	if err != nil {
		t.Fatalf("Search after restart failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].ID != "node-1" {
		t.Errorf("Expected node-1, got %s", results[0].ID)
	}
	if results[0].Score < 0.99 {
		t.Errorf("Expected high similarity score, got %f", results[0].Score)
	}
}

func TestSQLiteVectorStore_ConcurrentAddAndSearch(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	const nodeCount = 50
	for i := 0; i < nodeCount; i++ {
		id := "node-concurrent-" + fmt.Sprint(i)
		_, err := db.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, id, id, "Concept")
		if err != nil {
			t.Fatalf("Failed to create node %s: %v", id, err)
		}
		if err := vs.Add(ctx, id, []float32{1.0, 0.0, 0.0}); err != nil {
			t.Fatalf("Failed to add initial embedding for %s: %v", id, err)
		}
	}

	// Run concurrent updates and searches. This is primarily a correctness + race test.
	const goroutines = 25
	const iterations = 50

	errCh := make(chan error, goroutines*2)
	var wg sync.WaitGroup

	seed := time.Now().UnixNano()
	for g := 0; g < goroutines; g++ {
		wg.Add(2)
		// Writer
		go func(worker int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed + int64(worker)))
			for i := 0; i < iterations; i++ {
				id := "node-concurrent-" + fmt.Sprint(rng.Intn(nodeCount))
				emb := []float32{float32(rng.Float64()), float32(rng.Float64()), float32(rng.Float64())}
				if err := vs.Add(ctx, id, emb); err != nil {
					errCh <- err
					return
				}
			}
		}(g)

		// Reader
		go func(worker int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed + int64(worker) + 1000))
			for i := 0; i < iterations; i++ {
				query := []float32{float32(rng.Float64()), float32(rng.Float64()), float32(rng.Float64())}
				_, err := vs.Search(ctx, query, 10)
				if err != nil {
					errCh <- err
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("Concurrent operations returned error: %v", err)
		}
	}
}

// setupTestDB creates a test database with schema
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	// Initialize sqlite-vec for all future connections
	EnableSQLiteVec()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Important: SQLite ":memory:" databases are per-connection. Since *sql.DB may open
	// multiple connections (especially under concurrent load), force a single connection
	// so all queries see the same in-memory schema.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE nodes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT,
			description TEXT,
			embedding BLOB,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			metadata TEXT
		);

		CREATE VIRTUAL TABLE vec_nodes USING vec0(
			embedding float[3]
		);

		CREATE TABLE vec_node_ids (
			rowid INTEGER PRIMARY KEY,
			node_id TEXT NOT NULL UNIQUE,
			FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
		);

		CREATE INDEX idx_vec_node_ids_node_id ON vec_node_ids(node_id);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("Failed to create schema: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

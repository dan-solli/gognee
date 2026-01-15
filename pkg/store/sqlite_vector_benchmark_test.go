package store

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setupBenchmarkDB creates a test database for benchmarks
func setupBenchmarkDB(b *testing.B) (*sql.DB, func()) {
	EnableSQLiteVec()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatalf("Failed to open test database: %v", err)
	}

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
		b.Fatalf("Failed to create schema: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

// BenchmarkVectorSearch_1000Nodes establishes a performance baseline for vector search
// with 1K nodes. This is a rudimentary benchmark for regression detection.
//
// Expected performance: <500ms for 1K nodes with vec0 indexed search.
// Baseline will be documented in the QA report.
func BenchmarkVectorSearch_1000Nodes(b *testing.B) {
	ctx := context.Background()
	db, cleanup := setupBenchmarkDB(b)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	// Populate store with 1000 nodes and embeddings (3D for test schema)
	const nodeCount = 1000
	rng := rand.New(rand.NewSource(42)) // Deterministic for reproducibility

	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("node-%d", i)

		// Create node in database
		_, err := db.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, nodeID, nodeID, "Concept")
		if err != nil {
			b.Fatalf("Failed to create node %s: %v", nodeID, err)
		}

		// Generate deterministic fake embedding (3D for test schema)
		embedding := []float32{
			rng.Float32(),
			rng.Float32(),
			rng.Float32(),
		}

		// Add embedding to vector store
		if err := vs.Add(ctx, nodeID, embedding); err != nil {
			b.Fatalf("Failed to add embedding for %s: %v", nodeID, err)
		}
	}

	// Reset timer to exclude setup time
	b.ResetTimer()

	// Benchmark search operations
	for i := 0; i < b.N; i++ {
		// Generate random query vector
		query := []float32{
			rng.Float32(),
			rng.Float32(),
			rng.Float32(),
		}

		// Search for top 10 results
		results, err := vs.Search(ctx, query, 10)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}

		// Verify results to prevent compiler optimization
		if len(results) == 0 {
			b.Fatal("Expected search results, got none")
		}
	}
}

// BenchmarkVectorAdd_Concurrent benchmarks concurrent Add operations
// to validate transaction serialization performance.
func BenchmarkVectorAdd_Concurrent(b *testing.B) {
	ctx := context.Background()
	db, cleanup := setupBenchmarkDB(b)
	defer cleanup()

	vs := NewSQLiteVectorStore(db)

	// Pre-populate with nodes
	const nodeCount = 100
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		_, err := db.Exec(`INSERT INTO nodes (id, name, type) VALUES (?, ?, ?)`, nodeID, nodeID, "Concept")
		if err != nil {
			b.Fatalf("Failed to create node: %v", err)
		}
	}

	rng := rand.New(rand.NewSource(42))

	b.ResetTimer()

	// Run concurrent adds
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			nodeID := fmt.Sprintf("node-%d", rng.Intn(nodeCount))
			embedding := []float32{
				rng.Float32(),
				rng.Float32(),
				rng.Float32(),
			}

			if err := vs.Add(ctx, nodeID, embedding); err != nil {
				b.Fatalf("Concurrent add failed: %v", err)
			}
		}
	})
}

package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

// SQLiteVectorStore implements VectorStore using SQLite as the persistence layer.
// It stores embeddings in the nodes.embedding BLOB column and provides
// vector similarity search using cosine similarity computed in Go.
//
// Implementation notes:
// - Embeddings are stored directly in the nodes table's embedding column
// - Search performs a linear scan (SELECT all non-NULL embeddings, compute similarity in Go)
// - No in-memory caching - SQLite is the source of truth
// - Dimension mismatches are handled by skipping incompatible vectors during search
// - The database connection is shared with SQLiteGraphStore and must not be closed by this store
type SQLiteVectorStore struct {
	db *sql.DB
}

// NewSQLiteVectorStore creates a new SQLite-backed vector store.
// The database connection is shared and owned by the caller (typically SQLiteGraphStore).
// The SQLiteVectorStore must not close this connection.
func NewSQLiteVectorStore(db *sql.DB) *SQLiteVectorStore {
	return &SQLiteVectorStore{db: db}
}

// Add adds or updates an embedding for the given node ID.
// The node must already exist in the nodes table.
// Returns an error if the node doesn't exist or if the database operation fails.
func (s *SQLiteVectorStore) Add(ctx context.Context, id string, embedding []float32) error {
	if len(embedding) == 0 {
		return fmt.Errorf("embedding cannot be empty")
	}

	// Serialize embedding to binary format (little-endian float32 array)
	blob := serializeEmbedding(embedding)

	// Update the embedding column for the specified node
	result, err := s.db.ExecContext(ctx, `UPDATE nodes SET embedding = ? WHERE id = ?`, blob, id)
	if err != nil {
		return fmt.Errorf("failed to update embedding: %w", err)
	}

	// Check if the node exists
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("node %s not found", id)
	}

	return nil
}

// Search finds the most similar vectors to the query using cosine similarity.
// Performs a direct-query linear scan: SELECT all non-NULL embeddings from the database,
// compute cosine similarity in Go, and return the top-K results sorted by score descending.
//
// Behavior:
// - Nodes without embeddings (NULL) are skipped
// - Embeddings with dimensions different from the query are skipped (CosineSimilarity returns 0)
// - Results are sorted by similarity score in descending order
// - Returns up to topK results (may be fewer if the store has fewer vectors)
func (s *SQLiteVectorStore) Search(ctx context.Context, query []float32, topK int) ([]SearchResult, error) {
	if len(query) == 0 {
		return []SearchResult{}, nil
	}

	// Query all nodes with non-NULL embeddings
	rows, err := s.db.QueryContext(ctx, `SELECT id, embedding FROM nodes WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to query embeddings: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var id string
		var embeddingBlob []byte

		if err := rows.Scan(&id, &embeddingBlob); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Deserialize embedding
		embedding := deserializeEmbedding(embeddingBlob)
		if embedding == nil {
			// Skip malformed embeddings
			continue
		}

		// Compute similarity (CosineSimilarity returns 0 for dimension mismatches)
		score := CosineSimilarity(query, embedding)

		// Skip vectors with dimension mismatch (score will be 0)
		// Only include if dimensions match (non-zero similarity possible)
		if len(embedding) == len(query) {
			results = append(results, SearchResult{
				ID:    id,
				Score: score,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top-K
	if topK < len(results) {
		results = results[:topK]
	}

	return results, nil
}

// Delete removes the embedding for the given node ID.
// The node itself is not deleted, only the embedding column is set to NULL.
// This allows the node to remain in the graph while removing it from vector search.
func (s *SQLiteVectorStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET embedding = NULL WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete embedding: %w", err)
	}
	return nil
}

// Close is a no-op for SQLiteVectorStore because it shares the database connection
// with SQLiteGraphStore. The connection lifecycle is managed by the owner (GraphStore).
func (s *SQLiteVectorStore) Close() error {
	// No-op: connection is owned by GraphStore
	return nil
}

// serializeEmbedding converts a float32 slice to a binary BLOB for storage.
// Uses little-endian encoding for consistency across platforms.
func serializeEmbedding(embedding []float32) []byte {
	blob := make([]byte, len(embedding)*4)
	for i, val := range embedding {
		bits := math.Float32bits(val)
		binary.LittleEndian.PutUint32(blob[i*4:(i+1)*4], bits)
	}
	return blob
}

// deserializeEmbedding converts a binary BLOB back to a float32 slice.
// Returns nil if the data is malformed (not a multiple of 4 bytes).
func deserializeEmbedding(data []byte) []float32 {
	if len(data) == 0 {
		return nil
	}
	if len(data)%4 != 0 {
		// Malformed data
		return nil
	}

	embedding := make([]float32, len(data)/4)
	for i := 0; i < len(embedding); i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
		embedding[i] = math.Float32frombits(bits)
	}
	return embedding
}

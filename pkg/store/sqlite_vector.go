package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
)

// SQLiteVectorStore implements VectorStore using SQLite with sqlite-vec as the persistence layer.
// It uses vec0 virtual tables for indexed approximate nearest neighbor (ANN) vector search.
//
// Implementation notes:
// - Embeddings are stored in the vec_nodes virtual table (vec0) for indexed ANN search
// - A mapping table (vec_node_ids) correlates vec0 rowids with string node IDs
// - Search uses vec0 MATCH operator for efficient O(log n) complexity instead of O(n) linear scan
// - Legacy nodes.embedding column is maintained for backwards compatibility
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
//
// Implementation uses vec0 virtual table for indexed vector storage:
// 1. Checks if node exists in nodes table
// 2. Inserts/updates entry in vec_node_ids mapping table
// 3. Inserts/replaces vector in vec_nodes virtual table
// 4. Updates legacy embedding column in nodes table for backwards compatibility
func (s *SQLiteVectorStore) Add(ctx context.Context, id string, embedding []float32) error {
	if len(embedding) == 0 {
		return fmt.Errorf("embedding cannot be empty")
	}

	// Verify node exists
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM nodes WHERE id = ?`, id).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("node %s not found", id)
	}
	if err != nil {
		return fmt.Errorf("failed to check node existence: %w", err)
	}

	// Start transaction for atomic vec0 + mapping + legacy update
	// Use immediate transaction to avoid UNIQUE constraint issues with concurrent writes
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get or create rowid mapping
	var rowid int64
	err = tx.QueryRowContext(ctx, `SELECT rowid FROM vec_node_ids WHERE node_id = ?`, id).Scan(&rowid)
	if err == sql.ErrNoRows {
		// Insert new mapping (rowid will be auto-generated)
		result, err := tx.ExecContext(ctx, `INSERT INTO vec_node_ids (node_id) VALUES (?)`, id)
		if err != nil {
			return fmt.Errorf("failed to create vec_node_ids mapping: %w", err)
		}
		rowid, err = result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert rowid: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query vec_node_ids: %w", err)
	} else {
		// Rowid exists, delete old vec_nodes entry first to avoid UNIQUE constraint
		_, err = tx.ExecContext(ctx, `DELETE FROM vec_nodes WHERE rowid = ?`, rowid)
		if err != nil {
			return fmt.Errorf("failed to delete old vec_nodes entry: %w", err)
		}
	}

	// Serialize embedding for vec0 (float32 array as blob)
	blob := serializeEmbedding(embedding)

	// Insert new entry in vec_nodes virtual table
	_, err = tx.ExecContext(ctx, `INSERT INTO vec_nodes (rowid, embedding) VALUES (?, ?)`, rowid, blob)
	if err != nil {
		return fmt.Errorf("failed to insert into vec_nodes: %w", err)
	}

	// Update legacy embedding column in nodes table for backwards compatibility
	_, err = tx.ExecContext(ctx, `UPDATE nodes SET embedding = ? WHERE id = ?`, blob, id)
	if err != nil {
		return fmt.Errorf("failed to update nodes embedding column: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Search finds the most similar vectors to the query using vec0 indexed search.
// Uses sqlite-vec's MATCH operator for efficient approximate nearest neighbor (ANN) search.
//
// Behavior:
// - Performs indexed ANN search via vec0 virtual table (O(log n) complexity)
// - Returns distance metric from vec0, converted to similarity score (1 - distance)
// - Maps rowid back to node string ID via vec_node_ids table
// - Results are sorted by similarity score in descending order (best matches first)
// - Returns up to topK results
func (s *SQLiteVectorStore) Search(ctx context.Context, query []float32, topK int) ([]SearchResult, error) {
	if len(query) == 0 {
		return []SearchResult{}, nil
	}

	// Serialize query embedding for vec0 MATCH
	queryBlob := serializeEmbedding(query)

	// vec0 MATCH query with distance metric
	// The MATCH operator returns results ordered by distance (ascending)
	// We'll convert distance to similarity score (1 - distance for cosine-like behavior)
	// Note: vec0 requires 'k = ?' constraint for knn queries
	rows, err := s.db.QueryContext(ctx, `
		SELECT 
			vec_node_ids.node_id,
			distance
		FROM vec_nodes
		INNER JOIN vec_node_ids ON vec_nodes.rowid = vec_node_ids.rowid
		WHERE embedding MATCH ? AND k = ?
		ORDER BY distance
	`, queryBlob, topK)
	if err != nil {
		return nil, fmt.Errorf("failed to execute vec0 search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var nodeID string
		var distance float64

		if err := rows.Scan(&nodeID, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		// Convert distance to similarity score
		// vec0 returns L2 distance by default; convert to similarity (1 - normalized_distance)
		// For compatibility with previous cosine similarity scores (0-1 range)
		similarity := 1.0 - distance

		results = append(results, SearchResult{
			ID:    nodeID,
			Score: similarity,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return results, nil
}

// Delete removes the embedding for the given node ID.
// The node itself is not deleted, only the embedding is removed from:
// - vec_nodes virtual table
// - vec_node_ids mapping table
// - nodes.embedding column (legacy, set to NULL)
// This allows the node to remain in the graph while removing it from vector search.
func (s *SQLiteVectorStore) Delete(ctx context.Context, id string) error {
	// Start transaction for atomic deletion
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get rowid for this node
	var rowid int64
	err = tx.QueryRowContext(ctx, `SELECT rowid FROM vec_node_ids WHERE node_id = ?`, id).Scan(&rowid)
	if err == sql.ErrNoRows {
		// Node has no embedding - this is not an error
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to query vec_node_ids: %w", err)
	}

	// Delete from vec_nodes virtual table
	_, err = tx.ExecContext(ctx, `DELETE FROM vec_nodes WHERE rowid = ?`, rowid)
	if err != nil {
		return fmt.Errorf("failed to delete from vec_nodes: %w", err)
	}

	// Delete from mapping table
	_, err = tx.ExecContext(ctx, `DELETE FROM vec_node_ids WHERE rowid = ?`, rowid)
	if err != nil {
		return fmt.Errorf("failed to delete from vec_node_ids: %w", err)
	}

	// Set legacy embedding column to NULL
	_, err = tx.ExecContext(ctx, `UPDATE nodes SET embedding = NULL WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to clear nodes embedding column: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
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

package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteGraphStore implements GraphStore using SQLite as the backend.
type SQLiteGraphStore struct {
	db *sql.DB
}

// NewSQLiteGraphStore creates a new SQLite-backed graph store.
// The dbPath can be a file path or ":memory:" for an in-memory database.
// Creates tables and indexes if they don't exist.
func NewSQLiteGraphStore(dbPath string) (*SQLiteGraphStore, error) {
	// Initialize sqlite-vec for all future connections
	EnableSQLiteVec()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign key constraints (required for CASCADE)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	store := &SQLiteGraphStore{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database schema if it doesn't exist.
// Also performs schema migrations for new columns.
func (s *SQLiteGraphStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL COLLATE NOCASE,
		type TEXT,
		description TEXT,
		embedding BLOB,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name COLLATE NOCASE);

	CREATE TABLE IF NOT EXISTS edges (
		id TEXT PRIMARY KEY,
		source_id TEXT NOT NULL,
		relation TEXT NOT NULL,
		target_id TEXT NOT NULL,
		weight REAL DEFAULT 1.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (source_id) REFERENCES nodes(id),
		FOREIGN KEY (target_id) REFERENCES nodes(id)
	);

	CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source_id);
	CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_id);

	CREATE TABLE IF NOT EXISTS processed_documents (
		hash TEXT PRIMARY KEY,
		source TEXT,
		processed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		chunk_count INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_processed_documents_source ON processed_documents(source);

	-- vec0 virtual table for indexed vector search (sqlite-vec)
	CREATE VIRTUAL TABLE IF NOT EXISTS vec_nodes USING vec0(
		embedding float[1536]
	);

	-- ID mapping table: correlates vec_nodes.rowid with nodes.id (string UUIDs)
	CREATE TABLE IF NOT EXISTS vec_node_ids (
		rowid INTEGER PRIMARY KEY,
		node_id TEXT NOT NULL UNIQUE,
		FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_vec_node_ids_node_id ON vec_node_ids(node_id);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return err
	}

	// Run schema migrations for new columns
	return s.migrateSchema()
}

// migrateSchema adds new columns to existing tables if they don't exist.
func (s *SQLiteGraphStore) migrateSchema() error {
	// Check and add last_accessed_at column
	if !s.columnExists("nodes", "last_accessed_at") {
		_, err := s.db.Exec("ALTER TABLE nodes ADD COLUMN last_accessed_at DATETIME DEFAULT NULL")
		if err != nil {
			return fmt.Errorf("failed to add last_accessed_at column: %w", err)
		}
	}

	// Check and add access_count column
	if !s.columnExists("nodes", "access_count") {
		_, err := s.db.Exec("ALTER TABLE nodes ADD COLUMN access_count INTEGER DEFAULT 0")
		if err != nil {
			return fmt.Errorf("failed to add access_count column: %w", err)
		}
	}

	// Phase 2: Add memory CRUD tables (v1.0.0)
	if err := s.migrateMemoryTables(); err != nil {
		return err
	}

	return nil
}

// migrateMemoryTables adds memory CRUD tables for v1.0.0.
func (s *SQLiteGraphStore) migrateMemoryTables() error {
	// Check if memories table exists
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='memories'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for memories table: %w", err)
	}

	// If table exists, assume all memory tables exist
	if count > 0 {
		return nil
	}

	// Create memory tables
	schema := `
	CREATE TABLE memories (
		id TEXT PRIMARY KEY,
		topic TEXT NOT NULL,
		context TEXT NOT NULL,
		decisions_json TEXT,
		rationale_json TEXT,
		metadata_json TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		version INTEGER DEFAULT 1,
		doc_hash TEXT NOT NULL,
		source TEXT,
		status TEXT DEFAULT 'complete'
	);

	CREATE INDEX idx_memories_topic ON memories(topic);
	CREATE INDEX idx_memories_doc_hash ON memories(doc_hash);
	CREATE INDEX idx_memories_status ON memories(status);

	CREATE TABLE memory_nodes (
		memory_id TEXT NOT NULL,
		node_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (memory_id, node_id),
		FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE
	);

	CREATE INDEX idx_memory_nodes_node_id ON memory_nodes(node_id);

	CREATE TABLE memory_edges (
		memory_id TEXT NOT NULL,
		edge_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (memory_id, edge_id),
		FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE
	);

	CREATE INDEX idx_memory_edges_edge_id ON memory_edges(edge_id);
	`

	_, err = s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create memory tables: %w", err)
	}

	return nil
}

// columnExists checks if a column exists in a table.
func (s *SQLiteGraphStore) columnExists(tableName, columnName string) bool {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := s.db.Query(query)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int

		err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
		if err != nil {
			return false
		}

		if name == columnName {
			return true
		}
	}

	return false
}

// AddNode adds or updates a node in the graph.
func (s *SQLiteGraphStore) AddNode(ctx context.Context, node *Node) error {
	// Generate ID if not provided
	if node.ID == "" {
		node.ID = uuid.New().String()
	}

	// Set created time if not provided
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}

	// Serialize embedding to bytes
	var embeddingBytes []byte
	if len(node.Embedding) > 0 {
		embeddingBytes = make([]byte, len(node.Embedding)*4)
		for i, v := range node.Embedding {
			binary.LittleEndian.PutUint32(embeddingBytes[i*4:], math.Float32bits(v))
		}
	}

	// Serialize metadata to JSON
	var metadataJSON []byte
	var err error
	if node.Metadata != nil {
		metadataJSON, err = json.Marshal(node.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	query := `
		INSERT OR REPLACE INTO nodes (id, name, type, description, embedding, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		node.ID,
		node.Name,
		node.Type,
		node.Description,
		embeddingBytes,
		node.CreatedAt,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to add node: %w", err)
	}

	return nil
}

// GetNode retrieves a node by its ID.
// Also updates last_accessed_at timestamp to track access for decay.
func (s *SQLiteGraphStore) GetNode(ctx context.Context, id string) (*Node, error) {
	query := `
		SELECT id, name, type, description, embedding, created_at, metadata
		FROM nodes
		WHERE id = ?
	`

	var node Node
	var embeddingBytes []byte
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&node.ID,
		&node.Name,
		&node.Type,
		&node.Description,
		&embeddingBytes,
		&node.CreatedAt,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found, no error
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	// Deserialize embedding
	if len(embeddingBytes) > 0 {
		node.Embedding = make([]float32, len(embeddingBytes)/4)
		for i := range node.Embedding {
			node.Embedding[i] = math.Float32frombits(binary.LittleEndian.Uint32(embeddingBytes[i*4:]))
		}
	}

	// Deserialize metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &node.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Update last_accessed_at timestamp
	_, err = s.db.ExecContext(ctx, "UPDATE nodes SET last_accessed_at = ? WHERE id = ?", time.Now(), id)
	if err != nil {
		// Log but don't fail - access tracking is not critical
		// In production, could use a logger here
	}

	return &node, nil
}

// FindNodesByName searches for nodes by name using case-insensitive matching.
func (s *SQLiteGraphStore) FindNodesByName(ctx context.Context, name string) ([]*Node, error) {
	query := `
		SELECT id, name, type, description, embedding, created_at, metadata
		FROM nodes
		WHERE LOWER(name) = LOWER(?)
		ORDER BY created_at, id
	`

	rows, err := s.db.QueryContext(ctx, query, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find nodes by name: %w", err)
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		var node Node
		var embeddingBytes []byte
		var metadataJSON []byte

		err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.Type,
			&node.Description,
			&embeddingBytes,
			&node.CreatedAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}

		// Deserialize embedding
		if len(embeddingBytes) > 0 {
			node.Embedding = make([]float32, len(embeddingBytes)/4)
			for i := range node.Embedding {
				node.Embedding[i] = math.Float32frombits(binary.LittleEndian.Uint32(embeddingBytes[i*4:]))
			}
		}

		// Deserialize metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &node.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		nodes = append(nodes, &node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating nodes: %w", err)
	}

	return nodes, nil
}

// FindNodeByName is a convenience method that returns a single node if exactly one matches.
func (s *SQLiteGraphStore) FindNodeByName(ctx context.Context, name string) (*Node, error) {
	nodes, err := s.FindNodesByName(ctx, name)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, ErrNodeNotFound
	}

	if len(nodes) > 1 {
		return nil, ErrAmbiguousNode
	}

	return nodes[0], nil
}

// AddEdge adds or updates an edge in the graph.
func (s *SQLiteGraphStore) AddEdge(ctx context.Context, edge *Edge) error {
	// Generate ID if not provided
	if edge.ID == "" {
		edge.ID = uuid.New().String()
	}

	// Set created time if not provided
	if edge.CreatedAt.IsZero() {
		edge.CreatedAt = time.Now()
	}

	// Default weight to 1.0 if not provided
	if edge.Weight == 0 {
		edge.Weight = 1.0
	}

	query := `
		INSERT OR REPLACE INTO edges (id, source_id, relation, target_id, weight, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		edge.ID,
		edge.SourceID,
		edge.Relation,
		edge.TargetID,
		edge.Weight,
		edge.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to add edge: %w", err)
	}

	return nil
}

// GetEdges retrieves all edges incident to a node (both incoming and outgoing).
func (s *SQLiteGraphStore) GetEdges(ctx context.Context, nodeID string) ([]*Edge, error) {
	query := `
		SELECT id, source_id, relation, target_id, weight, created_at
		FROM edges
		WHERE source_id = ? OR target_id = ?
		ORDER BY created_at
	`

	rows, err := s.db.QueryContext(ctx, query, nodeID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get edges: %w", err)
	}
	defer rows.Close()

	var edges []*Edge
	for rows.Next() {
		var edge Edge
		err := rows.Scan(
			&edge.ID,
			&edge.SourceID,
			&edge.Relation,
			&edge.TargetID,
			&edge.Weight,
			&edge.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan edge: %w", err)
		}
		edges = append(edges, &edge)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating edges: %w", err)
	}

	return edges, nil
}

// GetNeighbors retrieves all nodes adjacent to a given node, up to the specified depth.
// Uses a recursive CTE for efficient single-query graph expansion (v1.4.0 optimization).
func (s *SQLiteGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int) ([]*Node, error) {
	if depth < 1 {
		return nil, fmt.Errorf("depth must be at least 1")
	}

	// Recursive CTE to traverse graph bidirectionally up to depth
	query := `
	WITH RECURSIVE
	graph_traversal(node_id, depth_level) AS (
		-- Base case: starting node at depth 0
		SELECT ? AS node_id, 0 AS depth_level
		
		UNION
		
		-- Recursive case: expand to neighbors
		SELECT 
			CASE 
				WHEN edges.source_id = graph_traversal.node_id THEN edges.target_id
				ELSE edges.source_id
			END AS node_id,
			graph_traversal.depth_level + 1 AS depth_level
		FROM graph_traversal
		JOIN edges ON (
			edges.source_id = graph_traversal.node_id OR 
			edges.target_id = graph_traversal.node_id
		)
		WHERE graph_traversal.depth_level < ?
	)
	SELECT DISTINCT 
		n.id, n.name, n.type, n.description, n.embedding, 
		n.created_at, n.last_accessed_at, n.metadata
	FROM graph_traversal gt
	JOIN nodes n ON gt.node_id = n.id
	WHERE gt.node_id != ? -- Exclude starting node
	`

	rows, err := s.db.QueryContext(ctx, query, nodeID, depth, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query neighbors with CTE: %w", err)
	}
	defer rows.Close()

	var neighbors []*Node
	for rows.Next() {
		node := &Node{}
		var embeddingData []byte
		var metadataJSON []byte
		var lastAccessed sql.NullTime

		err := rows.Scan(
			&node.ID, &node.Name, &node.Type, &node.Description,
			&embeddingData, &node.CreatedAt, &lastAccessed, &metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan neighbor node: %w", err)
		}

		// Deserialize embedding
		if len(embeddingData) > 0 {
			node.Embedding = deserializeEmbedding(embeddingData)
		}

		// Deserialize metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &node.Metadata); err != nil {
				node.Metadata = make(map[string]interface{})
			}
		} else {
			node.Metadata = make(map[string]interface{})
		}

		// Handle nullable last_accessed_at
		if lastAccessed.Valid {
			node.LastAccessedAt = &lastAccessed.Time
		}

		neighbors = append(neighbors, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating neighbor rows: %w", err)
	}

	return neighbors, nil
}

// NodeCount returns the total number of nodes in the graph.
func (s *SQLiteGraphStore) NodeCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM nodes").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count nodes: %w", err)
	}
	return count, nil
}

// EdgeCount returns the total number of edges in the graph.
func (s *SQLiteGraphStore) EdgeCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM edges").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count edges: %w", err)
	}
	return count, nil
}

// UpdateAccessTime updates the last_accessed_at timestamp for a batch of nodes.
// This is used for access reinforcement in memory decay.
func (s *SQLiteGraphStore) UpdateAccessTime(ctx context.Context, nodeIDs []string) error {
	if len(nodeIDs) == 0 {
		return nil
	}

	// Build IN clause with placeholders
	placeholders := make([]string, len(nodeIDs))
	args := make([]interface{}, len(nodeIDs)+1)
	args[0] = time.Now()

	for i, nodeID := range nodeIDs {
		placeholders[i] = "?"
		args[i+1] = nodeID
	}

	query := fmt.Sprintf("UPDATE nodes SET last_accessed_at = ? WHERE id IN (%s)",
		strings.Join(placeholders, ","))

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update access time: %w", err)
	}

	return nil
}

// GetAllNodes returns all nodes in the graph (for pruning operations).
func (s *SQLiteGraphStore) GetAllNodes(ctx context.Context) ([]*Node, error) {
	query := `
		SELECT id, name, type, description, embedding, created_at, metadata, last_accessed_at
		FROM nodes
		ORDER BY created_at, id
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		var node Node
		var embeddingBytes []byte
		var metadataJSON []byte
		var lastAccessed sql.NullTime

		err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.Type,
			&node.Description,
			&embeddingBytes,
			&node.CreatedAt,
			&metadataJSON,
			&lastAccessed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan node: %w", err)
		}

		// Deserialize embedding
		if len(embeddingBytes) > 0 {
			node.Embedding = make([]float32, len(embeddingBytes)/4)
			for i := range node.Embedding {
				node.Embedding[i] = math.Float32frombits(binary.LittleEndian.Uint32(embeddingBytes[i*4:]))
			}
		}

		// Deserialize metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &node.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		// Hydrate last_accessed_at if it's not NULL
		if lastAccessed.Valid {
			node.LastAccessedAt = &lastAccessed.Time
		}

		nodes = append(nodes, &node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating nodes: %w", err)
	}

	return nodes, nil
}

// DeleteNode removes a node from the graph.
func (s *SQLiteGraphStore) DeleteNode(ctx context.Context, nodeID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM nodes WHERE id = ?", nodeID)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}
	return nil
}

// DeleteEdge removes an edge from the graph.
func (s *SQLiteGraphStore) DeleteEdge(ctx context.Context, edgeID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM edges WHERE id = ?", edgeID)
	if err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}
	return nil
}

// Close releases database resources.
func (s *SQLiteGraphStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection.
// This connection is shared with other stores (e.g., SQLiteVectorStore)
// and must not be closed by consumers.
func (s *SQLiteGraphStore) DB() *sql.DB {
	return s.db
}

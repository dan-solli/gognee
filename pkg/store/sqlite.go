package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // SQLite driver
)

// SQLiteGraphStore implements GraphStore using SQLite as the backend.
type SQLiteGraphStore struct {
	db *sql.DB
}

// NewSQLiteGraphStore creates a new SQLite-backed graph store.
// The dbPath can be a file path or ":memory:" for an in-memory database.
// Creates tables and indexes if they don't exist.
func NewSQLiteGraphStore(dbPath string) (*SQLiteGraphStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteGraphStore{db: db}
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database schema if it doesn't exist.
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
	`

	_, err := s.db.Exec(schema)
	return err
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
func (s *SQLiteGraphStore) GetNeighbors(ctx context.Context, nodeID string, depth int) ([]*Node, error) {
	if depth < 1 {
		return nil, fmt.Errorf("depth must be at least 1")
	}

	// Track visited nodes to avoid duplicates
	visited := make(map[string]bool)
	visited[nodeID] = true

	// Current frontier of nodes to explore
	frontier := []string{nodeID}

	// For each depth level
	for d := 0; d < depth; d++ {
		var nextFrontier []string

		// For each node in current frontier
		for _, currentID := range frontier {
			// Get all incident edges
			edges, err := s.GetEdges(ctx, currentID)
			if err != nil {
				return nil, err
			}

			// Find neighbor node IDs
			for _, edge := range edges {
				var neighborID string
				if edge.SourceID == currentID {
					neighborID = edge.TargetID
				} else {
					neighborID = edge.SourceID
				}

				// Add to next frontier if not visited
				if !visited[neighborID] {
					visited[neighborID] = true
					nextFrontier = append(nextFrontier, neighborID)
				}
			}
		}

		frontier = nextFrontier
		if len(frontier) == 0 {
			break // No more neighbors to explore
		}
	}

	// Remove the starting node from visited set
	delete(visited, nodeID)

	// Fetch all neighbor nodes
	var neighbors []*Node
	for neighborID := range visited {
		node, err := s.GetNode(ctx, neighborID)
		if err != nil {
			return nil, err
		}
		if node != nil {
			neighbors = append(neighbors, node)
		}
	}

	return neighbors, nil
}

// Close releases database resources.
func (s *SQLiteGraphStore) Close() error {
	return s.db.Close()
}

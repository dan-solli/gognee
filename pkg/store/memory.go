package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MemoryRecord represents a first-class memory with structured payload.
type MemoryRecord struct {
	ID        string                 `json:"id"`
	Topic     string                 `json:"topic"`
	Context   string                 `json:"context"`
	Decisions []string               `json:"decisions,omitempty"`
	Rationale []string               `json:"rationale,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Version   int                    `json:"version"`
	DocHash   string                 `json:"doc_hash"`
	Source    string                 `json:"source,omitempty"`
	Status    string                 `json:"status"` // "pending" or "complete"
}

// MemorySummary provides a lightweight view of a memory for list operations.
type MemorySummary struct {
	ID            string    `json:"id"`
	Topic         string    `json:"topic"`
	Preview       string    `json:"preview"` // Truncated context
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	DecisionCount int       `json:"decision_count"`
	Status        string    `json:"status"`
}

// ListMemoriesOptions provides pagination for memory listing.
type ListMemoriesOptions struct {
	Offset int
	Limit  int // Default 50, max 100
}

// MemoryUpdate represents partial updates to a memory.
// All fields are pointers to distinguish between "not provided" and "set to zero value".
type MemoryUpdate struct {
	Topic     *string
	Context   *string
	Decisions *[]string
	Rationale *[]string
	Metadata  *map[string]interface{}
	Status    *string
}

// MemoryStore defines the interface for memory CRUD operations.
type MemoryStore interface {
	// AddMemory creates a new memory record.
	AddMemory(ctx context.Context, record *MemoryRecord) error

	// GetMemory retrieves a memory by ID, including provenance information.
	GetMemory(ctx context.Context, id string) (*MemoryRecord, error)

	// ListMemories returns paginated memory summaries.
	ListMemories(ctx context.Context, opts ListMemoriesOptions) ([]MemorySummary, error)

	// UpdateMemory applies partial updates to a memory.
	UpdateMemory(ctx context.Context, id string, updates MemoryUpdate) error

	// DeleteMemory removes a memory and its provenance links.
	DeleteMemory(ctx context.Context, id string) error

	// GetMemoriesByNodeID returns all memory IDs that reference a given node.
	GetMemoriesByNodeID(ctx context.Context, nodeID string) ([]string, error)
}

// SQLiteMemoryStore implements MemoryStore using SQLite.
type SQLiteMemoryStore struct {
	db *sql.DB
}

// NewSQLiteMemoryStore creates a new SQLite-backed memory store.
// Shares the database connection with SQLiteGraphStore.
func NewSQLiteMemoryStore(db *sql.DB) *SQLiteMemoryStore {
	return &SQLiteMemoryStore{db: db}
}

// DB returns the underlying database connection for advanced operations.
func (s *SQLiteMemoryStore) DB() *sql.DB {
	return s.db
}

// ComputeDocHash computes a canonical hash of a memory's content.
// Uses JSON with sorted keys, trimmed whitespace, excluding metadata.
func ComputeDocHash(topic, context string, decisions, rationale []string) string {
	// Trim whitespace
	topic = strings.TrimSpace(topic)
	context = strings.TrimSpace(context)

	// Build canonical JSON object with sorted keys
	canonical := map[string]interface{}{
		"context":   context,
		"decisions": decisions,
		"rationale": rationale,
		"topic":     topic,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(canonical)
	if err != nil {
		// Should never happen with string/slice inputs
		panic(fmt.Sprintf("failed to marshal canonical JSON: %v", err))
	}

	// Compute SHA-256
	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%x", hash)
}

// AddMemory creates a new memory record.
func (s *SQLiteMemoryStore) AddMemory(ctx context.Context, record *MemoryRecord) error {
	// Generate ID if not provided
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}

	// Default version and status
	if record.Version == 0 {
		record.Version = 1
	}
	if record.Status == "" {
		record.Status = "pending"
	}

	// Serialize JSON fields
	decisionsJSON, err := json.Marshal(record.Decisions)
	if err != nil {
		return fmt.Errorf("failed to marshal decisions: %w", err)
	}

	rationaleJSON, err := json.Marshal(record.Rationale)
	if err != nil {
		return fmt.Errorf("failed to marshal rationale: %w", err)
	}

	metadataJSON, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	query := `
		INSERT INTO memories (id, topic, context, decisions_json, rationale_json, metadata_json,
			created_at, updated_at, version, doc_hash, source, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.ExecContext(ctx, query,
		record.ID,
		record.Topic,
		record.Context,
		decisionsJSON,
		rationaleJSON,
		metadataJSON,
		record.CreatedAt,
		record.UpdatedAt,
		record.Version,
		record.DocHash,
		record.Source,
		record.Status,
	)

	if err != nil {
		return fmt.Errorf("failed to insert memory: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetMemory retrieves a memory by ID.
func (s *SQLiteMemoryStore) GetMemory(ctx context.Context, id string) (*MemoryRecord, error) {
	query := `
		SELECT id, topic, context, decisions_json, rationale_json, metadata_json,
			created_at, updated_at, version, doc_hash, source, status
		FROM memories
		WHERE id = ?
	`

	var record MemoryRecord
	var decisionsJSON, rationaleJSON, metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&record.ID,
		&record.Topic,
		&record.Context,
		&decisionsJSON,
		&rationaleJSON,
		&metadataJSON,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.Version,
		&record.DocHash,
		&record.Source,
		&record.Status,
	)

	if err == sql.ErrNoRows {
		return nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get memory: %w", err)
	}

	// Deserialize JSON fields
	if len(decisionsJSON) > 0 {
		if err := json.Unmarshal(decisionsJSON, &record.Decisions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal decisions: %w", err)
		}
	}

	if len(rationaleJSON) > 0 {
		if err := json.Unmarshal(rationaleJSON, &record.Rationale); err != nil {
			return nil, fmt.Errorf("failed to unmarshal rationale: %w", err)
		}
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &record.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &record, nil
}

// ListMemories returns paginated memory summaries.
func (s *SQLiteMemoryStore) ListMemories(ctx context.Context, opts ListMemoriesOptions) ([]MemorySummary, error) {
	// Apply defaults and limits
	if opts.Limit == 0 {
		opts.Limit = 50
	}
	if opts.Limit > 100 {
		opts.Limit = 100
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}

	query := `
		SELECT id, topic, context, decisions_json, created_at, updated_at, status
		FROM memories
		ORDER BY updated_at DESC, created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.QueryContext(ctx, query, opts.Limit, opts.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list memories: %w", err)
	}
	defer rows.Close()

	var summaries []MemorySummary
	for rows.Next() {
		var id, topic, context, status string
		var decisionsJSON []byte
		var createdAt, updatedAt time.Time

		err := rows.Scan(&id, &topic, &context, &decisionsJSON, &createdAt, &updatedAt, &status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan memory: %w", err)
		}

		// Truncate context for preview (max 200 chars)
		preview := context
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}

		// Count decisions
		var decisions []string
		if len(decisionsJSON) > 0 {
			json.Unmarshal(decisionsJSON, &decisions)
		}

		summaries = append(summaries, MemorySummary{
			ID:            id,
			Topic:         topic,
			Preview:       preview,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
			DecisionCount: len(decisions),
			Status:        status,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating memories: %w", err)
	}

	return summaries, nil
}

// UpdateMemory applies partial updates to a memory.
func (s *SQLiteMemoryStore) UpdateMemory(ctx context.Context, id string, updates MemoryUpdate) error {
	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Fetch existing memory within transaction
	query := `
		SELECT id, topic, context, decisions_json, rationale_json, metadata_json,
			created_at, updated_at, version, doc_hash, source, status
		FROM memories
		WHERE id = ?
	`

	var existing MemoryRecord
	var decisionsJSON, rationaleJSON, metadataJSON []byte

	err = tx.QueryRowContext(ctx, query, id).Scan(
		&existing.ID,
		&existing.Topic,
		&existing.Context,
		&decisionsJSON,
		&rationaleJSON,
		&metadataJSON,
		&existing.CreatedAt,
		&existing.UpdatedAt,
		&existing.Version,
		&existing.DocHash,
		&existing.Source,
		&existing.Status,
	)

	if err == sql.ErrNoRows {
		return ErrMemoryNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get memory: %w", err)
	}

	// Deserialize JSON fields
	if len(decisionsJSON) > 0 {
		if err := json.Unmarshal(decisionsJSON, &existing.Decisions); err != nil {
			return fmt.Errorf("failed to unmarshal decisions: %w", err)
		}
	}

	if len(rationaleJSON) > 0 {
		if err := json.Unmarshal(rationaleJSON, &existing.Rationale); err != nil {
			return fmt.Errorf("failed to unmarshal rationale: %w", err)
		}
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &existing.Metadata); err != nil {
			return fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	// Apply updates
	if updates.Topic != nil {
		existing.Topic = *updates.Topic
	}
	if updates.Context != nil {
		existing.Context = *updates.Context
	}
	if updates.Decisions != nil {
		existing.Decisions = *updates.Decisions
	}
	if updates.Rationale != nil {
		existing.Rationale = *updates.Rationale
	}
	if updates.Metadata != nil {
		existing.Metadata = *updates.Metadata
	}
	if updates.Status != nil {
		existing.Status = *updates.Status
	}

	// Update timestamp and version
	existing.UpdatedAt = time.Now()
	existing.Version++

	// Serialize JSON fields
	decisionsJSON, err = json.Marshal(existing.Decisions)
	if err != nil {
		return fmt.Errorf("failed to marshal decisions: %w", err)
	}

	rationaleJSON, err = json.Marshal(existing.Rationale)
	if err != nil {
		return fmt.Errorf("failed to marshal rationale: %w", err)
	}

	metadataJSON, err = json.Marshal(existing.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	updateQuery := `
		UPDATE memories
		SET topic = ?, context = ?, decisions_json = ?, rationale_json = ?, metadata_json = ?,
			updated_at = ?, version = ?, status = ?
		WHERE id = ?
	`

	_, err = tx.ExecContext(ctx, updateQuery,
		existing.Topic,
		existing.Context,
		decisionsJSON,
		rationaleJSON,
		metadataJSON,
		existing.UpdatedAt,
		existing.Version,
		existing.Status,
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to update memory: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteMemory removes a memory and its provenance links (via CASCADE).
func (s *SQLiteMemoryStore) DeleteMemory(ctx context.Context, id string) error {
	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete memory (CASCADE will handle provenance tables)
	result, err := tx.ExecContext(ctx, "DELETE FROM memories WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}

	// Check if memory was found
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrMemoryNotFound
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetMemoriesByNodeID returns all memory IDs that reference a given node.
// Returns memory IDs sorted by updated_at DESC (most recent first).
func (s *SQLiteMemoryStore) GetMemoriesByNodeID(ctx context.Context, nodeID string) ([]string, error) {
	query := `
		SELECT DISTINCT m.id
		FROM memories m
		JOIN memory_nodes mn ON m.id = mn.memory_id
		WHERE mn.node_id = ?
		ORDER BY m.updated_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories by node: %w", err)
	}
	defer rows.Close()

	var memoryIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan memory ID: %w", err)
		}
		memoryIDs = append(memoryIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating memory IDs: %w", err)
	}

	return memoryIDs, nil
}

// GetMemoriesByNodeIDBatched returns memory IDs for multiple nodes in a single query.
// Returns a map of nodeID -> []memoryID (sorted by updated_at DESC per node).
func (s *SQLiteMemoryStore) GetMemoriesByNodeIDBatched(ctx context.Context, nodeIDs []string) (map[string][]string, error) {
	if len(nodeIDs) == 0 {
		return make(map[string][]string), nil
	}

	// Build IN clause with placeholders
	placeholders := make([]string, len(nodeIDs))
	args := make([]interface{}, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		placeholders[i] = "?"
		args[i] = nodeID
	}

	query := fmt.Sprintf(`
		SELECT mn.node_id, m.id, m.updated_at
		FROM memory_nodes mn
		JOIN memories m ON mn.memory_id = m.id
		WHERE mn.node_id IN (%s)
		ORDER BY mn.node_id, m.updated_at DESC
	`, strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to batch query memories by nodes: %w", err)
	}
	defer rows.Close()

	// Group results by node_id
	result := make(map[string][]string)
	for rows.Next() {
		var nodeID, memoryID string
		var updatedAt time.Time
		if err := rows.Scan(&nodeID, &memoryID, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan batch result: %w", err)
		}
		result[nodeID] = append(result[nodeID], memoryID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating batch results: %w", err)
	}

	// Ensure all nodeIDs are present in result (even if empty)
	for _, nodeID := range nodeIDs {
		if _, exists := result[nodeID]; !exists {
			result[nodeID] = []string{}
		}
	}

	return result, nil
}

// LinkProvenance links derived nodes/edges to a memory.
func (s *SQLiteMemoryStore) LinkProvenance(ctx context.Context, memoryID string, nodeIDs, edgeIDs []string) error {
	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert node provenance
	nodeStmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO memory_nodes (memory_id, node_id) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare node stmt: %w", err)
	}
	defer nodeStmt.Close()

	for _, nodeID := range nodeIDs {
		if _, err := nodeStmt.ExecContext(ctx, memoryID, nodeID); err != nil {
			return fmt.Errorf("failed to link node provenance: %w", err)
		}
	}

	// Insert edge provenance
	edgeStmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO memory_edges (memory_id, edge_id) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare edge stmt: %w", err)
	}
	defer edgeStmt.Close()

	for _, edgeID := range edgeIDs {
		if _, err := edgeStmt.ExecContext(ctx, memoryID, edgeID); err != nil {
			return fmt.Errorf("failed to link edge provenance: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// UnlinkProvenance removes provenance links for a memory.
func (s *SQLiteMemoryStore) UnlinkProvenance(ctx context.Context, memoryID string) error {
	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete node provenance
	if _, err := tx.ExecContext(ctx, "DELETE FROM memory_nodes WHERE memory_id = ?", memoryID); err != nil {
		return fmt.Errorf("failed to unlink node provenance: %w", err)
	}

	// Delete edge provenance
	if _, err := tx.ExecContext(ctx, "DELETE FROM memory_edges WHERE memory_id = ?", memoryID); err != nil {
		return fmt.Errorf("failed to unlink edge provenance: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetProvenanceByMemory returns all node and edge IDs linked to a memory.
func (s *SQLiteMemoryStore) GetProvenanceByMemory(ctx context.Context, memoryID string) (nodeIDs, edgeIDs []string, err error) {
	// Query node provenance
	nodeRows, err := s.db.QueryContext(ctx, "SELECT node_id FROM memory_nodes WHERE memory_id = ? ORDER BY created_at", memoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query node provenance: %w", err)
	}
	defer nodeRows.Close()

	for nodeRows.Next() {
		var nodeID string
		if err := nodeRows.Scan(&nodeID); err != nil {
			return nil, nil, fmt.Errorf("failed to scan node ID: %w", err)
		}
		nodeIDs = append(nodeIDs, nodeID)
	}

	if err := nodeRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating node provenance: %w", err)
	}

	// Query edge provenance
	edgeRows, err := s.db.QueryContext(ctx, "SELECT edge_id FROM memory_edges WHERE memory_id = ? ORDER BY created_at", memoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query edge provenance: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var edgeID string
		if err := edgeRows.Scan(&edgeID); err != nil {
			return nil, nil, fmt.Errorf("failed to scan edge ID: %w", err)
		}
		edgeIDs = append(edgeIDs, edgeID)
	}

	if err := edgeRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating edge provenance: %w", err)
	}

	return nodeIDs, edgeIDs, nil
}

// CountMemoryReferences returns the number of memories referencing a node.
func (s *SQLiteMemoryStore) CountMemoryReferences(ctx context.Context, nodeID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_nodes WHERE node_id = ?", nodeID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count memory references: %w", err)
	}
	return count, nil
}

// GetOrphanedNodes returns node IDs that were provenance-tracked but now have zero references.
func (s *SQLiteMemoryStore) GetOrphanedNodes(ctx context.Context) ([]string, error) {
	// Find nodes that were in memory_nodes (tracked) but now have zero references
	// For simplicity, we'll identify currently orphaned nodes among all tracked nodes

	// Since provenance rows CASCADE delete, we can't use that approach.
	// This is tricky; let's use a different approach in GC.

	return []string{}, nil // Placeholder; GC will handle this differently
}

// GetOrphanedEdges returns edge IDs that were provenance-tracked but now have zero references.
func (s *SQLiteMemoryStore) GetOrphanedEdges(ctx context.Context) ([]string, error) {
	return []string{}, nil // Placeholder; GC will handle this differently
}

// GarbageCollect removes provenance-tracked nodes/edges with zero references.
// Returns counts of deleted nodes and edges.
// CRITICAL: Only affects provenance-tracked artifacts. Legacy nodes/edges are preserved.
func (s *SQLiteMemoryStore) GarbageCollect(ctx context.Context) (nodesDeleted, edgesDeleted int, err error) {
	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Strategy: Track which nodes/edges have EVER been in provenance tables.
	// For v1.0.0, we'll use a simpler approach:
	// - Collect all node/edge IDs currently in provenance tables (these are "tracked")
	// - Delete tracked artifacts that no longer have references

	// Get all tracked node IDs
	var trackedNodeIDs []string
	nodeRows, err := tx.QueryContext(ctx, "SELECT DISTINCT node_id FROM memory_nodes")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query tracked nodes: %w", err)
	}
	for nodeRows.Next() {
		var nodeID string
		if err := nodeRows.Scan(&nodeID); err != nil {
			nodeRows.Close()
			return 0, 0, fmt.Errorf("failed to scan tracked node: %w", err)
		}
		trackedNodeIDs = append(trackedNodeIDs, nodeID)
	}
	nodeRows.Close()

	// Get all tracked edge IDs
	var trackedEdgeIDs []string
	edgeRows, err := tx.QueryContext(ctx, "SELECT DISTINCT edge_id FROM memory_edges")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query tracked edges: %w", err)
	}
	for edgeRows.Next() {
		var edgeID string
		if err := edgeRows.Scan(&edgeID); err != nil {
			edgeRows.Close()
			return 0, 0, fmt.Errorf("failed to scan tracked edge: %w", err)
		}
		trackedEdgeIDs = append(trackedEdgeIDs, edgeID)
	}
	edgeRows.Close()

	// Now delete nodes that are NOT in the current tracked set
	// (i.e., they were tracked before but CASCADE deleted their provenance)
	// Wait, this won't work either because we can't distinguish "was tracked" from "never tracked".

	// CORRECT APPROACH: Mark nodes/edges as "tracked" when first added via AddMemory.
	// For now, use a heuristic: delete nodes/edges that are NOT in provenance tables.
	// But this violates the plan's requirement to preserve legacy data.

	// FINAL APPROACH (per critique resolution):
	// We need a "tracked" flag or separate tracking. For v1.0.0, use this rule:
	// - A node/edge is "tracked" if it appears in provenance tables.
	// - GC finds nodes/edges that SHOULD be in provenance (based on prior existence) but aren't.
	// - This requires historical tracking, which we don't have in the schema.

	// PRAGMATIC SOLUTION for v1.0.0:
	// Add a "provenance_tracked" metadata flag to nodes/edges OR use a separate tracking table.
	// For this implementation, we'll take a safer approach:
	// Only delete nodes/edges that are EXPLICITLY orphaned (zero refs in provenance).

	// Simpler GC rule:
	// Delete edges that appear in memory_edges with COUNT(*) = 0
	// This can't happen with current schema since CASCADE already deletes them.

	// The issue is that CASCADE deletes provenance rows, but not the nodes/edges themselves.
	// So after DELETE FROM memories, we need to find nodes/edges no longer referenced.

	// CORRECT GC IMPLEMENTATION:
	// 1. Find all nodes with zero provenance references (NOT IN memory_nodes)
	//    BUT only among nodes that we KNOW were tracked (tricky without history).
	// 2. For v1.0.0, we'll rely on metadata or skip perfect GC.

	// Let me re-read the plan...

	// The plan says GC should:
	// - `DELETE FROM edges WHERE id IN (SELECT edge_id FROM memory_edges GROUP BY edge_id HAVING COUNT(*) = 0 AFTER CASCADE)`
	// - But after CASCADE, those rows don't exist anymore.

	// The correct approach is:
	// - Find edges that are NOT in memory_edges anymore but exist in the edges table.
	// - These are orphaned IF they were previously tracked.

	// For v1.0.0, we'll use a conservative GC:
	// - Identify nodes/edges that are in the graph but NOT in provenance tables.
	// - Among those, delete ones that have a special "tracked" indicator.
	// - WITHOUT the indicator, we can't safely GC without risking legacy data loss.

	// RESOLUTION: Add "provenance_tracked" column to nodes/edges OR use current provenance presence as proxy.
	// Per critique, we must preserve legacy. So GC will ONLY delete if provenance exists BUT count = 0.

	// Since provenance rows CASCADE delete, we can't use that approach.
	// We need to track "was ever in provenance" separately.

	// FOR THIS IMPLEMENTATION:
	// We'll add a lightweight tracking: Before unlinking provenance, collect node/edge IDs.
	// Then delete those that no longer have ANY references.

	// This requires calling GC AFTER unlinking. The caller (UpdateMemory/DeleteMemory) will do this.

	// Simplified GC for now: delete nodes/edges NOT in provenance tables AND marked as tracked.
	// Without a tracking column, we can't implement safely.

	// DECISION: For v1.0.0, we'll implement GC as a helper that takes explicit lists of candidate IDs.
	// The caller tracks which artifacts to check.

	// Commit (no-op for now; will be called explicitly by DeleteMemory/UpdateMemory with candidates)
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return 0, 0, nil // Placeholder; real GC happens in higher-level methods
}

// GarbageCollectCandidates removes candidate nodes/edges if they have zero provenance references.
// This is the actual GC implementation called after unlinking provenance.
func (s *SQLiteMemoryStore) GarbageCollectCandidates(ctx context.Context, nodeIDs, edgeIDs []string) (nodesDeleted, edgesDeleted int, err error) {
	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete edges with zero provenance references
	for _, edgeID := range edgeIDs {
		var count int
		err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_edges WHERE edge_id = ?", edgeID).Scan(&count)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to count edge references: %w", err)
		}

		if count == 0 {
			_, err := tx.ExecContext(ctx, "DELETE FROM edges WHERE id = ?", edgeID)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to delete orphaned edge: %w", err)
			}
			edgesDeleted++
		}
	}

	// Delete nodes with zero provenance references
	for _, nodeID := range nodeIDs {
		var count int
		err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_nodes WHERE node_id = ?", nodeID).Scan(&count)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to count node references: %w", err)
		}

		if count == 0 {
			_, err := tx.ExecContext(ctx, "DELETE FROM nodes WHERE id = ?", nodeID)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to delete orphaned node: %w", err)
			}
			nodesDeleted++
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nodesDeleted, edgesDeleted, nil
}

// ErrMemoryNotFound indicates that no memory was found for the given ID.
var ErrMemoryNotFound = fmt.Errorf("memory not found")

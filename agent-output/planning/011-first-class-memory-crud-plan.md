# Plan 011: First-Class Memory CRUD

**Plan ID**: 011
**Target Release**: v1.0.0
**Epic Alignment**: Epic 8.1 - First-Class Memory CRUD (P0)
**Status**: Committed for Release v1.0.0
**Created**: 2026-01-03
**Completed**: 2026-01-03

## Changelog
| Date | Change |
|------|--------|
| 2026-01-03 | Initial plan creation based on architecture findings (011-memory-crud-architecture-findings.md) |
| 2026-01-03 | Revised per critique 011: clarified migration stance, legacy GC safety, two-phase transaction model, doc_hash canonicalization, batched provenance enrichment |
| 2026-01-03 | Implementation complete with all milestones delivered |
| 2026-01-03 | UAT Complete - all scenarios pass, objective achieved, approved for v1.0.0 release |
| 2026-01-03 | Plan 011 committed locally for release v1.0.0 |

---

## Value Statement and Business Objective

**As a** developer building a memory browser UI (like Glowbabe),
**I want** to store, list, retrieve, update, and delete memories as first-class entities with stable IDs,
**So that** users can manage their AI assistant's memory through a structured interface.

---

## Objective

Introduce a first-class `MemoryRecord` entity that preserves the original structured payload (topic, context, decisions, rationale, metadata) with full CRUD operations. Implement provenance tracking to map memories to derived graph artifacts, enabling safe update and delete with reference-counting semantics.

---

## Assumptions

1. Provenance-first approach: all derived nodes/edges created via `AddMemory` are linked back to their source memory
2. Shared entity deduplication is preserved (deterministic node IDs remain)
3. Delete removes the memory record and orphaned artifacts (nodes/edges with zero remaining references **among provenance-tracked nodes only**)
4. Update triggers re-cognify of the memory, replacing old provenance links atomically
5. Existing `Add/Cognify/Search` API remains functional but is considered legacy for new integrations
6. **Migration stance (breaking change)**: Pre-1.0.0 graph data created via legacy `Add/Cognify` is preserved but NOT retroactively tracked by provenance. Users who want full CRUD support must re-ingest via `AddMemory`. Legacy nodes/edges are exempt from garbage collection (they have no provenance and are therefore never orphaned by the GC rules).
7. Short database transactions are required for CRUD atomicity; long-running LLM calls occur **outside** the transaction boundary (two-phase commit model)

---

## Architecture References

- [011-memory-crud-architecture-findings.md](../architecture/011-memory-crud-architecture-findings.md) - R-1 through R-5
- [system-architecture.md](../architecture/system-architecture.md) - current state + problem areas

---

## Plan

### Milestone 1: Schema Design and Migration

**Objective**: Add `memories`, `memory_nodes`, `memory_edges` tables.

**Tasks**:
1. Design `memories` table schema:
   - `id TEXT PRIMARY KEY` (UUID)
   - `topic TEXT NOT NULL`
   - `context TEXT NOT NULL`
   - `decisions_json TEXT` (JSON array)
   - `rationale_json TEXT` (JSON array)
   - `metadata_json TEXT` (JSON object)
   - `created_at DATETIME DEFAULT CURRENT_TIMESTAMP`
   - `updated_at DATETIME DEFAULT CURRENT_TIMESTAMP`
   - `version INTEGER DEFAULT 1`
   - `doc_hash TEXT NOT NULL` (SHA-256 of canonicalized payload for dedup)
   - `source TEXT` (optional caller-provided identifier)
2. Design `memory_nodes` provenance table:
   - `memory_id TEXT NOT NULL`
   - `node_id TEXT NOT NULL`
   - `created_at DATETIME DEFAULT CURRENT_TIMESTAMP`
   - `PRIMARY KEY (memory_id, node_id)`
   - `FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE`
3. Design `memory_edges` provenance table:
   - `memory_id TEXT NOT NULL`
   - `edge_id TEXT NOT NULL`
   - `created_at DATETIME DEFAULT CURRENT_TIMESTAMP`
   - `PRIMARY KEY (memory_id, edge_id)`
   - `FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE`
4. Implement schema migration in `SQLiteGraphStore.initSchema()` / `migrateSchema()`
5. Add indexes: `idx_memories_topic`, `idx_memories_doc_hash`, `idx_memory_nodes_node_id`, `idx_memory_edges_edge_id`

**Acceptance Criteria**:
- Schema migration adds tables without data loss to existing nodes/edges (legacy data preserved)
- Provenance tables have correct foreign keys and cascading deletes on memory_id
- Indexes support efficient lookups
- Legacy nodes/edges (those without provenance) remain untouched by migration

**Dependencies**: None

---

### Milestone 2: MemoryStore Interface

**Objective**: Define a new interface for memory CRUD operations.

**Tasks**:
1. Create `pkg/store/memory.go` with `MemoryStore` interface:
   - `AddMemory(ctx, record *MemoryRecord) error`
   - `GetMemory(ctx, id string) (*MemoryRecord, error)`
   - `ListMemories(ctx, opts ListMemoriesOptions) ([]MemorySummary, error)`
   - `UpdateMemory(ctx, id string, updates MemoryUpdate) error`
   - `DeleteMemory(ctx, id string) error`
   - `GetMemoriesByNodeID(ctx, nodeID string) ([]string, error)` (for search provenance)
2. Define types:
   - `MemoryRecord`: id, topic, context, decisions, rationale, metadata, timestamps, version, doc_hash, source
   - `MemorySummary`: id, topic, preview (truncated context), created_at, updated_at, decision_count
   - `ListMemoriesOptions`: offset, limit (pagination only, no search)
   - `MemoryUpdate`: topic*, context*, decisions*, rationale*, metadata* (all optional, partial update)
3. Implement `SQLiteMemoryStore` in `pkg/store/sqlite_memory.go`
4. Ensure all operations use explicit transactions (BEGIN/COMMIT/ROLLBACK)

**Acceptance Criteria**:
- Interface defined with clear contracts
- SQLite implementation passes basic CRUD unit tests
- Transactions wrap multi-step operations

**Dependencies**: Milestone 1

---

### Milestone 3: Provenance Tracking

**Objective**: Link derived nodes/edges to their source memory.

**Tasks**:
1. Extend `AddMemory` to accept a callback or return value that collects created node/edge IDs during cognify
2. Create helper `LinkProvenance(ctx, memoryID string, nodeIDs, edgeIDs []string) error`
3. Implement provenance insert in same transaction as memory insert
4. Create `GetProvenanceByMemory(ctx, memoryID string) (nodeIDs, edgeIDs []string, error)`
5. Create `CountMemoryReferences(ctx, nodeID string) (int, error)` for refcount check

**Acceptance Criteria**:
- After AddMemory, provenance tables contain correct mappings
- CountMemoryReferences returns accurate refcounts

**Dependencies**: Milestone 2

---

### Milestone 4: AddMemory API

**Objective**: Implement `Gognee.AddMemory()` that stores + cognifies + links provenance.

**Tasks**:
1. Add `AddMemory(ctx, input MemoryInput) (*MemoryResult, error)` to Gognee
2. `MemoryInput`: topic, context, decisions, rationale, metadata, source
3. `MemoryResult`: memory_id, nodes_created, edges_created, errors
4. Implementation flow (two-phase model):
   - **Phase 1 (short transaction):**
     - Begin transaction
     - Compute doc_hash using canonical serialization (see Milestone 1 note)
     - Check for duplicate (same hash) → return existing memory_id if found
     - Insert memory record with status `pending`
     - Commit transaction
   - **Phase 2 (outside transaction, idempotent):**
     - Format text as `Topic: {topic}\n\n{context}` (backward compatible with existing cognify text format)
     - Run chunking → entity extraction → relation extraction (LLM calls happen here)
     - Collect created node_ids and edge_ids
   - **Phase 3 (short transaction):**
     - Begin transaction
     - Upsert nodes/edges
     - Insert provenance links
     - Update memory status to `complete`, set updated_at
     - Commit transaction
   - On Phase 2/3 failure: memory remains `pending`; caller may retry or delete
5. Return stable memory_id

**doc_hash Canonicalization**:
- Serialize as JSON object with sorted keys: `{"context": ..., "decisions": ..., "rationale": ..., "topic": ...}` (metadata excluded from hash)
- Trim leading/trailing whitespace from topic and context before hashing
- Compute SHA-256 of UTF-8 encoded canonical JSON

**Acceptance Criteria**:
- AddMemory returns stable ID
- Provenance is recorded
- Duplicate detection via doc_hash works
- Transaction rollback on failure

**Dependencies**: Milestone 2, Milestone 3

---

### Milestone 5: ListMemories and GetMemory APIs

**Objective**: Implement memory retrieval for browser UI.

**Tasks**:
1. Add `ListMemories(ctx, opts ListMemoriesOptions) ([]MemorySummary, error)` to Gognee
2. Add `GetMemory(ctx, id string) (*MemoryRecord, error)` to Gognee
3. ListMemories returns paginated summaries (default limit 50, max 100)
4. GetMemory returns full payload including linked node/edge IDs

**Acceptance Criteria**:
- ListMemories supports offset/limit pagination
- GetMemory returns complete record with provenance IDs
- Not-found returns appropriate error

**Dependencies**: Milestone 2

---

### Milestone 6: UpdateMemory API

**Objective**: Implement partial update with re-cognify.

**Tasks**:
1. Add `UpdateMemory(ctx, id string, updates MemoryUpdate) (*MemoryResult, error)` to Gognee
2. Implementation flow (two-phase model):
   - **Phase 1 (short transaction):**
     - Begin transaction
     - Fetch existing memory
     - Apply partial updates (only non-nil fields)
     - Recompute doc_hash
     - If hash unchanged, update metadata/updated_at/version++ and commit (no re-cognify)
     - If hash changed: set status to `pending`, commit
   - **Phase 2 (outside transaction, if hash changed):**
     - Re-run cognify pipeline (LLM calls)
     - Collect new node_ids and edge_ids
   - **Phase 3 (short transaction, if hash changed):**
     - Begin transaction
     - Delete old provenance links for this memory
     - Upsert new nodes/edges
     - Insert new provenance links
     - Run garbage collection (delete orphaned **provenance-tracked** nodes/edges with zero references)
     - Update memory status to `complete`, updated_at, version++
     - Commit transaction
3. Return updated memory_id and stats

**Acceptance Criteria**:
- Partial updates work (only specified fields change)
- Content change triggers re-cognify
- Old orphaned artifacts are garbage collected
- Transaction rollback on failure

**Dependencies**: Milestone 4, Milestone 7

---

### Milestone 7: DeleteMemory API and Garbage Collection

**Objective**: Implement delete with provenance-aware garbage collection.

**Tasks**:
1. Add `DeleteMemory(ctx, id string) error` to Gognee
2. Implementation flow:
   - Begin transaction
   - Delete memory record (CASCADE deletes memory_nodes, memory_edges)
   - Run garbage collection (**provenance-aware, preserves legacy data**):
     - Identify provenance-tracked nodes: `SELECT DISTINCT node_id FROM memory_nodes`
     - Delete orphaned edges: `DELETE FROM edges WHERE id IN (SELECT edge_id FROM memory_edges GROUP BY edge_id HAVING COUNT(*) = 0 AFTER CASCADE)` — i.e., edges that **were** tracked but now have zero references
     - Delete orphaned nodes: nodes that **were** in memory_nodes (ever) but now have zero references
     - **Legacy nodes/edges (never in provenance tables) are NOT deleted**
   - Commit transaction
3. Vector store cleanup: embeddings are in nodes.embedding column, so deleting nodes handles this automatically
4. Add `GarbageCollect(ctx) (nodesDeleted, edgesDeleted int, error)` as public utility (for manual cleanup)

**GC Safety Rule**: Only nodes/edges that have **at least one provenance record** (now or historically) are candidates for GC. Nodes/edges created via legacy `Add/Cognify` (which have no provenance) are never deleted by GC.

**Acceptance Criteria**:
- DeleteMemory removes memory and orphaned artifacts
- Shared artifacts (still referenced by other memories) are preserved
- Vector embeddings are cleaned up with node deletion

**Dependencies**: Milestone 3

---

### Milestone 8: Search Provenance Enrichment

**Objective**: Search results surface memory_ids for browser linking.

**Tasks**:
1. Extend `SearchResult` with `MemoryIDs []string` field
2. After search, enrich results using **batched query** for all returned node_ids in a single `SELECT ... WHERE node_id IN (...)` query
3. Sort memory_ids by memory.updated_at DESC (most recent first)
4. Keep enrichment optional via `SearchOptions.IncludeMemoryIDs bool` (default true for new API)
5. For nodes without provenance (legacy), `MemoryIDs` is empty slice (not null)

**Acceptance Criteria**:
- Search results include memory_ids for each node (empty for legacy nodes)
- Provenance lookup uses single batched query (no N+1)
- Glowbabe can link results to memory browser

**Dependencies**: Milestone 3

---

### Milestone 9: Backward Compatibility

**Objective**: Existing Add/Cognify/Search API continues to work.

**Tasks**:
1. Existing `Add()` + `Cognify()` flow does NOT create memory records (legacy mode)
2. Document that legacy mode does not support memory CRUD
3. Add migration guidance: "To enable memory CRUD, use AddMemory instead of Add+Cognify"
4. SearchResult.MemoryIDs returns empty for nodes without provenance (legacy nodes)

**Acceptance Criteria**:
- Existing tests pass without modification
- Legacy workflow documented as deprecated but functional

**Dependencies**: Milestone 8

---

### Milestone 10: Unit Tests

**Objective**: Comprehensive test coverage for memory CRUD.

**Tasks**:
1. Test MemoryStore CRUD operations
2. Test provenance linking and reference counting
3. Test garbage collection (orphan detection)
4. Test AddMemory with duplicate detection
5. Test UpdateMemory with re-cognify
6. Test DeleteMemory preserves shared nodes
7. Test search provenance enrichment
8. Test transaction rollback on errors

**Acceptance Criteria**:
- Coverage ≥80% for new code
- All edge cases covered

**Dependencies**: Milestones 4-8

---

### Milestone 11: Integration Tests

**Objective**: End-to-end validation with real LLM.

**Tasks**:
1. Test: AddMemory → ListMemories → GetMemory roundtrip
2. Test: AddMemory → Search → verify MemoryIDs in results
3. Test: Add two memories with shared entity → Delete one → verify shared node preserved
4. Test: UpdateMemory with content change → verify re-cognify and orphan cleanup

**Acceptance Criteria**:
- Integration tests pass with real OpenAI API (build-tagged)
- Provenance survives full pipeline

**Dependencies**: Milestone 10

---

### Milestone 12: Documentation

**Objective**: Document memory CRUD APIs.

**Tasks**:
1. Add "Memory Management" section to README
2. Document MemoryInput, MemoryRecord, MemoryUpdate types
3. Document AddMemory, ListMemories, GetMemory, UpdateMemory, DeleteMemory APIs
4. Add migration guide from legacy Add/Cognify to AddMemory
5. Document provenance model and garbage collection behavior

**Acceptance Criteria**:
- All new APIs documented with examples
- Migration path clear

**Dependencies**: Milestone 11

---

### Milestone 13: Version Management

**Objective**: Update version artifacts to v1.0.0.

**Tasks**:
1. Add v1.0.0 entry to CHANGELOG.md documenting:
   - First-class memory CRUD
   - Provenance tracking
   - Garbage collection
   - Search memory enrichment
   - Breaking change notice (legacy data incompatible)
2. Update any version references
3. Commit all changes

**Acceptance Criteria**:
- CHANGELOG documents all features
- Version is v1.0.0

**Dependencies**: All previous milestones

---

## Testing Strategy

**Unit Tests**:
- MemoryStore CRUD
- Provenance mapping
- Reference counting
- Garbage collection
- Transaction rollback
- Duplicate detection
- Partial updates

**Integration Tests**:
- Full AddMemory → Search roundtrip
- Shared entity preservation on delete
- Re-cognify on update

**Coverage Target**: ≥80% for new code

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Transaction complexity | Data corruption on partial failure | Two-phase model with idempotent retry; explicit BEGIN/COMMIT/ROLLBACK; test rollback paths |
| Garbage collection deletes legacy data | Data loss for pre-1.0.0 users | GC only affects provenance-tracked nodes; legacy nodes exempt; integration tests verify legacy preservation |
| Performance of provenance joins | Search latency | Index on memory_nodes.node_id; batched enrichment query |
| Breaking change confusion | User frustration | Clear migration docs; version bump to 1.0.0 signals breaking; legacy API preserved |
| Large memory payloads | Storage bloat | Accept for v1; future: compression or external storage |
| Long-running LLM calls block DB | Timeouts, lock contention | Two-phase model: LLM calls outside transaction boundary |

---

## Open Questions

1. **OPEN QUESTION [RESOLVED]**: Should ListMemories support full-text search?
   - **Resolution**: No. Pagination only for v1.0.0. Full-text search deferred.

2. **OPEN QUESTION [RESOLVED]**: Partial update or full replace?
   - **Resolution**: Partial update (patch). Backend re-cognifies on content change regardless.

3. **OPEN QUESTION [RESOLVED]**: Should GC delete legacy (pre-provenance) nodes/edges?
   - **Resolution**: No. GC only affects nodes/edges that have at least one provenance record. Legacy data is preserved.

4. **OPEN QUESTION [RESOLVED]**: Should long-running cognify hold a DB transaction?
   - **Resolution**: No. Use two-phase model: short transaction for memory insert, LLM calls outside transaction, short transaction for node/edge upserts and provenance.

5. **OPEN QUESTION [RESOLVED]**: How is doc_hash canonicalized?
   - **Resolution**: JSON object with sorted keys (context, decisions, rationale, topic), whitespace-trimmed, SHA-256 of UTF-8 JSON. Metadata excluded.

6. **OPEN QUESTION [RESOLVED]**: Should provenance enrichment be batched?
   - **Resolution**: Yes. Single `SELECT ... WHERE node_id IN (...)` query to avoid N+1.

7. **OPEN QUESTION [DEFERRED TO UAT]**: Delete semantics for shared entity descriptions.
   - When Memory A and Memory B both contribute to entity description, deleting A leaves the aggregated description unchanged. Is this acceptable?
   - **Current stance**: Accept for v1.0.0. Perfect contribution subtraction is future work.

---

## Handoff Notes

- **Migration stance**: Pre-1.0.0 graph data (legacy) is preserved but not retroactively provenance-tracked. To get full CRUD support, users should re-ingest via `AddMemory`. Legacy nodes/edges are exempt from garbage collection.
- Two-phase transaction model: DB transactions are short; LLM calls happen outside transaction boundaries with idempotent retry semantics.
- Provenance tables use foreign key CASCADE on memory_id; GC is explicit and only affects provenance-tracked artifacts.
- Existing `Add/Cognify` workflow is preserved but deprecated for new integrations.
- doc_hash uses canonical JSON (sorted keys, trimmed whitespace, metadata excluded).
- Search provenance enrichment uses batched query to avoid N+1.
- Future enhancement: Option C (memory-centric search) could replace node-centric search for stricter delete semantics.

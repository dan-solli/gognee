# Plan 007: Persistent Vector Store

**Plan ID**: 007
**Target Release**: v0.7.0
**Epic Alignment**: Epic 7.1 - Persistent Vector Store (P1)
**Status**: UAT Approved
**Created**: 2025-12-24

## Changelog
| Date | Change |
|------|--------|
| 2025-12-24 | Initial plan creation |
| 2025-12-24 | Revised per critic feedback (direct-query, DB lifecycle, dimension validation) |
| 2025-12-25 | Marked ready for review and implementation |
| 2025-12-25 | Implementation complete - all milestones delivered, tests passing |
| 2025-12-25 | UAT approved - implementation delivers stated value, ready for v0.7.0 release |

---

## Value Statement and Business Objective

**As a** developer deploying gognee in production,
**I want** vector embeddings to persist across application restarts,
**So that** I don't need to re-run Cognify() every time my application starts.

---

## Objective

Implement a SQLite-backed vector store that persists embeddings alongside the graph data, eliminating the need to re-Cognify documents after application restart. This completes the persistence story (nodes and edges already persist; embeddings currently don't).

---

## Assumptions

1. The existing SQLite database connection from `SQLiteGraphStore` can be reused for vector storage
2. Embedding dimensions are consistent within a single deployment (typically 1536 for OpenAI text-embedding-3-small)
3. A direct-query linear scan is acceptable for MVP; ANN indexing is a future optimization
4. The `VectorStore` interface does not need to change
5. Migration from in-memory to SQLite vector store should be seamless for existing users

**OPEN QUESTION [RESOLVED]**: Should embeddings be stored in the existing `nodes` table (already has `embedding BLOB` column) or a separate `embeddings` table?
**Resolution**: Use the existing `nodes.embedding` column. The schema already supports this; the issue was that `MemoryVectorStore` is a separate in-memory structure that doesn't sync with SQLite. The SQLite implementation can read/write embeddings from nodes table directly.

---

## Plan

### Milestone 1: SQLite Vector Store Implementation

**Objective**: Create `SQLiteVectorStore` that implements `VectorStore` interface using the nodes table's embedding column.

**Tasks**:
1. Create `pkg/store/sqlite_vector.go` with `SQLiteVectorStore` struct
2. Implement constructor that accepts shared `*sql.DB` connection from `SQLiteGraphStore`
3. Implement `Add()` method - UPDATE nodes SET embedding WHERE id = ?
4. Implement `Search()` method (direct-query) - SELECT all non-NULL embeddings, compute cosine similarity in Go, return top-K
5. Implement `Delete()` method - UPDATE nodes SET embedding = NULL WHERE id = ?
6. Add embedding dimension validation behavior (reject on Add, skip/ignore mismatches on Search)
7. Add documentation explaining linear scan performance characteristics

**Acceptance Criteria**:
- SQLiteVectorStore implements VectorStore interface
- Embeddings persist in SQLite nodes.embedding column
- Search returns correct top-K results by similarity
- Delete removes embedding without deleting node
- SQLiteVectorStore does not cache embeddings in memory (source of truth is SQLite)

**Dependencies**: None (new file)

---

### Milestone 2: Graph Store DB Accessor

**Objective**: Expose the database connection from SQLiteGraphStore so SQLiteVectorStore can share it.

**Tasks**:
1. Add `DB() *sql.DB` method to `SQLiteGraphStore`
2. Update GraphStore interface if needed (or keep as implementation detail)
3. Update existing tests to verify DB() returns valid connection

**Acceptance Criteria**:
- SQLiteGraphStore.DB() returns the underlying *sql.DB
- Connection can be shared with SQLiteVectorStore
- Connection lifecycle is owned by SQLiteGraphStore; SQLiteVectorStore must not close the DB

**Dependencies**: Milestone 1 (defines need)

---

### Milestone 3: Gognee Integration

**Objective**: Wire SQLiteVectorStore into the main Gognee struct when persistent storage is configured.

**Tasks**:
1. Modify `gognee.New()` to create `SQLiteVectorStore` when `DBPath` is not `:memory:`
2. Pass shared DB connection from GraphStore to VectorStore
3. Keep `MemoryVectorStore` as fallback for `:memory:` mode (in-memory SQLite + in-memory vector)
4. Update Close() semantics: GraphStore closes DB connection; SQLiteVectorStore Close() is a no-op

**Acceptance Criteria**:
- Persistent DBPath uses SQLiteVectorStore
- In-memory DBPath uses MemoryVectorStore  
- Restart test: Add + Cognify + Close + Reopen → Search returns results

**Dependencies**: Milestone 1, Milestone 2

---

### Milestone 4: Restart Semantics Validation

**Objective**: Validate that embeddings stored in SQLite are immediately searchable after restart.

**Tasks**:
1. Confirm SQLiteVectorStore.Search() queries SQLite as the source of truth (no cache warm-up required)
2. Ensure nodes with NULL embeddings are skipped
3. Ensure dimension mismatches are handled deterministically (documented behavior)

**Acceptance Criteria**:
- Opening an existing database makes embeddings immediately searchable
- No Cognify() required after restart for previously processed data
- Nodes without embeddings are gracefully skipped in search

**Dependencies**: Milestone 1

---

### Milestone 5: Unit Tests

**Objective**: Comprehensive offline tests for SQLiteVectorStore.

**Tasks**:
1. Create `pkg/store/sqlite_vector_test.go`
2. Test Add/Search/Delete basic operations
3. Test empty store behavior
4. Test persistence: add, close, reopen, verify searchable
5. Test large embedding dimension handling
6. Test concurrent access (multiple goroutines)

**Acceptance Criteria**:
- All tests pass offline (no network access)
- Coverage ≥80% for sqlite_vector.go
- Tests use temporary file or :memory: SQLite

**Dependencies**: Milestone 1

---

### Milestone 6: Integration Tests

**Objective**: End-to-end tests validating persistence across restarts.

**Tasks**:
1. Add integration test to `gognee_integration_test.go`
2. Test scenario: Add → Cognify → Close → New (same DBPath) → Search → verify results
3. Gate behind `//go:build integration` tag

**Acceptance Criteria**:
- Integration test validates embeddings survive restart
- Test uses real OpenAI API (gated appropriately)

**Dependencies**: Milestone 3, Milestone 5

---

### Milestone 7: Documentation Updates

**Objective**: Update README and API docs to reflect persistent vector storage.

**Tasks**:
1. Update README.md to document persistence behavior
2. Remove "MVP limitation" note about in-memory vector store
3. Add migration note for users upgrading from v0.6.0
4. Update ROADMAP.md MVP limitations section

**Acceptance Criteria**:
- README accurately describes persistence behavior
- No outdated "embedding lost on restart" warnings

**Dependencies**: Milestone 6

---

### Milestone 8: Version Management

**Objective**: Update version artifacts to v0.7.0.

**Tasks**:
1. Update CHANGELOG.md with v0.7.0 entry
2. Update any version references in documentation
3. Commit all changes

**Acceptance Criteria**:
- CHANGELOG reflects persistent vector store feature
- Version is v0.7.0
- Ready for release

**Dependencies**: All previous milestones

---

## Testing Strategy

**Unit Tests**:
- SQLiteVectorStore CRUD operations
- Cosine similarity search correctness
- Persistence across open/close cycles
- Concurrent access safety
- Edge cases (empty store, null embeddings, dimension mismatch)

**Integration Tests**:
- Full pipeline with persistence: Add → Cognify → restart → Search
- Real OpenAI API for embedding generation

**Coverage Target**: ≥80% for new code

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Linear scan too slow for large graphs | Performance degradation | Document limitation; plan ANN indexing as future enhancement |
| Shared DB connection complexity | Deadlocks, connection issues | Use same connection pattern as existing GraphStore |
| Migration breaks existing users | User frustration | Existing users with :memory: continue working unchanged |

---

## Handoff Notes

- Critic should verify shared DB connection approach is sound
- Consider whether MemoryVectorStore should be deprecated or kept for testing
- Linear scan is acceptable for MVP but should document scaling limits (~10K vectors reasonable)


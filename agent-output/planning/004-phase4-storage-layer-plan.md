# Plan 004 — Phase 4 Storage Layer

**Plan ID:** 004

**Target Release:** v0.4.0

**Epic Alignment:** ROADMAP Phase 4 — Storage Layer (SQLite Graph + Vector)

**Status:** UAT Approved

**Changelog**
- 2025-12-24: Created plan for Phase 4 implementation.
- 2025-12-24: Implementation completed - all milestones delivered with 86.2% test coverage.
- 2025-12-24: QA complete - `go test ./...`, `go test -race ./pkg/store/...`, store coverage 86.2%.
- 2025-12-24: UAT approved - all value delivery scenarios pass; ready for v0.4.0 release.

---

## Value Statement and Business Objective
As a developer embedding gognee into Glowbabe (Flowbaby-like assistant), I want gognee to persist extracted entities and relationships in a SQLite-backed graph store and provide vector similarity search, so that knowledge survives restarts and can be queried by meaning (not just exact match).

---

## Objective
Deliver Phase 4 from ROADMAP:
- Design SQLite schema for nodes and edges
- Implement graph storage with node/edge CRUD
- Implement in-memory vector store with cosine similarity search
- Write integration tests

This phase should remain library-only (no CLI) and stay aligned with existing patterns: interface-driven design, offline-first unit tests, minimal dependencies.

---

## Scope

**In scope**
1. Implement `pkg/store/graph.go`:
   - `Node` and `Edge` structs
   - `GraphStore` interface
   - SQLite implementation of `GraphStore`
2. Implement `pkg/store/vector.go`:
   - `VectorStore` interface
   - `SearchResult` struct
   - In-memory implementation (`MemoryVectorStore`)
   - `CosineSimilarity` function
3. SQLite schema creation (nodes, edges, indexes)
4. Offline unit tests for both graph and vector stores
5. Integration tests for SQLite persistence

**Out of scope**
- Hybrid search algorithm (Phase 5)
- High-level `Gognee.Add/Cognify/Search` orchestration (Phase 6)
- SQLite-backed vector persistence (documented as MVP limitation; Future Enhancement)
- Any CLI surface (explicitly out)

---

## Key Constraints
- Library-only: no `cmd/` directory, no executable concerns
- No Python
- SQLite is the only external dependency allowed (use `modernc.org/sqlite` for pure Go, or `mattn/go-sqlite3` with CGO)
- Unit tests must work offline without external services
- In-memory vector store is acceptable for MVP (persistence limitation documented)
- Interface-driven design for swappable implementations

---

## Plan-Level Decisions (to remove ambiguity)

1. **SQLite driver choice:**
   - Use `modernc.org/sqlite` (pure Go) as the default recommendation for easier cross-compilation.
   - CGO-based drivers are acceptable if the implementer prefers; document which driver is used.
   - Rationale: aligns with ROADMAP's "CGO is allowed" policy while preferring pure Go for simplicity.

2. **Node ID generation:**
   - Use UUIDs (e.g., `github.com/google/uuid`) for node and edge IDs.
   - Rationale: avoids collision, aligns with ROADMAP dependency list.

3. **Embedding storage in SQLite:**
   - Store embeddings as BLOB (binary) in the nodes table.
   - Serialize `[]float32` to bytes using `encoding/binary` or similar.
   - Rationale: keeps schema simple; full vector search uses in-memory store for MVP.

4. **Vector store persistence:**
   - MVP uses in-memory vector store only.
   - Document limitation: embeddings must be re-added after restart (or re-run `Cognify()`).
   - Rationale: matches ROADMAP MVP scope; SQLite-backed vector store is a Future Enhancement.

5. **Cognee-aligned edge/neighbor semantics:**
   - `GetEdges(ctx, nodeID)` returns all *incident* edges for the node (both incoming and outgoing); i.e., direction-agnostic discovery.
   - `GetNeighbors(ctx, nodeID, depth)` treats adjacency as direction-agnostic for discovery.
   - Default behavior should preserve Cognee-like expectations: depth=1 returns direct neighbors only.
   - Depth > 1 is allowed as a gognee extension but should be explicitly requested by callers (non-default).
   - Rationale: Cognee adapters implement single-hop, undirected neighbor/edge discovery (`MATCH (n)-[r]-(m)`).

6. **Concurrency for in-memory vector store:**
   - Use `sync.RWMutex` for thread-safe access.
   - Rationale: matches ROADMAP spec for `MemoryVectorStore`.

7. **Error handling for missing nodes/edges:**
   - `GetNode` returns `nil, nil` if node not found (no error for "not found").
   - `GetEdges` returns empty slice if no edges found.
   - Rationale: follows Go idioms for optional lookups; callers check for nil.

---

## Open Questions — Resolved

1. **Name matching case sensitivity:** ✅ RESOLVED — Use case-insensitive matching; case-sensitive matching is not semantic enough for typical entity use.

2. **AddNode upsert vs error:** ✅ RESOLVED — Use upsert behavior (INSERT OR REPLACE by ID) to simplify re-processing and avoid duplicate errors.

---

## Plan (Milestones)

### Milestone 1 — Graph Store Interface + Structs
**Objective:** Define the graph storage API surface in `pkg/store`.

**Tasks**
1. Create `pkg/store/graph.go` with:
   - `Node` struct (ID, Name, Type, Description, Embedding, CreatedAt, Metadata)
   - `Edge` struct (ID, SourceID, Relation, TargetID, Weight, CreatedAt)
   - `GraphStore` interface with methods:
     - Nodes: `AddNode`, `GetNode`, `FindNodesByName`
     - Edges: `AddEdge`, `GetEdges`, `GetNeighbors`
2. Define constructor pattern `NewSQLiteGraphStore(dbPath string) (*SQLiteGraphStore, error)`
3. Include a short note in docstrings: `FindNodesByName` is case-insensitive and can return multiple matches; callers must handle ambiguity.

**Acceptance criteria**
- Interface and structs compile without additional packages beyond stdlib + SQLite driver + UUID.
- API is Cognee-aligned in semantics (multi-match name lookup; undirected neighbor discovery).
- Any divergence from ROADMAP’s single-return `FindNodeByName` is explicitly documented as intentional for Cognee parity.

---

### Milestone 2 — SQLite Schema + Implementation
**Objective:** Implement SQLite-backed graph storage.

**Tasks**
1. Create `pkg/store/sqlite.go` with `SQLiteGraphStore` struct.
2. Implement schema creation (CREATE TABLE IF NOT EXISTS for nodes, edges, indexes).
3. Implement `AddNode`:
   - Serialize embedding to BLOB
   - Serialize metadata to JSON
   - Use INSERT OR REPLACE for upsert behavior
4. Implement `GetNode`:
   - Deserialize embedding and metadata
   - Return nil, nil if not found
5. Implement `FindNodesByName`:
   - Case-insensitive search (use LOWER() or COLLATE NOCASE)
   - Return all matches ordered deterministically (e.g., by `created_at`, then `id`) for stable results
6. Implement `AddEdge`:
   - Generate UUID for edge ID if not provided
   - Use INSERT OR REPLACE for upsert
7. Implement `GetEdges`:
   - Return all edges where `source_id = nodeID OR target_id = nodeID` (incident edges; direction-agnostic)
8. Implement `GetNeighbors`:
   - Depth=1 returns direct neighbors only (direction-agnostic)
   - If depth > 1, traverse direction-agnostically and return unique nodes discovered
9. Implement `Close()` method for cleanup.

**Acceptance criteria**
- All `GraphStore` interface methods implemented.
- Schema created on first open.
- Data persists across `Close()` and reopen.

---

### Milestone 3 — Graph Store Unit Tests
**Objective:** Lock in graph storage behavior with offline tests.

**Tasks**
1. Create `pkg/store/sqlite_test.go`.
2. Use in-memory SQLite (`:memory:` or temp file) for tests.
3. Cover:
   - AddNode + GetNode round-trip
   - FindNodesByName (case-insensitive; returns all matches)
   - AddEdge + GetEdges
   - GetNeighbors with depth 1, 2, 3
   - Upsert behavior (update existing node)
   - Empty results (no error, empty slice/nil)

**Acceptance criteria**
- `go test ./pkg/store/...` passes offline.
- Coverage > 80% for sqlite.go.

---

### Milestone 4 — Vector Store Interface + In-Memory Implementation
**Objective:** Provide vector similarity search capability.

**Tasks**
1. Create `pkg/store/vector.go` with:
   - `VectorStore` interface: `Add`, `Search`, `Delete`
   - `SearchResult` struct: `ID`, `Score`
   - `CosineSimilarity(a, b []float32) float64` function
2. Create `pkg/store/memory_vector.go` with:
   - `MemoryVectorStore` struct with `vectors map[string][]float32` and `sync.RWMutex`
   - `NewMemoryVectorStore() *MemoryVectorStore` constructor
   - Implement `Add`, `Search`, `Delete`
3. `Search` should:
   - Compute cosine similarity against all stored vectors
   - Return top-K results sorted by score descending

**Acceptance criteria**
- Interface and implementation compile.
- Thread-safe via RWMutex.
- Search returns correct top-K by similarity.

---

### Milestone 5 — Vector Store Unit Tests
**Objective:** Validate vector search correctness.

**Tasks**
1. Create `pkg/store/memory_vector_test.go`.
2. Cover:
   - Add + Search round-trip
   - CosineSimilarity correctness (known vectors)
   - Top-K ordering
   - Delete removes from search results
   - Empty store returns empty results
   - Concurrent access (basic race detection)

**Acceptance criteria**
- `go test ./pkg/store/...` passes.
- `go test -race ./pkg/store/...` passes.
- Coverage > 80% for memory_vector.go.

---

### Milestone 6 — Integration + Persistence Tests
**Objective:** Validate end-to-end persistence.

**Tasks**
1. Add integration test that:
   - Creates SQLite store with real file
   - Adds nodes and edges
   - Closes store
   - Reopens store
   - Verifies data persisted
2. Clean up temp files after tests.

**Acceptance criteria**
- Integration tests pass.
- No temp file leaks.

---

### Milestone 7 — Version and Release Artifacts
**Objective:** Update release artifacts for v0.4.0.

**Tasks**
1. Add CHANGELOG.md entry for `v0.4.0` documenting Phase 4 deliverables.
2. Update go.mod with new dependencies (SQLite driver, UUID).
3. Ensure ROADMAP Phase 4 goals are checked off after implementation.

**Acceptance criteria**
- CHANGELOG clearly documents storage layer capabilities.
- Dependencies added cleanly.

---

## Testing Strategy

**Unit Tests** (offline, no external services):
- Graph store: CRUD operations, traversal, upsert
- Vector store: similarity search, top-K, thread safety

**Integration Tests**:
- SQLite persistence across open/close cycles

**Race Detection**:
- Vector store must pass `go test -race`

**Coverage Target**: >80% for new packages

---

## Validation
- `go test ./...`
- `go test -race ./pkg/store/...`
- `go test ./... -cover` (expect strong coverage for new package code)
- `go vet ./...`

---

## Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| SQLite driver compatibility issues | Medium | Test with both `modernc.org/sqlite` and `mattn/go-sqlite3`; document chosen driver |
| In-memory vector store memory usage | Low | Acceptable for MVP; document limitation |
| Graph traversal performance at depth | Low | Default max depth of 3; can optimize later |
| Embedding serialization bugs | Medium | Thorough round-trip tests |

---

## Handoff Notes
- Phase 4 builds on Phase 2 (`Entity`) and Phase 3 (`Triplet`) outputs.
- Graph store will hold `Entity` → `Node` mappings and `Triplet` → `Edge` mappings.
- Phase 5 (Search) will use both `GraphStore` and `VectorStore` for hybrid search.
- Phase 6 will orchestrate the full pipeline: `Add()` → chunk → embed → extract → store.

---

## Dependencies

**New Go dependencies required:**
- `modernc.org/sqlite` (or `github.com/mattn/go-sqlite3`) — SQLite driver
- `github.com/google/uuid` — UUID generation

**No Python. No external services.**

---

## API Clarifications (Post-review)

- `GraphStore` interface includes both `FindNodesByName` (multi-match, case-insensitive) and a convenience `FindNodeByName` (returns a single node if and only if exactly one match is found; otherwise errors on ambiguity or not found).
- `Edge.Weight` defaults to 1.0 and is reserved for future search ranking (Phase 5); it is not used in Phase 4 logic or tests.

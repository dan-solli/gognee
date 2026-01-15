# Plan 018 — Vector Search Optimization (sqlite-vec)

**Plan ID:** 018  
**Target Release:** gognee v1.2.0  
**Epic Alignment:** Epic 7.6 (Performance Optimization) — ANN indexing for vector search  
**Status:** Released v1.2.0  
**Created:** 2026-01-15  

---

## Changelog

| Date | Change | Author |
|------|--------|--------|
| 2026-01-15 | Initial plan drafted per performance incident (17s search latency) | Planner |
| 2026-01-15 | Revised per critique: CGO-only, manual DB recreation, removed fallback/migration contradictions | Planner |
| 2026-01-15 | M1.2 & M1.3 implementation complete, QA verified with 74.7% coverage | QA |
| 2026-01-15 | M2 benchmarks (94µs/op) and M3 release artifacts complete, plan approved | QA |
| 2026-01-15 | UAT Complete - Value validated, 0.111ms performance (4,500x target), approved for release | UAT |

---

## Value Statement and Business Objective

> As a Glowbabe user performing memory searches,  
> I want vector search to complete in <500ms regardless of graph size,  
> So that memory retrieval feels instant and doesn't block my workflow.

---

## Problem Statement

**Incident**: Memory search in the Ottra project (Glowbabe workspace) takes **17 seconds** for a simple query against ~60 memories / ~600 nodes.

**Root Cause**: The current `SQLiteVectorStore.Search()` implementation in [pkg/store/sqlite_vector.go#L78-L84](pkg/store/sqlite_vector.go#L78) performs a **full table scan**:

```sql
SELECT id, embedding FROM nodes WHERE embedding IS NOT NULL
```

Then computes cosine similarity for **ALL vectors** in Go memory. This is O(n) where n = number of nodes with embeddings.

**Impact at scale**:
- 600 nodes × 1536 dimensions = ~921K float32 comparisons per search
- Linear degradation: 1K nodes → slower, 10K nodes → unusable
- README.md claims "acceptable for <10K nodes" but 17s at 600 nodes proves otherwise

**Approach**: Replace linear scan with sqlite-vec's `vec0` virtual table for approximate nearest neighbor (ANN) search using efficient vector indexing.

---

## Scope

**In scope (this plan):**
- Replace linear vector search with sqlite-vec indexed search
- Migrate SQLite driver from `modernc.org/sqlite` to `mattn/go-sqlite3` (CGO) to use sqlite-vec bindings
- vec0 virtual table for new databases
- Rudimentary benchmark for future regression detection

**Out of scope (this plan):**
- Automatic migration of existing databases (users must delete and recreate DB, then re-Cognify)
- Pure-Go fallback (project commits fully to CGO)
- HNSW pure-Go implementation
- Multi-tenancy / workspace isolation
- Embedding quantization (int8/binary)

---

## Architectural Constraints

| Constraint | Rationale |
|------------|-----------|
| CGO required | sqlite-vec bindings require `mattn/go-sqlite3` (CGO); no pure-Go fallback |
| Cross-compilation impact | CGO complicates `GOOS`/`GOARCH` builds; document in README |
| Breaking change for existing DBs | Existing databases must be deleted and recreated; document in CHANGELOG |
| Architectural divergence | Driver change from `modernc.org/sqlite` to `mattn/go-sqlite3` diverges from current architecture doc; note in release, update architecture post-MVP |

---

## Technical Analysis

### sqlite-vec Capabilities

sqlite-vec provides:
- `vec0` virtual table type for storing/indexing vector embeddings
- `MATCH` operator for KNN queries
- Distance metrics: L2 (Euclidean), cosine, inner product
- Efficient ANN indexing (flat or IVF depending on size)
- Supports float32 and int8 vectors

**Example query:**
```sql
SELECT rowid, distance
FROM vec_nodes
WHERE embedding MATCH :query_vector
ORDER BY distance
LIMIT 5;
```

### Driver Change Required

| Current | Target |
|---------|--------|
| `modernc.org/sqlite` (pure Go) | `github.com/mattn/go-sqlite3` (CGO) + `github.com/asg017/sqlite-vec-go-bindings/cgo` |

**Rationale**: sqlite-vec Go bindings only work with:
1. `mattn/go-sqlite3` (CGO) — recommended
2. `ncruces/go-sqlite3` (WASM) — slower, more complex

The `modernc.org/sqlite` vtab API could theoretically implement a custom vector index, but that would be reinventing sqlite-vec with significant effort.

### Upgrade Path

1. **New databases**: Use vec0 table from the start
2. **Existing databases**: Users must delete existing database file and re-run Cognify to rebuild with new schema. No automated migration.

---

## Key Constraints

- CGO build required; no pure-Go fallback
- Existing databases incompatible; users must delete and recreate
- Benchmark confirms <500ms for 10K nodes

---

## Success Criteria

| Metric | Current | Target |
|--------|---------|--------|
| Search latency (600 nodes) | 17,000 ms | <500 ms |
| Search latency (10K nodes) | N/A (estimated ~300s) | <500 ms |
| Search complexity | O(n) | O(log n) or O(1) |
| Build mode | Pure Go (modernc) | CGO only (mattn + sqlite-vec) |

---

## Plan (Milestones)

### Milestone 1: Core Implementation

**Objective**: Replace SQLite driver with CGO-based driver, implement vec0 schema, and indexed search.

**Tasks:**

**1.1 Driver Migration:**
1. Add `github.com/mattn/go-sqlite3` and `github.com/asg017/sqlite-vec-go-bindings/cgo` to go.mod
2. Remove `modernc.org/sqlite` from go.mod
3. Update `pkg/store/sqlite_graph.go` to use mattn driver
4. Update connection string handling for mattn driver
5. Initialize sqlite-vec extension via `sqlite_vec.Auto()` on database open
6. Verify all existing tests pass with new driver

**1.2 vec0 Schema:**
1. Create `vec_nodes` virtual table in schema initialization:
   ```sql
   CREATE VIRTUAL TABLE IF NOT EXISTS vec_nodes USING vec0(
     embedding float[1536]
   );
   ```
2. Create ID mapping table for string ID ↔ rowid correlation
3. Modify `SQLiteVectorStore.Add()` to INSERT into vec_nodes
4. Modify `SQLiteVectorStore.Delete()` to remove from vec_nodes

**1.3 Indexed Search:**
1. Implement new `Search()` using vec0 MATCH operator:
   ```sql
   SELECT rowid, distance FROM vec_nodes
   WHERE embedding MATCH ?
   ORDER BY distance
   LIMIT ?
   ```
2. Convert distance to similarity score (normalize appropriately)
3. Map rowid back to node string ID via mapping table
4. Remove old linear scan implementation
5. Add search timing to trace output

**Acceptance Criteria:**
- `go build` succeeds with CGO_ENABLED=1
- All existing unit tests pass with new driver
- vec_version() function returns valid sqlite-vec version
- Search uses vec0 MATCH query
- Search completes in <500ms for 1K+ nodes

---

### Milestone 2: Benchmarks (Rudimentary)

**Objective**: Establish basic performance baseline for future regression detection.

**Tasks:**
1. Create `pkg/store/sqlite_vector_benchmark_test.go`
2. Add benchmark: `BenchmarkVectorSearch_1000Nodes`
3. Use fake embeddings (deterministic, no OpenAI)
4. Document baseline number in QA report

**Acceptance Criteria:**
- Benchmark runs offline (no network)
- 1K node search completes in <500ms
- Baseline documented

---

### Milestone 3: Version Management & Release

**Objective**: Update release artifacts for v1.2.0.

**Tasks:**
1. Update CHANGELOG.md with v1.2.0 entry:
   - Breaking change: CGO now required (no pure-Go fallback)
   - Breaking change: Existing databases must be deleted and recreated
   - Performance: Vector search now uses sqlite-vec indexed search
   - Note: SQLite driver changed from `modernc.org/sqlite` to `mattn/go-sqlite3`
2. Update README.md:
   - Document CGO requirement and build prerequisites
   - Remove "Known Limitations" about linear scan
   - Add upgrade notes: "Delete existing database file and re-run Cognify"
3. Update go.mod version comment
4. Tag release after QA approval

**Acceptance Criteria:**
- CHANGELOG documents breaking changes clearly
- README accurately describes CGO requirement and upgrade path
- Version artifacts consistent

---

## Testing Strategy

**Expected test types:**
- **Unit tests**: VectorStore Add/Search/Delete operations
- **Integration tests**: Full pipeline with sqlite-vec (requires CGO)
- **Benchmarks**: Basic performance regression detection (1K nodes)

**Coverage expectations:**
- ≥80% for new code in pkg/store
- All existing tests must pass with new driver

**Critical scenarios:**
- Search with 1K+ nodes completes in <500ms
- Empty database initialization
- Add/Delete operations work correctly

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| CGO complicates cross-compilation | Medium | Medium | Document build requirements; accept tradeoff |
| sqlite-vec breaking changes | Low | Medium | Pin version; test upgrades |
| Build time increase (CGO) | Medium | Low | Document; CGO is cached after first build |
| Existing users must re-Cognify | Medium | Low | Clear CHANGELOG; acceptable for young project |

---

## Dependencies

- `github.com/mattn/go-sqlite3` — CGO SQLite driver
- `github.com/asg017/sqlite-vec-go-bindings/cgo` — sqlite-vec Go bindings
- SQLite 3.40+ (vec0 virtual table support)

---

## Future Considerations (Not This Plan)

- **HNSW Pure-Go Implementation**: If CGO becomes problematic in the future, implement HNSW algorithm in pure Go. Significant effort (~2-3 weeks).
- **Embedding Quantization**: sqlite-vec supports int8 vectors (4x storage reduction). Defer until storage becomes a concern.
- **Automatic Migration**: If demand exists, add automatic migration from old schema in a future release.
- **Dimension Configurability**: Make embedding dimension configurable (not just 1536) for Ollama/other models.

---

## Open Questions

None — all resolved per user feedback.

---

## Handoff Notes

**For Critic:**
- CGO-only approach confirmed per user feedback
- Breaking change (delete/recreate DB) confirmed acceptable
- Architecture doc update deferred to post-MVP

**For Implementer:**
- M1 is the bulk of the work (driver + schema + search in one milestone)
- Benchmark is rudimentary — just enough for future regression detection
- No automatic migration — users delete DB and re-Cognify
- Driver change diverges from architecture doc; note in release but no doc update required this release

**For QA:**
- Focus on: new DB creation, Add/Search/Delete operations
- Verify <500ms search for 1K nodes
- Verify existing tests pass with new driver
- Verify CGO build works on target platforms

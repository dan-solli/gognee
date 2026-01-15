# Plan 019 — Read/Write Path Optimization (Batch Embeddings + Graph Query)

**Plan ID:** 019  
**Target Release:** gognee v1.3.0 → v1.4.0 (expanded scope)  
**Epic Alignment:** Epic 7.7 (Performance Optimization) — Cognify/AddMemory/Search latency  
**Status:** UAT Approved  
**Created:** 2026-01-15  

---

## Changelog

| Date | Change | Author |
|------|--------|--------|
| 2026-01-15 | Initial plan drafted per performance incident (33s memory write) | Planner |
| 2026-01-15 | Revised per critique: adjusted target to <10s, added M5 for LLM optimization, updated roadmap | Planner |
| 2026-01-16 | Implementation complete: M1-M4, M6 delivered; M5 deferred as stretch goal | Implementer |
| 2026-01-15 | UAT approved: value statement delivered, QA technical findings assessed non-blocking | UAT |
| 2026-01-15 | Released: tagged v1.3.0 and pushed to origin | DevOps |
| 2026-01-15 | **EXTENDED**: Added M7-M10 for read path + remaining write path hotspots (11s search latency) | Planner |
| 2026-01-15 | Implementation complete: M7-M10 delivered for v1.4.0 (batch embeddings + recursive CTE) | Implementer |
| 2026-01-15 | QA complete: tests + coverage executed; artifacts recorded in agent-output/qa/ | QA |
| 2026-01-15 | UAT approved: value delivered, all objectives met; recommended for v1.4.0 release | UAT |

---

## Value Statement and Business Objective

> As a Glowbabe user storing and retrieving memories,  
> I want memory creation AND search to complete in <10 seconds,  
> So that saving and retrieving context doesn't significantly interrupt my workflow.

**Stretch Goal**: <5s with combined LLM extraction (M5)

---

## Problem Statement

### Write Path (v1.3.0 — FIXED)
Memory writes were taking **33 seconds** for a single memory with 16 nodes. Root cause: N+1 embedding problem in `Cognify()`. **Fixed in v1.3.0** via batch `Embed()` API.

### Read Path (NEW — 11s latency)
Memory search is taking **11 seconds** despite Plan 018's vector index optimization. 

**Evidence** (from user logs):
```
[2026-01-15 21:05:13.378] SearchMemories: calling gognee.Search topK=5
[2026-01-15 21:05:24.583] SearchMemories: gognee.Search completed in 11.171356001s results=5
```

**Root Cause Analysis — Complete Hotspot Inventory:**

| Location | Issue | Estimated Impact |
|----------|-------|------------------|
| `pkg/search/hybrid.go:39` | `EmbedOne()` per search query | ~1-2s |
| `pkg/search/vector.go:36` | `EmbedOne()` per search query | ~1-2s |
| `pkg/search/hybrid.go:170-208` | `expandFromNode()` BFS with N `GetNeighbors()` calls | ~8-10s (N+1 graph queries) |
| `pkg/gognee/gognee.go:1084` | `EmbedOne()` loop in `AddMemory()` | N+1 per AddMemory |
| `pkg/gognee/gognee.go:1326` | `EmbedOne()` loop in `UpdateMemory()` | N+1 per UpdateMemory |

**Key Insight**: The embedding call for a single search query isn't batchable (only 1 text), but it IS unavoidable (~1.5s). The **real problem** is the graph expansion BFS making **dozens of individual database queries**.

---

## Success Criteria

| Criterion | Current | Target | Measurement |
|-----------|---------|--------|-------------|
| Single memory write latency | 33s → **<10s (v1.3.0)** | <10s | End-to-end duration for 16-node memory |
| Single memory search latency | **11s** | <3s | End-to-end search with graph expansion |
| Embedding API calls per Cognify | ~~N~~ → **1 (v1.3.0)** | 1 | Count of OpenAI embedding requests |
| Embedding API calls per AddMemory | N (broken) | 1 | Count of OpenAI embedding requests |
| Graph queries per search | N (BFS loop) | O(1) or batched | Count of SQLite queries |
| No regression in correctness | N/A | All tests pass | `go test ./...` |

---

## Scope

### In Scope (v1.3.0 — COMPLETED)

1. **M1: Batch Embedding Collection** — Collect all entity texts before embedding ✅
2. **M2: Single Batch API Call** — Use `Embed()` instead of `EmbedOne()` loop ✅
3. **M3: Embedding Assignment** — Map batch results back to entities ✅
4. **M4: Benchmark Validation** — Add write-path benchmark to detect regressions ✅ (scaffolded, skipped)
5. **M5: Combined Entity+Relation Extraction (Stretch)** — Single LLM call for both ⏭️ DEFERRED
6. **M6: Version Management** — Update release artifacts for v1.3.0 ✅

### In Scope (v1.4.0 — NEW)

7. **M7: Batch Embeddings in AddMemory/UpdateMemory** — Same fix as Cognify for remaining write paths
8. **M8: Batched Graph Expansion** — Replace BFS N+1 GetNeighbors with batched query
9. **M9: Search Path Benchmark** — Add search benchmark to detect regressions
10. **M10: Version Management** — Update release artifacts for v1.4.0

### Out of Scope

- Caching query embeddings — Different optimization strategy
- Streaming/async memory creation — Different UX model
- Prompt engineering for faster LLM responses — Orthogonal concern

---

## Technical Approach

### Current Flow (Serial N+1)

```
for each chunk:
    entities = extractEntities(chunk)      # 1 LLM call
    relations = extractRelations(chunk)    # 1 LLM call
    
    for each entity:                       # N iterations
        node = createNode(entity)
        embedding = EmbedOne(text)         # 1 API call per entity ❌
        storeNodeWithEmbedding(node, embedding)
        indexVector(node.ID, embedding)
```

### Proposed Flow (Batched)

```
for each chunk:
    entities = extractEntities(chunk)      # 1 LLM call
    relations = extractRelations(chunk)    # 1 LLM call
    
    # Collect all texts for batch embedding
    texts = [entity.Name + " " + entity.Description for entity in entities]
    embeddings = Embed(texts)              # 1 API call total ✅
    
    for i, entity := range entities:
        node = createNode(entity)
        storeNodeWithEmbedding(node, embeddings[i])
        indexVector(node.ID, embeddings[i])
```

### Key Implementation Details

1. **Text collection order must match embedding result order** — OpenAI returns embeddings in input order
2. **Handle empty entities gracefully** — Skip entities with empty Name+Description
3. **Preserve existing error handling** — Individual node failures shouldn't break batch
4. **Batch size limits** — OpenAI supports up to 2048 texts per batch (far exceeds typical usage)

---

## Milestones

### M1: Batch Embedding Collection (Core Fix)

**Objective**: Collect entity texts before embedding generation, call `Embed()` once per chunk.

**Files to modify**:
- `pkg/gognee/gognee.go` — Refactor Cognify loop structure

**Acceptance Criteria**:
- All entity texts collected into slice before any embedding call
- Single `Embed()` call replaces loop of `EmbedOne()` calls
- Embeddings correctly mapped back to corresponding entities by index
- Existing unit tests pass

**Estimated complexity**: Low — straightforward refactor of existing loop

---

### M2: Error Handling for Batch Failures

**Objective**: Handle partial failures gracefully when batch embedding fails.

**Acceptance Criteria**:
- If batch embedding fails, error is recorded and chunk processing continues without embeddings
- Individual embedding assignment errors are logged but don't halt processing
- Error count reflected in CognifyResult.Errors

**Estimated complexity**: Low — error handling pattern already exists

---

### M3: Benchmark for Write Path

**Objective**: Add benchmark to detect write-path performance regressions.

**Files to create**:
- `pkg/gognee/cognify_benchmark_test.go`

**Acceptance Criteria**:
- `BenchmarkCognify_BatchEmbeddings` measures Cognify with realistic entity count
- Benchmark uses mock clients with simulated latency (~100ms per API call)
- Baseline established: <500ms for 16-entity chunk with mocked APIs

**Estimated complexity**: Low — follows existing benchmark patterns

---

### M4: Benchmark for Write Path

**Objective**: Add benchmark to detect write-path performance regressions.

**Files to create**:
- `pkg/gognee/cognify_benchmark_test.go`

**Acceptance Criteria**:
- `BenchmarkCognify_BatchEmbeddings` measures Cognify with realistic entity count
- Benchmark uses mock clients with simulated latency (~100ms per API call)
- Baseline established: <500ms for 16-entity chunk with mocked APIs

**Estimated complexity**: Low — follows existing benchmark patterns

---

### M5: Combined Entity+Relation Extraction (Stretch Goal)

**Objective**: Reduce LLM calls from 2 to 1 by extracting entities and relations in a single prompt.

**Files to modify**:
- `pkg/extraction/combined.go` (new file)
- `pkg/gognee/gognee.go` — Use combined extractor when available

**Acceptance Criteria**:
- Single LLM call returns both entities and triplets
- JSON schema includes both `entities` and `relations` arrays
- Validation logic preserved (entity type checking, triplet filtering)
- Existing extractors remain available for backward compatibility
- Total Cognify time <5s for 16-node memory

**Estimated complexity**: Medium — new prompt design, schema validation, integration

**Note**: This milestone is a stretch goal. Core value (33s → <10s) is delivered by M1-M4. M5 provides additional improvement (10s → <5s) but is not required for release.

---

### M6: Version Management

**Objective**: Update release artifacts for v1.3.0.

**Tasks**:
1. Update CHANGELOG.md with v1.3.0 entry
2. Update go.mod version comment
3. Commit with plan reference

**Acceptance Criteria**:
- CHANGELOG documents performance improvement
- Version artifacts consistent

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Batch API rate limiting | Low | Medium | OpenAI batch limits are generous (2048 texts); typical usage is <100 |
| Index mismatch in batch results | Low | High | Rely on OpenAI's documented behavior (returns in input order); add assertion |
| Empty entity text causes batch failure | Medium | Low | Filter empty texts before batching; skip embedding for those entities |
| Combined extraction prompt too complex | Medium | Medium | M5 is stretch goal; core value delivered by M1-M4; can defer if needed |
| LLM returns malformed combined JSON | Medium | Low | Validate schema strictly; fall back to separate extractors on parse failure |

---

## Dependencies

- None — uses existing `Embed()` method in embeddings client

---

## Assumptions

1. OpenAI's `Embed()` endpoint returns embeddings in the same order as input texts
2. Typical memory creation produces <100 entities per chunk
3. Network latency dominates embedding time (batch reduces round trips)
4. Combined extraction prompt can reliably produce valid JSON with both entities and relations
5. LLM can infer relations without explicit entity list (self-referential extraction)

---

## Testing Strategy

**Unit Tests**:
- Verify batch embedding is called with correct texts
- Verify embeddings are assigned to correct nodes
- Verify error handling for batch failures
- (M5) Verify combined extractor returns valid entities and triplets
- (M5) Verify fallback to separate extractors on parse failure

**Integration Tests** (gated):
- End-to-end Cognify with real OpenAI API
- Verify <10s for 16-node memory (primary target)
- (M5) Verify <5s for 16-node memory with combined extraction

**Benchmarks**:
- `BenchmarkCognify_BatchEmbeddings` with mocked clients
- Compare against pre-optimization baseline

---

## Validation

1. Run `go test ./...` — all tests pass
2. Run `go test -bench=. ./pkg/gognee` — benchmark validates improvement
3. Manual test: Create memory in Glowbabe, verify <10s latency (primary)
4. (M5) Manual test: Verify <5s latency with combined extraction
5. Coverage: `go test -cover ./...` — ≥73%

---

## Implementation Notes

**ILLUSTRATIVE ONLY** — Simplified batch collection pattern:

```
// Collect texts for batch embedding
var textsToEmbed []string
var entityIndices []int // Track which entities need embeddings

for i, entity := range entities {
    text := strings.TrimSpace(entity.Name + " " + entity.Description)
    if text != "" {
        textsToEmbed = append(textsToEmbed, text)
        entityIndices = append(entityIndices, i)
    }
}

// Single batch call
embeddings, err := g.embeddings.Embed(ctx, textsToEmbed)
if err != nil {
    // Handle batch failure
}

// Assign embeddings back to entities
for j, embedding := range embeddings {
    entityIdx := entityIndices[j]
    // ... assign to entities[entityIdx] ...
}
```

---

## Handoff Notes

- Plan is self-contained; no analyst investigation required
- Implementation should be straightforward refactor for M1-M4
- M5 (combined extraction) is stretch goal; can be deferred if prompt engineering is complex
- Key file: `pkg/gognee/gognee.go` lines 475-520 (entity processing loop)
- Existing trace instrumentation will capture improvement automatically
- **Roadmap**: Update product-roadmap.md to include v1.3.0 with Plan 019

---

## v1.4.0 Extension: New Milestones

### M7: Batch Embeddings in AddMemory/UpdateMemory

**Objective**: Apply same batch embedding fix to `AddMemory()` and `UpdateMemory()` functions.

**Files to modify**:
- `pkg/gognee/gognee.go` — Lines 1060-1100 (AddMemory entity loop)
- `pkg/gognee/gognee.go` — Lines 1300-1340 (UpdateMemory entity loop)

**Hotspots to fix**:
```go
// AddMemory (line 1084) — BEFORE
embedding, err := g.embeddings.EmbedOne(ctx, entity.Name+" "+entity.Description)

// UpdateMemory (line 1326) — BEFORE  
embedding, err := g.embeddings.EmbedOne(ctx, entity.Name+" "+entity.Description)
```

**Acceptance Criteria**:
- Same batch pattern as Cognify: collect texts → batch Embed() → assign by index
- Error handling matches Cognify pattern (continue without embeddings on failure)
- Unit tests pass

**Estimated complexity**: Low — copy pattern from Cognify refactor

---

### M8: Batched Graph Expansion (Critical for Search Performance)

**Objective**: Replace BFS loop with batched query to eliminate N+1 graph queries.

**Root Cause**: `expandFromNode()` in `pkg/search/hybrid.go` lines 170-208 performs BFS with individual `GetNeighbors()` calls per node in the queue. With a large graph, this results in dozens of sequential SQLite queries.

**Current Flow (N+1)**:
```go
for len(queue) > 0 {
    current := queue[0]
    queue = queue[1:]
    neighbors, err := h.graphStore.GetNeighbors(ctx, current.nodeID, 1) // 1 query per node ❌
    // ... enqueue neighbors ...
}
```

**Proposed Fix Options**:

**Option A: Single Recursive CTE Query** (Recommended)
Replace BFS loop with a single SQL query using recursive CTE to fetch all neighbors up to depth N:
```sql
WITH RECURSIVE graph_walk AS (
    SELECT target_id AS node_id, 1 AS depth FROM edges WHERE source_id = ?
    UNION
    SELECT target_id, depth + 1 FROM edges e
    JOIN graph_walk g ON e.source_id = g.node_id
    WHERE depth < ?
)
SELECT DISTINCT node_id, MIN(depth) as depth FROM graph_walk GROUP BY node_id
```

**Option B: Batch GetNeighbors**
Add `GetNeighborsBatched(ctx, nodeIDs []string, depth int)` to GraphStore interface that fetches neighbors for multiple nodes in one query.

**Files to modify**:
- `pkg/store/graph.go` — Add batched method to interface
- `pkg/store/sqlite.go` — Implement recursive CTE or batch query
- `pkg/search/hybrid.go` — Replace BFS loop with single call

**Acceptance Criteria**:
- Search with graph expansion completes in <3s (down from 11s)
- Single or O(depth) SQL queries instead of O(nodes)
- Existing search tests pass

**Estimated complexity**: Medium — requires SQL expertise for recursive CTE

---

### M9: Search Path Benchmark

**Objective**: Add benchmark for search path to detect performance regressions.

**Files to create**:
- `pkg/search/hybrid_benchmark_test.go`

**Acceptance Criteria**:
- `BenchmarkHybridSearch_GraphExpansion` measures search with graph depth 1-2
- Benchmark uses mock stores or pre-populated in-memory DB
- Baseline established: <100ms for 100-node graph with mocked APIs

**Estimated complexity**: Low — follows existing benchmark patterns

---

### M10: Version Management for v1.4.0

**Objective**: Update release artifacts for v1.4.0.

**Files to modify**:
- `CHANGELOG.md` — Add v1.4.0 entry
- `go.mod` — Update version comment

**Acceptance Criteria**:
- CHANGELOG documents M7-M9 improvements
- Version comment reflects v1.4.0

---

## Open Questions

None — approach is well-understood and low-risk.

---

## References

- Plan 018: Vector Search Optimization (related performance work)
- [pkg/embeddings/client.go](pkg/embeddings/client.go) — `Embed()` batch interface
- [pkg/gognee/gognee.go](pkg/gognee/gognee.go) — Cognify implementation
- [pkg/search/hybrid.go](pkg/search/hybrid.go) — Graph expansion BFS loop
- [pkg/store/sqlite.go](pkg/store/sqlite.go) — SQLite graph store implementation


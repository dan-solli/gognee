# Plan 005 — Phase 5 Search

**Plan ID:** 005

**Target Release:** v0.5.0

**Epic Alignment:** ROADMAP Phase 5 — Hybrid Search

**Status:** UAT Approved

**Changelog**
- 2025-12-24: Created plan for Phase 5 implementation.
- 2025-12-24: Revised based on critique — addressed M1 (unified interface via SeedNodeIDs in SearchOptions), M2 (explicit score formula), L1-L3 (test case and clarifications).
- 2025-12-24: Implementation complete — all 6 milestones delivered, 85.0% test coverage, ready for QA.
- 2025-12-24: UAT approved — all value delivery scenarios pass; APPROVED FOR RELEASE as v0.5.0.

---

## Value Statement and Business Objective
As a developer integrating gognee into Glowbabe, I want to query the knowledge graph by meaning (vector similarity), by relationship structure (graph traversal), or by a combination of both (hybrid search), so that I can retrieve contextually relevant information regardless of exact wording and discover connected knowledge in a single query.

---

## Objective
Deliver Phase 5 from ROADMAP:
- Implement vector-only search (embed query → find similar nodes)
- Implement graph traversal search (find nodes connected to a seed set)
- Implement hybrid search combining both strategies
- Add result ranking and scoring with deduplication

This phase remains library-only (no CLI) and follows existing patterns: interface-driven design, offline-first unit tests, minimal dependencies.

---

## Scope

**In scope**
1. Create `pkg/search/` package with:
   - `SearchType` enum (`vector`, `graph`, `hybrid`)
   - `SearchResult` struct (NodeID, Node pointer, Score, Source, GraphDepth)
   - `SearchOptions` struct (Type, TopK, GraphDepth)
   - `Searcher` interface
2. Implement `VectorSearcher` (vector-only search using embeddings + VectorStore)
3. Implement `GraphSearcher` (graph traversal from seed nodes using GraphStore)
4. Implement `HybridSearcher` (combines vector and graph search with score merging)
5. Unit tests for all three searcher implementations
6. Score normalization and ranking logic

**Out of scope**
- High-level `Gognee.Search()` orchestration (Phase 6)
- Full `Add()` → `Cognify()` → `Search()` pipeline integration (Phase 6)
- Persisting search history or query caching
- Any CLI surface

---

## Key Constraints
- Library-only: no `cmd/` directory, no executable concerns
- Unit tests must work offline using mock/fake stores
- Reuse existing interfaces: `EmbeddingClient`, `VectorStore`, `GraphStore`
- Interface-driven design for swappable searcher implementations
- Cognee-aligned: hybrid search should expand using direction-agnostic graph traversal

---

## Plan-Level Decisions (to remove ambiguity)

1. **Package location:**
   - Create new `pkg/search/` package for search implementations.
   - Keep it decoupled from `pkg/store/` (searchers accept store interfaces, not implementations).
   - Rationale: separation of concerns; searchers are consumers of stores.

2. **SearchResult design:**
   - `SearchResult.Node` is a pointer to `*store.Node` (allows nil for deleted nodes).
   - `SearchResult.Source` indicates origin: `"vector"`, `"graph"`, or `"hybrid"`.
     - `"vector"`: node found only via vector similarity search.
     - `"graph"`: node found only via graph expansion (not a direct vector hit).
     - `"hybrid"`: node found by BOTH vector search AND graph expansion (score boosted).
   - `SearchResult.GraphDepth` is 0 for direct vector hits, >0 for nodes discovered via graph expansion.
   - Rationale: enables callers to understand why a result was returned and filter accordingly.

3. **Score semantics:**
   - Vector search: score is cosine similarity (0 to 1, higher is better).
   - Graph search: score decreases with graph distance: `graph_score = 1.0 / (1 + depth)`.
   - Hybrid search: combined score formula:
     ```
     combined_score = vector_score + graph_score
     where:
       vector_score = cosine_similarity if found by vector search, else 0
       graph_score  = 1.0 / (1 + depth) if found by graph expansion, else 0
     ```
   - Example: A node with vector_score=0.8 that is also a depth=1 neighbor gets `0.8 + 0.5 = 1.3`.
   - Example: A node found only by vector with score=0.7 gets `0.7 + 0 = 0.7`.
   - Example: A node found only by graph at depth=2 gets `0 + 0.33 = 0.33`.
   - Rationale: simple, predictable, additive scoring; can be refined post-MVP with weights.

4. **Hybrid search algorithm:**
   - Step 1: Embed the query text using `EmbeddingClient`.
   - Step 2: Vector search for top-K similar nodes.
   - Step 3: For each vector result, expand via `GraphStore.GetNeighbors(nodeID, GraphDepth)`.
   - Step 4: Deduplicate nodes (keep highest combined score per NodeID).
   - Step 5: Sort by combined score descending.
   - Step 6: Return top-K results.
   - Rationale: matches ROADMAP 5.2 algorithm outline; Cognee-aligned neighbor expansion.

5. **Graph-only search semantics:**
   - Uses `SeedNodeIDs` field in `SearchOptions` (not the query string).
   - `GraphSearcher.Search` ignores the query string parameter; uses seeds from options.
   - If `SeedNodeIDs` is empty, returns an error (seeds are required for graph search).
   - Expands from seeds using `GetNeighbors` with configurable depth.
   - Score based on distance from nearest seed: `1.0 / (1 + depth)`.
   - Rationale: graph search needs starting points; unified interface via SearchOptions.

6. **VectorSearcher design:**
   - Accepts text query, embeds it, calls `VectorStore.Search`, enriches results with full `Node` data from `GraphStore`.
   - Requires both `VectorStore` and `GraphStore` to return complete `SearchResult` with `Node` pointer.
   - Rationale: vector store only holds embeddings; node metadata lives in graph store.

7. **Default TopK and GraphDepth:**
   - `TopK` defaults to 10 if not specified (or ≤0).
   - `GraphDepth` defaults to 1 for hybrid search (Cognee-aligned single-hop expansion).
   - Rationale: sensible defaults; callers can override.

9. **Hybrid search initial vector fetch:**
   - To ensure adequate graph expansion base, hybrid search fetches `max(TopK * 2, 20)` initial vector results before expansion.
   - After graph expansion and score merging, the final result set is cut to `TopK`.
   - Rationale: small initial TopK (e.g., 5) would limit which neighbors get discovered; fetching more upfront improves recall.

8. **Error handling:**
   - If embedding fails, return error (do not silently return empty results).
   - If a node ID from vector search is not found in graph store, skip it with a logged warning (stale vector index scenario).
   - Rationale: fail-fast on critical errors; graceful degradation for stale data.

---

## Open Questions — None

All decisions are resolved based on ROADMAP guidance and Cognee-alignment analysis.

---

## Plan (Milestones)

### Milestone 1 — Search Interface + Types
**Objective:** Define the search API surface in `pkg/search`.

**Tasks**
1. Create `pkg/search/search.go` with:
   - `SearchType` type and constants (`SearchTypeVector`, `SearchTypeGraph`, `SearchTypeHybrid`)
   - `SearchResult` struct (NodeID, Node, Score, Source, GraphDepth)
   - `SearchOptions` struct (Type, TopK, GraphDepth, SeedNodeIDs []string)
   - `Searcher` interface with `Search(ctx, query string, opts SearchOptions) ([]SearchResult, error)`
2. Add helper function `applyDefaults(opts *SearchOptions)` to set TopK=10 and GraphDepth=1 if unspecified.
3. Document interface in docstrings explaining score semantics and Source field meaning.

**Acceptance criteria**
- Interface and structs compile without additional dependencies beyond `pkg/store`.
- Docstrings explain score semantics (cosine similarity for vector, distance-based for graph).
- API aligns with ROADMAP 5.1 specification.

---

### Milestone 2 — Vector Searcher Implementation
**Objective:** Implement vector-only search capability.

**Tasks**
1. Create `pkg/search/vector.go` with `VectorSearcher` struct.
2. `VectorSearcher` holds references to `EmbeddingClient`, `VectorStore`, and `GraphStore`.
3. Implement constructor `NewVectorSearcher(embClient, vectorStore, graphStore)`.
4. Implement `Search` method:
   - Embed query text using `EmbeddingClient.EmbedOne`.
   - Call `VectorStore.Search(embedding, topK)`.
   - For each `SearchResult` from vector store, call `GraphStore.GetNode(id)` to populate `Node` pointer.
   - Skip missing nodes gracefully (stale vector index scenario).
   - Set `Source = "vector"` and `GraphDepth = 0`.
   - Return results sorted by score descending.

**Acceptance criteria**
- `VectorSearcher` implements `Searcher` interface.
- Returns complete `SearchResult` with populated `Node` data.
- Handles missing nodes gracefully without crashing.

---

### Milestone 3 — Graph Searcher Implementation
**Objective:** Implement graph traversal search from seed nodes.

**Tasks**
1. Create `pkg/search/graph.go` with `GraphSearcher` struct.
2. `GraphSearcher` holds reference to `GraphStore`.
3. Implement constructor `NewGraphSearcher(graphStore)`.
4. Implement `Search(ctx, query string, opts SearchOptions) ([]SearchResult, error)`:
   - Ignore `query` parameter (graph search uses seeds, not text).
   - Read `opts.SeedNodeIDs`; return error if empty.
   - For each seed, call `GraphStore.GetNeighbors(seedID, opts.GraphDepth)`.
   - Include seed nodes themselves at depth=0 (score=1.0).
   - Track depth from nearest seed for each discovered node.
   - Score nodes: `1.0 / (1 + depth)`.
   - Deduplicate by NodeID (keep highest score).
   - Sort by score descending, return top-K.
   - Set `Source = "graph"` and populate `GraphDepth`.

**Acceptance criteria**
- `GraphSearcher` implements `Searcher` interface.
- Uses `opts.SeedNodeIDs` for seeds; returns error if empty.
- Score decreases with graph distance.
- Results deduplicated and sorted.

---

### Milestone 4 — Hybrid Searcher Implementation
**Objective:** Combine vector and graph search with score merging.

**Tasks**
1. Create `pkg/search/hybrid.go` with `HybridSearcher` struct.
2. `HybridSearcher` holds references to `EmbeddingClient`, `VectorStore`, `GraphStore`.
3. Implement constructor `NewHybridSearcher(embClient, vectorStore, graphStore)`.
4. Implement `Search` method following ROADMAP algorithm:
   - Embed query text.
   - Vector search for `max(TopK * 2, 20)` initial results (expansion base).
   - For each vector result, expand via `GraphStore.GetNeighbors(nodeID, GraphDepth)`.
   - Combine scores using additive formula:
     - `combined_score = vector_score + graph_score`
     - `vector_score = cosine_similarity` if found by vector, else 0
     - `graph_score = 1.0 / (1 + depth)` if found by graph expansion, else 0
   - Set `Source` based on how node was found:
     - `"vector"`: found only by vector search
     - `"graph"`: found only by graph expansion
     - `"hybrid"`: found by BOTH vector AND graph (score boosted)
   - Deduplicate by NodeID (keep highest combined score).
   - Sort by combined score descending.
   - Return top-K results (cut to final TopK from opts).
5. Document score combination formula in docstrings.

**Acceptance criteria**
- `HybridSearcher` implements `Searcher` interface.
- Combines vector similarity and graph proximity.
- Results deduplicated, sorted, and capped at TopK.

---

### Milestone 5 — Unit Tests (Offline)
**Objective:** Lock in search behavior with comprehensive offline tests.

**Tasks**
1. Create `pkg/search/vector_test.go` with tests for `VectorSearcher`:
   - Mock `EmbeddingClient`, `VectorStore`, `GraphStore`.
   - Test basic search returns correct nodes.
   - Test score ordering.
   - Test handling of missing nodes (stale index).
   - Test empty results.
2. Create `pkg/search/graph_test.go` with tests for `GraphSearcher`:
   - Test single seed, depth=1.
   - Test multiple seeds.
   - Test depth > 1.
   - Test deduplication.
   - Test score decay.
3. Create `pkg/search/hybrid_test.go` with tests for `HybridSearcher`:
   - Test vector+graph combination.
   - Test score merging for nodes found by both (Source="hybrid", boosted score).
   - Test node found ONLY by vector (Source="vector", score = vector_score).
   - Test node found ONLY by graph expansion (Source="graph", score = graph_score).
   - Test TopK limiting (initial fetch > final TopK).
   - Test GraphDepth expansion.

**Acceptance criteria**
- `go test ./pkg/search/...` passes offline.
- No network calls in unit tests (all dependencies mocked).
- Coverage > 80% for `pkg/search`.

---

### Milestone 6 — Version Management
**Objective:** Update version artifacts to reflect v0.5.0 release.

**Tasks**
1. Add `[0.5.0]` section to CHANGELOG.md documenting Phase 5 deliverables.
2. Update any version constants if present (none currently).
3. Commit with message following project conventions.

**Acceptance criteria**
- CHANGELOG reflects Phase 5 changes (search package, three searcher types, hybrid algorithm).
- Version matches roadmap target (v0.5.0).

---

## Testing Strategy

**Expected test types:**
- Unit tests with mocked dependencies (offline)
- No integration tests hitting real OpenAI API (that's Phase 6 territory)

**Coverage expectations:**
- Target > 80% for `pkg/search` package
- All public API methods covered

**Critical scenarios:**
- Vector search returns nodes sorted by similarity
- Graph search expands correctly from seeds
- Hybrid search combines and deduplicates results
- Missing nodes handled gracefully
- Edge cases: empty results, single result, exact TopK

---

## Validation

**Handoff notes for Implementer:**
- Reuse existing interfaces; do not modify `pkg/store` or `pkg/embeddings`.
- Follow TDD pattern (write failing tests first).
- Keep searchers stateless (no caching) for MVP simplicity.

**Rollback considerations:**
- Phase 5 is additive (new `pkg/search` package); no breaking changes to existing packages.
- Rollback is simply removing the search package if needed.

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Score normalization produces unintuitive results | Medium | Medium | Document scoring formula; can tune weights post-MVP |
| Graph expansion produces too many results before filtering | Low | Low | TopK applied at final step; intermediate expansion is bounded by depth |
| Stale vector store entries (nodes deleted from graph) | Low | Low | Graceful skip with warning; caller can re-index if needed |

---

## Dependencies

- Phase 4 complete (v0.4.0) — `GraphStore`, `VectorStore` interfaces and implementations available
- `EmbeddingClient` from Phase 1 for query embedding

---

## Post-Phase Notes

Phase 6 (Integration) will:
- Wire `HybridSearcher` into `Gognee.Search()` API
- Implement full `Add()` → `Cognify()` → `Search()` pipeline
- Add end-to-end tests with real LLM/embedding calls


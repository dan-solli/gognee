# Critique — Plan 005: Phase 5 Search

**Artifact:** [agent-output/planning/005-phase5-search-plan.md](../planning/005-phase5-search-plan.md)

**Date:** 2025-12-24

**Status:** CLOSED — All findings addressed

---

## Changelog

| Date | Handoff | Request | Summary |
|------|---------|---------|---------|
| 2025-12-24 | Planner → Critic | Review for clarity, completeness, architectural alignment | Initial critique |
| 2025-12-24 | Critic → Planner | Address findings M1, M2, L1-L3 | Planner revised plan; all findings resolved |

---

## Value Statement Assessment

**Rating:** ✅ PASS

The value statement is well-formed and follows the "As a…I want…so that…" structure:

> *As a developer integrating gognee into Glowbabe, I want to query the knowledge graph by meaning (vector similarity), by relationship structure (graph traversal), or by a combination of both (hybrid search), so that I can retrieve contextually relevant information regardless of exact wording and discover connected knowledge in a single query.*

The statement correctly identifies:
- The user (developer integrating gognee into Glowbabe)
- The capability (three search modes: vector, graph, hybrid)
- The downstream value (semantic retrieval + connected knowledge discovery)

This aligns with ROADMAP Phase 5 goals and maintains library-only positioning.

---

## Unresolved Open Questions

**None.** The plan states "All decisions are resolved based on ROADMAP guidance and Cognee-alignment analysis."

No blockers requiring Planner attention before implementation.

---

## Overview

Plan 005 delivers the search layer as Phase 5 of gognee development. It builds on Phase 4 storage and introduces:
- `pkg/search/` package with `Searcher` interface
- Three implementations: `VectorSearcher`, `GraphSearcher`, `HybridSearcher`
- Score normalization and deduplication logic
- Comprehensive offline unit tests

The plan follows established patterns from Phases 1-4 and stays within ROADMAP scope.

---

## Architectural Alignment

**Rating:** ✅ ALIGNED

| Criterion | Assessment |
|-----------|------------|
| Library-only constraint | ✅ Maintained — no CLI surface |
| Dependency surface | ✅ No new external dependencies |
| Interface-driven design | ✅ `Searcher` interface for swappability |
| Package location | ✅ `pkg/search/` matches ROADMAP |
| API signatures | ✅ Matches ROADMAP 5.1 specification exactly |
| Cognee parity | ✅ Direction-agnostic traversal, depth=1 default |
| Testing strategy | ✅ Offline-first with mocked dependencies |
| Phase 4 integration | ✅ Correctly consumes `GraphStore`, `VectorStore`, `EmbeddingClient` interfaces |

The plan correctly creates a new `pkg/search` package and respects existing interface boundaries.

---

## Scope Assessment

**Rating:** ✅ APPROPRIATE

**In-scope items** are correctly bounded to Phase 5 deliverables from the ROADMAP:
- Vector-only search ✅
- Graph traversal search ✅
- Hybrid search combining both ✅
- Result ranking and scoring ✅

**Out-of-scope items** appropriately defer:
- `Gognee.Search()` orchestration (Phase 6)
- Full pipeline integration (Phase 6)
- Search caching/history

The scope is focused and achievable within the estimated 1-week duration.

---

## Technical Debt Risks

| Risk | Severity | Mitigation in Plan |
|------|----------|-------------------|
| Score normalization may need tuning | Low | ✅ Documented as refinable post-MVP |
| Stale vector store entries | Low | ✅ Graceful skip with warning |
| Graph expansion explosion | Low | ✅ Bounded by depth; TopK at final step |
| Scoring formula complexity | Low | ✅ Simple additive approach documented |

**No high-severity debt risks identified.**

---

## Findings

### Finding M1 — GraphSearcher Interface Asymmetry
**Severity:** Medium  
**Status:** RESOLVED

**Issue:** The `Searcher` interface takes a `query string`, but `GraphSearcher` requires seed node IDs, not a text query. The plan handles this by:
- Adding a separate `SearchFromSeeds` method
- Making `Search` error for text queries

**Impact:** This creates an asymmetric API where `GraphSearcher` doesn't cleanly implement `Searcher`. Callers must know which searcher type they're using to call the right method.

**Recommendation:** Consider one of:
1. **Accept the asymmetry** (documented clearly) — graph search is fundamentally different from text-based search.
2. **Add `SeedNodeIDs` to `SearchOptions`** — unified interface, graph search ignores query string if seeds provided.
3. **Don't implement `Searcher` for `GraphSearcher`** — only `HybridSearcher` and `VectorSearcher` implement it.

The plan should explicitly decide which approach to take. Option 2 or 3 would be cleanest.

**Resolution:** Plan revised to use Option 2 — `SeedNodeIDs []string` added to `SearchOptions`. GraphSearcher now implements `Searcher` interface cleanly, using seeds from options and ignoring query string.

---

### Finding M2 — Hybrid Score Combination Ambiguity
**Severity:** Medium  
**Status:** RESOLVED

**Issue:** The plan states hybrid search "normalizes both signals and sums them (weighted equally by default)" but doesn't specify:
- How vector scores (0-1) and graph scores (1/(1+depth)) are combined
- Whether a node found only by vector gets score = vector_score + 0, or just vector_score
- Whether a node found only by graph expansion gets score = 0 + graph_score

**Impact:** Implementer must make decisions that could affect search quality. A node at depth=1 with graph_score=0.5 would score lower than a direct vector hit with score=0.6, which may or may not be desired.

**Recommendation:** Clarify the formula explicitly. Example:
```
combined_score = α * vector_score + β * graph_score
where α = β = 1.0 by default
vector_score = 0 if not found by vector
graph_score = 0 if not found by graph expansion
```

**Resolution:** Plan revised with explicit formula and examples:
- `combined_score = vector_score + graph_score` where missing = 0
- Added three concrete examples showing different scenarios

---

### Finding L1 — Missing Context Propagation Test Case
**Severity:** Low  
**Status:** RESOLVED

**Issue:** Milestone 5 tests don't explicitly cover the scenario where a direct vector hit is *also* a neighbor of another vector hit (found by both paths). This is the core hybrid value proposition.

**Impact:** Implementer might miss testing the score boosting behavior for nodes discovered via multiple paths.

**Recommendation:** Add explicit test case: "Test node found by both vector and graph expansion gets boosted score."

**Resolution:** Milestone 5 hybrid tests now explicitly include three Source-based test cases: vector-only, graph-only, and hybrid (both paths with boosted score).

---

### Finding L2 — No TopK Expansion Strategy for Hybrid
**Severity:** Low  
**Status:** RESOLVED

**Issue:** Milestone 4 says "Vector search for initial top-K (use TopK from opts, or higher to get expansion base)" but doesn't specify *how much higher*. If TopK=5 and we only vector-search for 5, graph expansion might return 20+ additional nodes, but we started with a small base.

**Impact:** Minor — the final TopK cut handles it, but the initial vector search size affects which neighbors get discovered.

**Recommendation:** Specify a concrete strategy, e.g., "Initial vector search uses `max(TopK, 10)` to ensure adequate expansion base" or leave as implementation detail with a note.

**Resolution:** Plan revised with explicit strategy: `max(TopK * 2, 20)` for initial vector fetch, with rationale documented in Decision #9.

---

### Finding L3 — Source Field for Mixed-Origin Nodes
**Severity:** Low  
**Status:** RESOLVED

**Issue:** Decision #2 says `Source = "hybrid"` for nodes found via graph expansion, `"vector"` for direct hits only. But what about a node that's both a direct vector hit AND discovered via graph expansion from another node?

**Impact:** Minor confusion — should it be "hybrid" or "vector"? The score boosting implies both contributed.

**Recommendation:** Clarify: "Nodes found by both vector AND graph should have `Source = "hybrid"` to indicate combined origin."

**Resolution:** Decision #2 revised with explicit three-way Source semantics: "vector" (vector only), "graph" (graph only), "hybrid" (both paths, score boosted).

---

## Risk Assessment

**Hotfix Risk Question:** *"How will this plan result in a hotfix after deployment?"*

| Scenario | Likelihood | Mitigation |
|----------|------------|------------|
| Score formula produces poor ranking | Medium | Plan documents as refinable post-MVP; no runtime failure |
| Empty results when graph has data | Low | Tests cover empty cases; stale index handling documented |
| Performance issues with large graphs | Low | Bounded by depth=1 default; TopK limits output |
| API confusion (GraphSearcher interface) | Medium | Finding M1 — needs decision |

**Overall risk:** Low-Medium. The additive nature of Phase 5 limits blast radius. Main risk is API design clarity (M1) which should be resolved before implementation.

---

## Recommendations

All recommendations have been addressed in the revised plan:

1. ✅ **M1 addressed**: `SeedNodeIDs` added to `SearchOptions`; `GraphSearcher` implements `Searcher` cleanly.
2. ✅ **M2 addressed**: Explicit score formula with examples added to Decision #3.
3. ✅ **L1 addressed**: Test cases for all three Source scenarios added to Milestone 5.
4. ✅ **L2 addressed**: `max(TopK * 2, 20)` expansion strategy documented in Decision #9.
5. ✅ **L3 addressed**: Three-way Source semantics clarified in Decision #2.

---

## Summary

Plan 005 is **approved for implementation**. All critique findings have been resolved.

**Blocking issues:** None

**Status:** CLOSED — Ready for handoff to Implementer.

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| Initial | 2025-12-24 | First critique of Plan 005 |
| Revised | 2025-12-24 | All findings addressed; status CLOSED |


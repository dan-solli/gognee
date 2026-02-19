# gognee - Product Roadmap

**Last Updated**: 2026-01-27
**Roadmap Owner**: roadmap agent
**Strategic Vision**: Build a Go library package that provides AI assistants with persistent memory across conversations, knowledge graph for understanding relationships, and hybrid search combining graph traversal and vector similarity. Pure library design (no CLI), embeddable in Go projects like Glowbabe.

## Change Log
| Date & Time | Change | Rationale |
|-------------|--------|-----------|
| 2026-02-18 | Created Epic 10.1: Memory Decay Observability (slog-based structured logging for decay subsystem) | Decay/prune runs silently; consumers cannot verify effectiveness for long-running assistants |
| 2026-01-27 | Plan 021 APPROVED after Critic revisions; search instrumentation added, bidirectional supersession confirmed, real-time velocity locked | Critic found HIGH-severity gap: access tracking must capture search path (primary read). Resolved with BatchUpdateMemoryAccess. Handoff to Implementer. |
| 2026-01-27 | Plan 021 drafted for Epic 9.1 (v1.1.0); 13 milestones, 4 sub-epics | Implementation planning for Intelligent Memory Lifecycle |
| 2026-01-27 | Created Epic 9.1 (Intelligent Memory Lifecycle) with 7 sub-epics | Strategic analysis: calendar-time decay insufficient; need access-frequency scoring, explicit supersession, semantic consolidation, retention policies, pinning, conflict detection, provenance weighting |
| 2026-01-15 | Plan 019 v1.4.0 committed locally (11d14e3); v1.3.0 released | Read path optimization (M7–M10) UAT approved; write path v1.3.0 released |
| 2026-01-15 | Added v1.3.0 release track with Plan 019 (Write Path Optimization) | Performance incident: 33s memory write latency |
| 2026-01-15 | Released v1.2.0 (Plans 017, 018); vector search + observability | Vector search 17s→<1ms; CGO required (breaking change); always-on metrics |
| 2026-01-03 | Plan 011 drafted for Epic 8.1; provenance-first design confirmed | Architecture findings approved; plan ready for Critic review |
| 2026-01-02 | Created Epic 8.1 (First-Class Memory CRUD) and Release v1.0.0; P0 priority | Glowbabe feature request identifies architecture gap blocking Memory Browser feature (Epic 6.1 in Glowbabe) |
| 2025-12-25 | Marked Plan 009 (Incremental Cognify) as Delivered; v0.8.0 ready for release | Retrospective closed for Plan 009; UAT approved incremental Cognify with cost/time reduction value delivered |
| 2025-12-25 | Marked Plan 008 (Edge ID Correctness) as Delivered; v0.7.1 released | Retrospective closed for Plan 008; QA+UAT verified edge endpoint IDs match node IDs correctly |
| 2025-12-25 | Marked Plan 010 (Memory Decay/Forgetting) as Delivered; v0.9.0 released | Retrospective closed for Plan 010; Epic 7.5 complete with full UAT approval |
| 2025-12-25 | Marked Plans 007-010 as Critic Approved in Active Release Tracker | Plans revised per critique; critiques updated to RESOLVED/APPROVED |
| 2025-12-25 | Marked Plan 008 as QA Complete | QA executed: unit tests + coverage verified; integration suite warning documented |
| 2025-12-24 | Plans 007-010 created for post-MVP epics 7.1, 7.3, 7.4, 7.5 | User requested backlog planning; Skipped 7.2 and 7.6 |
| 2025-12-24 23:30 | Created product roadmap; marked v0.6.0 as Released | Retrospective closed for Plan 006 (Phase 6 Integration); MVP delivered |

---

## Master Product Objective

🚨 **IMMUTABLE - ONLY USER CAN CHANGE** 🚨

Deliver a production-ready Go knowledge graph memory library that enables AI assistants to:
- Store and retrieve information relationships persistently
- Extract entities and relationships from text using LLM
- Search semantically using hybrid vector + graph traversal
- Integrate with minimal code (single import, ~3 method calls)
- Require no external services beyond SQLite

**Target User**: Developers building AI assistants with long-term memory (e.g., Glowbabe project)

---

## Release v0.6.0 - MVP Complete ✅
**Target Date**: 2025-12-24
**Actual Release Date**: 2025-12-24
**Status**: Released
**Strategic Goal**: Deliver complete MVP with unified API for knowledge graph memory integration

### Epic 6.1: Phase 6 Integration - Unified API
**Priority**: P0
**Status**: Delivered

**User Story**:
As a developer building an AI assistant with persistent memory (like Glowbabe),
I want a unified API that lets me `Add()` text, `Cognify()` it into a knowledge graph, and `Search()` for relevant context,
So that I can integrate knowledge graph memory into my application with a single library import and three method calls.

**Business Value**:
- Completes MVP milestone - gognee ready for production use
- Enables Glowbabe integration with minimal code (~20 lines)
- Provides persistent memory capability to AI assistants
- Three-method workflow reduces integration complexity

**Dependencies**:
- Phase 1-5 deliverables (chunking, embeddings, extraction, storage, search)

**Acceptance Criteria** (outcome-focused):
- ✅ Single library import provides all core functionality
- ✅ Add() method buffers text without processing (no LLM calls yet)
- ✅ Cognify() method runs full extraction pipeline (chunk → extract → store → embed)
- ✅ Search() method queries knowledge graph with semantic ranking
- ✅ Persistent SQLite storage for nodes and edges
- ✅ Deterministic node IDs enable upsert semantics (same entity → same node)
- ✅ Integration tests validate end-to-end workflow with real OpenAI API
- ✅ Comprehensive documentation with Quick Start and API Reference

**Constraints**:
- Library-only (no CLI)
- No new dependencies beyond existing (SQLite, UUID, standard library)
- Unit tests must be offline-first (no network access)
- Test coverage ≥80%

**Status Notes**:
- 2025-12-24: Plan 006 created and approved by critic
- 2025-12-24: Implementation complete - all milestones delivered
- 2025-12-24: QA Complete - 89% coverage, all tests pass (unit + integration)
- 2025-12-24: UAT Complete - 100% value delivery, no deferrals, approved for release
- 2025-12-24: Retrospective closed - v0.6.0 released

**Delivered Artifacts**:
- Unified API: Add(), Cognify(), Search(), Close(), Stats()
- Type re-exports for convenience (SearchResult, SearchOptions, Node, Edge)
- GraphStore interface extension (NodeCount, EdgeCount)
- Integration tests (gated with build tag)
- README with Quick Start example
- CHANGELOG v0.6.0 entry

---

## Active Release Tracker

**Current Working Release**: None (all approved work released)

### v1.4.0 Release - Read & Write Path Optimization
**Target Date**: 2026-01-16
**Actual Release Date**: 2026-01-16
**Status**: Released ✅
**Strategic Goal**: Reduce search latency from ~11s to <3s via recursive CTE graph traversal and batch embeddings

| Plan ID | Title | Epic | Status | Committed | Released |
|---------|-------|------|--------|----------|----------|
| 019 | Read/Write Path Optimization (v1.4.0) | 7.7 | Released | ✅ Yes (3e65eac) | ✅ 2026-01-16 |

**Release Status**: ✅ RELEASED
**Ready for Release**: N/A
**Blocking Items**: None
**Release Notes**:
- M7: Batch embeddings for AddMemory/UpdateMemory (N+1 → single Embed() call)
- M8: Recursive CTE for GetNeighbors (BFS N+1 → single SQL query)
- M9: Search benchmarks added for regression detection
- M10: CHANGELOG and version artifacts updated
- Test coverage: 73.6% overall
**Architecture**: Two-pass batch embedding pattern; recursive CTE graph traversal; benchmark harness

### v1.3.0 Release - Write Path Optimization
**Target Date**: 2026-01-15
**Actual Release Date**: 2026-01-15
**Status**: Released ✅
**Strategic Goal**: Reduce memory write latency from 33s to <10s via batch embeddings

| Plan ID | Title | Epic | Status | Committed | Released |
|---------|-------|------|--------|----------|----------|
| 019 | Write Path Optimization (M1–M6) | 7.7 | Released | ✅ Yes | ✅ 2026-01-15 |

**Release Status**: ✅ RELEASED
**Blocking Items**: None
**Release Notes**:
- Batch embedding for Cognify write path (33s → <5s)
- Metrics for write latency monitoring
**Architecture**: Batch embedding collection; optional combined entity+relation extraction

### v1.2.0 Release - Vector Search Optimization + Always-On Observability
**Target Date**: 2026-01-15
**Actual Release Date**: 2026-01-15
**Status**: Released ✅
**Strategic Goal**: Instant memory search via sqlite-vec indexed ANN search; remove build-tag complexity for observability

| Plan ID | Title | Epic | Status | Committed | Released |
|---------|-------|------|--------|----------|----------|
| 017 | Always-On Observability | 7.1 | Released | ✅ Yes | ✅ 2026-01-15 |
| 018 | Vector Search Optimization (sqlite-vec) | 7.6 | Released | ✅ Yes | ✅ 2026-01-15 |

**Release Status**: ✅ RELEASED
**Blocking Items**: None
**Release Notes**:
- **BREAKING**: CGO now required for all builds (sqlite-vec dependency)
- **BREAKING**: Existing databases must be deleted and recreated (vec0 schema change)
- Vector search reduced from 17s → <1ms for typical workloads (4,500x faster than target)
- vec0 virtual table with MATCH operator for O(log n) ANN search
- Always-on metrics and trace (no build tags; runtime no-op when disabled)
- Metrics endpoint: 127.0.0.1:8899/metrics (default)
- Benchmarks: 111µs/op for 1K-node search
- Test coverage: 73.5% overall, 74.7% pkg/store
**Architecture**: sqlite-vec v0.1.6 via mattn/go-sqlite3 CGO driver

### v1.1.0 Release - Observability: Metrics Infrastructure
**Target Date**: 2026-01-14
**Actual Release Date**: 2026-01-14
**Status**: Released ✅
**Strategic Goal**: Add Prometheus metrics collection to gognee library; enable observability for diagnosing ~50% memory operation failure rate

| Plan ID | Title | Epic | Status | Committed | Released |
|---------|-------|------|--------|----------|----------|
| 016 | Observability: Prometheus Metrics & Trace Export (M1) | 7.1 | Delivered | ✅ Yes | ✅ 2026-01-14 |

**Release Status**: ✅ RELEASED
**Blocking Items**: None
**Release Notes**:
- Prometheus metrics collection infrastructure (gognee library)
- Error classification for diagnosing operation failures by category
- Build-tag opt-in (`-tags=metrics`) for zero-overhead builds
- Metrics: `gognee_operations_total`, `gognee_operation_duration_seconds`, `gognee_errors_total`, `gognee_storage_count`
- Instrumented operations: Cognify, Search, AddMemory
- Test coverage: 75.8% (metrics build), benchmarks verify <1% overhead for realistic workloads
- Remaining milestones (M2–M8): HTTP metrics endpoint, trace export, viewer, documentation, etc.
**Architecture**: Build-tag-driven opt-in via `//go:build metrics` and `//go:build !metrics`
**Note**: Build-tag opt-in superseded by v1.2.0 always-on observability

### v1.0.0 Release - First-Class Memory CRUD
**Target Date**: TBD
**Status**: Planned
**Strategic Goal**: Enable user-facing memory management with CRUD operations and graph/vector synchronization

| Plan ID | Title | Epic | Status | Committed | Released |
|---------|-------|------|--------|----------|----------|
| 011 | First-Class Memory CRUD | 8.1 | Draft | ✗ | ✗ |

**Release Status**: 0 of 1 plans committed
**Blocking Items**: Plan 011 pending Critic review
**Release Notes**: (pending)
**Architecture**: Provenance-first design per [011-memory-crud-architecture-findings.md](architecture/011-memory-crud-architecture-findings.md)

### v0.7.0 Release Summary
| Plan ID | Title | UAT Status | Committed | Released |
|---------|-------|------------|----------|----------|
| 007 | Persistent Vector Store | ✅ Approved | ✅ Yes | ✅ 2025-12-25 |

**Release Status**: ✅ RELEASED
**Blocking Items**: None
**Release Notes**:
- SQLite-backed persistent vector store for file-based databases
- Embeddings persist across application restarts without re-Cognify()
- Automatic mode selection (SQLite for persistent DBPath, MemoryVectorStore for :memory:)
- Direct-query linear scan search (acceptable for <10K nodes)
- Test coverage: 87.1% overall

### v0.7.1 Release - Edge ID Correctness
**Target Date**: 2025-12-25
**Actual Release Date**: 2025-12-25
**Status**: Released ✅
**Strategic Goal**: Fix edge ID derivation bug to ensure graph traversal correctness

| Plan ID | Title | Epic | Status | Committed | Released |
|---------|-------|------|--------|----------|----------|
| 008 | Edge ID Correctness Fix | 7.3 | Delivered | ✅ Yes | ✅ 2025-12-25 |

**Release Status**: ✅ RELEASED
**Blocking Items**: None
**Release Notes**:
- Fixed edge endpoint ID generation to include entity types
- Edges now correctly reference node IDs derived from (name, type) pairs
- Added case-insensitive and whitespace-normalized entity name matching
- Ambiguous entity detection with EdgesSkipped tracking
- 6 new unit tests validating edge ID correctness
- Integration test confirms edge-node connectivity
- Improved relation extraction robustness (filtering instead of failing on invalid triplets)
- Test coverage: 86.9% overall

### v0.8.0 Release - Efficiency
**Target Date**: 2025-12-25
**Actual Release Date**: 2025-12-25
**Status**: Released ✅
**Strategic Goal**: Reduce processing costs for updates

| Plan ID | Title | Epic | Status | Committed | Released |
|---------|-------|------|--------|----------|----------|
| 009 | Incremental Cognify | 7.4 | Delivered | ✅ Yes | ✅ 2025-12-25 |

**Release Status**: ✅ RELEASED
**Blocking Items**: None
**Release Notes**:
- Document-level deduplication via SHA-256 content hash
- SkipProcessed defaults to true (incremental by default)
- Force option to override caching and reprocess all documents
- New `processed_documents` SQLite table for tracking
- DocumentTracker interface implemented by SQLiteGraphStore
- CognifyResult reports DocumentsSkipped count
- Performance: ~0ms for cached documents vs 5-10s with LLM
- Backward compatible (incremental mode can be disabled)
- Test coverage: 84.9% (pkg/gognee), 85.5% (pkg/store)

### v0.9.0 Release Summary
| Plan ID | Title | UAT Status | Committed | Released |
|---------|-------|------------|----------|----------|
| 010 | Memory Decay / Forgetting | ✅ Approved | ✅ Yes | ✅ 2025-12-25 |

**Release Status**: ✅ RELEASED
**Blocking Items**: None
**Release Notes**:
- Time-based memory decay system with exponential scoring
- Explicit Prune() API with DryRun and cascade deletion
- Access reinforcement via automatic timestamp tracking
- Schema migration adds last_accessed_at and access_count columns
- DecayingSearcher decorator pattern (no interface changes)
- Backward compatible (decay OFF by default)
- Test coverage: 87.1% overall

### v0.6.0 Release Summary
| Plan ID | Title | UAT Status | Committed | Released |
|---------|-------|------------|----------|----------|
| 006 | Phase 6 Integration | ✅ Approved | ✅ Yes | ✅ 2025-12-24 |

**Release Status**: ✅ RELEASED
**Blocking Items**: None
**Known Limitations**:
- In-memory vector store (embeddings not persisted across restarts)
- Edge ID derivation uses empty type (may affect graph traversal)

### Previous Releases
| Version | Date | Plans Included | Status | Notes |
|---------|------|----------------|--------|-------|
| v1.2.0 | 2026-01-15 | 017, 018 | Released | Vector search optimization + Always-on observability (BREAKING: CGO required) |
| v1.1.0 | 2026-01-14 | 016 | Released | Prometheus metrics infrastructure |
| v0.9.0 | 2025-12-25 | 010 (Memory Decay/Forgetting) | Released | Time-based decay + Prune() API |
| v0.8.0 | 2025-12-25 | 009 (Incremental Cognify) | Released | Document-level deduplication |
| v0.7.1 | 2025-12-25 | 008 (Edge ID Correctness) | Released | Fixed edge endpoint ID derivation |
| v0.7.0 | 2025-12-25 | 007 (Persistent Vector Store) | Released | SQLite-backed embeddings persistence |
| v0.6.0 | 2025-12-24 | 006 (Phase 6 Integration) | Released | MVP complete - unified API |
| v0.5.0 | 2025-12-24 | 005 (Phase 5 Search) | Released | Hybrid search implementation |
| v0.4.0 | 2025-12-24 | 004 (Phase 4 Storage) | Released | SQLite graph + in-memory vector store |
| v0.3.0 | 2025-12-24 | 003 (Phase 3 Relations) | Released | Relationship extraction via LLM |
| v0.2.0 | 2025-12-24 | 002 (Phase 2 Entities) | Released | Entity extraction via LLM |
| v0.1.0 | 2025-12-24 | 001 (Phase 1 Foundation) | Released | Chunking + embeddings |

---

## Release v1.0.0 - Memory Management
**Target Date**: TBD
**Status**: Planning
**Strategic Goal**: Enable first-class memory CRUD operations for user-facing memory management

### Epic 8.1: First-Class Memory CRUD
**Priority**: P0
**Status**: Planned

**User Story**:
As a developer building a memory browser UI (like Glowbabe),
I want to store, list, retrieve, update, and delete memories as first-class entities with stable IDs,
So that users can manage their AI assistant's memory through a structured interface.

**Business Value**:
- **Unblocks Glowbabe Epic 6.1**: Glowbabe cannot build a Memory Browser without this capability
- **Completes the memory lifecycle**: Current Add→Cognify→Search is one-way; true persistence requires CRUD
- **Preserves structured data**: Topic, context, decisions, rationale, metadata survive round-trips
- **Enables user agency**: Users can audit, edit, and delete what the AI "remembers"
- **Production-ready memory system**: Moves gognee from "knowledge graph library" to "complete memory solution"

**Dependencies**:
- v0.9.0 complete (all post-MVP enhancements delivered)
- Requires data model evolution (new MemoryRecord table)
- Requires node/edge/vector cascade deletion logic

**Acceptance Criteria** (outcome-focused):
- [ ] **First-class MemoryRecord**: Persisted table with id, topic, context, decisions[], rationale[], metadata, timestamps, version, doc_hash
- [ ] **AddMemory API**: Store memory record → chunk → embed → cognify; return stable memory_id
- [ ] **ListMemories API**: Paginated list with offset/limit; optional topic search filter
- [ ] **GetMemory API**: Retrieve full memory payload by ID including linked node/vector IDs
- [ ] **UpdateMemory API**: Re-chunk, re-embed, re-cognify on content change; atomic node/vector replacement
- [ ] **DeleteMemory API**: Remove memory record + cascade delete linked nodes/edges/vectors
- [ ] **Data linkage**: Nodes/edges/vectors track memory_id for cascade operations
- [ ] **Search surfaces memory_id**: Results include memory_id so callers can link to browser
- [ ] **Backward compatibility**: Existing Add/Cognify/Search API continues to work
- [ ] **Migration path**: Optional hydration of legacy graph nodes to v1 MemoryRecords
- [ ] **Test coverage ≥80%** for new code

**Constraints**:
- Library-only (no CLI)
- No new dependencies beyond SQLite
- Atomic operations (update/delete must be transactional)
- decisions/rationale serialized as JSON arrays (versioned schema)

**Resolved Questions**:
1. ✅ **ListMemories**: Pagination only (offset/limit); full-text search deferred to future release
2. ✅ **UpdateMemory**: Partial update semantics (patch); backend re-embeds/re-cognifies on content change regardless

**Open Questions** (to resolve during Architect/UAT):
1. How to handle orphan nodes/edges after memory deletion (nodes shared across memories)?

**Status Notes**:
- 2026-01-02: Epic created based on Glowbabe feature request (docs/requests/glowbabe-request.md)
- 2026-01-02: Identified as P0 - blocks primary consumer (Glowbabe Memory Browser)
- 2026-01-02: Resolved Q1 (pagination only) and Q2 (partial updates); Q3 deferred to Architect/UAT

---

## Backlog / Future Consideration

### Epic 7.1: Persistent Vector Store (Post-MVP)
**Priority**: P1
**Status**: Critic Approved (Plan 007)
**Target Release**: v0.7.0

**User Story**:
As a developer deploying gognee in production,
I want vector embeddings to persist across application restarts,
So that I don't need to re-run Cognify() every time my application starts.

**Business Value**:
- Reduces startup time for production applications
- Eliminates redundant LLM API calls on restart
- Completes full persistence story (currently nodes/edges persist, embeddings don't)

**Dependencies**:
- v0.6.0 MVP complete

**Technical Approach (TBD)**:
- SQLite-backed vector store implementation
- Serialize embeddings as BLOB in nodes table or separate embeddings table
- Migration path from in-memory to persistent store

---

### Epic 7.2: Multiple LLM Provider Support (Post-MVP)
**Priority**: P2
**Status**: Deferred (user decision)

**User Story**:
As a developer with diverse infrastructure,
I want to use different LLM providers (Anthropic, Ollama, local models),
So that I'm not locked into OpenAI and can optimize for cost/performance.

**Business Value**:
- Reduces vendor lock-in
- Enables cost optimization (cheaper/free alternatives)
- Supports air-gapped/local deployments

**Dependencies**:
- v0.6.0 MVP complete

---

### Epic 7.3: Edge ID Correctness Fix (Post-MVP)
**Priority**: P2
**Status**: Delivered (Plan 008) ✅
**Target Release**: v0.7.1

**User Story**:
As a developer relying on graph traversal,
I want edges to correctly reference node IDs including entity types,
So that graph queries return accurate relationship paths.

**Business Value**:
- Fixes correctness issue identified in QA (Finding 3)
- Improves reliability of graph traversal search
- Enables more sophisticated graph queries
- Ensures graph traversal returns valid node endpoints

**Dependencies**:
- v0.6.0 MVP complete

**Technical Implementation** ✅:
- Maps triplet endpoints (Subject/Object) to entity types from extracted entities
- Generates edge endpoint IDs using correct (name, type) pairs
- Case-insensitive + whitespace-normalized entity name matching
- Detects and skips ambiguous entity references (multiple types for same name)
- Comprehensive validation tests for edge-node ID consistency

**Status Notes**:
- 2025-12-25: Implementation complete with edge ID determinism fix
- 2025-12-25: QA Complete - 86.9% coverage, all unit + integration tests pass
- 2025-12-25: UAT Complete - edge connectivity verified, graph queries validated
- 2025-12-25: Retrospective closed - v0.7.1 released

---

### Epic 7.4: Incremental Cognify (Post-MVP)
**Priority**: P2
**Status**: Delivered ✅
**Target Release**: v0.8.0
**Actual Release**: v0.8.0 (2025-12-25)

**User Story**:
As a developer with large document corpora,
I want to process only new/changed documents,
So that I can update my knowledge graph efficiently without reprocessing everything.

**Business Value**:
- Reduces processing time for updates (~0ms for cached vs 5-10s per doc with LLM)
- Reduces LLM API costs for incremental updates (zero API calls for cached documents)
- Enables continuous knowledge graph updates in production environments

**Dependencies**:
- v0.6.0 MVP complete

**Acceptance Criteria** (outcome-focused):
- ✅ Document identity based on SHA-256 hash of text content
- ✅ SkipProcessed defaults to true (incremental by default)
- ✅ Force option reprocesses all documents regardless of cache
- ✅ DocumentTracker interface separate from GraphStore
- ✅ CognifyResult reports DocumentsSkipped count
- ✅ Tracking persists across restarts (file DB mode)
- ✅ :memory: mode limitation documented
- ✅ Test coverage ≥80% for new code
- ✅ Backward compatible (opt-out via Force or SkipProcessed=false)

**Status Notes**:
- 2025-12-24: Plan 009 created and approved by critic
- 2025-12-25: Implementation complete - all milestones delivered
- 2025-12-25: QA Complete - 84.9% (pkg/gognee), 85.5% (pkg/store) coverage
- 2025-12-25: UAT Complete - value delivery validated, approved for release
- 2025-12-25: Retrospective closed - v0.8.0 released

**Delivered Artifacts**:
- DocumentTracker interface (pkg/store/tracker.go)
- processed_documents SQLite table + index
- Incremental Cognify logic with hash checking
- CognifyOptions: SkipProcessed, Force fields
- CognifyResult: DocumentsSkipped field
- computeDocumentHash() helper function
- 6 Plan 009-tagged unit tests (DocumentTracker CRUD + incremental behavior)
- README Incremental Cognify section
- CHANGELOG v0.8.0 entry

---

### Epic 7.5: Memory Decay / Forgetting (Post-MVP)
**Priority**: P3
**Status**: Delivered
**Target Release**: v0.9.0
**Actual Release**: v0.9.0 (2025-12-25)

**User Story**:
As a developer building a long-lived AI assistant,
I want old/stale information to decay or be forgotten,
So that the knowledge graph stays relevant and doesn't grow unbounded.

**Business Value**:
- Prevents unbounded knowledge graph growth
- Improves relevance of search results (recent info ranks higher)
- Mimics human-like memory behavior

**Dependencies**:
- v0.6.0 MVP complete

**Acceptance Criteria** (outcome-focused):
- ✅ Configurable decay parameters (DecayEnabled, DecayHalfLifeDays, DecayBasis)
- ✅ Exponential decay formula reduces search scores for old nodes
- ✅ Access reinforcement: frequently searched nodes resist decay
- ✅ Explicit Prune() API for permanent node deletion
- ✅ DryRun mode for safe pruning preview
- ✅ Cascade deletion: edges removed when endpoints pruned
- ✅ Schema migration adds timestamp tracking columns
- ✅ Backward compatible (decay OFF by default)
- ✅ Test coverage ≥80% for new code

**Status Notes**:
- 2025-12-24: Plan 010 created and approved by critic
- 2025-12-25: Implementation complete - all 10 milestones delivered
- 2025-12-25: QA Complete - 87.1% coverage, all 160+ tests pass, GetAllNodes bug fixed
- 2025-12-25: UAT Complete - 100% value delivery, approved for release
- 2025-12-25: Retrospective closed - v0.9.0 released

**Delivered Artifacts**:
- DecayingSearcher decorator (pkg/search)
- calculateDecay() exponential formula (pkg/gognee)
- Prune() API with PruneOptions and PruneResult
- Schema migration (last_accessed_at, access_count columns)
- UpdateAccessTime() batch operations (pkg/store)
- GetAllNodes(), DeleteNode(), DeleteEdge() APIs
- 27+ new unit tests + 2 integration tests
- README Memory Decay section
- CHANGELOG v0.9.0 entry

---

### Epic 7.6: Graph Visualization (Post-MVP)
**Priority**: P3
**Status**: Deferred (user decision)

**User Story**:
As a developer debugging knowledge graph issues,
I want to visualize the graph structure,
So that I can understand entity relationships and diagnose extraction problems.

**Business Value**:
- Improves developer experience
- Facilitates debugging and QA
- Provides transparency into knowledge graph structure

**Dependencies**:
- v0.6.0 MVP complete

---

### Epic 9.1: Intelligent Memory Lifecycle (Post-v1.0.0)
**Priority**: P1
**Status**: Planned
**Target Release**: v1.1.0 or v1.2.0

**User Story**:
As a developer building a long-lived AI assistant,
I want memories to be thinned based on usage patterns, explicit supersession, and semantic redundancy—not just calendar time,
So that the knowledge graph remains relevant, bounded, and preserves truly important information regardless of age.

**Business Value**:
- **Preserves foundational truths**: Original architectural decisions don't decay just because they're old
- **Reduces noise**: Redundant/superseded memories are consolidated or deprecated
- **Respects user intent**: Explicit supersession chains preserve decision provenance
- **Scales sustainably**: Frequency-based scoring keeps high-value memories alive
- **Enables diverse retention**: Different memory types (permanent, ephemeral, session) have appropriate lifespans

**Dependencies**:
- v1.0.0 complete (First-Class Memory CRUD)
- Existing v0.9.0 decay infrastructure (`access_count`, `last_accessed_at`)

**Design Principles**:
1. **Don't delete, deprecate**: Supersession > deletion. History has value.
2. **Access patterns reveal value**: What users retrieve is what matters.
3. **Semantic similarity signals redundancy**: Consolidation > accumulation.
4. **Different facts have different lifespans**: One policy doesn't fit all.
5. **Give users control**: Pinning, policies, explicit supersession.
6. **Decay is for irrelevance, not age**: Time is a weak proxy; behavior is stronger.

**Sub-Epics / Features**:

#### 9.1.1: Access Frequency Scoring
**Priority**: P1 (build on existing `access_count` column)
**Effort**: Small

Memories that are frequently retrieved are demonstrably useful. Evolve decay formula:
```
relevance_score = base_score × heat_multiplier
heat_multiplier = min(1.0, log(access_count + 1) / log(reference_count))
```

Schema additions:
- `access_velocity REAL` - rolling window retrieval rate
- `last_30d_hits INTEGER` - recent access count

**Acceptance Criteria**:
- [ ] High-hit memories resist decay regardless of age
- [ ] 6-month-old memory retrieved 50× ranks higher than 1-week-old memory never accessed
- [ ] Access tracking adds minimal write overhead

---

#### 9.1.2: Explicit Supersession Chains
**Priority**: P1 (critical for decision history)
**Effort**: Medium

When storing a new memory, explicitly declare which prior memories it supersedes. Superseded memories become prunable; the chain preserves provenance.

Schema additions:
```sql
CREATE TABLE memory_supersession (
  id TEXT PRIMARY KEY,
  superseding_id TEXT NOT NULL,
  superseded_id TEXT NOT NULL,
  reason TEXT,
  created_at DATETIME
);
ALTER TABLE memories ADD COLUMN status TEXT DEFAULT 'Active';
-- Values: Active, Superseded, Archived, Consolidated
```

API surface:
```go
type AddMemoryOptions struct {
  Supersedes []string  // IDs of memories this one replaces
}
```

**Acceptance Criteria**:
- [ ] AddMemory accepts `Supersedes` option
- [ ] Superseded memories marked with status and link to successor
- [ ] Prune respects supersession (only prune Superseded with no active dependents)
- [ ] Supersession chain queryable for provenance ("why was this decided?")

---

#### 9.1.3: Retention Policies
**Priority**: P2 (different memory types)
**Effort**: Medium

Different types of memories have different natural lifespans. Configuration decisions may be permanent; debugging sessions are ephemeral.

Schema additions:
```sql
ALTER TABLE memories ADD COLUMN retention_policy TEXT DEFAULT 'standard';
ALTER TABLE memories ADD COLUMN retention_until DATETIME;
```

Policy definitions:
| Policy | Half-life | Prunable | Use Case |
|--------|-----------|----------|----------|
| `permanent` | ∞ | Never | Architectural decisions, core facts |
| `decision` | 365 days | After supersession | Important choices that may evolve |
| `standard` | 90 days | By decay | Default |
| `ephemeral` | 7 days | By decay | Debug sessions, temp context |
| `session` | 1 day | Always | Single-session scratch |

**Acceptance Criteria**:
- [ ] AddMemory accepts `RetentionPolicy` option
- [ ] Decay formula respects per-memory half-life
- [ ] `permanent` memories exempt from Prune
- [ ] ListMemories filterable by policy

---

#### 9.1.4: Semantic Consolidation
**Priority**: P2 (reduce redundancy)
**Effort**: Large

Periodically identify memories with high semantic overlap and offer to merge/consolidate them into a single, richer memory.

Algorithm:
1. Compute pairwise similarity for Active memories
2. Cluster memories with similarity > threshold (0.85)
3. Use LLM to synthesize cluster into consolidated memory
4. Mark originals as `Consolidated` → prunable after retention period

API surface:
```go
func (g *Gognee) SuggestConsolidations(ctx context.Context, opts ConsolidationOptions) ([]ConsolidationSuggestion, error)
func (g *Gognee) ApplyConsolidation(ctx context.Context, suggestionID string) error
```

**Acceptance Criteria**:
- [ ] Consolidation suggestions generated without user intervention
- [ ] User approves before any data modification
- [ ] Consolidated memory links back to originals
- [ ] Originals marked Consolidated, not deleted immediately

---

#### 9.1.5: User-Defined Anchors (Pin/Protect)
**Priority**: P2 (simple, high value)
**Effort**: Small

Let users explicitly pin memories they know are important. Pinned memories exempt from automatic decay/prune.

Schema additions:
```sql
ALTER TABLE memories ADD COLUMN pinned BOOLEAN DEFAULT FALSE;
ALTER TABLE memories ADD COLUMN pinned_at DATETIME;
ALTER TABLE memories ADD COLUMN pinned_reason TEXT;
```

**Acceptance Criteria**:
- [ ] PinMemory/UnpinMemory APIs
- [ ] Pinned memories exempt from Prune
- [ ] ListMemories filterable by pinned status
- [ ] Optional: pin limits or pin decay to prevent "pin everything"

---

#### 9.1.6: Conflict Detection
**Priority**: P3 (advanced feature)
**Effort**: Medium

When memories contradict each other, flag them for resolution rather than letting both decay naturally.

API surface:
```go
func (g *Gognee) DetectConflicts(ctx context.Context) ([]MemoryConflict, error)
func (g *Gognee) ResolveConflict(ctx context.Context, conflictID string, resolution ConflictResolution) error
```

**Acceptance Criteria**:
- [ ] Conflicts detected based on semantic similarity + opposing conclusions
- [ ] Conflicts surfaced to user for resolution
- [ ] Resolution creates supersession link

---

#### 9.1.7: Provenance-Weighted Scoring
**Priority**: P3 (graph-based importance)
**Effort**: Medium

Memories linked to important entities inherit their importance. A memory about a core architectural decision stays valuable because the entity is structurally central.

Formula:
```
memory_importance = base_importance + sum(linked_entity_importances × edge_weight)
```

**Acceptance Criteria**:
- [ ] Node importance computed from reference count, edge count, manual boost
- [ ] Memory decay modified by linked node importance
- [ ] Core entities (database, API, etc.) confer protection to linked memories

---

**Open Questions**:
1. Should consolidation be automatic or always user-approved? → Deferred to v1.2.0 (9.1.4)
2. How to handle supersession chains when the superseding memory is later deleted? → RESOLVED: ON DELETE SET NULL preserves chain history
3. Should pin limits exist to prevent "pin everything"? → RESOLVED: No limit in v1.1.0; add optional config in v1.2.0 if needed
4. How deep should provenance weighting traverse the graph? → Deferred to v1.2.0 (9.1.7)

**Status Notes**:
- 2026-01-27: Epic created based on strategic discussion about memory thinning beyond calendar-time decay
- 2026-01-27: Plan 021 drafted with 13 milestones covering sub-epics 9.1.1, 9.1.2, 9.1.3, 9.1.5; pending Critic review

---

### Epic 10.1: Memory Decay Observability (Post-MVP)
**Priority**: P1
**Status**: Planned
**Target Release**: v1.6.0

**User Story**:
As a developer running a long-lived AI assistant with gognee,
I want structured logging for the memory decay subsystem,
So that I can verify decay/prune operations are working correctly and debug retention issues without guessing.

**Business Value**:
- **Operational visibility**: Currently decay/prune runs silently; consumers have no way to confirm it's working
- **Debug capability**: When memories unexpectedly disappear or persist, logs provide audit trail
- **Tuning feedback**: Logs reveal whether decay parameters (half-life, thresholds) are appropriate for workload
- **Production confidence**: Long-running AI assistants need observability to trust memory management

**Dependencies**:
- v1.5.0 complete (Intelligent Memory Lifecycle with access frequency, supersession, retention policies)
- Go `log/slog` available (standard library since Go 1.21)

**Technical Approach**:
- Injectable `slog.Logger` interface (consumers provide their logger; default to no-op)
- Three logging domains:
  1. **Configuration** (INFO): Log decay settings at startup (DecayEnabled, DecayHalfLifeDays, DecayBasis, etc.)
  2. **Evaluation** (DEBUG): Log per-node/memory decay score calculations during Search
  3. **Actions** (INFO): Log prune operations (nodes/edges/memories evaluated, pruned, skipped)

**Acceptance Criteria** (outcome-focused):
- [ ] **Injectable logger**: `Config.Logger` field of type `*slog.Logger`; nil means no logging
- [ ] **Startup logging**: When decay enabled, log config values (DecayEnabled, DecayHalfLifeDays, DecayBasis, AccessFrequencyEnabled, ReferenceAccessCount) at INFO level
- [ ] **DecayingSearcher logging**: At DEBUG level, log per-result decay evaluation (nodeID, age, basis, score, frequency_multiplier, final_score)
- [ ] **Prune startup logging**: At INFO level, log prune options (MaxAgeDays, MinDecayScore, DryRun, PruneSuperseded, SupersededAgeDays)
- [ ] **Prune evaluation logging**: At DEBUG level, log each node/memory evaluation (id, status, retention_policy, age, score, decision)
- [ ] **Prune summary logging**: At INFO level, log prune outcome (memories_evaluated, nodes_evaluated, nodes_pruned, edges_pruned, superseded_memories_pruned)
- [ ] **Structured fields**: Use slog Attrs for all logged values (not string interpolation)
- [ ] **Zero overhead when disabled**: When Logger is nil, no logging code executes (check-then-log pattern)
- [ ] **Test coverage ≥80%** for logging paths
- [ ] **Documentation**: README section on enabling decay logging with slog example

**Constraints**:
- Use Go standard library `log/slog` only (no third-party logging frameworks)
- DEBUG logs must be opt-in (not emitted at default log level)
- Logging must not change functional behavior (pure observability)
- No new dependencies

**Logging Schema Examples**:

```go
// Startup (INFO)
logger.Info("decay enabled",
    slog.Bool("enabled", true),
    slog.Int("half_life_days", 30),
    slog.String("basis", "access"),
    slog.Bool("access_frequency_enabled", true),
    slog.Int("reference_access_count", 10))

// Decay evaluation (DEBUG)
logger.Debug("decay score calculated",
    slog.String("node_id", nodeID),
    slog.Duration("age", age),
    slog.Float64("decay_score", decayScore),
    slog.Float64("frequency_multiplier", heatMultiplier),
    slog.Float64("final_score", finalScore))

// Prune summary (INFO)
logger.Info("prune completed",
    slog.Int("memories_evaluated", result.MemoriesEvaluated),
    slog.Int("nodes_evaluated", result.NodesEvaluated),
    slog.Int("nodes_pruned", result.NodesPruned),
    slog.Int("edges_pruned", result.EdgesPruned),
    slog.Bool("dry_run", opts.DryRun))
```

**Status Notes**:
- 2026-02-18: Epic created based on user request for decay subsystem observability

---

## Strategic Alignment

**Current Phase**: Post-MVP Enhancement Complete → Memory CRUD (v1.0.0) → Intelligent Memory Lifecycle (v1.1.0+)

**Active Priority**:
Epic 8.1 (First-Class Memory CRUD) is P0 because:
1. **Primary consumer blocked**: Glowbabe's Memory Browser (Epic 6.1) cannot proceed without this
2. **Architecture evolution**: Current design treats text as ephemeral input; memory management requires treating it as a persistent entity
3. **Value proposition gap**: "Persistent memory" isn't truly persistent if users can't manage it

**Upcoming Priority**:
Epic 9.1 (Intelligent Memory Lifecycle) is P1 because:
1. **Calendar-time decay is insufficient**: Original truths don't become less true with age
2. **Access patterns reveal value**: High-retrieval memories are demonstrably useful
3. **Supersession preserves history**: "Memory B replaces A" is better than silent decay
4. **Different memory types need different lifespans**: Architectural decisions ≠ debug sessions

**Next Phase Options**:
1. ✅ **Production Hardening**: Address known limitations (persistent vector store, edge ID fix) - DELIVERED (v0.7.0-v0.9.0)
2. 🚀 **Memory CRUD**: First-class memory entities with full lifecycle management - IN PROGRESS
3. 🔮 **Intelligent Memory Lifecycle**: Beyond time-decay to usage-based, semantic, and policy-driven retention - PLANNED (v1.1.0+)
4. **Provider Diversification**: Add Anthropic/Ollama support for vendor flexibility - DEFERRED

**Recommended Next Steps**:
1. Complete Plan 011 for Epic 8.1 implementation (v1.0.0)
2. Begin architecture review for Epic 9.1 sub-epics (access frequency, supersession chains)
3. Coordinate with Glowbabe on supersession API surface (how should glowbabe tool expose `Supersedes`?)
4. Target v1.1.0 for P1 sub-epics (9.1.1, 9.1.2), v1.2.0 for P2 sub-epics (9.1.3, 9.1.4, 9.1.5)

**Success Metrics (MVP)**:
- ✅ Can add text and build knowledge graph
- ✅ Can search and retrieve relevant context
- ✅ Single binary, no external dependencies (beyond SQLite)
- ✅ Works on macOS, Linux, Windows (Go cross-platform)
- ⚠️ < 5MB binary size (not measured, likely met)
- ✅ < 100ms search latency for small graphs (integration tests show reasonable performance)

**Success Metrics (v1.0.0 - Memory CRUD)**:
- [ ] Can add, list, get, update, delete memories as first-class entities
- [ ] Memories preserve structured fields (topic, context, decisions, rationale, metadata)
- [ ] Glowbabe can build functional Memory Browser using new APIs
- [ ] Deletion cascades correctly (memory → nodes → edges → vectors)
- [ ] Update re-indexes atomically without orphaned data

**Success Metrics (v1.1.0+ - Intelligent Memory Lifecycle)**:
- [ ] High-access memories resist decay regardless of age
- [ ] Supersession chains preserve decision provenance
- [ ] Retention policies allow permanent/standard/ephemeral memory types
- [ ] Semantic consolidation reduces redundant memories
- [ ] Users can pin important memories

---

## Lessons Learned (v0.6.0 Retrospective)

**What Went Well**:
- 6-week MVP delivery met ROADMAP estimate (6-8 weeks)
- TDD approach caught issues early (LLM JSON parsing bug found in integration tests)
- Interface-driven design enabled easy testing with mocks
- Deterministic node IDs solved duplicate entity problem elegantly
- Best-effort Cognify semantics balanced resilience with error visibility

**What Could Be Improved**:
- Edge ID derivation issue (QA Finding 3) should have been caught earlier in design phase
- Integration test setup complexity (API key management, temp files)
- In-memory vector store limitation means embeddings lost on restart

**Action Items for Next Phase**:
- Front-load architecture review for ID generation strategies
- Document testing patterns for future contributors
- Consider SQLite vector store as P1 for production readiness

---

## Notes

This product roadmap tracks strategic direction and value delivery for gognee. Technical implementation details live in:
- `agent-output/planning/` - Detailed implementation plans
- `agent-output/qa/` - Quality assurance reports
- `agent-output/uat/` - User acceptance testing
- `ROADMAP.md` - Technical roadmap (phases, deliverables, specs)

**Roadmap Maintenance**:
- Update after each plan UAT approval
- Update Active Release Tracker when plans are targeted/committed/released
- Add new epics based on user feedback and strategic goals
- Review backlog priorities quarterly or when new strategic goals emerge

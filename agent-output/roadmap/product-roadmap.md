# gognee - Product Roadmap

**Last Updated**: 2025-12-25
**Roadmap Owner**: roadmap agent
**Strategic Vision**: Build a Go library package that provides AI assistants with persistent memory across conversations, knowledge graph for understanding relationships, and hybrid search combining graph traversal and vector similarity. Pure library design (no CLI), embeddable in Go projects like Glowbabe.

## Change Log
| Date & Time | Change | Rationale |
|-------------|--------|-----------|| 2025-12-25 | Marked Plan 008 (Edge ID Correctness) as Delivered; v0.7.1 released | Retrospective closed for Plan 008; QA+UAT verified edge endpoint IDs match node IDs correctly |
| 2025-12-25 | Marked Plan 010 (Memory Decay/Forgetting) as Delivered; v0.9.0 released | Retrospective closed for Plan 010; Epic 7.5 complete with full UAT approval || 2025-12-25 | Marked Plans 007-010 as Critic Approved in Active Release Tracker | Plans revised per critique; critiques updated to RESOLVED/APPROVED || 2025-12-25 | Marked Plan 008 as QA Complete | QA executed: unit tests + coverage verified; integration suite warning documented |
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

**Current Working Release**: v0.7.1 (Edge ID Correctness)

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
**Target Date**: TBD
**Status**: Planning
**Strategic Goal**: Reduce processing costs for updates

| Plan ID | Title | Epic | Status | Target |
|---------|-------|------|--------|--------|
| 009 | Incremental Cognify | 7.4 | Critic Approved | v0.8.0 |

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
| v0.6.0 | 2025-12-24 | 006 (Phase 6 Integration) | Released | MVP complete - unified API |
| v0.5.0 | 2025-12-24 | 005 (Phase 5 Search) | Released | Hybrid search implementation |
| v0.4.0 | 2025-12-24 | 004 (Phase 4 Storage) | Released | SQLite graph + in-memory vector store |
| v0.3.0 | 2025-12-24 | 003 (Phase 3 Relations) | Released | Relationship extraction via LLM |
| v0.2.0 | 2025-12-24 | 002 (Phase 2 Entities) | Released | Entity extraction via LLM |
| v0.1.0 | 2025-12-24 | 001 (Phase 1 Foundation) | Released | Chunking + embeddings |

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
**Status**: Critic Approved (Plan 009)
**Target Release**: v0.8.0

**User Story**:
As a developer with large document corpora,
I want to process only new/changed documents,
So that I can update my knowledge graph efficiently without reprocessing everything.

**Business Value**:
- Reduces processing time for updates
- Reduces LLM API costs for incremental updates
- Enables continuous knowledge graph updates

**Dependencies**:
- v0.6.0 MVP complete

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

## Strategic Alignment

**Current Phase**: MVP Complete (6-week delivery from inception to v0.6.0)

**Next Phase Options**:
1. **Production Hardening**: Address known limitations (persistent vector store, edge ID fix)
2. **Glowbabe Integration**: Integrate gognee into Glowbabe project (primary use case)
3. **Provider Diversification**: Add Anthropic/Ollama support for vendor flexibility

**Recommended Next Steps**:
1. Integrate gognee into Glowbabe project (validate real-world usage)
2. Gather production feedback from Glowbabe integration
3. Prioritize post-MVP epics based on Glowbabe pain points

**Success Metrics (MVP)**:
- ✅ Can add text and build knowledge graph
- ✅ Can search and retrieve relevant context
- ✅ Single binary, no external dependencies (beyond SQLite)
- ✅ Works on macOS, Linux, Windows (Go cross-platform)
- ⚠️ < 5MB binary size (not measured, likely met)
- ✅ < 100ms search latency for small graphs (integration tests show reasonable performance)

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

# Plan 021: Intelligent Memory Lifecycle — Critique

- **Artifact**: agent-output/planning/021-intelligent-memory-lifecycle-plan.md
- **Analysis Scope**: Pre-implementation review for clarity, completeness, and architectural alignment
- **Date**: 2026-01-27
- **Status**: APPROVED

## Changelog
| Date | Handoff / Request | Summary |
|------|-------------------|---------|
| 2026-01-27 | User request | Initial critique of Plan 021 (milestones 1-13)

## Value Statement Assessment
- Value statement is present, user-facing, and ties directly to the Master Product Objective (a long-lived AI assistant needs memory thinning that respects usage and provenance). The “So that” clause explains why the new lifecycle keeps the graph relevant.

## Overview
- Plan 021 breaks Epic 9.1 into 13 milestones, lists the included sub-epics, and documents dependencies/acceptance criteria. It covers schema migrations, API extensions, decay/prune modifications, retention policies, pinning, docs, and release housekeeping.
- The plan is concise, sticks to WHAT/WHY, and avoids implementation-level code (pseudocode blocks are marked and limited to type shapes).

## Architectural Alignment
- Schema changes extend the Memory CRUD tables and reuse the provenance-first architecture (memory nodes/edges, deterministic IDs) described in [011-memory-crud-architecture-findings.md](../architecture/011-memory-crud-architecture-findings.md). Decay enhancements build on the existing `DecayingSearcher`, and retention policies map cleanly to the memory-level abstraction.
- The new `memory_supersession` table and status field respect SQLite’s referential integrity expectations; transactions are highlighted for multi-step updates, mirroring the two-phase commit mindset from Plan 011.

## Scope Assessment
- Scope is focused on the P1 sub-epics (9.1.1/9.1.2) plus well-justified P2 work (retention policies, pinning). Semantic consolidation, conflict detection, and provenance-weighted scoring are explicitly deferred, preventing scope creep.
- Dependencies are spelled out per milestone; version target (v1.1.0) is documented in Milestone 13, aligning with the roadmap release plan.

## Technical Debt Risks
- Memory access tracking as described only increments counters during `GetMemory()` calls. Search results—the primary read path—are not mentioned, so access_frequency will never reflect actual usage unless search instrumentation is added.
- Access velocity computation strategy is unresolved. Without a defined cadence, the new column risks being stale, reducing its usefulness for ranking.
- Open question about how the supersession chain survives when the superseding memory is deleted remains unresolved, creating ambiguity for downstream API semantics.

## Findings
### 1: Search hits do not increment memory-level access counts
- **Severity**: HIGH
- **Status**: OPEN
- **Location**: [agent-output/planning/021-intelligent-memory-lifecycle-plan.md#L71-L95](agent-output/planning/021-intelligent-memory-lifecycle-plan.md#L71-L95)
- **Description**: Milestone 1 only calls `UpdateMemoryAccess` when `GetMemory()` runs, leaving `access_count` unchanged when a user retrieves context via search results or other cached views. Since access frequency is the central signal for scoring (Milestone 2), the plan lacks the instrumentation needed to capture the majority of retrievals.
- **Impact**: Frequencies will remain artificially low for everything but manual memory fetches, so the new decay formula will not differentiate frequently used memories from stale ones. The primary value statement (“memories thinned based on usage patterns”) cannot be delivered.
- **Recommendation**: Extend the plan around Milestone 1/2 to specify how search (and other read paths that return memory IDs) call `UpdateMemoryAccess` (ids returned by `SearchResult.MemoryIDs` should increment their respective counts in a batched write). Document the expected consistency model for concurrent updates.

## Questions
- **RESOLVED**: Search instrumentation now included in Milestone 1 (CRITICAL task 6).
- **RESOLVED**: Supersession chain behavior confirmed via bidirectional linking with CASCADE delete (Milestone 3).
- **RESOLVED**: `access_velocity` computation confirmed as real-time (Milestone 1).

## Risk Assessment
- **Risk Level**: Low. All critical findings resolved. Plan is now implementation-ready.

## Recommendations
1. ✅ All critical findings addressed in revised plan.
2. ✅ All open questions resolved and locked into the plan.
3. ✅ Plan is ready for implementation approval.

No outstanding blockers.

## Revision History
| Date | Changes | Status |
|------|---------|--------|
| 2026-01-27 | Plan revised: search instrumentation added, bidirectional supersession linking confirmed, real-time velocity resolved, all questions marked RESOLVED | APPROVED |
| 2026-01-27 | Initial critique created for Plan 021; Finding 1 marked HIGH severity; 2 open questions identified | OPEN |

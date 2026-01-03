# Plan 011: First-Class Memory CRUD — Critique

- **Artifact**: agent-output/planning/011-first-class-memory-crud-plan.md
- **Analysis Scope**: Plan review for clarity, completeness, architectural alignment (pre-implementation)
- **Date**: 2026-01-03
- **Status**: APPROVED

## Changelog
| Date | Handoff / Request | Summary |
|------|-------------------|--------|
| 2026-01-03 | User request | Initial critique of Plan 011 (milestones 1-13) |
| 2026-01-03 | Plan revised | All 5 findings addressed; plan approved for implementation |

## Value Statement Assessment
- Value statement is clear and user-facing (memory browser UI needs CRUD with stable IDs). Direct business benefit is explicit and aligns with Epic 8.1 objective.

## Overview
- Plan is milestone-structured with clear acceptance criteria per milestone. Scope covers schema, API, provenance, GC, search enrichment, testing, docs, and versioning.
- Status in plan remains "Draft" despite user indicating completion—clarify source of truth before implementation sign-off.

## Architectural Alignment
- Aligns with system architecture direction (provenance-first, deterministic node/edge IDs, SQLite substrate). Introduces MemoryRecord and provenance tables consistent with architecture doc problem statements.
- Transactional semantics are emphasized, matching architecture debt notes on CRUD atomicity.
- Potential drift: proposed GC deletes any node/edge not in provenance tables, which conflicts with legacy/legacy-mode data model in system architecture (nodes/edges may exist without provenance). Requires alignment decision.

## Scope Assessment
- Coverage is broad (schema through versioning). Milestones are ordered logically with dependencies. Assumptions allow breaking changes but acceptance criteria for migration claim no data loss—contradiction needs resolution.
- Conflates long-running cognify + remote LLM calls within a single transaction (Milestones 4/6) without timeout/locking guidance; may be risky for SQLite.

## Technical Debt Risks
- Legacy compatibility unresolved: GC rule could erase legacy nodes/edges lacking provenance, undermining backward compatibility claims.
- Transaction boundaries around network-bound cognify could hold write locks for extended periods, risking contention and failures.
- Duplicate detection relies on doc_hash but canonicalization rules unspecified (whitespace/ordering/metadata), leading to inconsistent dedup behavior.
- Update/delete semantics rely on deterministic IDs; lack of plan for collisions with existing legacy-derived graph artifacts.

## Findings
- **Critical — Legacy data erasure risk (RESOLVED)**: Plan revised. GC now only affects provenance-tracked nodes/edges. Legacy data (never in provenance tables) is exempt from deletion. GC Safety Rule documented in Milestone 7.
- **Critical — Conflicting migration stance (RESOLVED)**: Plan revised. Assumption 6 now states: legacy data preserved but not retroactively tracked; users who want CRUD support re-ingest via AddMemory. Milestone 1 acceptance updated to confirm legacy nodes/edges remain untouched.
- **Medium — Long transactions with remote LLM calls (RESOLVED)**: Plan revised. Milestones 4 and 6 now use two-phase model: short transaction for memory insert/update, LLM calls outside transaction, short transaction for node/edge upserts and provenance.
- **Medium — Dedup hash undefined (RESOLVED)**: Plan revised. Milestone 4 now specifies canonical JSON serialization (sorted keys: context, decisions, rationale, topic), trimmed whitespace, metadata excluded, SHA-256 of UTF-8 JSON.
- **Low — Performance of per-result provenance enrichment (RESOLVED)**: Plan revised. Milestone 8 now requires batched query (`SELECT ... WHERE node_id IN (...)`) with acceptance criterion verifying no N+1.

## Questions
1. ~~Should legacy graph data be preserved by default during GC?~~ **Resolved**: Yes. GC only affects provenance-tracked artifacts; legacy data exempt.
2. ~~What is the authoritative migration path?~~ **Resolved**: In-place, no-loss migration. Legacy data preserved; CRUD support requires re-ingestion via AddMemory.
3. ~~Do we want long-running cognify to hold DB transactions?~~ **Resolved**: No. Two-phase model adopted.
4. ~~What canonicalization rules define doc_hash?~~ **Resolved**: Canonical JSON (sorted keys), trimmed whitespace, metadata excluded, SHA-256.
5. ~~Can search provenance enrichment be batched?~~ **Resolved**: Yes. Batched `SELECT ... WHERE node_id IN (...)` query required.

## Risk Assessment
- Overall risk: **Low**. All critical and medium findings resolved. Plan ready for implementation.

## Recommendations
- All prior recommendations addressed in plan revision. No outstanding blockers.
- QA should ensure integration tests verify legacy data preservation after GC.
- Implementer should test two-phase model failure/retry paths.

## Revision History
- 2026-01-03: Initial critique created; findings marked OPEN.
- 2026-01-03: Plan revised per critique; all findings RESOLVED; status changed to APPROVED.
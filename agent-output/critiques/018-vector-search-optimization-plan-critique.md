# Plan 018 — Vector Search Optimization (sqlite-vec) — Critique

- **Artifact:** [agent-output/planning/018-vector-search-optimization-plan.md](agent-output/planning/018-vector-search-optimization-plan.md)
- **Date:** 2026-01-15
- **Status:** OPEN (Initial)
- **Analysis Inputs:** Roadmap, system architecture

## Changelog
| Date | Handoff/Request | Summary |
|------|-----------------|---------|
| 2026-01-15 | Critic review requested | Initial critique created |
| 2026-01-15 | User clarifications received | CGO-only; manual DB recreation; architecture doc update not requested |

## Value Statement/Context Assessment
- Value statement present and measurable (<500ms target) with user-centric outcome; aligns to performance epic. No deferral of core value identified.

## Overview
- Plan targets replacing linear vector search with sqlite-vec ANN via CGO driver swap and adds basic benchmarks. Scope includes schema changes and release updates.

## Architectural Alignment
- Architecture sets SQLite driver to modernc pure-Go and acknowledges linear-scan ceiling [agent-output/architecture/system-architecture.md#L87-L99]. Plan proposes mattn/cgo driver plus sqlite-vec without noting architectural update or downstream embedding implications [agent-output/planning/018-vector-search-optimization-plan.md#L51-L52]. Needs explicit reconciliation and architecture doc update path.

## Scope Assessment
- Scope is focused but contains contradictory boundaries on migration and fallback, creating ambiguity for implementer/QA. Success criteria hinge on CGO + vec0 but build/ops impacts not fully threaded to consumers.

## Technical Debt Risks
- CGO adoption may regress portability and complicate embedding in pure-Go consumers; fallback story unclear. Dual driver/fallback ambiguity risks maintenance debt.

## Findings

### F1: Contradictory stance on CGO vs pure-Go fallback
- **Severity:** HIGH
- **Status:** RESOLVED
- **Location:** [agent-output/planning/018-vector-search-optimization-plan.md#L56-L62], [agent-output/planning/018-vector-search-optimization-plan.md#L118-L123]
- **Description:** Out-of-scope explicitly rejects a pure-Go fallback, yet key constraints require providing a pure-Go fallback with deprecation warning. User clarified CGO-only, so the plan now needs edits to drop fallback language and align milestones/acceptance with CGO-only posture.
- **Impact:** Until plan is updated, implementer/QA cannot rely on a single build matrix or release messaging.
- **Recommendation:** Update plan to remove fallback references and state CGO-only across scope, constraints, success criteria, and release notes.
- **Resolution:** Plan revised 2026-01-15: fallback language removed; CGO-only stance enforced throughout scope, constraints, success criteria, and handoff notes.

### F2: Migration approach conflicts with scope
- **Severity:** HIGH
- **Status:** RESOLVED
- **Location:** [agent-output/planning/018-vector-search-optimization-plan.md#L56-L62], [agent-output/planning/018-vector-search-optimization-plan.md#L108-L115], [agent-output/planning/018-vector-search-optimization-plan.md#L118-L121]
- **Description:** Scope states no automatic migration (users re-Cognify), but migration strategy and key constraints define an automated schema-versioned migration with atomicity requirements. User chose manual migration (clear/recreate DB), so the plan must drop automated migration tasks/constraints.
- **Impact:** Until plan is revised, implementer/QA cannot align work and messaging (manual vs automated upgrade path).
- **Recommendation:** Remove migration strategy/atomicity constraints; explicitly direct users to rebuild DB via Cognify and capture in release notes/acceptance.
- **Resolution:** Plan revised 2026-01-15: migration strategy replaced with upgrade path (delete DB, re-Cognify); atomicity constraints removed; release notes updated.

### F3: Architectural divergence not gated
- **Severity:** HIGH
- **Status:** DEFERRED
- **Location:** [agent-output/planning/018-vector-search-optimization-plan.md#L51-L52], [agent-output/architecture/system-architecture.md#L87-L99]
- **Description:** Plan switches core dependency from modernc (pure Go) to mattn/cgo to support sqlite-vec but does not record required architecture update, compatibility analysis, or acceptance criteria for downstream embedders. User declined architecture doc updates for MVP discovery.
- **Impact:** Downstream embedders may face CGO build breakage; architecture SSOT will remain outdated, increasing future reconciliation cost.
- **Recommendation:** At minimum, note the divergence in plan/release notes and consider an architecture doc update post-MVP to restore SSOT accuracy.

## Questions
None open. Clarifications: CGO-only; no automatic migration (advise recreate DB); architecture doc update deferred for MVP.

## Risk Assessment
- Overall risk: Low. CGO/fallback stance and migration strategy resolved; architectural drift noted and deferred.

## Recommendations

- ~~Resolve F1/F2 via plan edits: enforce CGO-only stance throughout (remove fallback language) and drop automated migration/atomicity constraints in favor of "recreate DB then Cognify," with release messaging.~~ **DONE**
- Note architectural divergence in plan/release artifacts; consider a post-MVP architecture update to reconcile SSOT and CGO build expectations. **Captured in plan constraints; deferred per user.**

**Verdict:** Plan approved for implementation.

## Revision History
| Date | Changes Reviewed | Findings Addressed | Status |
|------|------------------|--------------------|--------|
| 2026-01-15 | Initial plan draft | None | OPEN |
| 2026-01-15 | User clarified CGO-only/manual migration | Findings acknowledged; F3 deferred per user | OPEN |
| 2026-01-15 | Plan revised per critique | F1, F2 resolved; F3 deferred | APPROVED |

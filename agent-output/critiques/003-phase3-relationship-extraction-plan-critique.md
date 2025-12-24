# Critique — Plan 003: Phase 3 Relationship Extraction

**Artifact:** [agent-output/planning/003-phase3-relationship-extraction-plan.md](../planning/003-phase3-relationship-extraction-plan.md)

**Date:** 2025-12-24

**Status:** Initial Review

---

## Changelog

| Date | Handoff | Request | Summary |
|------|---------|---------|---------|
| 2025-12-24 | Planner → Critic | Review for clarity, completeness, architectural alignment | Initial critique |
| 2025-12-24 | User → Critic | Confirm open questions | v0.3.0 confirmed; strict mode preferred over drop-on-unknown |

---

## Value Statement Assessment

**Rating:** ✅ PASS

The value statement is well-formed and follows the "As a…I want…so that…" structure:

> *As a developer embedding gognee into Glowbabe…I want gognee to extract relationships between previously extracted entities, so that the knowledge graph can represent meaningful edges (triplets) and later support graph traversal + hybrid search.*

The statement correctly identifies:
- The user (developer embedding gognee)
- The capability (relationship/triplet extraction)
- The downstream value (knowledge graph edges for future traversal/search)

This aligns with the ROADMAP Phase 3 goals and maintains the library-only positioning.

---

## Overview

Plan 003 proposes implementing relationship extraction as the third phase of gognee development. It builds on the existing Phase 2 entity extraction work and introduces:
- `Triplet` struct and `RelationExtractor` in `pkg/extraction`
- Linking triplets to known entities with drop-on-unknown behavior
- Case-insensitive entity matching
- Offline unit tests and optional gated integration tests

The plan follows the established patterns from Phase 2 and stays within ROADMAP scope.

---

## Architectural Alignment

**Rating:** ✅ ALIGNED

| Criterion | Assessment |
|-----------|------------|
| Library-only constraint | ✅ Maintained — no CLI surface |
| Dependency surface | ✅ Stdlib + existing deps only |
| Interface reuse | ✅ Uses existing `LLMClient` interface |
| Package location | ✅ `pkg/extraction/relations.go` matches ROADMAP |
| API signature | ✅ `Extract(ctx, text, entities) ([]Triplet, error)` matches ROADMAP spec |
| Testing strategy | ✅ Offline-first with optional gated integration |

The plan correctly extends the existing extraction package rather than creating a new package, which keeps the codebase cohesive.

---

## Scope Assessment

**Rating:** ✅ APPROPRIATE

**In-scope items** are correctly bounded to Phase 3 deliverables from the ROADMAP.

**Out-of-scope items** appropriately defer storage, search, and orchestration to later phases.

The scope avoids gold-plating by not introducing:
- Relation normalization (deferred to Phase 5)
- Strict relation allowlists (permissive now, can tighten later)
- Configuration options (deferred to Phase 6)

---

## Technical Debt Risks

| Risk | Severity | Mitigation in Plan |
|------|----------|-------------------|
| Case-insensitive matching may be too loose | Low | Acceptable for MVP; can add fuzzy matching later |
| Dropping unknown triplets may hide LLM issues | Low | Addressed; permissive behavior documented with rationale |
| No relation normalization | Low | Explicitly deferred to Phase 5 |

**No high-severity debt risks identified.**

---

## Findings

### Critical

*None identified.*

### Medium

#### M1 — Missing `NewRelationExtractor` Constructor Pattern

**Status:** OPEN

**Issue:** The plan specifies the `RelationExtractor` struct and `Extract` method but does not mention a constructor function (`NewRelationExtractor`).

**Impact:** Phase 2 established a pattern with `NewEntityExtractor(llmClient)`. Inconsistency would create API asymmetry.

**Recommendation:** Add explicit task in Milestone 1 to implement `NewRelationExtractor(llmClient)` constructor for consistency with existing patterns.

---

#### M2 — Validation Logic for Triplets Not Fully Specified

**Status:** OPEN

**Issue:** Milestone 2 describes validation (non-empty fields, trim whitespace) but doesn't specify behavior for:
- What if `Relation` field is empty after trimming?
- Should relation names be normalized (uppercase)?

**Impact:** Implementer may make inconsistent choices.

**Recommendation:** Clarify:
1. Empty relation → drop triplet (consistent with subject/object behavior)
2. Relation normalization → none in Phase 3 (document explicitly)

---

### Low

#### L1 — "Stable ordering if applicable" Is Ambiguous

**Status:** OPEN

**Issue:** Milestone 2 acceptance criteria says "stable ordering if applicable" but doesn't define what ordering should be used.

**Impact:** Minor inconsistency in test assertions.

**Recommendation:** Specify: preserve order from LLM response after deduplication (first occurrence wins).

---

#### L2 — No Constructor Parameter for Retry Configuration

**Status:** OPEN (Informational)

**Issue:** Plan states "reuse existing retry/backoff in `pkg/llm`" but doesn't clarify whether relationship extraction should have configurable retry behavior independent of the LLM client.

**Impact:** Low — retry logic lives in LLM client, not extractor. Just confirm this is intentional.

**Recommendation:** Confirm in plan: "Retry logic is encapsulated in `LLMClient`; no additional retry layer in `RelationExtractor`."

---

## Open Questions — RESOLVED

The plan contained 2 open questions. User confirmed resolutions on 2025-12-24:

1. **Target release versioning:** ✅ RESOLVED — v0.3.0 confirmed for Phase 3.
2. **Strictness vs permissiveness:** ✅ RESOLVED — **Strict mode preferred.** If a triplet references an entity not in the provided list, extraction should fail with a clear error (not silently drop). Add a note to reevaluate if integration tests show this is too brittle.

**Impact on Plan:** The plan currently specifies drop-on-unknown behavior. Planner should update Plan-Level Decision #1 to reflect strict mode before implementation.

---

## Questions for Planner

1. Should `NewRelationExtractor(llmClient)` be added to maintain API consistency with Phase 2?
2. What should happen if `Relation` is empty after trimming — drop triplet or error?
3. Confirm ordering after deduplication: first-occurrence-wins?

---

## Risk Assessment

| Area | Risk Level | Notes |
|------|-----------|-------|
| Implementation complexity | Low | Straightforward extension of Phase 2 patterns |
| Architectural fit | Low | Aligns with ROADMAP and existing code |
| Scope creep | Low | Scope is well-defined |
| Testing coverage | Low | Test cases are comprehensive |

**Overall Risk:** LOW

---

## Recommendations

1. **Update Plan-Level Decision #1** to specify strict mode (fail on unknown entities) instead of drop-on-unknown. Include note: "Reevaluate if integration tests show strict mode is too brittle."
2. **Address M1 and M2** before implementation to ensure API consistency and complete validation spec.
3. **Minor clarifications** on ordering (L1) and retry encapsulation (L2) would improve implementer experience.

---

## Hotfix Scenario Analysis

*How might this plan result in a hotfix after deployment?*

1. **LLM returns unexpected entity names:** The case-insensitive trim-and-match approach is reasonable, but if the LLM returns synonyms or partial names (e.g., "React" vs "React.js"), triplets will be dropped silently. This is acceptable for MVP but could cause user confusion if too many triplets disappear.

2. **Empty relations from LLM:** If the LLM returns `{"subject": "A", "relation": "", "object": "B"}`, behavior is unspecified. Could cause downstream issues if not validated.

3. **Unicode normalization:** Case-insensitive comparison using `strings.EqualFold` handles ASCII but may behave unexpectedly with Unicode entity names. Low risk for MVP scope.

**Mitigation:** The plan's drop-on-unknown behavior provides graceful degradation. Adding explicit empty-relation handling (M2) would close the remaining gap.

---

## Revision History

| Revision | Date | Changes | Findings Addressed | New Findings | Status |
|----------|------|---------|-------------------|--------------|--------|
| Initial | 2025-12-24 | First review | — | M1, M2, L1, L2 | OPEN |

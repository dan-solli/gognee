# Critique: Plan 008 — Edge ID Correctness Fix

**Artifact Path**: `agent-output/planning/008-edge-id-correctness-fix-plan.md`
**Date**: 2025-12-24
**Status**: Follow-up Review
**Critique Status**: RESOLVED

## Changelog
| Date | Agent Handoff | Request | Summary |
|------|---------------|---------|---------|
| 2025-12-24 | Planner → Critic | Review for clarity, completeness, architectural alignment | Initial critique |
| 2025-12-24 | Planner → Critic | Revise plan per critique | Plan updated: ambiguity policy specified for duplicate normalized names; lookup helper tracks ambiguity explicitly |

---

## Value Statement Assessment

✅ **PRESENT AND WELL-FORMED**

> "As a developer relying on graph traversal, I want edges to correctly reference node IDs including entity types, So that graph queries return accurate relationship paths."

**Assessment**: Clear value statement addressing a documented QA finding (Finding 3 from 006-phase6-integration-qa.md). This is a correctness fix, not a feature — appropriate framing.

---

## Overview

Plan 008 addresses a bug where edge source/target IDs are generated with empty entity type while node IDs use actual types, causing ID mismatch and broken graph traversal.

**Strengths**:
- Clear problem statement with code examples
- Correctly identifies root cause
- Graceful handling strategy (skip edge, log warning)
- Backward-compatible (new edges correct, old edges remain)

**Concerns**: See findings below.

---

## Architectural Alignment

✅ **ALIGNED** with existing patterns:
- Uses existing deterministic ID generation function
- Extends CognifyResult (established pattern from MVP)
- No interface changes required

**Consistency Check**:
- Fix is localized to `Cognify()` method in gognee.go
- No changes to GraphStore or VectorStore interfaces

---

## Scope Assessment

**Scope**: Appropriate — small, focused bug fix
**Complexity**: Low — straightforward lookup table implementation

**Boundary Check**:
- ✅ Does not add fuzzy matching (correctly deferred to future)
- ✅ Does not attempt to repair existing data
- ✅ Focuses only on new edge creation

---

## Technical Debt Risks

| Risk | Severity | Notes |
|------|----------|-------|
| Existing orphaned edges not repaired | Low | Documented; users must re-Cognify |
| LLM extraction inconsistencies | Medium | Entity names in triplets may not exactly match extraction |

---

## Unresolved Open Questions

None — the one OPEN QUESTION is marked `[RESOLVED]`.

---

## Findings

### Finding 1: Entity Name Normalization Mismatch Risk
**Status**: OPEN
**Severity**: Medium

**Issue**: The plan assumes entity names in triplets match entity names from extraction. However, LLMs may produce variations:
- Extraction: "PostgreSQL" (type: "Technology")
- Triplet: "Postgres" or "postgresql" (subject in relation)

The plan mentions "case-insensitive lookup" but doesn't address semantic variations.

**Impact**: Edges may still be skipped even when the entity exists, just with slight name variation.

**Recommendation**: 
1. Document this limitation explicitly in the plan
2. Consider whitespace normalization in addition to case normalization
3. Add a specific test case for common variations (e.g., "PostgreSQL" vs "Postgres")
4. Log the actual names that failed lookup for debugging

---

### Finding 2: Duplicate Entity Names with Different Types
**Status**: RESOLVED
**Severity**: Low

**Issue**: Milestone 1 Acceptance Criteria mentions "Handles edge cases (duplicate entity names with different types)" but doesn't specify the resolution strategy.

**Impact**: If "Python" exists as both "Technology" and "Concept", which type should be used for edge generation?

**Recommendation**: Specify strategy — suggest using first match (by extraction order) and logging a warning when ambiguity exists. Alternatively, skip the edge if ambiguous.

**Resolution**: Plan now defines an ambiguity policy: if a normalized name maps to multiple types, treat it as ambiguous and skip edge creation while recording a warning.

---

### Finding 3: EdgesSkipped vs Errors Redundancy
**Status**: OPEN  
**Severity**: Low

**Issue**: Plan adds both `EdgesSkipped int` field AND adds errors to the Errors list. This is redundant — callers must check two places.

**Impact**: API complexity; potential for count mismatch if error recording diverges from skip counting.

**Recommendation**: Either:
- Use only `EdgesSkipped` count (no individual error per edge)
- OR use only Errors list (derive skip count from error count)

For consistency with existing ChunksFailed pattern, recommend keeping Errors list and deriving count.

---

### Finding 4: Missing "How will this break in production?" Analysis
**Status**: OPEN
**Severity**: Medium

**Issue**: Per critic methodology, we should ask "How will this plan result in a hotfix after deployment?"

**Potential Failure Modes**:
1. LLM changes extraction format → names don't match → all edges skipped
2. Very long entity names truncated differently → no match
3. Unicode normalization differences → no match

**Recommendation**: Add test cases for:
- Unicode entity names (e.g., "Café")
- Long entity names (>100 chars)
- Entity names with special characters

---

## Questions for Planner

1. What normalization is applied to entity names before lookup? (lowercase + trim? More aggressive?)
2. What happens if the same entity name appears with multiple types in extraction?
3. Should there be a minimum match threshold before skipping becomes a warning vs error?

---

## Risk Assessment

**Overall Risk**: LOW

The fix is straightforward and well-scoped. Main risk is edge cases from LLM extraction inconsistencies, which are inherent to LLM-based systems.

---

## Recommendations

1. **Document name variation limitation** (Finding 1)
2. **Specify ambiguous entity handling** (Finding 2)
3. **Simplify EdgesSkipped/Errors relationship** (Finding 3)
4. **Add edge case tests** (Finding 4)

---

## Approval Status

**APPROVED** — Blocking ambiguity handling is now specified in the plan.

Non-blocking recommendations to consider during implementation:
- Improve normalization beyond case-insensitive matching (whitespace normalization at minimum).
- Decide whether `EdgesSkipped` is redundant with Errors, or define a strict counting contract.
- Add edge-case tests (Unicode, long names, special characters) to reduce hotfix risk.


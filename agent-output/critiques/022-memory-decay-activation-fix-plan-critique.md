# 022 - Memory Decay Activation Fix Plan - Critique

**Artifact Path**: `agent-output/planning/022-memory-decay-activation-fix-plan.md`  
**Analysis Reference**: `agent-output/analysis/003-memory-decay-functionality-analysis.md`  
**Date**: 2026-02-04  
**Status**: Initial Review  
**Gate 2 Decision**: **APPROVED**

## Changelog

| Date | Handoff | Request | Summary |
|------|---------|---------|---------|
| 2026-02-04 | User → Critic | Review for approval | Initial Gate 2 evaluation |

---

## Value Statement Assessment

**PRESENT**: ✅ Clear user story format  
**CLARITY**: ✅ "memories naturally forget over time without requiring explicit configuration" — verifiable outcome  
**ALIGNMENT**: ✅ Supports intelligent memory lifecycle (Plan 021 follow-up)  
**DIRECTNESS**: ✅ Value delivered immediately, no deferrals for core functionality

**Assessment**: Strong value statement. The "so that" clause describes a concrete behavioral change that can be verified in tests and production. The value is delivered directly through the plan, not deferred.

---

## Overview

This is a **focused bug fix + defaults change** plan addressing critical issues identified in Analysis 003:

1. **M1 (Bug)**: `GetNode()` doesn't hydrate `last_accessed_at` → access-based decay is broken
2. **M2 (Defaults)**: Decay features default to OFF → system is inert without explicit config
3. **M3 (Double-counting)**: Appropriately deferred as non-blocking
4. **M4 (Assessment only)**: Correctly scoped

---

## Technical Correctness Assessment

### Question 1: Is the GetNode() fix approach technically correct?

**VERIFIED CORRECT**: ✅

Evidence from code review:
- [pkg/store/sqlite.go#L444](pkg/store/sqlite.go#L444) confirms SELECT omits `last_accessed_at`
- [pkg/store/graph.go#L17](pkg/store/graph.go#L17) confirms `Node.LastAccessedAt *time.Time` field exists
- [pkg/store/sqlite.go#L681](pkg/store/sqlite.go#L681) shows `FindNodesByQuery` DOES include `last_accessed_at` — proving the pattern is already established elsewhere
- [pkg/store/sqlite.go#L788](pkg/store/sqlite.go#L788) shows another query including `last_accessed_at`

The plan correctly identifies:
- Need to add `last_accessed_at` to SELECT
- Need `sql.NullTime` scan target (nullable column)
- Need to populate `node.LastAccessedAt`

**Note**: Plan correctly identifies `FindNodesByName()` may have the same gap — I verified it does (lines 496-504 in sqlite.go also omit `last_accessed_at`). Good catch.

### Question 2: Are all decay-related defaults identified and addressed?

**VERIFIED CORRECT**: ✅

| Default | Current | Proposed | Evidence |
|---------|---------|----------|----------|
| `DecayEnabled` | `false` (zero value) | `true` | [gognee.go#L45](pkg/gognee/gognee.go#L45) |
| `AccessFrequencyEnabled` | `false` (zero value) | `true` | [gognee.go#L57](pkg/gognee/gognee.go#L57) |
| `ReferenceAccessCount` | `0` (zero value) | `10` | Already documented as default:10 but not enforced |
| `PruneSuperseded` | `false` (zero value) | `true` | [gognee.go#L170](pkg/gognee/gognee.go#L170) (comment says "default: true") |

**Complete coverage**: All four defaults are correctly identified.

### Question 3: Is the breaking change handling acceptable?

**ACCEPTABLE**: ✅ (with conditions)

The plan correctly:
- Documents this as a breaking change in CHANGELOG
- Notes that existing users with minimal config will now get decay enabled
- Provides explicit opt-out path (`DecayEnabled: false`)

**Condition**: The CHANGELOG entry MUST clearly call out this is a breaking change for users who relied on decay being disabled by default. The plan mentions this but should be explicit in the milestone.

### Question 4: Is the double-counting deferral appropriate?

**APPROPRIATE**: ✅

Rationale for deferral is sound:
- User directive focuses on M1/M2 (activation)
- Double-counting affects accuracy, not correctness
- System will function correctly; access counts will be inflated
- Inflated counts = faster frequency heat = stronger decay resistance (arguably acceptable behavior)

**Recommendation accepted**: The residual `RES-022-01` should be created to track this for future cleanup.

### Question 5: Any missing milestones or risks?

**FINDINGS**:

1. **MEDIUM - Missing Milestone: Test updates**  
   The plan notes "tests that explicitly set config values are unaffected" but doesn't account for tests that may implicitly rely on decay being OFF. A milestone should explicitly call out reviewing/updating tests that break due to new defaults.

2. **LOW - Tri-state resolution for PruneSuperseded**  
   The plan correctly identifies the Go zero-value problem but the "unconditional override" solution means users CANNOT explicitly disable `PruneSuperseded: false`. The plan acknowledges this ("Revised" note) but the acceptance criteria is inconsistent.
   
   **Recommendation**: Accept the limitation for urgency, but the CHANGELOG must document that `PruneSuperseded: false` has no effect and provide the workaround (high `SupersededAgeDays`).

3. **LOW - ReferenceAccessCount already has logic**  
   The plan shows `if cfg.ReferenceAccessCount == 0 { cfg.ReferenceAccessCount = 10 }` — this is actually the correct pattern and already exists in the codebase. No change may be needed here. Implementer should verify.

---

## Architectural Alignment

**ALIGNED**: ✅

- Changes are localized to `pkg/store` (GetNode) and `pkg/gognee` (defaults)
- No new dependencies introduced
- Follows existing patterns (see other queries that include `last_accessed_at`)
- Respects interface boundaries (`GraphStore.GetNode` contract unchanged)

---

## Scope Assessment

**APPROPRIATE**: ✅

- Single epic (Plan 021 follow-up)
- ~3-4 files touched
- Estimated < 1 day implementation
- Focused, atomic changes

---

## Technical Debt Risks

| Risk | Severity | Status |
|------|----------|--------|
| Double-counting access (deferred) | Low | Documented as residual |
| `PruneSuperseded: false` ineffective | Low | Acceptable per urgency |
| Potential test failures | Medium | Mitigated by explicit test review |

---

## Findings Summary

### Critical
*None*

### Medium

| Issue | Status | Description | Impact | Recommendation |
|-------|--------|-------------|--------|----------------|
| M-1: Test update milestone missing | OPEN | No explicit step to review/fix tests that assume decay=OFF | Tests may fail unexpectedly | Add sub-task in M2/M3 to review and update affected tests |

### Low

| Issue | Status | Description | Impact | Recommendation |
|-------|--------|-------------|--------|----------------|
| L-1: PruneSuperseded disable path | OPEN | Unconditional override prevents explicit disable | Minor user impact | Document workaround in CHANGELOG |
| L-2: ReferenceAccessCount may already work | OPEN | Default logic may already exist | Minor | Implementer verify before changing |

---

## Open Questions Check

**RESOLVED OPEN QUESTIONS**: ✅

1. `OPEN QUESTION [RESOLVED]`: Go zero-value bool defaults → Apply unconditionally
2. `OPEN QUESTION [RESOLVED]`: Double-counting fix → Deferred

**No unresolved open questions blocking approval.**

---

## Residuals Reconciliation

Plan correctly notes:
- No existing residuals ledger
- Creates recommended residual: `RES-022-01` for double-counting

**Compliant** with planner process.

---

## Compile Verification Gate

**PASSED**: ✅

```
$ go build ./...
# Compiles successfully
```

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Breaking change affects production users | Medium | Medium | CHANGELOG documentation, explicit disable path |
| Tests fail due to new defaults | Medium | Low | Review and update tests as needed |
| Double-counting inflates metrics | High | Low | Documented as known issue/residual |

---

## Recommendations

1. **Add test review sub-task**: In Milestone 2 or 3, explicitly call out "Review existing tests; update any that fail due to new defaults"

2. **Strengthen CHANGELOG entry**: Ensure it clearly states:
   - "BREAKING CHANGE: Decay is now enabled by default"
   - "BREAKING CHANGE: `PruneSuperseded` defaults to true; set high `SupersededAgeDays` to effectively disable"

3. **Verify ReferenceAccessCount**: Implementer should check if default logic already exists before adding duplicate

---

## Gate 2 Decision

### **APPROVED**

**Rationale**:
1. Value statement is clear and directly delivered
2. Technical approach is verified correct against codebase
3. All decay-related defaults are identified
4. Breaking change handling is acceptable with minor documentation enhancements
5. Double-counting deferral is appropriate for urgency
6. No unresolved open questions
7. Scope is appropriate for patch release
8. Codebase compiles

**Conditions for Implementation**:
- Address M-1 (test review sub-task) during implementation
- Ensure CHANGELOG clearly documents breaking changes
- Create residual `RES-022-01` after implementation

Plan is ready for **Implementer handoff**.

---

## Revision History

| Date | Version | Changes |
|------|---------|---------|
| 2026-02-04 | Initial | Gate 2 review completed - APPROVED |

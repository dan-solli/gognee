# Critique: Plan 010 — Memory Decay / Forgetting

**Artifact Path**: `agent-output/planning/010-memory-decay-forgetting-plan.md`
**Date**: 2025-12-25
**Status**: Final Review
**Critique Status**: RESOLVED

## Changelog
| Date | Agent Handoff | Request | Summary |
|------|---------------|---------|---------|
| 2025-12-24 | Planner → Critic | Review for clarity, completeness, architectural alignment | Initial critique |
| 2025-12-24 | Planner → Critic | Revise plan per critique | Plan updated: explicit SQLite migration strategy; DecayingSearcher decorator approach; NULL last_accessed_at fallback; TopK-only + batched access updates; clarified node-only decay semantics |
| 2025-12-25 | User → Critic | Final review for clarity, completeness, architectural alignment | Verified all critical findings resolved; plan approved for implementation |

---

## Value Statement Assessment

✅ **PRESENT AND WELL-FORMED**

> "As a developer building a long-lived AI assistant, I want old or stale information to decay or be forgotten, So that the knowledge graph stays relevant and doesn't grow unbounded."

**Assessment**: Clear value statement addressing a real production concern (unbounded growth). The "so that" clause captures both relevance and resource management.

---

## Overview

Plan 010 implements time-based memory decay affecting search ranking, plus an explicit Prune API for permanent deletion. This is the most complex plan of the four.

**Strengths**:
- Decay is OFF by default (backward compatible)
- Prune is explicit, not automatic (safe)
- DryRun mode prevents accidental data loss
- Configurable half-life allows tuning
- Access reinforcement mimics human memory

**Concerns**: See findings below.

---

## Architectural Alignment

⚠️ **MOSTLY ALIGNED** with concerns:

- Uses existing SQLite schema extension pattern
- Extends Config (established pattern)
- Adds new API methods (Prune) following existing patterns

**Consistency Concerns**:
- Search scoring modification may require changes across multiple searchers (VectorSearcher, GraphSearcher, HybridSearcher)
- Access time update on search may have performance implications

---

## Scope Assessment

**Scope**: Large for a single version — 10 milestones
**Complexity**: High — touches search, storage, and new API surface

**Boundary Check**:
- ✅ Does not implement auto-prune (correctly deferred)
- ✅ Does not implement frequency-based decay (access_count added for future)
- ⚠️ Access reinforcement on every search may be over-scoped

---

## Technical Debt Risks

| Risk | Severity | Notes |
|------|----------|-------|
| Performance impact of timestamp updates | Medium | Every search hit updates timestamp; may slow search |
| Schema migration complexity | Medium | ALTER TABLE for existing databases |
| Searcher modification scope | Medium | All three searchers may need decay logic |

---

## Unresolved Open Questions

None — the one OPEN QUESTION is marked `[RESOLVED]`.

---

## Findings

### Finding 1: Schema Migration Not Addressed
**Status**: RESOLVED
**Severity**: High

**Issue**: Milestone 1 says "Add `last_accessed_at DATETIME` column to nodes table via schema migration" but doesn't specify HOW migration works.

**Impact**: SQLite doesn't support `ALTER TABLE ADD COLUMN` with defaults in all cases. Existing databases may fail to upgrade.

**Recommendation**: Specify migration strategy:
```sql
-- SQLite allows ADD COLUMN if column has default or allows NULL
ALTER TABLE nodes ADD COLUMN last_accessed_at DATETIME DEFAULT NULL;
ALTER TABLE nodes ADD COLUMN access_count INTEGER DEFAULT 0;
```
Add migration detection logic in initSchema() — check if column exists before adding.

**Resolution**: Plan now specifies column detection and per-column `ALTER TABLE ... ADD COLUMN` strategy.

---

### Finding 2: Search Performance Impact Not Analyzed
**Status**: RESOLVED
**Severity**: Medium

**Issue**: Milestone 5 updates `last_accessed_at` on every search hit. For searches returning 100 nodes, this means 100 UPDATE statements.

**Impact**: Search latency could increase significantly, especially for large result sets.

**Recommendation**: 
1. Use batch UPDATE (single statement with IN clause)
2. Or make access tracking async (background goroutine)
3. Or only track access for top-K final results (not intermediate)
4. Add performance acceptance criteria to Milestone 5

**Resolution**: Plan now limits access updates to final TopK results and requires batched updates, with an explicit acceptance criterion around search latency impact.

---

### Finding 3: Decay Integration Across Searchers
**Status**: RESOLVED
**Severity**: Medium

**Issue**: Milestone 4 says "Modify search result scoring to apply decay multiplier" but gognee has 3 searcher implementations (Vector, Graph, Hybrid). The plan doesn't specify which to modify.

**Impact**: Inconsistent decay behavior if only some searchers implement decay.

**Recommendation**: Clarify:
- Option A: Apply decay in HybridSearcher only (covers most use cases)
- Option B: Apply decay in all searchers (consistent but more work)
- Option C: Apply decay in a wrapper/decorator (cleanest separation)

Recommend Option C — create a `DecayingSearcher` wrapper that applies decay to any underlying Searcher.

**Resolution**: Plan now adopts the decorator pattern via a `DecayingSearcher` wrapper, avoiding Searcher interface changes.

---

### Finding 4: Edge Decay Not Fully Specified
**Status**: RESOLVED
**Severity**: Low

**Issue**: Assumption 6 says "Edges decay with their connected nodes" and Prune mentions cascading, but decay scoring doesn't mention edges.

**Impact**: Unclear whether edges affect search ranking or just pruning.

**Recommendation**: Clarify that decay applies to nodes only for search ranking. Edges are only affected during Prune (cascading delete when endpoints are pruned).

**Resolution**: Plan now explicitly clarifies node-only decay for scoring and edge impact only via pruning cascades.

---

### Finding 5: "How will this break in production?" Analysis
**Status**: RESOLVED
**Severity**: Medium

**Potential Failure Modes**:
1. User enables decay mid-lifecycle → all existing nodes have NULL last_accessed_at → all treated as ancient → all decay to near-zero
2. Clock skew or time zone issues → decay calculations incorrect
3. Aggressive half-life (1 day) + Prune → accidental data loss
4. Prune without DryRun first → irreversible data loss

**Recommendation**:
1. Handle NULL last_accessed_at as "use created_at" (fallback)
2. Use UTC consistently; document this
3. Add warning when half-life < 7 days
4. Add confirmation step or "min nodes to keep" safeguard for Prune

**Resolution**: Plan now specifies NULL last_accessed_at fallback to created_at and documents safer access tracking; additional prune safeguards remain recommended (see Finding 6 note).

---

### Finding 6: Missing Stats Extensions
**Status**: OPEN
**Severity**: Low

**Issue**: Handoff Notes mention "Consider adding Stats.OldestNode and Stats.AverageNodeAge" but this isn't in any milestone.

**Impact**: Users can't easily assess graph age distribution before pruning.

**Recommendation**: Add these to Stats output (quick addition, high value for Prune decision-making).

---

## Questions for Planner

1. What is the migration strategy for adding columns to existing databases?
2. How will access tracking be performant for large result sets?
3. Should decay be applied in a wrapper/decorator or inline in each searcher?
4. How should NULL last_accessed_at be handled (fallback to created_at)?
5. Should Prune have a safeguard (e.g., "keep at least N nodes")?

---

## Risk Assessment

**Overall Risk**: MEDIUM-HIGH

This is the most complex plan and touches multiple subsystems. The core concept is sound, but implementation details need more specification to avoid production issues.

---

## Recommendations

1. **Specify schema migration strategy** (Finding 1) — CRITICAL
2. **Address search performance impact** (Finding 2)
3. **Specify searcher integration approach** (Finding 3) — recommend decorator pattern
4. **Handle NULL timestamp fallback** (Finding 5)
5. **Add Stats extensions for observability** (Finding 6)

---

## Approval Status

✅ **APPROVED FOR IMPLEMENTATION**

All critical and medium severity findings have been resolved in the plan text:

| Finding | Severity | Resolution |
|---------|----------|------------|
| Schema Migration | High | ✅ Milestone 1 specifies column detection + ALTER TABLE strategy |
| Search Performance | Medium | ✅ Milestone 5 limits to TopK + batch updates |
| Searcher Integration | Medium | ✅ Milestone 4 uses DecayingSearcher decorator |
| NULL Timestamp Handling | Medium | ✅ Milestone 4 specifies fallback to created_at |
| Edge Decay Semantics | Low | ✅ Clarification added: node-only for scoring |
| Stats Extensions | Low | OPEN — Non-blocking enhancement suggestion |

**Non-blocking recommendations** (defer to implementer discretion):
- Consider adding `Stats.OldestNode` and `Stats.AverageNodeAge` to improve prune observability
- Consider an additional Prune safeguard (e.g., `KeepAtLeastN int`) to reduce accidental data loss risk

**Final Review Notes** (2025-12-25):
The plan is well-structured with clear milestones, explicit acceptance criteria, and thoughtful risk mitigations. The decorator pattern for decay integration is architecturally clean. The decision to keep decay OFF by default ensures backward compatibility. Ready for implementation.


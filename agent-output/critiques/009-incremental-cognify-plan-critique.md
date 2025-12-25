# Critique: Plan 009 — Incremental Cognify

**Artifact Path**: `agent-output/planning/009-incremental-cognify-plan.md`
**Date**: 2025-12-24
**Status**: Follow-up Review
**Critique Status**: RESOLVED

## Changelog
| Date | Agent Handoff | Request | Summary |
|------|---------------|---------|---------|
| 2025-12-24 | Planner → Critic | Review for clarity, completeness, architectural alignment | Initial critique |
| 2025-12-24 | Planner → Critic | Revise plan per critique | Plan updated: SkipProcessed default set to true; DocumentTracker interface introduced; source semantics clarified as metadata; reset capability added via ClearProcessedDocuments |

---

## Value Statement Assessment

✅ **PRESENT AND WELL-FORMED**

> "As a developer with large document corpora, I want to process only new or changed documents, So that I can update my knowledge graph efficiently without reprocessing everything."

**Assessment**: Clear value statement with quantifiable benefit (reduced processing time and API costs). Directly supports production use cases.

---

## Overview

Plan 009 implements document-level deduplication using content hashing. Previously processed documents are skipped on subsequent Cognify calls.

**Strengths**:
- Content-based identity (hash) is robust and intuitive
- Document-level granularity is simpler than chunk-level
- Force option provides escape hatch
- Backward-compatible schema change (new table)

**Concerns**: See findings below.

---

## Architectural Alignment

✅ **ALIGNED** with existing patterns:
- Extends SQLiteGraphStore schema (established pattern)
- Extends CognifyOptions/CognifyResult (established pattern)
- Uses SHA-256 (consistent with existing ID generation)

**Consistency Check**:
- New `processed_documents` table follows existing schema patterns
- Interface extension follows existing GraphStore pattern

**Potential Concern**: Adding document tracking methods to GraphStore interface may bloat the interface. Consider whether a separate `DocumentTracker` interface is more appropriate.

---

## Scope Assessment

**Scope**: Appropriate for a minor version bump (v0.8.0)
**Complexity**: Medium — straightforward concept, but default behavior decision is critical

**Boundary Check**:
- ✅ Does not track chunk-level changes (correctly scoped to document)
- ✅ Does not automatically expire cached entries
- ✅ Does not provide "smart" content diffing

---

## Technical Debt Risks

| Risk | Severity | Notes |
|------|----------|-------|
| Hash collision | Very Low | SHA-256 collision is astronomically unlikely; documented |
| Stale cache after chunker config change | Medium | Documented; Force=true is workaround |
| `:memory:` mode loses tracking | Medium | Document tracking table also in-memory |

---

## Unresolved Open Questions

None — the one OPEN QUESTION is marked `[RESOLVED]`.

---

## Findings

### Finding 1: Default Behavior Not Specified
**Status**: RESOLVED
**Severity**: High

**Issue**: Milestone 3 says "default: true for backward compat consideration" for `SkipProcessed` but this is marked "TBD". The plan doesn't definitively choose the default.

**Impact**: 
- If default is `true` (incremental by default): Existing users get new behavior automatically — could be surprising
- If default is `false` (opt-in): Existing users unchanged, but feature is "hidden"

**Recommendation**: Choose explicitly. Suggest:
- `SkipProcessed` default = `true` (incremental by default) — this is the expected production behavior
- Document the behavior change prominently in CHANGELOG
- First-time Cognify on existing DB will process everything (no prior tracking)

**Resolution**: Plan now specifies `SkipProcessed` default = true and documents incremental-by-default behavior.

---

### Finding 2: GraphStore Interface Bloat
**Status**: RESOLVED
**Severity**: Medium

**Issue**: Milestone 2 proposes adding `IsDocumentProcessed`, `MarkDocumentProcessed`, `GetProcessedDocumentCount` to GraphStore interface. This adds 3 methods unrelated to graph operations.

**Impact**: GraphStore interface becomes less cohesive; implementations must implement document tracking even if they don't use it.

**Recommendation**: Create a separate interface:
```go
type DocumentTracker interface {
    IsDocumentProcessed(ctx, hash string) (bool, error)
    MarkDocumentProcessed(ctx, hash, source string, chunkCount int) error
    GetProcessedDocumentCount(ctx) (int64, error)
}
```
SQLiteGraphStore can implement both GraphStore and DocumentTracker.

**Resolution**: Plan now introduces a separate `DocumentTracker` interface and avoids expanding GraphStore.

---

### Finding 3: Source Field Semantics Unclear
**Status**: RESOLVED
**Severity**: Low

**Issue**: Plan stores `source TEXT` in processed_documents but doesn't specify how it's used. Is it just metadata, or does it participate in identity?

**Impact**: Confusion about whether same content from different sources is considered "same document".

**Recommendation**: Clarify that source is metadata only; identity is hash-only. Document this explicitly.

**Resolution**: Plan now explicitly defines identity as hash-only and source as metadata.

---

### Finding 4: No API to Clear Tracking Table
**Status**: RESOLVED
**Severity**: Medium

**Issue**: Plan provides no way to clear the `processed_documents` table other than Force=true (which still processes everything).

**Impact**: If user wants to "start fresh" tracking without reprocessing, there's no API.

**Recommendation**: Add `ClearDocumentTracking()` method or `ResetAll bool` option in CognifyOptions. This is noted in Handoff Notes but should be a milestone.

**Resolution**: Plan now includes `ClearProcessedDocuments(ctx)` on DocumentTracker as an optional reset capability.

---

### Finding 5: "How will this break in production?" Analysis
**Status**: OPEN
**Severity**: Medium

**Potential Failure Modes**:
1. Document updated but hash is same (whitespace-only changes) → not reprocessed → stale graph
2. User changes chunker settings → old chunks in graph, new chunks expected → inconsistent graph
3. tracking table in `:memory:` mode is lost on restart → documents reprocessed anyway

**Recommendation**: 
1. Document that hash is computed on exact text (no normalization)
2. Warn users to Force=true after chunker config changes
3. Document that `:memory:` mode does not benefit from incremental Cognify across restarts

---

## Questions for Planner

1. What is the definitive default for SkipProcessed?
2. Should source field affect document identity, or is it metadata only?
3. Is there a need for `ClearDocumentTracking()` API in v0.8.0?
4. Should document tracking be a separate interface from GraphStore?

---

## Risk Assessment

**Overall Risk**: MEDIUM

The plan is sound but needs explicit decisions on default behavior and interface design. The feature is valuable but introduces complexity that should be carefully considered.

---

## Recommendations

1. **Explicitly set default for SkipProcessed** (Finding 1) — recommend `true`
2. **Consider separate DocumentTracker interface** (Finding 2)
3. **Clarify source field semantics** (Finding 3)
4. **Add ClearDocumentTracking milestone** (Finding 4)
5. **Document `:memory:` limitation** (Finding 5)

---

## Approval Status

**APPROVED** — Blocking decisions (default behavior + interface cohesion) are now specified.

Non-blocking recommendation:
- Explicitly document `:memory:` limitations and the exact hash input rules (exact text vs normalized) to prevent surprises.


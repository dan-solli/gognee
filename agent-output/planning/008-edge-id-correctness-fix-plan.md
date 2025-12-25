# Plan 008: Edge ID Correctness Fix

**Plan ID**: 008
**Target Release**: v0.7.1
**Epic Alignment**: Epic 7.3 - Edge ID Correctness Fix (P2)
**Status**: QA Complete
**Created**: 2025-12-24

## Changelog
| Date | Change |
|------|--------|
| 2025-12-24 | Initial plan creation |
| 2025-12-25 | Revised per critic feedback; target release updated to v0.7.1; finalized for implementation |
| 2025-12-25 | Implementation complete - all milestones delivered |
| 2025-12-25 | QA complete - unit tests + coverage verified; integration suite warning documented |

---

## Value Statement and Business Objective

**As a** developer relying on graph traversal,
**I want** edges to correctly reference node IDs including entity types,
**So that** graph queries return accurate relationship paths.

---

## Objective

Fix the edge ID derivation bug identified in QA Finding 3. Currently, edge source/target IDs are generated using `generateDeterministicNodeID(name, "")` (empty type), while node IDs are generated with actual entity types. This mismatch causes edges to reference non-existent nodes, breaking graph traversal.

---

## Background

### Current Behavior (Buggy)

In [gognee.go](../../pkg/gognee/gognee.go):

```go
// Nodes are created with type:
nodeID := generateDeterministicNodeID(entity.Name, entity.Type)

// But edges use empty type:
sourceID := generateDeterministicNodeID(triplet.Subject, "")
targetID := generateDeterministicNodeID(triplet.Object, "")
```

This means an entity "PostgreSQL" with type "Technology" gets node ID `hash("postgresql|Technology")`, but an edge referencing PostgreSQL gets source/target ID `hash("postgresql|")` — a different ID that doesn't exist in the nodes table.

### Expected Behavior

Edge source/target IDs should use the same (name, type) derivation as nodes, ensuring edges connect to actual nodes.

---

## Assumptions

1. Entity names in triplets (Subject/Object) match entity names exactly (case differences handled by normalization)
2. We need to look up entity type from the entities list when creating edges
3. If an entity referenced in a triplet wasn't extracted, we should handle gracefully (log warning, skip edge, or use fallback)
4. The fix is backward-compatible: new edges are correct, existing edges remain (users should re-Cognify for full correction)

**Ambiguity policy**: If multiple extracted entities share the same normalized name but have different types, treat the name→type mapping as ambiguous. In this case, skip edge creation for triplets referencing that name and record a warning in CognifyResult.Errors (preferred over guessing).

**OPEN QUESTION [RESOLVED]**: What if a triplet references an entity name that wasn't in the extracted entities list?
**Resolution**: Log a warning and skip the edge. This preserves data quality over completeness. Document that callers should inspect CognifyResult.Errors for skipped edges.

### Normalization Specification (Critic Finding 1)

Entity name normalization for lookup:
1. `strings.ToLower()` - case-insensitive matching
2. `strings.TrimSpace()` - remove leading/trailing whitespace
3. `strings.Join(strings.Fields(normalized), " ")` - collapse internal whitespace

**Known Limitation**: Semantic variations (e.g., "PostgreSQL" vs "Postgres") will NOT match. This is documented as a limitation of LLM extraction. Future enhancement: fuzzy matching or entity resolution.

**Diagnostic logging**: When an edge is skipped due to missing entity, log both the triplet name and available entity names for debugging.

---

## Plan

### Milestone 1: Entity Lookup Helper

**Objective**: Create a helper to look up entity type by name from the extracted entities list.

**Tasks**:
1. Create a map from normalized entity name to entity type during Cognify
2. Implement case-insensitive lookup (normalize both triplet names and entity names)
3. Track ambiguity: if a normalized name maps to multiple distinct types, mark as ambiguous
4. Return empty string (and an ambiguity flag) if entity not found or ambiguous (signals skip-this-edge condition)

**Acceptance Criteria**:
- Helper finds entity type by name with case-insensitive matching
- Returns indication when entity not found
- Handles edge cases (empty name, duplicate entity names with different types via ambiguity policy)

**Dependencies**: None

---

### Milestone 2: Fix Edge ID Generation

**Objective**: Update Cognify to use correct entity types when generating edge endpoint IDs.

**Tasks**:
1. Build entity name→type map before processing triplets
2. Look up source entity type from map
3. Look up target entity type from map
4. Generate edge source/target IDs with correct types
5. If source or target entity not found, log warning and skip edge
6. Add skipped edge count to CognifyResult

**Acceptance Criteria**:
- Edge IDs match corresponding node IDs
- Missing entity references logged as warnings
- CognifyResult reports edges skipped due to missing entities

**Dependencies**: Milestone 1

---

### Milestone 3: CognifyResult Enhancement

**Objective**: Extend CognifyResult to report edges skipped due to entity lookup failure.

**Tasks**:
1. Add `EdgesSkipped int` field to CognifyResult
2. Record each skipped edge as an entry in Errors list (with specific message format)
3. Document the field in type comments
4. Contract: `EdgesSkipped == count(Errors where message contains "skipped edge")`

**Acceptance Criteria**:
- CognifyResult.EdgesSkipped accurately counts skipped edges
- Errors list includes details about which edges were skipped (subject, relation, object, reason)
- EdgesSkipped count is derivable from Errors (single source of truth pattern)

**Dependencies**: Milestone 2

---

### Milestone 4: Unit Tests

**Objective**: Test edge ID generation correctness.

**Tasks**:
1. Add test case: extracted entities + triplets → verify edge source/target IDs match node IDs
2. Add test case: triplet references entity not in extraction → edge skipped, warning logged
3. Add test case: case mismatch between triplet and entity → still matches correctly
4. Add test case: whitespace normalization (e.g., "  React  " matches "React")
5. Add test case: Unicode entity names (e.g., "Café") handled correctly
6. Add test case: ambiguous entity names (same name, different types) → edge skipped
7. Verify EdgesSkipped count in result

**Acceptance Criteria**:
- Tests verify edge IDs are consistent with node IDs
- Tests verify graceful handling of missing entity references
- Tests cover edge cases: Unicode, whitespace, case variations
- All tests pass offline

**Dependencies**: Milestone 2, Milestone 3

---

### Milestone 5: Integration Test Update

**Objective**: Verify fix works end-to-end with real LLM extraction.

**Tasks**:
1. Update existing integration test to validate edge→node connectivity
2. After Cognify, verify that edge source/target IDs exist in nodes table
3. Verify graph traversal returns expected neighbors

**Acceptance Criteria**:
- Integration test validates edges connect to existing nodes
- Graph traversal works correctly after fix

**Dependencies**: Milestone 4

---

### Milestone 6: Documentation

**Objective**: Document the fix and any behavior changes.

**Tasks**:
1. Add CHANGELOG entry describing the fix
2. Document EdgesSkipped in API documentation
3. Add note about re-running Cognify to fix existing data

**Acceptance Criteria**:
- CHANGELOG describes the bug fix
- Users understand they may need to re-Cognify

**Dependencies**: Milestone 5

---

### Milestone 7: Version Management

**Objective**: Version update (bundled with v0.7.0).

**Tasks**:
1. Add fix to v0.7.0 CHANGELOG entry (with Plan 007)
2. Commit all changes

**Acceptance Criteria**:
- Fix documented in CHANGELOG
- Ready for v0.7.0 release

**Dependencies**: All previous milestones

---

## Testing Strategy

**Unit Tests**:
- Entity lookup helper correctness
- Edge ID consistency with node IDs
- Missing entity handling
- Case-insensitive matching

**Integration Tests**:
- End-to-end edge→node connectivity validation
- Graph traversal after fix

**Coverage Target**: ≥80% for modified code

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Entity name variations between extraction stages | Edges still orphaned | Normalize aggressively; consider fuzzy matching as future enhancement |
| Breaking change for existing data | User confusion | Document re-Cognify requirement; old edges remain (just orphaned) |
| LLM extracts triplets with entities not in extraction | Data loss (skipped edges) | Log warnings; document limitation |

---

## Handoff Notes

- This fix is scoped to new edge creation; existing orphaned edges are not automatically fixed
- Consider whether a migration/repair tool is needed (future enhancement)
- Critic should verify the entity lookup approach handles common LLM extraction inconsistencies


# Plan 012: Entity Type Validation Resilience

**Plan ID**: 012  
**Target Release**: v1.0.1 (patch)  
**Epic Alignment**: Bug fix / Usability improvement  
**Status**: Released (v1.0.1)  
**Created**: 2026-01-04  

## Changelog
| Date | Change | Rationale |
|------|--------|-----------|
| 2026-01-04 | Created plan | End user reported blocking error when LLM returned "Problem" entity type |
| 2026-01-04 | Status: UAT Approved | Implementation complete; QA passed; UAT validated value delivery; approved for release |

---

## Value Statement and Business Objective

**As an** AI assistant developer using gognee,  
**I want** entity extraction to gracefully handle LLM-returned entity types that aren't in the hardcoded allowlist,  
**So that** memory creation doesn't fail when the LLM reasonably infers semantically valid entity types like "Problem", "Goal", "Location", etc.

---

## Problem Statement

An end user encountered a blocking error during memory creation:

```
entity extraction failed for memory 855a08f5-...: entity at index 1 (duplicate or repeated version sections) 
has invalid type: Problem (must be one of: Person, Concept, System, Decision, Event, Technology, Pattern)
```

**Root Cause**: [pkg/extraction/entities.go#L19-L27](../../../pkg/extraction/entities.go) contains a hardcoded `validEntityTypes` map that rejects any entity type not in the allowlist. When the LLM extracts a semantically reasonable type like "Problem", "Goal", or "Location", gognee rejects the entire extraction—causing memory creation to fail completely.

**Impact**: This is a usability bug that leaks internal validation constraints to end users and blocks otherwise valid memory operations.

---

## Success Criteria

1. Entity extraction no longer fails when LLM returns types outside the original 7-type allowlist
2. Common entity types (Problem, Goal, Location, Organization, etc.) are explicitly supported
3. Unknown types are gracefully normalized to "Concept" with a warning log (not silently)
4. Existing tests continue to pass
5. Original LLM-returned type is preserved in node metadata for debugging/analytics

---

## Assumptions

1. The LLM prompt already instructs the model to use specific types; this fix handles when it doesn't comply
2. Normalizing unknown types to "Concept" is acceptable behavior (vs. blocking)
3. Logging a warning is sufficient notification; no error escalation needed
4. No schema changes required (metadata is already stored as JSON)

---

## Plan

### Milestone 1: Expand Entity Type Allowlist

**Objective**: Add commonly-needed entity types to the allowlist.

**Location**: `pkg/extraction/entities.go`

**New types to add**:
- `Problem` - Issues, bugs, challenges
- `Goal` - Objectives, targets
- `Location` - Places, regions, environments
- `Organization` - Companies, teams, groups
- `Document` - Files, specs, references
- `Process` - Workflows, procedures
- `Requirement` - Needs, constraints
- `Feature` - Capabilities, functionality
- `Task` - Action items, work units

**Acceptance Criteria**:
- `validEntityTypes` map includes all 16 types (original 7 + new 9)
- LLM prompt updated to list all valid types
- Tests updated to cover new types

---

### Milestone 2: Implement Graceful Fallback for Unknown Types

**Objective**: When an entity has an unrecognized type, normalize it to "Concept" instead of failing.

**Location**: `pkg/extraction/entities.go` - `Extract()` method

**Behavior**:
1. If entity type is not in `validEntityTypes`:
   - Log a warning using stdlib `log.Printf`: `"gognee: entity %q has unrecognized type %q, normalizing to Concept"`
   - Set `entity.Type = "Concept"`
2. Continue processing (do not return error)

**Design Decision (locked)**: Do NOT modify the `Entity` struct. The original type is captured in the warning log for observability. If future requirements need original-type preservation in data, the `Node.Metadata` field (already present in storage layer) can be used at the `gognee.Cognify()` level—but that is out of scope for this patch.

**Rationale**: Avoids API-breaking change to `Entity` struct; keeps patch minimal and safe.

**Acceptance Criteria**:
- Unknown types are normalized to "Concept"
- Warning is logged via `log.Printf` with entity name and original type (prefix: `gognee:`)
- Extraction never fails due to unknown type alone
- Warning is test-observable by capturing log output in unit tests

---

### Milestone 3: Update Tests

**Objective**: Ensure test coverage for new behavior.

**Location**: `pkg/extraction/entities_test.go`

**Test cases to add/modify**:
1. Test that new allowlist types (Problem, Goal, etc.) are accepted
2. Test that unknown types are normalized to "Concept" (not rejected)
3. Test that warning is logged for unknown types (capture `log` output via `log.SetOutput` to buffer)
4. Verify original 7 types still work
5. Verify relation extraction still works with normalized entity types (entities matched by name, not type)

**Acceptance Criteria**:
- All existing tests pass
- New test cases cover expanded allowlist
- New test cases cover fallback behavior
- Coverage maintained ≥80%

---

### Milestone 4: Update Version and Release Artifacts

**Objective**: Prepare v1.0.1 patch release.

**Tasks**:
1. Add CHANGELOG entry for v1.0.1 under `### Fixed`
2. Update any version references if present
3. Commit with message: `fix: graceful handling of unknown entity types (#012)`

**CHANGELOG Entry** (template):
```markdown
## [1.0.1] - 2026-01-XX

### Fixed
- Entity extraction no longer fails when LLM returns types outside the original allowlist
- Unknown entity types are now normalized to "Concept" with a warning log
- Added 9 new entity types: Problem, Goal, Location, Organization, Document, Process, Requirement, Feature, Task
```

**Acceptance Criteria**:
- CHANGELOG updated with v1.0.1 entry
- Version artifacts consistent

---

## Files Affected

| File | Change |
|------|--------|
| `pkg/extraction/entities.go` | Expand allowlist, add fallback logic, update prompt |
| `pkg/extraction/entities_test.go` | Add test cases for new types and fallback |
| `CHANGELOG.md` | Add v1.0.1 entry |

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| LLM starts returning many varied types, all becoming "Concept" | Medium | Low | Warning log provides visibility; allowlist can be expanded in future patches |
| Tests become flaky due to LLM non-determinism | Low | Low | Unit tests use mocks; integration tests already handle this |
| Log output in tests affects parallel test execution | Low | Low | Use `t.Parallel()` carefully; each test captures its own buffer |

---

## Testing Strategy

**Unit Tests**:
- Mock LLM returns entity with each new allowlist type → accepted
- Mock LLM returns entity with unknown type → normalized to Concept, warning logged
- Mock LLM returns entity with empty type → still rejected (existing behavior)

**Integration Tests** (if run):
- End-to-end memory creation with text likely to produce "Problem" entities
- Verify no extraction failures

**Coverage Target**: Maintain ≥80% on `pkg/extraction`

---

## Handoff Notes

1. Use stdlib `log.Printf` for warning output (prefix with `gognee:` for grep-ability)
2. The prompt string in `entityExtractionPrompt` const needs updating with new type list
3. **Relation extraction compatibility**: Relation/triplet extraction matches entities by **name**, not type. Normalizing unknown types to "Concept" does not affect relation linking. No changes needed in `pkg/extraction/relations.go`.
4. To test warning output, use `log.SetOutput()` to redirect to a buffer, then restore original output after test

---

## Open Questions

None remaining. All critique findings addressed:
- **Preservation strategy**: Warning log only; no `Entity` struct changes (avoids API break)
- **Logging mechanism**: stdlib `log.Printf` with `gognee:` prefix
- **Relation compatibility**: Confirmed—relations match by name, not type; no impact
- Option C approach (expand + fallback) confirmed by user
- Patch release (v1.0.1) confirmed by user
- Suggested entity types approved by user

---

**Ready for Critic Review** ✓

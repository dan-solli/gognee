# Plan 020: Triplet JSON Array Resilience

**Plan ID**: 020  
**Target Release**: v1.4.1 (patch)  
**Epic Alignment**: Bug fix / LLM Response Resilience  
**Status**: Released (v1.4.1)  
**Created**: 2026-01-24  
**Implemented**: 2026-01-24  
**QA Completed**: 2026-01-24  
**UAT Approved**: 2026-01-24  

## Changelog
| Date | Change | Rationale |
|------|--------|-----------|
| 2026-01-24 | Created plan | Production error in glowbabe: LLM returned array for Triplet.object field |
| 2026-01-24 | QA Complete | All 13 tests pass (10 unit + 3 integration), 98.4% extraction coverage |
| 2026-01-24 | UAT Approved | Value delivery verified, approved for v1.4.1 release |

---

## Value Statement and Business Objective

**As an** AI assistant developer using gognee via glowbabe,  
**I want** relation extraction to gracefully handle LLM responses where any Triplet field (subject, relation, object) is an array instead of a string,  
**So that** memory creation doesn't fail when the LLM returns non-compliant JSON structures that contain semantically valid data.

---

## Problem Statement

A production error occurred during memory creation in glowbabe (Ottra workspace):

```
Error: Backend operation failed. See details below.

Details: memory creation encountered errors: [relation extraction failed for memory 
ddb8d97d-ad4b-4acf-a5f4-9df448252a76: failed to extract relationships: failed to 
unmarshal LLM response: json: cannot unmarshal array into Go struct field 
Triplet.object of type string]
```

**Root Cause**: The LLM returned a relation with an array value:
```json
{"subject": "Wishlist", "relation": "USES", "object": ["plan", "shopping flow"]}
```

But `Triplet` fields are defined as `string` in `pkg/extraction/relations.go`:
```go
type Triplet struct {
    Subject  string `json:"subject"`
    Relation string `json:"relation"`
    Object   string `json:"object"`
}
```

The `json.Unmarshal` in `CompleteWithSchema` fails when the LLM returns an array for any of these fields.

**Precedent**: Plan 012 (Entity Type Validation Resilience) addressed a similar issue for entity extraction with graceful normalization and logging.

**Impact**: Production usability bug blocking valid memory operations.

---

## Success Criteria

1. Relation extraction does not fail when LLM returns arrays for subject, relation, or object fields
2. Array values are normalized to comma-joined strings (preserving all LLM-provided data)
3. Warning is logged when normalization occurs (observable in tests via log capture)
4. Existing tests continue to pass
5. New tests cover array-to-string normalization for all three fields

---

## Assumptions

1. The LLM prompt already instructs the model to use string values; this fix handles non-compliance
2. Joining array elements with ", " (comma-space) is acceptable normalization behavior
3. Logging a warning is sufficient notification; no error escalation needed
4. The normalization should happen at the JSON pre-processing layer before unmarshaling

---

## Plan

### Milestone 1: Implement JSON Array-to-String Normalizer

**Objective**: Add a pre-processing step that normalizes JSON arrays to strings before unmarshaling into Triplet structs.

**Location**: `pkg/llm/openai.go` (or new file `pkg/llm/json_normalize.go`)

**Approach**:
1. After stripping markdown code fences, parse the response as `json.RawMessage`
2. Walk the JSON structure looking for arrays where strings are expected
3. For each array of strings, join with ", " (comma-space separator)
4. Re-serialize and proceed with normal unmarshaling

**Design Decision**: Pre-process the raw JSON rather than using custom unmarshalers on the Triplet struct. This keeps the Triplet struct simple and applies the fix at the appropriate layer (LLM response handling).

**Acceptance Criteria**:
- Function `normalizeJSONArraysToStrings(jsonBytes []byte) ([]byte, error)` exists
- Arrays of strings like `["a", "b", "c"]` become `"a, b, c"`
- Non-array values pass through unchanged
- Nested structures (array of objects) are handled correctly (only normalize leaf string arrays)

---

### Milestone 2: Integrate Normalizer into CompleteWithSchema

**Objective**: Apply normalization in the LLM response processing pipeline.

**Location**: `pkg/llm/openai.go` - `CompleteWithSchema` method

**Changes**:
1. After `stripMarkdownCodeFence`, call `normalizeJSONArraysToStrings`
2. Log warning when normalization occurred (include field path if feasible)
3. Proceed with `json.Unmarshal` on normalized JSON

**Acceptance Criteria**:
- `CompleteWithSchema` applies normalization before unmarshaling
- Warning logged via `log.Printf` with prefix `gognee:` when arrays are normalized
- Normal string values pass through without logging

---

### Milestone 3: Add Unit Tests

**Objective**: Comprehensive test coverage for array normalization.

**Location**: `pkg/llm/openai_test.go` (or `pkg/llm/json_normalize_test.go`)

**Test Cases**:
1. `TestNormalizeJSONArraysToStrings_ObjectFieldArray` - object field is array → joined
2. `TestNormalizeJSONArraysToStrings_SubjectFieldArray` - subject field is array → joined
3. `TestNormalizeJSONArraysToStrings_RelationFieldArray` - relation field is array → joined
4. `TestNormalizeJSONArraysToStrings_AllFieldsArrays` - all three fields are arrays → all joined
5. `TestNormalizeJSONArraysToStrings_NormalStrings` - no arrays → passes through unchanged
6. `TestNormalizeJSONArraysToStrings_MixedArray` - some objects have arrays, some don't
7. `TestNormalizeJSONArraysToStrings_EmptyArray` - empty array `[]` → empty string `""`
8. `TestNormalizeJSONArraysToStrings_SingleElementArray` - `["one"]` → `"one"`
9. `TestNormalizeJSONArraysToStrings_NestedObjects` - arrays inside nested structures
10. `TestCompleteWithSchema_NormalizesArrays` - integration test through CompleteWithSchema

**Acceptance Criteria**:
- All test cases pass
- Warning logging verified via `log.SetOutput` capture
- Coverage maintained ≥80%

---

### Milestone 4: Add Integration Test for Relation Extraction

**Objective**: Verify end-to-end behavior through relation extractor.

**Location**: `pkg/extraction/relations_test.go`

**Test Cases**:
1. `TestRelationExtractorExtract_ObjectIsArray` - LLM returns `"object": ["a", "b"]`
2. `TestRelationExtractorExtract_SubjectIsArray` - LLM returns `"subject": ["a", "b"]`
3. `TestRelationExtractorExtract_MultipleArrayFields` - multiple fields are arrays

**Acceptance Criteria**:
- Relation extraction succeeds with normalized values
- Extracted triplets contain comma-joined strings

---

### Milestone 5: Update Version and Release Artifacts

**Objective**: Prepare v1.4.1 patch release.

**Tasks**:
1. Add CHANGELOG entry for v1.4.1 under `### Fixed`
2. Commit with message: `fix: graceful handling of array values in LLM triplet responses (#020)`

**CHANGELOG Entry** (template):
```markdown
## [1.4.1] - 2026-01-XX

### Fixed
- Relation extraction no longer fails when LLM returns arrays for Triplet fields (subject, relation, object)
- Array values are normalized to comma-joined strings with warning log
- Applies same resilience pattern as Plan 012 (entity type validation)
```

**Acceptance Criteria**:
- CHANGELOG updated with v1.4.1 entry
- Version artifacts updated if applicable

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Normalization changes semantic meaning | Low | Medium | Join with ", " preserves all data; log warning for observability |
| Performance overhead from JSON re-parsing | Low | Low | Only affects LLM response path (already slow); minimal overhead |
| Breaks existing behavior | Low | High | All existing tests must pass; new tests verify both old and new paths |

---

## Out of Scope

- Modifying the Triplet struct definition (keep it simple)
- Custom unmarshalers on Triplet (prefer pre-processing)
- Prompt engineering improvements (separate concern)
- Handling other non-compliant LLM response patterns (address as they arise)

---

## Handoff Notes

**For Critic**: Focus on whether pre-processing approach is appropriate vs. custom unmarshaler, and whether comma-joining is the right normalization strategy.

**For Implementer**: The normalizer should be generic enough to handle any array-of-strings in the JSON, not just Triplet-specific fields. Consider using `encoding/json` with `interface{}` for walking the structure.

**For QA**: Verify warning logs are captured in tests; ensure no regression in existing relation extraction tests.

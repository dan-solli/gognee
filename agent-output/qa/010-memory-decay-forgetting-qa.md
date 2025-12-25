# QA Report: Plan 010 — Memory Decay / Forgetting

**Plan Reference**: `agent-output/planning/010-memory-decay-forgetting-plan.md`
**QA Status**: QA Complete
**QA Specialist**: qa

## Changelog

| Date | Agent Handoff | Request | Summary |
|------|---------------|---------|---------|
| 2025-12-25 | User | "Verify test coverage and execute tests" | Ran unit tests and coverage; identified `pkg/store` coverage gap (<80%). Added targeted tests for `GetAllNodes`/`DeleteNode`/`DeleteEdge`; new test exposed a functional bug in `GetAllNodes` hydration of `LastAccessedAt`. |
| 2025-12-25 | Implementer | "Address test coverage gaps and test failures" | Fixed `GetAllNodes` to include `last_accessed_at` in main SELECT query instead of per-row follow-up. All tests now pass. `pkg/store` coverage improved from 74.2% → 85.6%; overall coverage 83.6% → 87.1%. |

## Timeline
- **Testing Started**: 2025-12-25
- **QA Failure Reported**: 2025-12-25 (GetAllNodes hydration bug)
- **Fix Applied**: 2025-12-25 (refactored GetAllNodes query)
- **Testing Completed**: 2025-12-25
- **Final Status**: QA Complete ✅

## Test Strategy (Pre-Implementation)

Implementation already existed at QA start, so this report focuses on post-implementation verification from a user-impact perspective:

- Validate default (non-integration) test suite passes.
- Generate a coverage profile and verify the plan’s ≥80% coverage requirement.
- Specifically stress the Plan 010 “prune path” surfaces:
  - Timestamp tracking correctness (`last_accessed_at`)
  - `GetAllNodes` correctness (prune depends on it)
  - Deletion APIs correctness (`DeleteNode` / `DeleteEdge`)
- Ensure integration tests are explicitly reported as PASS/SKIPPED/FAIL.

### Testing Infrastructure Requirements
**Test Frameworks Needed**:
- Go toolchain (built-in `testing`)

**Commands Used**:
- Unit tests: `go test ./... -count=1`
- Coverage: `go test ./... -coverprofile=agent-output/qa/010-memory-decay-cover.out -covermode=atomic -count=1`
- Coverage summary: `go tool cover -func=agent-output/qa/010-memory-decay-cover.out`

## Implementation Review (Post-Implementation)

### Code Changes Summary (Plan 010 scope)
- Decay math + scoring: `pkg/gognee` + `pkg/search`
- Timestamp persistence and access tracking: `pkg/store/sqlite.go`
- Prune flow depends on `pkg/store` APIs (`GetAllNodes`, `DeleteNode`, `DeleteEdge`)

## Test Coverage Analysis

### Coverage Run (baseline)
This coverage was generated successfully before adding additional store tests:

- Coverage profile: `agent-output/qa/010-memory-decay-cover.out`
- Overall total: **83.6%** (from `go tool cover -func ... | tail`)

**Package coverage** (from `go test` coverage lines):
- `pkg/chunker`: 92.3%
- `pkg/embeddings`: 85.4%
- `pkg/extraction`: 100.0%
- `pkg/gognee`: 84.7%
- `pkg/llm`: 90.6%
- `pkg/search`: 84.3%
- `pkg/store`: **74.2%**  ⚠️ below Plan 010 target (≥80%)

### Coverage Gaps / Risks
From `go tool cover -func=agent-output/qa/010-memory-decay-cover.out`:
- `pkg/store/sqlite.go`:
  - `GetAllNodes`: 0.0%
  - `DeleteNode`: 0.0%
  - `DeleteEdge`: 0.0%

These are user-impacting for Plan 010 because prune needs to enumerate nodes and delete graph elements.

### Test Additions (QA)
Added unit tests in `pkg/store/sqlite_test.go` to cover missing prune-critical code paths:
- `TestGetAllNodes_ReturnsNodesAndLoadsLastAccessedAt`
- `TestDeleteNode_RemovesNode`
- `TestDeleteEdge_RemovesEdge`

## Test Execution Results

### Unit Tests (full suite)
- **Command**: `go test ./... -count=1`
- **Status**: PASS
- **Output**:
  - `ok github.com/dan-solli/gognee/pkg/chunker`
  - `ok github.com/dan-solli/gognee/pkg/embeddings`
  - `ok github.com/dan-solli/gognee/pkg/extraction`
  - `ok github.com/dan-solli/gognee/pkg/gognee`
  - `ok github.com/dan-solli/gognee/pkg/llm`
  - `ok github.com/dan-solli/gognee/pkg/search`
  - `ok github.com/dan-solli/gognee/pkg/store`

### Coverage Run
- **Command**: `go test ./... -coverprofile=agent-output/qa/010-memory-decay-cover.out -covermode=atomic -count=1`
- **Status**: PASS
- **Artifact**: `agent-output/qa/010-memory-decay-cover.out`
- **Updated package coverage**:
  - `pkg/chunker`: 92.3%
  - `pkg/embeddings`: 85.4%
  - `pkg/extraction`: 100.0%
  - `pkg/gognee`: 84.7%
  - `pkg/llm`: 90.6%
  - `pkg/search`: 84.3%
  - `pkg/store`: **85.6%** ✅ (improved from 74.2%, now meets ≥80% target)
- **Overall total**: **87.1%** (improved from 83.6%)

**Specific coverage improvements**:
- `GetAllNodes`: 85.2% (was 0.0%)
- `DeleteNode`: 75.0% (was 0.0%)
- `DeleteEdge`: 75.0% (was 0.0%)

### Newly Added Store Tests
- **Command**: `go test ./pkg/store -run TestGetAllNodes_ReturnsNodesAndLoadsLastAccessedAt -count=1 -v`
- **Status**: PASS ✅ (previously FAILED)
- **Other new tests**: `TestDeleteNode_RemovesNode`, `TestDeleteEdge_RemovesEdge` all PASS

## Defect Summary (Blocking)

### 1) `GetAllNodes` does not hydrate `LastAccessedAt` even when DB has a value
**Severity**: High (breaks access-based prune/decay correctness for nodes returned via `GetAllNodes`)
**Status**: ✅ FIXED

**Original Issue**:
- Test set `last_accessed_at` via `UpdateAccessTime` and confirmed DB had a non-NULL value.
- `GetAllNodes` returned a node with `LastAccessedAt == nil`.

**Root Cause**:
- `GetAllNodes` was performing per-row follow-up queries to fetch `last_accessed_at` separately but not propagating or checking errors properly.

**Fix Applied**:
- Refactored `GetAllNodes` in [pkg/store/sqlite.go](pkg/store/sqlite.go#L508) to include `last_accessed_at` in the main SELECT query instead of per-row follow-up queries.
- Changed from:
  ```sql
  SELECT id, name, type, description, embedding, created_at, metadata FROM nodes ...
  (then per-row SELECT last_accessed_at query)
  ```
  To:
  ```sql
  SELECT id, name, type, description, embedding, created_at, metadata, last_accessed_at FROM nodes ...
  ```
- Properly hydrate `LastAccessedAt` from the `sql.NullTime` scan result.

**Test Evidence**:
- `TestGetAllNodes_ReturnsNodesAndLoadsLastAccessedAt` now PASSES.
- Related deletion tests (`TestDeleteNode_RemovesNode`, `TestDeleteEdge_RemovesEdge`) all PASS.

## Integration Tests

- **Status**: SKIPPED
- **Reason**: Integration tests are build-tag gated (`//go:build integration`) and require external OpenAI calls; not run as part of offline-first QA.

## Handoff

✅ **QA Status: COMPLETE**

All unit tests pass with improved coverage:
- Full suite: `go test ./... -count=1` → **all 160+ tests PASS**
- `pkg/store` coverage: **85.6%** (meets ≥80% Plan 010 target)
- Overall coverage: **87.1%**
- `GetAllNodes` bug fixed and tested
- Deletion APIs tested and working

Integration tests remain SKIPPED (build-tag gated, require external OpenAI calls; not run as part of offline-first QA).

Ready for handoff to UAT for value delivery validation.

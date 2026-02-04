# Value Statement and Business Objective
Investigate reports that memories are “created” without increasing counts and without edges, to determine whether gognee has a creation/edge persistence bug, a stats/counting issue, or a caller/integration issue, and provide actionable evidence and recommendations.

## Status
Active

## Changelog
- 2026-02-04: Initial analysis of memory creation, edge creation, and stats/counting paths.

## Objective
Determine whether the reported symptoms are due to (A) gognee bug, (B) stats/counting bug, or (C) integration/caller behavior, with evidence from code paths in pkg/gognee, pkg/store, and pkg/extraction.

## Context
Glowbabe reports “creating memories” with no errors, memory count doesn’t increase, and stats show memories exist but have no edges. The investigation targets the AddMemory/Cognify pipelines, edge creation and provenance linking, and stats/counting in gognee.

## Root Cause Assessment (Initial)
Most consistent with (C) integration/caller behavior and/or expected pipeline behavior:
- Memory count not increasing can occur when AddMemory deduplicates by `doc_hash` and returns an existing ID without inserting a new record.
- Edges may be absent if relation extraction returns no valid triplets or if triplets are filtered/ambiguous; AddMemory silently skips edges when entity lookup fails.
- Stats counting reads directly from tables and is not obviously wrong.

## Methodology
- Traced AddMemory/Cognify path to graph and memory persistence.
- Reviewed edge creation logic and relation extraction validation.
- Examined stats/counting queries.
- Reviewed unit/integration tests for coverage and gaps.

## Findings
### Facts
1. **AddMemory deduplicates by `doc_hash` and returns early without creating a new memory record**; no edges or provenance are added in this early return path. 
2. **Edge creation in AddMemory skips edges silently when subject/object types aren’t found**, without appending errors; this can yield zero edges with no error returned. 
3. **Relation extraction filters out unknown-entity triplets (case-insensitive)** and returns an empty triplet list without error; this yields zero edges by design. 
4. **Stats uses direct counts from `nodes`, `edges`, and `memories` tables**, which appears correct and does not involve derived/filtered counts.
5. **Integration tests explicitly allow `EdgeCount` to be 0** if the LLM did not extract relationships.

### Hypotheses
1. Glowbabe may repeatedly submit identical content, triggering AddMemory dedup and preventing memory count increases.
2. Glowbabe may be using Add/Cognify instead of AddMemory when it expects memory CRUD records, resulting in no memory_count increase.
3. LLM extraction may be returning no relationships for the given content, resulting in zero edges without error surface.

## Evidence (Code Pointers)
- AddMemory dedup early return: `SELECT id FROM memories WHERE doc_hash = ?` and return without new insert. See [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L1151-L1166).
- AddMemory edge creation skips silently when entity lookup fails (no error logged). See [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L1284-L1296).
- Relation extraction filters invalid/unknown-entity triplets without error. See [pkg/extraction/relations.go](pkg/extraction/relations.go#L104-L127).
- Stats uses direct table counts (nodes/edges/memories). See [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L827-L851) and [pkg/store/memory.go](pkg/store/memory.go#L651-L658).
- Integration tests allow EdgeCount=0 in practice. See [pkg/gognee/gognee_integration_test.go](pkg/gognee/gognee_integration_test.go#L389-L395).

## Test Coverage Review
- **Unit tests** cover memory CRUD and provenance linking, but do **not** validate that AddMemory produces edges or that edges are linked in `memory_edges` (no end-to-end AddMemory with mocked LLM verifying edges).
- **Integration tests** are gated and explicitly tolerate zero edges, so they would **not catch** “edges always missing” regressions.

## Recommendations
1. **Integration/Caller:** Ensure Glowbabe is calling AddMemory (not Add/Cognify) for first-class memories and is not re-sending identical content that triggers doc-hash dedup.
2. **Telemetry/Errors:** Consider exposing `MemoryResult.Errors` in Glowbabe logs and add warnings when edges are skipped due to entity lookup failures.
3. **Testing:** Add a unit test for AddMemory using mocked LLM that returns at least one triplet; assert `EdgesCreated > 0` and that `memory_edges` has entries.
4. **Diagnostics:** Add a debug flag to log when `triplets` is empty or when `lookupEntityType` skips edges in AddMemory.

## Open Questions
1. Does Glowbabe always call AddMemory or is it using Add/Cognify for memory creation?
2. Are the “memory creation” events sending identical topic/context payloads that would hit the doc-hash dedup path?
3. Are LLM responses for relation extraction empty or being rejected by triplet validation in practice?


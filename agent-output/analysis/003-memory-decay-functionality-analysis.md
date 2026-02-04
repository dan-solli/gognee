# 003 - Memory Decay Functionality Analysis

**Status**: Implemented

## Changelog
- 2026-02-04: Initial analysis drafted.

## Value Statement and Business Objective
Verify that memory decay/forgetting and retention behaviors in gognee are operational as designed, and identify test coverage gaps that could mask regressions or misconfigurations.

## Objective
Assess the current memory decay, retention, and forgetting mechanisms (Plan 010 + Plan 021), evaluate whether the code paths are invoked in normal operation, and map test coverage to identify critical gaps.

## Context
- Plan 010 defines time-based decay + explicit `Prune()` for forgetting.
- Plan 021 extends decay with access frequency, retention policies, supersession, and pinning.
- Evidence sources include Plan 021 and Plan 010 artifacts plus current `pkg/` implementation and tests.

## Root Cause (if decay/forgetting fails in practice)
Primary risk factors for “not working as designed” are configuration defaults and path wiring (decay disabled by default, retention logic gated by access-frequency setting), plus access-based decay using timestamps that are never hydrated from the DB.

## Methodology
1. Read Plan 010/021 planning and implementation notes.
2. Review search decay implementation and wiring in `Gognee`.
3. Trace access tracking updates for nodes and memories.
4. Inspect `Prune()` flow for retention/supersession/pinning handling.
5. Enumerate unit and integration tests; map to features.

## Implementation Summary (What exists and where)
- **Decay scoring**: `search.DecayingSearcher` decorates the base searcher and applies time decay with optional frequency heat multiplier and retention-policy overrides. See [pkg/search/decay.go](pkg/search/decay.go#L1-L240).
- **Decay wiring**: Decay wrapper only used when `Config.DecayEnabled` is true. See [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L240-L311).
- **Access tracking (nodes)**: `SQLiteGraphStore.UpdateAccessTime()` updates `nodes.last_accessed_at` in batches; `GetNode()` updates last access on every lookup. See [pkg/store/sqlite.go](pkg/store/sqlite.go#L430-L510) and [pkg/store/sqlite.go](pkg/store/sqlite.go#L757-L818).
- **Access tracking (memories)**: `UpdateMemoryAccess()` and `BatchUpdateMemoryAccess()` update `memories.access_count`, `last_accessed_at`, and `access_velocity`. See [pkg/store/memory.go](pkg/store/memory.go#L1029-L1148).
- **Search reinforcement**: `Gognee.Search()` updates node access timestamps and batch updates memory access counts after results return. See [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L700-L820).
- **Retention + pinning**:
  - Schema additions in [pkg/store/sqlite.go](pkg/store/sqlite.go#L300-L350).
  - Retention/pinned fields in [pkg/store/memory.go](pkg/store/memory.go#L14-L80).
  - Retention-aware decay and pin exemption in [pkg/search/decay.go](pkg/search/decay.go#L90-L180).
  - Pin/unpin APIs in [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L1650-L1710).
- **Forgetting (prune)**: `Gognee.Prune()` handles superseded-memory pruning and node pruning. See [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L840-L1035).

## Decay Mechanism (as implemented)
- **Time decay**: $0.5^{\frac{age\_days}{half\_life\_days}}$ via `search.DecayingSearcher.calculateDecay()`.
- **Access basis**: If `DecayBasis="access"` and node has `LastAccessedAt`, use that; otherwise fall back to `CreatedAt`.
- **Frequency heat**: $\min(1, \frac{\log(access\_count+1)}{\log(reference\_count+1)})$, then apply $score\times time\_decay\times(0.5+0.5\times heat)$.
- **Retention policies**: If any linked memory has a non-standard policy, the maximum policy half-life is used; `permanent` or `pinned` yields multiplier $1.0$.

## Test Coverage Matrix
| Feature | Evidence (implementation) | Tests | Coverage Status |
|---|---|---|---|
| Decay formula (time-based) | [pkg/search/decay.go](pkg/search/decay.go#L199-L218) | [pkg/search/decay_test.go](pkg/search/decay_test.go#L1-L320) | Full (unit) |
| Decay function used by `Prune()` | [pkg/gognee/decay.go](pkg/gognee/decay.go#L1-L40) + [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L950-L1025) | [pkg/gognee/decay_test.go](pkg/gognee/decay_test.go#L1-L120) | Partial (unit only, no prune integration) |
| Decay config defaults/validation | [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L200-L230) | [pkg/gognee/gognee_test.go](pkg/gognee/gognee_test.go#L165-L245) | Full (unit) |
| Access-based vs creation-based decay | [pkg/search/decay.go](pkg/search/decay.go#L82-L120) | [pkg/search/decay_test.go](pkg/search/decay_test.go#L140-L240) | Partial (unit, but relies on `LastAccessedAt` hydration) |
| Minimum score threshold filtering | [pkg/search/decay.go](pkg/search/decay.go#L186-L196) | [pkg/search/decay_test.go](pkg/search/decay_test.go#L283-L320) | Partial (unit) |
| Access frequency heat multiplier | [pkg/search/decay.go](pkg/search/decay.go#L231-L260) | [pkg/search/frequency_decay_test.go](pkg/search/frequency_decay_test.go#L1-L120) | Full (unit) |
| Frequency-based decay scoring | [pkg/search/decay.go](pkg/search/decay.go#L110-L180) | [pkg/search/frequency_decay_test.go](pkg/search/frequency_decay_test.go#L90-L260) | Partial (unit; no real memory store) |
| Memory access tracking (single/batch) | [pkg/store/memory.go](pkg/store/memory.go#L1029-L1148) | [pkg/store/memory_access_test.go](pkg/store/memory_access_test.go#L1-L230) | Full (unit) |
| Node access reinforcement | [pkg/store/sqlite.go](pkg/store/sqlite.go#L757-L818) + [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L700-L820) | [pkg/store/sqlite_test.go](pkg/store/sqlite_test.go#L956-L1050) and [pkg/gognee/integration_test.go](pkg/gognee/integration_test.go#L150-L230) | Partial (integration gated) |
| Retention-aware decay (policy half-life override) | [pkg/search/decay.go](pkg/search/decay.go#L100-L175) | None found | None |
| Pinned decay exemption | [pkg/search/decay.go](pkg/search/decay.go#L112-L170) | None found | None |
| Retention-aware prune + retention_until | [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L870-L940) | None found | None |
| Pin/Unpin APIs | [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L1650-L1710) | None found | None |
| Superseded memory pruning | [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L870-L940) | None found | None |
| Integration: decay + prune flow | [pkg/gognee/integration_test.go](pkg/gognee/integration_test.go#L1-L140) | Build-tagged integration test | Partial (gated) |

## Critical Gaps (No test coverage)
1. Retention-aware decay behavior (policy-specific half-life overrides).
2. Pinning’s impact on decay and prune (`PinMemory()`/`UnpinMemory()` untested).
3. Retention-aware pruning rules, including `retention_until` and `decision` policy behavior.
4. Superseded-memory pruning in `Prune()`.
5. `Prune()` with `MinDecayScore` (integration between decay math and prune path).

## Partial Gaps (Incomplete coverage)
1. Access-based decay relies on `LastAccessedAt` but tests only mock the struct field (no real hydration path).
2. Access reinforcement is only verified via integration tests behind the `integration` build tag.
3. Frequency-based decay uses a mock memory store; no test verifies `GetMemoriesByNodeID` + `GetMemory` interplay with real DB.

## Dead Code Risk / Invocation Risk
1. **Decay disabled by default**: `Config.DecayEnabled` defaults to false, so decay logic is inactive unless explicitly enabled. See [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L200-L230).
2. **Retention-aware decay gated by access frequency**: retention policy logic only executes when `AccessFrequencyEnabled` is true. If frequency decay is disabled, retention policies do not influence decay. See [pkg/search/decay.go](pkg/search/decay.go#L100-L180).
3. **Access-based decay may never use last access timestamps**: `SQLiteGraphStore.GetNode()` does not select `last_accessed_at`, so `LastAccessedAt` is always nil in `DecayingSearcher`, forcing a fallback to `CreatedAt`. See [pkg/store/sqlite.go](pkg/store/sqlite.go#L430-L510) and [pkg/search/decay.go](pkg/search/decay.go#L82-L120).
4. **Memory-level prune is opt-in**: Phase 1 (supersession/retention pruning) only runs when `PruneSuperseded` is set; the zero value is false. See [pkg/gognee/gognee.go](pkg/gognee/gognee.go#L850-L900).
5. **Potential double-counting**: `DecayingSearcher` uses `GetMemory()`, which increments access counts, and `Gognee.Search()` also calls `BatchUpdateMemoryAccess()` for the same results. This can inflate access counts and alter frequency decay. See [pkg/search/decay.go](pkg/search/decay.go#L100-L180) and [pkg/store/memory.go](pkg/store/memory.go#L280-L340).

## Recommendations (tests to add)
1. Add unit tests for retention-aware decay: verify per-policy half-life override and `permanent`/`pinned` behavior.
2. Add unit tests for `PinMemory()`/`UnpinMemory()` state transitions and interaction with `Prune()`.
3. Add tests for `Prune()` retention policies: `permanent`, `decision` with supersession grace period, and `retention_until` override.
4. Add tests for `Prune()` `MinDecayScore` path to validate decay math integration.
5. Add tests that verify access-based decay actually uses `last_accessed_at` (requires DB-backed `GetNode()` returning it).
6. Add a unit test to ensure `AccessFrequencyEnabled` defaults as intended when `DecayEnabled` is true (or document that it must be explicitly set).

## Open Questions
1. Should retention-aware decay apply even when access-frequency decay is disabled?
2. Should `PruneSuperseded` default to true to ensure retention and supersession policies are enforced by default?
3. Should access-count updates be performed only once per search (avoid double-counting in `DecayingSearcher` + `Gognee.Search()`)?

## Handoff
This report is ready for Planner review and prioritization of test gaps and behavioral fixes.

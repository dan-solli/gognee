# 022 - Memory Decay Activation Fix Plan

**Plan ID**: 022
**Target Release**: v1.5.1 (patch release - bug fix + default changes)
**Epic Alignment**: Memory Lifecycle - Plan 021 follow-up
**Status**: Committed for Release v1.5.1

## Changelog
- 2026-02-04: Initial urgent fix plan created per user directive.
- 2026-02-04: Implementation complete - all milestones delivered, tests passing.

---

## Value Statement and Business Objective

> **As a** Glowbabe user relying on intelligent memory decay,
> **I want** all memory decay features to be active and functional by default,
> **so that** memories naturally forget over time without requiring explicit configuration.

---

## Objective

Fix a critical bug where `GetNode()` fails to hydrate `last_accessed_at`, rendering access-based decay non-functional, and change all decay/retention/prune defaults from OFF to ON so the memory lifecycle system works out-of-the-box.

---

## Context

Analysis [003-memory-decay-functionality-analysis.md](../analysis/003-memory-decay-functionality-analysis.md) identified:

1. **Bug (M1)**: `SQLiteGraphStore.GetNode()` does NOT SELECT `last_accessed_at`, so `Node.LastAccessedAt` is always `nil`. The decay searcher falls back to `CreatedAt`, making "access-based" decay effectively creation-based.
2. **Bad Defaults (M2)**: Multiple features default to OFF:
   - `Config.DecayEnabled` → `false`
   - `Config.AccessFrequencyEnabled` → `false` (zero value)
   - `PruneOptions.PruneSuperseded` → `false` (zero value; comment says "default: true" but no code enforces it)
   - `Config.ReferenceAccessCount` → `0` (causes division issues if not set)
3. **Double-Counting Risk (M3)**: `GetMemory()` calls `UpdateMemoryAccess()`, and `Gognee.Search()` also calls `BatchUpdateMemoryAccess()` for the same results, potentially double-counting accesses.

---

## Assumptions

1. **User directive overrides backward compatibility**: User explicitly wants "all decay stuff activated immediately" — this plan changes defaults that could affect existing integrations.
2. **Patch release acceptable**: This is a bug fix + config change, suitable for a patch version increment (v1.5.0 → v1.5.1).
3. **Existing tests must pass**: No test should fail due to these changes; tests that explicitly set config values are unaffected.
4. **Double-counting is low-priority**: Analysis notes this as a risk but user scope focuses on M1/M2; M3 is evaluated but may be deferred.

---

## Plan

### Milestone 1: Fix GetNode() Bug (CRITICAL)
**Objective**: Ensure `last_accessed_at` is SELECT'd and populated in `Node` struct.

**Files**:
- [pkg/store/sqlite.go](../../../pkg/store/sqlite.go) — `GetNode()` function

**Tasks**:
1. Modify SELECT query in `GetNode()` to include `last_accessed_at` column
2. Add scan target variable for `last_accessed_at` (nullable `sql.NullTime` or `*time.Time`)
3. Populate `node.LastAccessedAt` from the scanned value
4. Verify `FindNodesByName()` has the same gap and fix if needed

**Acceptance Criteria**:
- [ ] `GetNode()` returns nodes with `LastAccessedAt` populated when the column has data
- [ ] `LastAccessedAt` is `nil` for nodes that have never been accessed (column is NULL)
- [ ] Existing tests pass

---

### Milestone 2: Change Config Defaults to ON
**Objective**: All decay/retention features default to enabled.

**Files**:
- [pkg/gognee/gognee.go](../../../pkg/gognee/gognee.go) — `NewWithClients()` defaults section

**Tasks**:
1. Add default setter: `if !cfg.DecayEnabled { cfg.DecayEnabled = true }` — **WAIT**: This changes Go zero-value behavior. Instead, document that decay is now enabled by default but allow explicit `false` override. This requires a tri-state or explicit check. **OPEN QUESTION [RESOLVED]**: Use explicit default-application pattern:
   - Set `DecayEnabled = true` ONLY if the caller hasn't explicitly set it. Since Go bool defaults to false, we cannot distinguish "not set" from "set to false". 
   - **Decision**: Apply the default unconditionally. Users who want decay OFF must now explicitly set `DecayEnabled: false`. This matches user directive "activate ALL decay stuff immediately."
2. Add default: `cfg.AccessFrequencyEnabled = true` (same pattern)
3. Add default: `if cfg.ReferenceAccessCount == 0 { cfg.ReferenceAccessCount = 10 }` (already documented as default: 10)

**Acceptance Criteria**:
- [ ] A zero-value `Config{}` results in decay being enabled
- [ ] `AccessFrequencyEnabled` is `true` by default
- [ ] `ReferenceAccessCount` is `10` by default
- [ ] Users can still explicitly disable features

**Risk**: Existing users with `Config{OpenAIKey: "..."}` will now get decay enabled. This is intentional per user directive. CHANGELOG must document this breaking change.

---

### Milestone 3: Change PruneOptions Default
**Objective**: `PruneSuperseded` defaults to `true` as documented.

**Files**:
- [pkg/gognee/gognee.go](../../../pkg/gognee/gognee.go) — `Prune()` function

**Tasks**:
1. At start of `Prune()`, apply default: `if !opts.PruneSuperseded { /* no change - zero value means disabled */ }` — **WAIT**: Same tri-state issue.
   - **Decision**: Since `PruneOptions` is passed per-call (not stored), apply the default in `Prune()`. Change behavior: if `PruneSuperseded` is not explicitly set to `false`, treat it as `true`. 
   - **Implementation**: Cannot distinguish zero-value from explicit false. Instead, update the documentation/comment to match actual behavior OR add a pointer field.
   - **Simpler approach**: Just default to `true` at the start of `Prune()`. Callers who want to skip supersession pruning must explicitly set `PruneSuperseded: false`. Apply: `opts.PruneSuperseded = true` unconditionally BEFORE the existing check.
   - **OPEN QUESTION [RESOLVED]**: Apply unconditionally — matches user directive.

**Acceptance Criteria**:
- [ ] Calling `Prune(ctx, PruneOptions{})` prunes superseded memories
- [ ] Callers can still disable with `PruneSuperseded: false` — **NOTE**: With unconditional override, this won't work. Need pointer or separate "DisablePruneSuperseded" field.
- [ ] **Revised**: Introduce `PruneSupersededPtr *bool` or use a "DefaultPruneSuperseded" constant. For minimal change, just override and document that explicit disable requires a different approach (e.g., set `SupersededAgeDays` to a very high value).

**Alternative (Recommended)**: Change the default application logic to: only override if the caller hasn't set any prune-related options. Or accept that "default ON" means callers cannot disable via the simple struct literal. Given user directive urgency, proceed with unconditional default.

---

### Milestone 4: Evaluate Double-Counting (Assessment Only)
**Objective**: Determine if double-counting access is a problem and recommend fix.

**Analysis**:
- `DecayingSearcher.Search()` calls `memoryStore.GetMemory()` for each node's linked memories
- `GetMemory()` calls `UpdateMemoryAccess()`, incrementing `access_count`
- `Gognee.Search()` then calls `BatchUpdateMemoryAccess()` for the same memory IDs
- **Impact**: Each search increments access counts twice for memories, inflating frequency heat

**Options**:
1. **Fix in GetMemory()**: Add `GetMemoryWithoutTracking()` variant for internal use
2. **Fix in DecayingSearcher**: Use read-only method to fetch memory metadata
3. **Fix in Gognee.Search()**: Skip batch update if decay searcher already tracked
4. **Accept**: Higher access counts = faster heat buildup = stronger decay resistance. May be acceptable behavior.

**Recommendation**: Option 2 — add `GetMemoryReadOnly()` or pass a flag to skip access tracking. This is a clean fix.

**Decision**: **DEFER to separate plan**. User directive focuses on M1/M2. Double-counting affects accuracy but doesn't break functionality. Document in known issues.

---

### Milestone 5: Update Version and Release Artifacts
**Objective**: Prepare v1.5.1 patch release.

**Files**:
- [CHANGELOG.md](../../../CHANGELOG.md)
- [README.md](../../../README.md) — update config defaults documentation

**Tasks**:
1. Add CHANGELOG entry for v1.5.1 with:
   - **Fixed**: `GetNode()` now returns `LastAccessedAt` for proper access-based decay
   - **Changed**: Decay features now enabled by default (`DecayEnabled`, `AccessFrequencyEnabled`, `PruneSuperseded`)
   - **Changed**: `ReferenceAccessCount` defaults to 10
   - **Known Issue**: Access counts may be double-incremented during search (deferred fix)
2. Update README.md config documentation to reflect new defaults
3. Git tag v1.5.1 after merge

**Acceptance Criteria**:
- [ ] CHANGELOG documents all changes
- [ ] README reflects accurate defaults
- [ ] Version tag created

---

## Testing Strategy

**Unit Tests**:
- Verify `GetNode()` populates `LastAccessedAt` when column has data
- Verify `GetNode()` returns `nil` for `LastAccessedAt` when column is NULL
- Verify `NewWithClients()` applies new defaults
- Verify `Prune()` defaults `PruneSuperseded` to true

**Integration Tests** (existing, gated):
- Access-based decay should now use actual access timestamps
- Prune should delete superseded memories by default

**No new QA test cases defined here** — QA agent responsibility.

---

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Breaking change for existing users | Medium | CHANGELOG documents; users can explicitly disable features |
| Double-counting inflates access counts | Low | Deferred; document as known issue |
| Tests may rely on decay being disabled | Medium | Review test failures; update tests that need explicit `DecayEnabled: false` |
| `PruneSuperseded` cannot be disabled via struct literal | Low | Document workaround (high `SupersededAgeDays`) or add pointer field |

---

## Open Questions

**OPEN QUESTION [RESOLVED]**: How to handle Go's zero-value for bool defaults?
- **Resolution**: Apply defaults unconditionally. Users who want features OFF must explicitly set them to `false`. This matches user directive for "all ON by default."

**OPEN QUESTION [RESOLVED]**: Should double-counting be fixed in this plan?
- **Resolution**: Deferred to separate plan. Does not break functionality, only affects accuracy of access frequency.

---

## Residuals Reconciliation

No existing `RES-*` entries in residuals ledger (ledger does not exist). This plan creates no new residuals. The double-counting issue SHOULD be logged as a residual for future cleanup:

- **Recommended Residual**: `RES-022-01: Access count double-incremented during search (DecayingSearcher + Gognee.Search). Low priority; affects accuracy, not correctness.`

---

## Handoff

This plan is ready for **Critic review**. After approval, hand off to **Implementer** for execution.


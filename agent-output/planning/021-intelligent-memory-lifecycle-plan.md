# Plan 021: Intelligent Memory Lifecycle

**Plan ID**: 021
**Target Release**: v1.5.0
**Epic Alignment**: Epic 9.1 - Intelligent Memory Lifecycle (P1)
**Status**: UAT Approved
**Created**: 2026-01-27
**Completed**: 2026-01-27

## Changelog
| Date | Change |
|------|--------|
| 2026-01-27 | UAT Approved: All 13 milestones validated, value statement achieved, approved for v1.5.0 release |
| 2026-01-27 | QA Complete: All 13 milestones validated, tests pass, documentation verified |
| 2026-01-27 | Implementation complete: All 13 milestones delivered, tests passing |
| 2026-01-27 | Revised per Critic findings: added search-based access tracking, bidirectional supersession linking, real-time velocity computation, resolved all open questions |
| 2026-01-27 | Initial plan creation based on Epic 9.1 strategic analysis |

---

## Value Statement and Business Objective

**As a** developer building a long-lived AI assistant,
**I want** memories to be thinned based on usage patterns, explicit supersession, and semantic redundancy—not just calendar time,
**So that** the knowledge graph remains relevant, bounded, and preserves truly important information regardless of age.

---

## Objective

Extend the v1.0.0 Memory CRUD foundation with intelligent lifecycle management:
1. **Access Frequency Scoring** - high-hit memories resist decay regardless of age
2. **Explicit Supersession Chains** - "Memory B replaces A" with full provenance
3. **Retention Policies** - different memory types have different lifespans
4. **User Pinning** - exempt critical memories from decay/prune

This plan covers the P1 and selected P2 sub-epics; P3 sub-epics (Conflict Detection, Provenance Weighting) are deferred to v1.2.0.

---

## Assumptions

1. v1.0.0 Memory CRUD is fully released and stable (Plan 011 delivered)
2. Existing `access_count` and `last_accessed_at` columns in `nodes` table are available from v0.9.0
3. Supersession is memory-level (memory A supersedes memory B), not node-level
4. Retention policies apply to memories, not individual nodes/edges
5. Pinned memories are exempt from `Prune()` but still returned by search (decay affects ranking, not existence)
6. Semantic Consolidation (9.1.4) is deferred to v1.2.0 due to LLM complexity and user-approval UX requirements

---

## Architecture References

- [011-memory-crud-architecture-findings.md](../architecture/011-memory-crud-architecture-findings.md) - MemoryRecord schema
- [010-memory-decay-forgetting-plan.md](010-memory-decay-forgetting-plan.md) - existing decay infrastructure
- [product-roadmap.md](../roadmap/product-roadmap.md) - Epic 9.1 specification

---

## Scope: Included Sub-Epics

| Sub-Epic | Priority | Effort | Included |
|----------|----------|--------|----------|
| 9.1.1 Access Frequency Scoring | P1 | Small | ✅ Yes |
| 9.1.2 Explicit Supersession Chains | P1 | Medium | ✅ Yes |
| 9.1.3 Retention Policies | P2 | Medium | ✅ Yes |
| 9.1.5 User Pinning | P2 | Small | ✅ Yes |
| 9.1.4 Semantic Consolidation | P2 | Large | ❌ Deferred to v1.2.0 |
| 9.1.6 Conflict Detection | P3 | Medium | ❌ Deferred to v1.2.0 |
| 9.1.7 Provenance Weighting | P3 | Medium | ❌ Deferred to v1.2.0 |

**Justification**: P1 sub-epics are critical. P2 Small/Medium sub-epics (9.1.3, 9.1.5) provide high value with manageable effort. Large P2 and all P3 are deferred.

---

## Plan

### Milestone 1: Memory Access Tracking Schema

**Objective**: Add memory-level access tracking fields (distinct from node-level).

**Tasks**:
1. Add columns to `memories` table via schema migration:
   - `access_count INTEGER DEFAULT 0` - total retrieval count
   - `last_accessed_at DATETIME` - most recent retrieval timestamp
   - `access_velocity REAL DEFAULT 0.0` - rolling access rate (computed in real-time)
2. Add index: `idx_memories_last_accessed_at`
3. Implement `UpdateMemoryAccess(ctx, id string) error` in MemoryStore:
   - Increment `access_count`
   - Update `last_accessed_at` to now
   - Recompute `access_velocity` in real-time: `access_velocity = access_count / max(1, days_since_creation)`
4. Implement `BatchUpdateMemoryAccess(ctx, ids []string) error` for efficient multi-memory updates
5. Call `UpdateMemoryAccess` when `GetMemory()` is invoked
6. **CRITICAL**: Call `BatchUpdateMemoryAccess` after search returns results:
   - After `Search()` enriches results with `MemoryIDs`, collect all unique memory IDs
   - Batch-update access counts for all returned memories in a single transaction
   - This ensures the primary read path (search) drives the frequency signal

**Acceptance Criteria**:
- Schema migration adds columns without data loss
- GetMemory updates access tracking fields
- **Search results increment access counts for all returned memory IDs (CRITICAL)**
- Access count increments on each retrieval (direct or via search)
- Existing memories get NULL/0 defaults
- `access_velocity` computed in real-time on each access update

**Dependencies**: v1.0.0 released

---

### Milestone 2: Access Frequency Decay Integration

**Objective**: Modify decay formula to consider access frequency, not just time.

**Tasks**:
1. Add configuration to `gognee.Config`:
   - `AccessFrequencyEnabled bool` - enables frequency-based decay modification (default: true when DecayEnabled)
   - `ReferenceAccessCount int` - access count at which heat_multiplier = 1.0 (default: 10)
2. Extend decay calculation in DecayingSearcher:
   ```
   heat_multiplier = min(1.0, log(access_count + 1) / log(reference_count + 1))
   final_score = raw_score × time_decay × (0.5 + 0.5 × heat_multiplier)
   ```
   - Minimum 0.5× base score even for zero-access memories
   - High-access memories get up to 1.0× (full preservation)
3. For memory-level decay:
   - Propagate memory access_count to node scoring when IncludeMemoryIDs is true
   - If node linked to multiple memories, use max(access_count) across linked memories
4. Add unit tests for frequency-based decay formula

**Formula Rationale**:
- `log(access_count + 1)` compresses high values, prevents runaway scores
- Reference count of 10 means: 10 accesses = full heat protection
- 0.5 floor ensures even unused memories don't completely vanish
- Multiplicative with time_decay preserves both signals

**Acceptance Criteria**:
- Frequently accessed memories resist time-based decay
- 6-month-old memory with 50 accesses ranks higher than 1-week memory with 0 accesses
- Formula matches specification
- Backward compatible (disabled when AccessFrequencyEnabled=false)

**Dependencies**: Milestone 1

---

### Milestone 3: Supersession Schema

**Objective**: Add tables and columns for explicit supersession chains.

**Tasks**:
1. Create `memory_supersession` table:
   ```sql
   CREATE TABLE memory_supersession (
     id TEXT PRIMARY KEY,
     superseding_id TEXT NOT NULL,  -- new memory
     superseded_id TEXT NOT NULL,   -- old memory being replaced
     reason TEXT,                   -- optional explanation
     created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
     FOREIGN KEY (superseding_id) REFERENCES memories(id) ON DELETE CASCADE,
     FOREIGN KEY (superseded_id) REFERENCES memories(id) ON DELETE CASCADE
   );
   ```
   Note: `ON DELETE CASCADE` ensures chain cleanup when either memory is deleted
2. Add columns to `memories` table:
   - `status TEXT DEFAULT 'Active'` - values: Active, Superseded, Archived, Pinned
   - `superseded_by TEXT` - bidirectional link to memory that supersedes this one (nullable)
3. Add indexes:
   - `idx_supersession_superseding_id`
   - `idx_supersession_superseded_id`
   - `idx_memories_status`
   - `idx_memories_superseded_by`
4. Implement SupersessionStore interface:
   - `RecordSupersession(ctx, supersedingID, supersededID string, reason string) error`
   - `GetSupersessionChain(ctx, memoryID string) ([]SupersessionRecord, error)`
   - `GetSupersedingMemory(ctx, memoryID string) (*string, error)` - returns ID of memory that supersedes this one
   - `GetSupersededBy(ctx, memoryID string) (*string, error)` - returns ID of memory this one supersedes

**Acceptance Criteria**:
- Schema migration creates tables without data loss
- Existing memories get status='Active' (or NULL treated as Active)
- Foreign keys prevent invalid references
- Bidirectional linking via `superseded_by` column maintains provenance chain
- CASCADE delete on supersession record when either memory deleted
- Superseded memory reverts to Active or gets aggregated per implementer decision

**Dependencies**: v1.0.0 released

---

### Milestone 4: AddMemory Supersession Support

**Objective**: Extend AddMemory API to accept supersession declarations.

**Tasks**:
1. Extend `MemoryInput` struct:
   ```go
   type MemoryInput struct {
     // ... existing fields ...
     Supersedes []string // IDs of memories this one replaces (optional)
     SupersessionReason string // Explanation for supersession (optional)
   }
   ```
2. Extend `MemoryResult` struct:
   ```go
   type MemoryResult struct {
     // ... existing fields ...
     MemoriesSuperseded int // Count of memories marked as Superseded
   }
   ```
3. Update `AddMemory` implementation:
   - After successful memory creation, for each ID in Supersedes:
     - Validate target memory exists and is Active
     - Call `RecordSupersession(newID, supersededID, reason)`
     - Update superseded memory's status to 'Superseded'
   - Return count in MemoriesSuperseded
4. Add validation: cannot supersede already-Superseded memories (or allow with warning?)
5. Update AddMemory unit tests

**OPEN QUESTION**: Should superseding an already-superseded memory be allowed (creating a chain) or rejected?
**Proposed Resolution**: Allow it. Creates legitimate chains like: Decision v1 → Decision v2 → Decision v3.

**Acceptance Criteria**:
- AddMemory with Supersedes marks old memories as Superseded
- Supersession records created with timestamps
- Chain traversal works correctly
- Validation prevents invalid references

**Dependencies**: Milestone 3

---

### Milestone 5: Supersession-Aware Prune

**Objective**: Prune considers Superseded status when identifying prunable memories.

**Tasks**:
1. Extend `PruneOptions`:
   ```go
   type PruneOptions struct {
     // ... existing fields ...
     PruneSuperseded bool    // Prune Superseded memories (default: true)
     SupersededAgeDays int   // Only prune Superseded memories older than this (default: 30)
   }
   ```
2. Update Prune logic:
   - When PruneSuperseded=true:
     - Include memories with status='Superseded' AND updated_at < (now - SupersededAgeDays)
   - Superseded memories are prime candidates for cleanup (they've been explicitly replaced)
3. Extend `PruneResult`:
   ```go
   type PruneResult struct {
     // ... existing fields ...
     SupersededMemoriesPruned int
   }
   ```
4. Add DryRun support for supersession pruning

**Acceptance Criteria**:
- Superseded memories automatically eligible for prune after grace period
- Grace period prevents accidental data loss during transition
- Active/Pinned memories never pruned by supersession rules

**Dependencies**: Milestone 4

---

### Milestone 6: Retention Policy Schema

**Objective**: Add retention policy support to memories.

**Tasks**:
1. Add columns to `memories` table:
   - `retention_policy TEXT DEFAULT 'standard'` - values: permanent, decision, standard, ephemeral, session
   - `retention_until DATETIME` - explicit expiration (nullable)
2. Define policy parameters in Config or as constants:
   ```go
   var RetentionPolicies = map[string]RetentionPolicyDef{
     "permanent":  {HalfLifeDays: 0, Prunable: false},      // ∞, never
     "decision":   {HalfLifeDays: 365, Prunable: true},     // 1 year, after supersession
     "standard":   {HalfLifeDays: 90, Prunable: true},      // 3 months
     "ephemeral":  {HalfLifeDays: 7, Prunable: true},       // 1 week
     "session":    {HalfLifeDays: 1, Prunable: true},       // 1 day
   }
   ```
3. Extend MemoryInput with `RetentionPolicy string` field
4. Validate retention_policy on AddMemory (reject unknown values)

**Acceptance Criteria**:
- Schema migration adds columns
- AddMemory accepts and validates retention_policy
- Default is 'standard'
- ListMemories can filter by policy

**Dependencies**: v1.0.0 released

---

### Milestone 7: Retention-Aware Decay

**Objective**: Decay formula respects per-memory retention policies.

**Tasks**:
1. Modify decay calculation to use memory-specific half-life:
   - Fetch linked memory's retention_policy via provenance
   - Override global DecayHalfLifeDays with policy-specific value
   - For nodes linked to multiple memories, use max(half_life) (most protective)
2. Handle `permanent` policy: time_decay = 1.0 always (no decay)
3. For legacy nodes (no memory linkage): use global DecayHalfLifeDays

**Acceptance Criteria**:
- Permanent memories have decay multiplier of 1.0 regardless of age
- Ephemeral memories decay faster than standard
- Decision memories decay slower than standard
- Mixed-provenance nodes use most protective policy

**Dependencies**: Milestone 2, Milestone 6

---

### Milestone 8: Retention-Aware Prune

**Objective**: Prune respects retention policies.

**Tasks**:
1. Update Prune logic:
   - `permanent` memories: NEVER pruned, regardless of other criteria
   - `decision` memories: only pruned when Superseded + grace period expired
   - Others: standard decay-based pruning
2. Add `retention_until` enforcement:
   - If `retention_until` is set and in the future: exempt from prune
   - If `retention_until` is set and in the past: eligible for prune (regardless of policy)
3. PruneResult tracks pruned-by-policy breakdown:
   ```go
   type PruneResult struct {
     // ... existing fields ...
     PrunedByPolicy map[string]int // policy → count
   }
   ```

**Acceptance Criteria**:
- Permanent memories never pruned
- retention_until overrides policy when set
- Policy breakdown reported in PruneResult

**Dependencies**: Milestone 5, Milestone 6

---

### Milestone 9: User Pinning

**Objective**: Allow users to pin memories, exempting them from decay/prune.

**Tasks**:
1. Add columns to `memories` table:
   - `pinned BOOLEAN DEFAULT FALSE`
   - `pinned_at DATETIME`
   - `pinned_reason TEXT`
2. Add Gognee APIs:
   ```go
   func (g *Gognee) PinMemory(ctx context.Context, id string, reason string) error
   func (g *Gognee) UnpinMemory(ctx context.Context, id string) error
   ```
3. Implementation:
   - PinMemory: set pinned=true, pinned_at=now, pinned_reason=reason, status='Pinned'
   - UnpinMemory: set pinned=false, status='Active'
4. Update decay calculation:
   - Pinned memories get time_decay = 1.0 (like permanent policy)
5. Update Prune:
   - Pinned memories NEVER pruned
6. Add `Pinned bool` to ListMemoriesOptions filter

**OPEN QUESTION**: Should there be a pin limit to prevent "pin everything"?
**Proposed Resolution**: No hard limit in v1.1.0. Add optional `MaxPinnedMemories` config in v1.2.0 if abuse patterns emerge.

**Acceptance Criteria**:
- PinMemory/UnpinMemory APIs work correctly
- Pinned memories exempt from decay and prune
- ListMemories can filter by pinned status
- Status correctly reflects pinned state

**Dependencies**: Milestone 6 (uses status column)

---

### Milestone 10: ListMemories Enhancements

**Objective**: Extend ListMemories for memory management UI needs.

**Tasks**:
1. Extend `ListMemoriesOptions`:
   ```go
   type ListMemoriesOptions struct {
     Offset          int
     Limit           int
     Status          *string   // Filter by status (Active, Superseded, Pinned, etc.)
     RetentionPolicy *string   // Filter by retention_policy
     Pinned          *bool     // Filter pinned only
     OrderBy         string    // "created_at", "updated_at", "access_count", "last_accessed_at"
     OrderDesc       bool      // Default true (newest first)
   }
   ```
2. Extend `MemorySummary`:
   ```go
   type MemorySummary struct {
     // ... existing fields ...
     Status          string
     RetentionPolicy string
     Pinned          bool
     AccessCount     int
     SupersededBy    *string  // ID of memory that superseded this one (if any)
   }
   ```
3. Implement filtering and ordering in ListMemories query

**Acceptance Criteria**:
- All filters work correctly
- Ordering options supported
- UI can show complete memory status

**Dependencies**: Milestone 6, Milestone 9

---

### Milestone 11: Unit Tests

**Objective**: Comprehensive test coverage for all new functionality.

**Tasks**:
1. Access frequency scoring tests:
   - Decay formula with various access counts
   - High-access old memories vs low-access new memories
   - AccessFrequencyEnabled toggle
2. Supersession chain tests:
   - Create chain A → B → C
   - Query chain in both directions
   - Supersession-aware prune
   - Edge cases: supersede non-existent, supersede already-superseded
3. Retention policy tests:
   - All five policies with correct half-lives
   - Policy-aware decay
   - Policy-aware prune
   - retention_until override
4. Pinning tests:
   - Pin/unpin lifecycle
   - Pinned exemption from decay
   - Pinned exemption from prune
5. ListMemories filter tests

**Acceptance Criteria**:
- Coverage ≥80% for new code
- All edge cases covered
- Integration tests validate end-to-end flows

**Dependencies**: Milestones 1-10

---

### Milestone 12: Documentation and Examples

**Objective**: Document new features for users.

**Tasks**:
1. Add "Intelligent Memory Lifecycle" section to README:
   - Access frequency scoring explanation
   - Supersession chains with examples
   - Retention policies table
   - Pinning usage
2. Update API reference for new methods/options
3. Add configuration examples for each feature
4. Document migration from v1.0.0

**Acceptance Criteria**:
- All features documented with examples
- Configuration options explained
- Common use cases covered

**Dependencies**: Milestone 11

---

### Milestone 13: Version Management

**Objective**: Update version artifacts for v1.1.0 release.

**Tasks**:
1. Add v1.1.0 entry to CHANGELOG.md
2. Update any version constants
3. Commit all changes

**Acceptance Criteria**:
- CHANGELOG documents all new features
- Version correctly reflects v1.1.0

**Dependencies**: All previous milestones

---

## Testing Strategy

**Unit Tests**:
- Decay formula mathematics with access frequency
- Supersession chain CRUD operations
- Retention policy enforcement
- Pinning lifecycle
- ListMemories filtering

**Integration Tests**:
- End-to-end: AddMemory with Supersedes → verify chain → prune superseded
- Decay ranking: old+frequent vs new+infrequent
- Policy-based prune behavior

**Coverage Target**: ≥80% for new code

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Access tracking overhead | Write latency | Batch access updates; async if needed |
| Supersession chain corruption | Data integrity | Foreign keys, transaction wrapping |
| Policy misconfiguration | Unexpected data loss | Permanent memories never pruned; DryRun |
| Pin abuse (everything pinned) | Unbounded growth | Monitor; add limits in v1.2.0 if needed |
| Schema migration on large DBs | Startup latency | Test with production-scale data; progressive migration |

---

## Residuals Reconciliation

No residuals ledger exists for this project. No deferred items to address.

---

## Open Questions

| # | Question | Status | Resolution |
|---|----------|--------|------------|
| 1 | Should superseding an already-superseded memory be allowed? | RESOLVED | Yes, creates legitimate chains (v1 → v2 → v3) |
| 2 | Should there be a pin limit? | RESOLVED | No limit in v1.1.0; add optional config in v1.2.0 if abuse emerges |
| 3 | How to handle supersession when superseding memory is deleted? | RESOLVED | Bidirectional linking via `superseded_by` column with CASCADE delete; superseded memory reverts to Active or gets aggregated per implementer decision |
| 4 | Should access_velocity be computed in real-time or batch? | RESOLVED | Real-time computation on each access update; formula: `access_count / max(1, days_since_creation)` |

---

## Handoff Notes

- Plan ready for Critic review
- Access frequency builds on existing v0.9.0 `access_count` infrastructure in nodes table, but needs new columns in `memories` table
- Supersession is a breaking change in MemoryInput/MemoryResult types (additive, should be compatible)
- Consider: Semantic Consolidation (9.1.4) is explicitly OUT of scope for v1.1.0 due to LLM + UX complexity
- Critic should verify: decay formula with access frequency doesn't create unexpected ranking inversions


# Plan 010: Memory Decay / Forgetting

**Plan ID**: 010
**Target Release**: v0.9.0
**Epic Alignment**: Epic 7.5 - Memory Decay / Forgetting (P3)
**Status**: UAT Approved
**Created**: 2025-12-24

## Changelog
| Date | Change |
|------|--------|
| 2025-12-24 | Initial plan creation |
| 2025-12-25 | Implementation complete, QA passed, UAT approved for v0.9.0 release |

---

## Value Statement and Business Objective

**As a** developer building a long-lived AI assistant,
**I want** old or stale information to decay or be forgotten,
**So that** the knowledge graph stays relevant and doesn't grow unbounded.

---

## Objective

Implement time-based memory decay that reduces the relevance of older nodes in search results, and optionally prunes nodes that haven't been accessed or reinforced within a configurable time window.

---

## Assumptions

1. Decay is based on time since creation OR last access (configurable)
2. Decay affects search ranking (score multiplier), not immediate deletion
3. Pruning (actual deletion) is a separate, explicit operation users can invoke
4. "Access" means the node was returned in a search result or explicitly retrieved
5. Decay parameters are configurable per-Gognee instance
6. Edges decay with their connected nodes (if both endpoints are decayed, edge is also pruned)

**Clarification**: Decay applies to node scoring only. Edges are affected only during Prune via cascading deletion when an endpoint is pruned.

**OPEN QUESTION**: Should decay be based on creation time, last access time, or both?
**Resolution**: Support both via configuration. Default to last-access-based decay (mimics human memory reinforcement). Creation time is fallback for nodes never accessed.

---

## Plan

### Milestone 1: Node Timestamp Tracking

**Objective**: Track last access time for nodes.

**Tasks**:
1. Add `last_accessed_at DATETIME` column to nodes table via schema migration (SQLite `ALTER TABLE ... ADD COLUMN`)
2. Update `GetNode()` to update `last_accessed_at` when node is retrieved
3. Update search result node fetching to update `last_accessed_at`
4. Add `AccessCount INT DEFAULT 0` column for future use (frequency-based decay)

**Migration strategy**:
- On startup, detect missing columns and apply `ALTER TABLE nodes ADD COLUMN ...` per column.
- Columns must allow NULL and/or have defaults so existing rows remain valid.
- Treat NULL `last_accessed_at` as "never accessed".

**Acceptance Criteria**:
- Schema migration adds columns without data loss
- Node access updates timestamp
- Existing nodes get NULL (never accessed) which can be handled

**Dependencies**: None

---

### Milestone 2: Decay Configuration

**Objective**: Define configuration for decay behavior.

**Tasks**:
1. Add decay fields to gognee.Config:
   - `DecayEnabled bool` - enables decay scoring (default: false)
   - `DecayHalfLifeDays int` - time after which score is halved (default: 30)
   - `DecayBasis string` - "access" or "creation" (default: "access")
2. Document configuration options
3. Validate configuration on New()

**Acceptance Criteria**:
- Configuration controls decay behavior
- Defaults are sensible and backward-compatible (decay off by default)

**Dependencies**: None

---

### Milestone 3: Decay Score Function

**Objective**: Implement mathematical decay function.

**Tasks**:
1. Create decay calculation function: `calculateDecay(nodeAge time.Duration, halfLifeDays int) float64`
2. Use exponential decay formula: `score_multiplier = 0.5^(age_days / half_life_days)`
3. Handle edge cases: negative age, zero half-life, NULL timestamps
4. Add unit tests for decay function

**Acceptance Criteria**:
- Decay function returns 1.0 for brand new nodes
- Decay function returns 0.5 for nodes exactly at half-life age
- Decay function approaches 0 for very old nodes

**Dependencies**: None

---

### Milestone 4: Search Decay Integration

**Objective**: Apply decay to search result scoring.

**Tasks**:
1. Implement a `DecayingSearcher` wrapper (decorator) that wraps any existing Searcher and applies decay post-search
2. Fetch per-node timestamps (basis: last_accessed_at, falling back to created_at when NULL)
3. Compute final score as: `final_score = raw_score * decay_multiplier`
4. Optionally filter out nodes below minimum decay threshold (e.g., 0.01)
5. Wire decorator in `gognee.New()` when decay is enabled (no changes required to Searcher interface)

**Acceptance Criteria**:
- Recent nodes rank higher than old nodes (all else equal)
- Decay is configurable and can be disabled
- Performance impact is minimal

**Dependencies**: Milestone 1, Milestone 2, Milestone 3

---

### Milestone 5: Access Reinforcement

**Objective**: Nodes that are frequently accessed should resist decay.

**Tasks**:
1. Update `last_accessed_at` only for final TopK returned results (not intermediate candidates)
2. Perform access updates in batch (single UPDATE with IN clause) where feasible
3. Optionally increment `access_count`
4. Keep frequency-based decay as future enhancement (flagged but not required)

**Acceptance Criteria**:
- Searched nodes have their access time updated
- Reinforcement keeps frequently-used nodes relevant
- Access updates do not materially impact search latency for typical TopK values

**Dependencies**: Milestone 1

---

### Milestone 6: Prune API

**Objective**: Provide explicit API to prune decayed nodes.

**Tasks**:
1. Add `Prune(ctx context.Context, opts PruneOptions) (*PruneResult, error)` method to Gognee
2. PruneOptions:
   - `MaxAgeDays int` - prune nodes older than this (access or creation based)
   - `MinDecayScore float64` - prune nodes with decay below this
   - `DryRun bool` - report what would be pruned without deleting
3. PruneResult:
   - `NodesPruned int`
   - `EdgesPruned int`
   - `NodesEvaluated int`
4. Cascade prune: delete edges when either endpoint is deleted
5. Also remove from vector store

**Acceptance Criteria**:
- Prune removes old nodes and their edges
- DryRun shows impact without modifying data
- Vector store stays in sync

**Dependencies**: Milestone 1, Milestone 3

---

### Milestone 7: Unit Tests

**Objective**: Test decay and prune functionality.

**Tasks**:
1. Test decay function with various ages and half-lives
2. Test search ranking with decay enabled
3. Test access time updates on search
4. Test prune with various criteria
5. Test prune cascades to edges and vector store
6. Test dry run returns accurate counts

**Acceptance Criteria**:
- All decay scenarios tested
- All prune scenarios tested
- Coverage ≥80%

**Dependencies**: Milestone 4, Milestone 6

---

### Milestone 8: Integration Tests

**Objective**: End-to-end decay and prune validation.

**Tasks**:
1. Test: Add docs, wait (simulate time), search → verify decay affects ranking
2. Test: Prune with max age → verify old nodes removed
3. Test: Reinforcement → accessed nodes survive prune

**Acceptance Criteria**:
- Integration tests validate time-based behavior
- Tests can simulate time passage (or use very short half-lives)

**Dependencies**: Milestone 7

---

### Milestone 9: Documentation

**Objective**: Document decay and prune features.

**Tasks**:
1. Add "Memory Decay" section to README
2. Document configuration options
3. Document Prune API with examples
4. Add guidance on choosing half-life values

**Acceptance Criteria**:
- Feature is fully documented
- Common use cases covered

**Dependencies**: Milestone 8

---

### Milestone 10: Version Management

**Objective**: Update version artifacts to v0.9.0.

**Tasks**:
1. Add v0.9.0 entry to CHANGELOG.md
2. Commit all changes

**Acceptance Criteria**:
- CHANGELOG documents decay/prune features
- Version is v0.9.0

**Dependencies**: All previous milestones

---

## Testing Strategy

**Unit Tests**:
- Decay function mathematics
- Timestamp tracking
- Search score modification
- Prune operations
- Cascade deletion
- Configuration validation

**Integration Tests**:
- Time-based ranking changes
- Prune effectiveness
- Reinforcement behavior

**Coverage Target**: ≥80% for new code

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Decay math errors | Incorrect ranking | Thorough unit tests with known values |
| Prune deletes important data | Data loss | DryRun mode; require explicit call (no auto-prune) |
| Performance impact on search | Latency increase | Batch timestamp updates; keep decay calc simple |
| Schema migration complexity | Upgrade issues | Use IF NOT EXISTS; handle NULL gracefully |

---

## Handoff Notes

- Decay is OFF by default for backward compatibility
- Prune is never automatic - users must explicitly call Prune()
- Critic should verify decay formula aligns with cognitive science norms
- Consider adding Stats.OldestNode and Stats.AverageNodeAge for visibility
- Future: auto-prune on Cognify or on a schedule (not in this plan)


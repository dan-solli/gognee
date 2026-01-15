# Plan 019 — Write Path Optimization (Batch Embeddings)

**Plan ID:** 019  
**Target Release:** gognee v1.3.0  
**Epic Alignment:** Epic 7.7 (Performance Optimization) — Cognify/AddMemory write latency  
**Status:** Draft  
**Created:** 2026-01-15  

---

## Changelog

| Date | Change | Author |
|------|--------|--------|
| 2026-01-15 | Initial plan drafted per performance incident (33s memory write) | Planner |

---

## Value Statement and Business Objective

> As a Glowbabe user storing memories,  
> I want memory creation to complete in <5 seconds,  
> So that saving context doesn't interrupt my workflow.

---

## Problem Statement

Memory writes are taking **33 seconds** for a single memory with 16 nodes and 13 edges. This makes the system impractical for real-time use.

**Evidence** (from user logs):
```
[2026-01-15 20:29:54.807] REQ [5] method=memory.create
[2026-01-15 20:30:28.237] RES [5] method=memory.create status=OK duration=33430ms
```

**Root Cause Analysis:**

The current implementation has an **N+1 embedding problem**:

```go
// pkg/gognee/gognee.go lines 498-502
for _, entity := range entities {
    // ... create node ...
    embedding, err := g.embeddings.EmbedOne(ctx, entity.Name+" "+entity.Description)
    // ... store embedding ...
}
```

With 16 entities, this results in **16 separate OpenAI API calls** instead of 1 batched call.

**Estimated time breakdown** (33s total):
- LLM entity extraction: ~3-5s (1 call)
- LLM relation extraction: ~3-5s (1 call)
- Embedding generation: **16 × ~1.5s = ~24s** (16 serial calls)
- Database writes: <1s

**The embedding batching problem accounts for ~70% of the latency.**

---

## Success Criteria

| Criterion | Current | Target | Measurement |
|-----------|---------|--------|-------------|
| Single memory write latency | 33s | <5s | End-to-end duration for 16-node memory |
| Embedding API calls per Cognify | N (one per entity) | 1 (batched) | Count of OpenAI embedding requests |
| No regression in correctness | N/A | All tests pass | `go test ./...` |
| No regression in coverage | 73.5% | ≥73% | Coverage report |

---

## Scope

### In Scope

1. **M1: Batch Embedding Collection** — Collect all entity texts before embedding
2. **M2: Single Batch API Call** — Use `Embed()` instead of `EmbedOne()` loop
3. **M3: Embedding Assignment** — Map batch results back to entities
4. **M4: Benchmark Validation** — Add write-path benchmark to detect regressions

### Out of Scope

- Parallel LLM extraction (entity + relation) — Future optimization
- Database write batching — Marginal gain for current scale
- Streaming/async memory creation — Different UX model
- LLM call optimization — Separate concern (prompt engineering)

---

## Technical Approach

### Current Flow (Serial N+1)

```
for each chunk:
    entities = extractEntities(chunk)      # 1 LLM call
    relations = extractRelations(chunk)    # 1 LLM call
    
    for each entity:                       # N iterations
        node = createNode(entity)
        embedding = EmbedOne(text)         # 1 API call per entity ❌
        storeNodeWithEmbedding(node, embedding)
        indexVector(node.ID, embedding)
```

### Proposed Flow (Batched)

```
for each chunk:
    entities = extractEntities(chunk)      # 1 LLM call
    relations = extractRelations(chunk)    # 1 LLM call
    
    # Collect all texts for batch embedding
    texts = [entity.Name + " " + entity.Description for entity in entities]
    embeddings = Embed(texts)              # 1 API call total ✅
    
    for i, entity := range entities:
        node = createNode(entity)
        storeNodeWithEmbedding(node, embeddings[i])
        indexVector(node.ID, embeddings[i])
```

### Key Implementation Details

1. **Text collection order must match embedding result order** — OpenAI returns embeddings in input order
2. **Handle empty entities gracefully** — Skip entities with empty Name+Description
3. **Preserve existing error handling** — Individual node failures shouldn't break batch
4. **Batch size limits** — OpenAI supports up to 2048 texts per batch (far exceeds typical usage)

---

## Milestones

### M1: Batch Embedding Collection (Core Fix)

**Objective**: Collect entity texts before embedding generation, call `Embed()` once per chunk.

**Files to modify**:
- `pkg/gognee/gognee.go` — Refactor Cognify loop structure

**Acceptance Criteria**:
- All entity texts collected into slice before any embedding call
- Single `Embed()` call replaces loop of `EmbedOne()` calls
- Embeddings correctly mapped back to corresponding entities by index
- Existing unit tests pass

**Estimated complexity**: Low — straightforward refactor of existing loop

---

### M2: Error Handling for Batch Failures

**Objective**: Handle partial failures gracefully when batch embedding fails.

**Acceptance Criteria**:
- If batch embedding fails, error is recorded and chunk processing continues without embeddings
- Individual embedding assignment errors are logged but don't halt processing
- Error count reflected in CognifyResult.Errors

**Estimated complexity**: Low — error handling pattern already exists

---

### M3: Benchmark for Write Path

**Objective**: Add benchmark to detect write-path performance regressions.

**Files to create**:
- `pkg/gognee/cognify_benchmark_test.go`

**Acceptance Criteria**:
- `BenchmarkCognify_BatchEmbeddings` measures Cognify with realistic entity count
- Benchmark uses mock clients with simulated latency (~100ms per API call)
- Baseline established: <500ms for 16-entity chunk with mocked APIs

**Estimated complexity**: Low — follows existing benchmark patterns

---

### M4: Version Management

**Objective**: Update release artifacts for v1.3.0.

**Tasks**:
1. Update CHANGELOG.md with v1.3.0 entry
2. Update go.mod version comment
3. Commit with plan reference

**Acceptance Criteria**:
- CHANGELOG documents performance improvement
- Version artifacts consistent

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Batch API rate limiting | Low | Medium | OpenAI batch limits are generous (2048 texts); typical usage is <100 |
| Index mismatch in batch results | Low | High | Rely on OpenAI's documented behavior (returns in input order); add assertion |
| Empty entity text causes batch failure | Medium | Low | Filter empty texts before batching; skip embedding for those entities |

---

## Dependencies

- None — uses existing `Embed()` method in embeddings client

---

## Assumptions

1. OpenAI's `Embed()` endpoint returns embeddings in the same order as input texts
2. Typical memory creation produces <100 entities per chunk
3. Network latency dominates embedding time (batch reduces round trips)

---

## Testing Strategy

**Unit Tests**:
- Verify batch embedding is called with correct texts
- Verify embeddings are assigned to correct nodes
- Verify error handling for batch failures

**Integration Tests** (gated):
- End-to-end Cognify with real OpenAI API
- Verify <5s for 16-node memory

**Benchmarks**:
- `BenchmarkCognify_BatchEmbeddings` with mocked clients
- Compare against pre-optimization baseline

---

## Validation

1. Run `go test ./...` — all tests pass
2. Run `go test -bench=. ./pkg/gognee` — benchmark validates improvement
3. Manual test: Create memory in Glowbabe, verify <5s latency
4. Coverage: `go test -cover ./...` — ≥73%

---

## Implementation Notes

**ILLUSTRATIVE ONLY** — Simplified batch collection pattern:

```
// Collect texts for batch embedding
var textsToEmbed []string
var entityIndices []int // Track which entities need embeddings

for i, entity := range entities {
    text := strings.TrimSpace(entity.Name + " " + entity.Description)
    if text != "" {
        textsToEmbed = append(textsToEmbed, text)
        entityIndices = append(entityIndices, i)
    }
}

// Single batch call
embeddings, err := g.embeddings.Embed(ctx, textsToEmbed)
if err != nil {
    // Handle batch failure
}

// Assign embeddings back to entities
for j, embedding := range embeddings {
    entityIdx := entityIndices[j]
    // ... assign to entities[entityIdx] ...
}
```

---

## Handoff Notes

- Plan is self-contained; no analyst investigation required
- Implementation should be straightforward refactor
- Key file: `pkg/gognee/gognee.go` lines 475-520 (entity processing loop)
- Existing trace instrumentation will capture improvement automatically

---

## Open Questions

None — approach is well-understood and low-risk.

---

## References

- Plan 018: Vector Search Optimization (related performance work)
- [pkg/embeddings/client.go](pkg/embeddings/client.go) — `Embed()` batch interface
- [pkg/gognee/gognee.go](pkg/gognee/gognee.go) — Cognify implementation


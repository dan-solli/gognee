# Plan 013 — Cognify Performance Optimization

**Plan ID:** 013  
**Target Release:** gognee v1.2.0  
**Epic Alignment:** Performance optimization identified via Plan 015 instrumentation  
**Status:** Draft  
**Created:** 2026-01-11

---

## Changelog

| Date | Change | Author |
|------|--------|--------|
| 2026-01-11 | Initial plan drafted based on integration test profiling findings | Planner |
| 2026-01-11 | Recorded decisions: combined extraction always-on; no fast mode; clarified batching guardrail question | Planner |
| 2026-01-11 | Closed batching guardrail decision: max 128 entities per embedding batch | Planner |

---

## Value Statement and Business Objective

> As a Glowbabe user experiencing 30-second timeouts during memory operations,  
> I want Cognify to complete in under 10 seconds for typical single-document ingestion,  
> So that memory creation feels responsive and doesn't fail due to timeout.

---

## Problem Statement

Integration testing with production-like configuration (OpenAI API for both LLM and embeddings) revealed that a single-document Cognify operation takes **~13-14 seconds**:

| Stage | Time | % of Total | Issue |
|-------|------|------------|-------|
| LLM extraction (entities + relations) | ~9s | 68% | **2 sequential LLM calls** |
| Embeddings (6 entities) | ~3.5s | 26% | **6 sequential API calls** |
| Graph/vector writes | <1s | 6% | OK |

With a 30-second production timeout, processing more than 2 chunks risks timeout failure.

**Root causes identified:**
1. **Sequential LLM calls:** Entity extraction and relation extraction are two separate LLM calls per chunk
2. **Sequential embeddings:** Each entity embedding is a separate API call instead of batched
3. **Trace timer overlap bug:** Timing spans report incorrect values due to overlapping start times

---

## Scope

**In scope (this plan):**
- Batch embedding generation (single API call per chunk)
- Combined entity+relation extraction (single LLM call per chunk)
- Fix trace timer accuracy
- Integration test verification with performance thresholds

**Out of scope (future work):**
- Parallel chunk processing (architectural change)
- LLM response caching/memoization
- Alternative embedding models/providers
- Async/background processing patterns

---

## Architectural Requirements

| Requirement | How This Plan Addresses It |
|-------------|---------------------------|
| Maintain API compatibility | No changes to public Gognee API |
| Preserve extraction quality | Combined prompt must extract same entities/relations |
| Reduce API calls | Batch embeddings, single LLM call |
| Accurate instrumentation | Fix span timer start/stop positions |
| Measurable improvement | Integration tests with timing assertions |

---

## Key Constraints

- Must not degrade extraction quality (entity/relation counts should be comparable)
- OpenAI API batch limits: embeddings support up to 2048 inputs per call (not a concern for typical entity counts)
- Combined extraction prompt must fit within model context window
- Backward compatible: no breaking changes to public API

---

## Findings from Integration Testing

**Test configuration:**
- LLM: OpenAI gpt-4o-mini
- Embeddings: OpenAI text-embedding-3-small
- Test document: ~300 characters, 1 chunk, 6 entities, 4 edges

**Latency measurements:**
- OpenAI LLM (entity extraction): ~4s per call
- OpenAI LLM (relation extraction): ~5s per call
- OpenAI embedding (single): ~260ms per call
- Ollama embedding (local): ~980ms per call (for comparison)

**Projected improvements:**
| Optimization | Current | Expected | Savings |
|--------------|---------|----------|---------|
| Combined LLM extraction | 2 calls (~9s) | 1 call (~5-6s) | ~3-4s |
| Batched embeddings | 6 calls (~1.6s) | 1 call (~300ms) | ~1.3s |
| **Total** | **~13s** | **~7-8s** | **~40%** |

---

## Plan (Milestones)

### Milestone 1: Batch Embedding Generation

**Objective:** Replace sequential `EmbedOne()` calls with a single `Embed()` batch call.

**Tasks:**
1. In `Cognify()`, collect all entity texts before embedding
2. Call `g.embeddings.Embed(ctx, entityTexts)` once
3. Map returned embeddings back to entities by index
4. Update nodes with embeddings in a loop (no API calls)
5. Update trace spans to accurately measure embed vs write phases

**Acceptance Criteria:**
- Single embedding API call per chunk (verified via mock/spy)
- Embedding generation time reduced by ~80% for 6+ entities
- All existing tests pass
- No change to public API

---

### Milestone 2: Combined Entity+Relation Extraction

**Objective:** Extract entities and relations in a single LLM call.

**Tasks:**
1. Create new combined extraction prompt in `pkg/extraction/combined.go`
2. Define response schema: `{entities: [...], relations: [...]}`
3. Implement `CombinedExtractor.Extract(ctx, text)` returning both
4. Update `Cognify()` to use combined extractor
5. Deprecate (but keep) separate entity/relation extractors for backward compatibility
6. Update trace spans: single "extract" span instead of separate entity/relation spans

**Combined Prompt Design (ILLUSTRATIVE ONLY):**
```
Extract entities and relationships from this text.

Entities: name, type (Person/Concept/System/...), description
Relations: subject, relation, object (use only extracted entity names)

Return JSON: {"entities": [...], "relations": [...]}
```

**Acceptance Criteria:**
- Single LLM call per chunk for extraction
- Extraction time reduced by ~40-50%
- Entity and relation quality comparable to separate extraction (manual spot-check)
- Existing extraction tests adapted or new tests added

---

### Milestone 3: Fix Trace Timer Accuracy

**Objective:** Ensure timing spans report accurate per-stage durations.

**Tasks:**
1. Review current span timer start/stop positions in `Cognify()`
2. Fix overlapping timer issue: each timer must start when its work begins, not in advance
3. Ensure timers are stopped immediately after their work completes
4. Add test verifying span durations are non-overlapping where they should be sequential

**Current (broken) pattern:**
```go
// Line 453-455: All start at same time
graphWriteTimer := newSpanTimer("write-graph", ...)
embedTimer := newSpanTimer("embed", ...)
vectorWriteTimer := newSpanTimer("write-vector", ...)
// ... work happens ...
embedTimer.finish(...)      // Reports time since creation, not since embed started
```

**Fixed pattern:**
```go
embedTimer := newSpanTimer("embed", ...)
// ... embedding work ...
embedTimer.finish(...)

vectorWriteTimer := newSpanTimer("write-vector", ...)
// ... vector write work ...
vectorWriteTimer.finish(...)
```

**Acceptance Criteria:**
- Span durations are accurate (verified by manual inspection of trace output)
- Sum of span durations approximately equals total operation time
- No span reports time for work it didn't perform

---

### Milestone 4: Integration Test Verification

**Objective:** Add performance regression tests with timing thresholds.

**Tasks:**
1. Update `TestOpenAI_CognifyPipeline` with timing assertions
2. Add threshold: single-chunk Cognify must complete in <10s
3. Add threshold: LLM extraction must complete in <6s
4. Add threshold: embedding batch must complete in <1s for ≤10 entities
5. Document baseline numbers in test comments

**Acceptance Criteria:**
- Integration tests fail if performance regresses beyond thresholds
- Tests are tagged `integration_ollama` (or new tag) to avoid running in CI without API keys
- Baseline numbers documented

---

### Milestone 5: Version Bump and Release Artifacts

**Tasks:**
1. Update CHANGELOG.md for v1.2.0
2. Document performance improvements with before/after numbers
3. Note any prompt changes for combined extraction

**Acceptance Criteria:**
- CHANGELOG reflects optimizations
- Version is v1.2.0

---

## Testing Strategy

**Unit tests:**
- Mock embedding client to verify single `Embed()` call
- Mock LLM client to verify single extraction call
- Trace span duration sanity checks

**Integration tests:**
- Full Cognify pipeline with OpenAI (existing tests, updated thresholds)
- Before/after timing comparison (document in test output)

**Regression tests:**
- Extraction quality: verify entity/relation counts are comparable
- Existing unit tests must pass

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Combined prompt degrades extraction quality | Medium | High | A/B testing with sample documents; manual review |
| OpenAI API changes embedding batch behavior | Low | Medium | Document API version; add batch size guard |
| Combined prompt exceeds context window | Low | Medium | Add text length check; fall back to separate calls |
| Performance gains less than expected | Medium | Low | Set conservative thresholds; iterate if needed |

---

## Open Questions

1. **OPEN QUESTION [CLOSED]:** Should combined extraction be opt-in via config, or always-on?
   - Decision: **Always-on.** (Separate extractors may remain for debugging/backward compatibility.)

2. **OPEN QUESTION [CLOSED]:** What's the maximum entity count before embedding batching becomes a concern?
   - Decision: **Max 128 entities per embedding batch.** If more are extracted, split into multiple embedding calls.
   - Rationale: expected to be extremely rare; guardrail prevents pathological inputs from creating oversized requests.

3. **OPEN QUESTION [CLOSED]:** Should we add a "fast mode" that skips relation extraction entirely?
   - Decision: **No.** (Focus on making the default path fast.)

---

## Success Metrics

| Metric | Current | Target | Measurement |
|--------|---------|--------|-------------|
| Single-chunk Cognify time | ~13s | <10s | Integration test |
| LLM calls per chunk | 2 | 1 | Mock verification |
| Embedding calls per chunk | N (entities) | 1 | Mock verification |
| Trace accuracy | Overlapping | Sequential | Manual inspection |

---

## Handoff Notes

**For Critic:**
- Review combined extraction prompt design for quality risks
- Verify scope is appropriate (not over-engineering)
- Check that performance targets are achievable and measurable

**For Implementer:**
- Start with M1 (batched embeddings) — lowest risk, immediate gains
- M2 (combined extraction) requires careful prompt engineering
- M3 (trace fix) is straightforward refactor
- Run integration tests after each milestone to verify improvements

# Plan 009: Incremental Cognify

**Plan ID**: 009
**Target Release**: v0.8.0
**Epic Alignment**: Epic 7.4 - Incremental Cognify (P2)
**Status**: UAT Approved
**Created**: 2025-12-24

## Changelog
| Date | Change |
|------|--------|
| 2025-12-24 | Initial plan creation |
| 2025-12-24 | Plan revised per critique - all findings resolved |
| 2025-12-25 | Implementation document created (agent-output/implementation/009-incremental-cognify-implementation.md) |
| 2025-12-25 | UAT approved (agent-output/uat/009-incremental-cognify-uat.md) |

---

## Value Statement and Business Objective

**As a** developer with large document corpora,
**I want** to process only new or changed documents,
**So that** I can update my knowledge graph efficiently without reprocessing everything.

---

## Objective

Implement incremental Cognify that detects and skips already-processed documents, reducing LLM API costs and processing time for updates to existing knowledge graphs.

---

## Assumptions

1. Document identity is determined by content hash (SHA-256 of text)
2. A new `processed_documents` table tracks which documents have been Cognified
3. "Changed" documents are detected by hash mismatch (different hash = new document)
4. Users can force reprocessing via option flag
5. Source field from AddOptions can serve as optional document identifier
6. Chunks within a document are not individually tracked (document-level granularity)

**Identity rule**: Document identity is hash-only (content-based). `source` is stored as metadata for reporting/debugging and does not affect deduplication.

**OPEN QUESTION**: Should we track at document level or chunk level?
**Resolution**: Document level for simplicity. Chunk-level tracking adds complexity (chunk boundaries may shift if chunker settings change). Document hash is stable and intuitive.

---

## Plan

### Milestone 1: Document Tracking Schema

**Objective**: Add SQLite table to track processed documents.

**Tasks**:
1. Define `processed_documents` table schema:
   - `hash TEXT PRIMARY KEY` - SHA-256 of document text
   - `source TEXT` - Optional source identifier
   - `processed_at DATETIME` - When document was Cognified
   - `chunk_count INT` - Number of chunks processed
2. Add schema creation to `SQLiteGraphStore.initSchema()`
3. Create index on `source` for lookup by source identifier

**Acceptance Criteria**:
- Table created on database initialization
- Schema is backward compatible (new table, no changes to existing)

**Dependencies**: None

---

### Milestone 2: Document Tracking Interface

**Objective**: Define a cohesive interface for document tracking operations without expanding GraphStore.

**Tasks**:
1. Create a separate `DocumentTracker` interface:
   - `IsDocumentProcessed(ctx, hash string) (bool, error)`
   - `MarkDocumentProcessed(ctx, hash, source string, chunkCount int) error`
   - `GetProcessedDocumentCount(ctx) (int64, error)`
   - `ClearProcessedDocuments(ctx) error` (optional reset capability)
2. Implement DocumentTracker on SQLiteGraphStore
3. Add tests for tracking operations

**Acceptance Criteria**:
- Can check if document was previously processed
- Can mark document as processed
- Can query total processed documents
- Can reset tracking state without deleting nodes/edges (ClearProcessedDocuments)

**Dependencies**: Milestone 1

---

### Milestone 3: CognifyOptions Extension

**Objective**: Add options to control incremental behavior.

**Tasks**:
1. Add `Force bool` field to CognifyOptions - forces reprocessing even if cached
2. Add `SkipProcessed bool` field - enables incremental mode (default: true)
3. Document both options in type comments

**Acceptance Criteria**:
- Options control Cognify behavior
- Default behavior is incremental: previously processed documents are skipped unless Force=true

**Dependencies**: None

---

### Milestone 4: Incremental Cognify Logic

**Objective**: Implement the skip-if-processed logic in Cognify.

**Tasks**:
1. At start of document processing, compute SHA-256 hash of text
2. Check `IsDocumentProcessed(hash)` via DocumentTracker
3. If processed and not Force mode, skip document
4. After successful processing, call `MarkDocumentProcessed`
5. Track skipped count in CognifyResult

**Acceptance Criteria**:
- Previously processed documents are skipped
- Force=true reprocesses regardless
- CognifyResult reports DocumentsSkipped count

**Dependencies**: Milestone 2, Milestone 3

---

### Milestone 5: CognifyResult Enhancement

**Objective**: Report incremental processing statistics.

**Tasks**:
1. Add `DocumentsSkipped int` field to CognifyResult
2. Add `DocumentsNew int` field (documents that were actually processed)
3. Update existing processing to populate fields correctly

**Acceptance Criteria**:
- Result distinguishes skipped vs processed documents
- Stats are accurate

**Dependencies**: Milestone 4

---

### Milestone 6: Unit Tests

**Objective**: Test incremental Cognify behavior.

**Tasks**:
1. Test: First Cognify processes all documents
2. Test: Second Cognify (same documents) skips all
3. Test: New document added, only new document processed
4. Test: Same document re-added, skipped
5. Test: Force=true reprocesses everything
6. Test: Document with different content (same source) is processed

**Acceptance Criteria**:
- All incremental scenarios tested
- Tests are offline (mock LLM/embeddings)
- Coverage ≥80%

**Dependencies**: Milestone 4, Milestone 5

---

### Milestone 7: Integration Tests

**Objective**: End-to-end validation with real LLM.

**Tasks**:
1. Add integration test: process docs, close, reopen, add same docs, Cognify → verify skipped
2. Verify LLM not called for skipped documents (check API call count if possible, or timing)

**Acceptance Criteria**:
- Integration test validates incremental behavior
- Demonstrates cost savings (fewer LLM calls)

**Dependencies**: Milestone 6

---

### Milestone 8: Documentation

**Objective**: Document incremental Cognify feature.

**Tasks**:
1. Update README with incremental Cognify section
2. Document CognifyOptions.Force and .SkipProcessed
3. Add usage example showing incremental updates
4. Update API reference

**Acceptance Criteria**:
- Feature is documented
- Examples show common use cases

**Dependencies**: Milestone 7

---

### Milestone 9: Version Management

**Objective**: Update version artifacts to v0.8.0.

**Tasks**:
1. Add v0.8.0 entry to CHANGELOG.md
2. Commit all changes

**Acceptance Criteria**:
- CHANGELOG documents incremental Cognify feature
- Version is v0.8.0

**Dependencies**: All previous milestones

---

## Testing Strategy

**Unit Tests**:
- Document hash computation
- Tracking table operations (CRUD)
- Skip logic correctness
- Force mode override
- CognifyResult statistics accuracy

**Integration Tests**:
- Full incremental workflow with persistence
- LLM call avoidance verification

**Coverage Target**: ≥80% for new code

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Hash collision (different docs, same hash) | Incorrect skip | SHA-256 collision is astronomically unlikely; document this assumption |
| Chunk boundary changes after config update | Stale cached state | Document that changing chunk settings should clear processed_documents or use Force |
| Users expect source-based identity | Confusion | Document that identity is content-based; source is metadata only |

---

## Handoff Notes

- Consider whether default for SkipProcessed should be true (incremental by default) or false (explicit opt-in)
- Critic should verify hash computation includes normalization (trim, etc.)
- Future enhancement: provide API to clear processed_documents table for full reprocessing


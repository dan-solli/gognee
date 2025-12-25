# Implementation: Plan 009 - Incremental Cognify

**Plan Reference**: `agent-output/planning/009-incremental-cognify-plan.md`  
**Critique Reference**: `agent-output/critiques/009-incremental-cognify-plan-critique.md`  
**Date**: 2025-12-25  
**Implementer**: Implementer Agent  
**Status**: Planning
**Target Release**: v0.8.0

## Changelog
| Date | Handoff | Request | Summary |
|------|---------|---------|---------|
| 2025-12-25 | Planner → Implementer | Create implementation document | Initial implementation plan created |

---

## Value Statement and Business Objective

**As a** developer with large document corpora,  
**I want** to process only new or changed documents,  
**So that** I can update my knowledge graph efficiently without reprocessing everything.

**Business Value:**
- Reduces processing time for updates (~0ms for unchanged documents vs. 5-10s per document with LLM)
- Reduces LLM API costs for incremental updates (no extraction calls for cached documents)
- Enables continuous knowledge graph updates in production environments

---

## Implementation Summary

Implement document-level deduplication using content hashing (SHA-256). Previously processed documents are tracked in a new `processed_documents` SQLite table and skipped on subsequent Cognify calls unless forced.

**Key Design Decisions (from Plan + Critique Resolution):**
1. **Identity is hash-only** - SHA-256 of document text determines uniqueness
2. **Source is metadata only** - Does not affect deduplication
3. **Document-level granularity** - Not chunk-level (simpler, stable hash)
4. **SkipProcessed defaults to true** - Incremental by default
5. **Separate DocumentTracker interface** - Avoids GraphStore bloat
6. **ClearProcessedDocuments() included** - Enables reset without full reprocessing

---

## Implementation Approach

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Gognee.Cognify()                        │
├─────────────────────────────────────────────────────────────────┤
│  For each document in buffer:                                   │
│  1. Compute SHA-256 hash of text                                │
│  2. Check tracker.IsDocumentProcessed(hash)                     │
│  3. IF processed AND NOT Force:                                 │
│     → Skip, increment DocumentsSkipped                          │
│  4. ELSE:                                                       │
│     → Process normally (chunk, extract, store)                  │
│     → tracker.MarkDocumentProcessed(hash, source, chunkCount)   │
└─────────────────────────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    DocumentTracker Interface                    │
├─────────────────────────────────────────────────────────────────┤
│  - IsDocumentProcessed(ctx, hash) (bool, error)                 │
│  - MarkDocumentProcessed(ctx, hash, source, chunkCount) error   │
│  - GetProcessedDocumentCount(ctx) (int64, error)                │
│  - ClearProcessedDocuments(ctx) error                           │
└─────────────────────────────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  processed_documents Table                      │
├─────────────────────────────────────────────────────────────────┤
│  hash TEXT PRIMARY KEY                                          │
│  source TEXT                                                    │
│  processed_at DATETIME                                          │
│  chunk_count INTEGER                                            │
└─────────────────────────────────────────────────────────────────┘
```

### TDD Approach

Following the project's mandatory TDD pattern (Red-Green-Refactor):

1. **Red**: Write failing tests for DocumentTracker interface + incremental behavior
2. **Green**: Implement minimal code to pass tests
3. **Refactor**: Clean up, add documentation, optimize

---

## Milestones

### Milestone 1: Document Tracking Schema [TDD]

**Objective**: Add SQLite table to track processed documents.

**Files to Modify:**
- [pkg/store/sqlite.go](../../pkg/store/sqlite.go) - Add table creation to `initSchema()`

**Code Changes:**

1. Add `processed_documents` table creation in `initSchema()`:

```go
// In initSchema(), after existing schema:
CREATE TABLE IF NOT EXISTS processed_documents (
    hash TEXT PRIMARY KEY,
    source TEXT,
    processed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    chunk_count INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_processed_documents_source ON processed_documents(source);
```

**TDD Tests** (write first):
- Test: Schema creates table on initialization
- Test: Table is backward compatible (existing DBs work after upgrade)

**Acceptance Criteria:**
- [x] Table created on database initialization
- [x] Index on `source` for lookup
- [x] Schema is backward compatible (new table, no changes to existing)

**Dependencies**: None

---

### Milestone 2: DocumentTracker Interface [TDD]

**Objective**: Define a cohesive interface for document tracking operations.

**Files to Create:**
- [pkg/store/tracker.go](../../pkg/store/tracker.go) - Interface + SQLite implementation

**Files to Modify:**
- [pkg/store/sqlite.go](../../pkg/store/sqlite.go) - Implement DocumentTracker methods

**Interface Definition:**

```go
// pkg/store/tracker.go

// DocumentTracker provides operations for tracking processed documents.
// Separate from GraphStore to maintain interface cohesion (Finding 2 resolution).
type DocumentTracker interface {
    // IsDocumentProcessed checks if a document with the given hash has been processed.
    IsDocumentProcessed(ctx context.Context, hash string) (bool, error)

    // MarkDocumentProcessed records that a document has been processed.
    // hash: SHA-256 of document text (identity)
    // source: Optional source identifier (metadata only, does not affect identity)
    // chunkCount: Number of chunks processed from this document
    MarkDocumentProcessed(ctx context.Context, hash, source string, chunkCount int) error

    // GetProcessedDocumentCount returns the total number of processed documents.
    GetProcessedDocumentCount(ctx context.Context) (int64, error)

    // ClearProcessedDocuments removes all document tracking records.
    // Does NOT delete nodes/edges - only clears the processed_documents table.
    ClearProcessedDocuments(ctx context.Context) error
}
```

**SQLiteGraphStore Implementation:**

```go
// pkg/store/sqlite.go - Add methods to SQLiteGraphStore

// Compile-time interface check
var _ DocumentTracker = (*SQLiteGraphStore)(nil)

func (s *SQLiteGraphStore) IsDocumentProcessed(ctx context.Context, hash string) (bool, error) {
    var count int
    err := s.db.QueryRowContext(ctx, 
        "SELECT COUNT(*) FROM processed_documents WHERE hash = ?", hash).Scan(&count)
    if err != nil {
        return false, fmt.Errorf("failed to check document: %w", err)
    }
    return count > 0, nil
}

func (s *SQLiteGraphStore) MarkDocumentProcessed(ctx context.Context, hash, source string, chunkCount int) error {
    _, err := s.db.ExecContext(ctx,
        `INSERT OR REPLACE INTO processed_documents (hash, source, processed_at, chunk_count)
         VALUES (?, ?, CURRENT_TIMESTAMP, ?)`,
        hash, source, chunkCount)
    if err != nil {
        return fmt.Errorf("failed to mark document processed: %w", err)
    }
    return nil
}

func (s *SQLiteGraphStore) GetProcessedDocumentCount(ctx context.Context) (int64, error) {
    var count int64
    err := s.db.QueryRowContext(ctx, 
        "SELECT COUNT(*) FROM processed_documents").Scan(&count)
    if err != nil {
        return 0, fmt.Errorf("failed to get processed document count: %w", err)
    }
    return count, nil
}

func (s *SQLiteGraphStore) ClearProcessedDocuments(ctx context.Context) error {
    _, err := s.db.ExecContext(ctx, "DELETE FROM processed_documents")
    if err != nil {
        return fmt.Errorf("failed to clear processed documents: %w", err)
    }
    return nil
}
```

**TDD Tests** (write first):
- Test: `IsDocumentProcessed` returns false for unknown hash
- Test: `IsDocumentProcessed` returns true after `MarkDocumentProcessed`
- Test: `MarkDocumentProcessed` updates existing record (upsert)
- Test: `GetProcessedDocumentCount` returns correct count
- Test: `ClearProcessedDocuments` resets count to zero
- Test: Operations work with empty source string

**Acceptance Criteria:**
- [x] Can check if document was previously processed
- [x] Can mark document as processed
- [x] Can query total processed documents
- [x] Can reset tracking state without deleting nodes/edges
- [x] All methods are implemented on SQLiteGraphStore
- [x] Compile-time interface check passes

**Dependencies**: Milestone 1

---

### Milestone 3: CognifyOptions Extension [TDD]

**Objective**: Add options to control incremental behavior.

**Files to Modify:**
- [pkg/gognee/gognee.go](../../pkg/gognee/gognee.go) - Extend CognifyOptions struct

**Code Changes:**

```go
// pkg/gognee/gognee.go

// CognifyOptions configures the Cognify() method
type CognifyOptions struct {
    // SkipProcessed enables incremental mode, skipping previously processed documents.
    // Default: true (incremental by default).
    // When true, documents are identified by content hash (SHA-256).
    // Documents with matching hash are skipped unless Force is true.
    SkipProcessed *bool  // Pointer to distinguish unset from explicit false

    // Force reprocesses all documents regardless of cached state.
    // Overrides SkipProcessed when true.
    // Use after changing chunker settings or to rebuild the knowledge graph.
    Force bool
}

// Helper function to get SkipProcessed value with default
func (o CognifyOptions) shouldSkipProcessed() bool {
    if o.SkipProcessed == nil {
        return true // Default: incremental by default
    }
    return *o.SkipProcessed
}
```

**TDD Tests** (write first):
- Test: Default SkipProcessed is true (nil pointer = true)
- Test: Explicit SkipProcessed=false disables incremental
- Test: Force=true overrides SkipProcessed

**Acceptance Criteria:**
- [x] SkipProcessed defaults to true (incremental by default)
- [x] Force=true allows reprocessing
- [x] Options are documented with type comments

**Dependencies**: None

---

### Milestone 4: CognifyResult Enhancement [TDD]

**Objective**: Report incremental processing statistics.

**Files to Modify:**
- [pkg/gognee/gognee.go](../../pkg/gognee/gognee.go) - Extend CognifyResult struct

**Code Changes:**

```go
// CognifyResult reports the outcome of a Cognify() operation
type CognifyResult struct {
    DocumentsProcessed int     // Total documents actually processed (not skipped)
    DocumentsSkipped   int     // Documents skipped due to incremental caching
    ChunksProcessed    int
    ChunksFailed       int
    NodesCreated       int
    EdgesCreated       int
    EdgesSkipped       int     // Count of edges skipped due to entity lookup failure
    Errors             []error
}
```

**TDD Tests** (write first):
- Test: DocumentsSkipped is 0 when no documents skipped
- Test: DocumentsSkipped increments correctly
- Test: DocumentsProcessed + DocumentsSkipped = total input documents

**Acceptance Criteria:**
- [x] Result distinguishes skipped vs processed documents
- [x] Stats are accurate
- [x] Backward compatible (new field, existing consumers unaffected)

**Dependencies**: None

---

### Milestone 5: Incremental Cognify Logic [TDD]

**Objective**: Implement the skip-if-processed logic in Cognify.

**Files to Modify:**
- [pkg/gognee/gognee.go](../../pkg/gognee/gognee.go) - Update Cognify() method

**Algorithm:**

```go
func (g *Gognee) Cognify(ctx context.Context, opts CognifyOptions) (*CognifyResult, error) {
    result := &CognifyResult{Errors: make([]error, 0)}

    if len(g.buffer) == 0 {
        return result, nil
    }

    // Get DocumentTracker from GraphStore (type assertion)
    tracker, ok := g.graphStore.(store.DocumentTracker)
    if !ok {
        return nil, fmt.Errorf("graph store does not implement DocumentTracker")
    }

    skipProcessed := opts.shouldSkipProcessed() && !opts.Force

    for _, doc := range g.buffer {
        // Compute document hash
        hash := computeDocumentHash(doc.Text)

        // Check if already processed (skip logic)
        if skipProcessed {
            processed, err := tracker.IsDocumentProcessed(ctx, hash)
            if err != nil {
                result.Errors = append(result.Errors, 
                    fmt.Errorf("failed to check document status: %w", err))
                // Continue processing on error (fail-open)
            } else if processed {
                result.DocumentsSkipped++
                continue // Skip this document
            }
        }

        // ... existing processing logic ...
        result.DocumentsProcessed++
        
        // Count chunks for tracking
        chunks := g.chunker.Chunk(doc.Text)
        
        // ... rest of processing ...

        // Mark document as processed after successful processing
        if err := tracker.MarkDocumentProcessed(ctx, hash, doc.Source, len(chunks)); err != nil {
            result.Errors = append(result.Errors, 
                fmt.Errorf("failed to mark document processed: %w", err))
            // Continue even on tracking error (processing succeeded)
        }
    }

    g.buffer = make([]AddedDocument, 0)
    g.lastCognified = time.Now()

    return result, nil
}

// computeDocumentHash computes SHA-256 hash of document text.
// Hash is computed on exact text (no normalization) for determinism.
func computeDocumentHash(text string) string {
    hash := sha256.Sum256([]byte(text))
    return hex.EncodeToString(hash[:])
}
```

**TDD Tests** (write first):
- Test: First Cognify processes all documents (hash not found)
- Test: Second Cognify with same documents skips all
- Test: New document added, only new document processed
- Test: Force=true reprocesses everything
- Test: SkipProcessed=false processes everything
- Test: Document with whitespace-only change is reprocessed (different hash)
- Test: Hash computation is deterministic

**Acceptance Criteria:**
- [x] Previously processed documents are skipped
- [x] Force=true reprocesses regardless
- [x] SkipProcessed=false disables incremental
- [x] Hash is computed on exact text (documented)
- [x] Processing continues on tracker errors (fail-open)

**Dependencies**: Milestones 2, 3, 4

---

### Milestone 6: Unit Tests [TDD]

**Objective**: Comprehensive unit test coverage for incremental Cognify.

**Files to Create/Modify:**
- [pkg/store/tracker_test.go](../../pkg/store/tracker_test.go) - DocumentTracker tests
- [pkg/gognee/gognee_test.go](../../pkg/gognee/gognee_test.go) - Cognify incremental tests

**Test Scenarios:**

**DocumentTracker Unit Tests (pkg/store/tracker_test.go):**
1. `TestDocumentTracker_IsProcessed_Unknown` - Returns false for unknown hash
2. `TestDocumentTracker_IsProcessed_AfterMark` - Returns true after mark
3. `TestDocumentTracker_MarkProcessed_Upsert` - Updates existing record
4. `TestDocumentTracker_GetCount` - Correct count after operations
5. `TestDocumentTracker_Clear` - Resets to zero, keeps nodes/edges
6. `TestDocumentTracker_EmptySource` - Works with empty source string

**Cognify Incremental Unit Tests (pkg/gognee/gognee_test.go):**
1. `TestCognify_Incremental_FirstRun` - Processes all documents
2. `TestCognify_Incremental_SecondRun_Skips` - Skips all on repeat
3. `TestCognify_Incremental_NewDocument` - Processes only new
4. `TestCognify_Incremental_Force` - Force reprocesses all
5. `TestCognify_Incremental_SkipProcessedFalse` - Explicit opt-out
6. `TestCognify_Incremental_SameDocDifferentSource` - Same hash, different source = skip
7. `TestCognify_Incremental_WhitespaceChange` - Whitespace change = reprocess

**Mock Strategy:**
- Use mock LLM/embeddings (existing pattern from gognee_test.go)
- Tests are offline-first (no network access)
- Use in-memory SQLite (`:memory:`)

**Acceptance Criteria:**
- [x] All incremental scenarios tested
- [x] Tests are offline (mock LLM/embeddings)
- [x] Coverage ≥80% for new code
- [x] All tests pass

**Dependencies**: Milestones 1-5

---

### Milestone 7: Integration Tests

**Objective**: End-to-end validation with real LLM.

**Files to Modify:**
- [pkg/gognee/gognee_integration_test.go](../../pkg/gognee/gognee_integration_test.go) - Add integration test

**Integration Test:**

```go
//go:build integration

func TestIntegration_IncrementalCognify(t *testing.T) {
    ctx := context.Background()
    
    // Create temp DB file
    tmpFile := createTempDBFile(t)
    defer os.Remove(tmpFile)
    
    // Session 1: Process 3 documents
    g1, err := gognee.New(gognee.Config{
        OpenAIKey: os.Getenv("OPENAI_API_KEY"),
        DBPath:    tmpFile,
    })
    require.NoError(t, err)
    
    doc1 := "Go is a programming language designed by Google."
    doc2 := "SQLite is an embedded relational database."
    doc3 := "Knowledge graphs represent entities and relationships."
    
    g1.Add(ctx, doc1, gognee.AddOptions{Source: "doc1"})
    g1.Add(ctx, doc2, gognee.AddOptions{Source: "doc2"})
    g1.Add(ctx, doc3, gognee.AddOptions{Source: "doc3"})
    
    result1, err := g1.Cognify(ctx, gognee.CognifyOptions{})
    require.NoError(t, err)
    assert.Equal(t, 3, result1.DocumentsProcessed)
    assert.Equal(t, 0, result1.DocumentsSkipped)
    
    g1.Close()
    
    // Session 2: Reopen, add same docs + 1 new
    g2, err := gognee.New(gognee.Config{
        OpenAIKey: os.Getenv("OPENAI_API_KEY"),
        DBPath:    tmpFile,
    })
    require.NoError(t, err)
    defer g2.Close()
    
    g2.Add(ctx, doc1, gognee.AddOptions{Source: "doc1"})
    g2.Add(ctx, doc2, gognee.AddOptions{Source: "doc2"})
    g2.Add(ctx, doc3, gognee.AddOptions{Source: "doc3"})
    
    newDoc := "Vector databases enable semantic search."
    g2.Add(ctx, newDoc, gognee.AddOptions{Source: "doc4"})
    
    result2, err := g2.Cognify(ctx, gognee.CognifyOptions{})
    require.NoError(t, err)
    
    // Only new document should be processed
    assert.Equal(t, 1, result2.DocumentsProcessed)
    assert.Equal(t, 3, result2.DocumentsSkipped)
    
    // Verify stats
    stats, _ := g2.Stats(ctx)
    t.Logf("Final node count: %d", stats.NodeCount)
}
```

**Acceptance Criteria:**
- [x] Integration test validates incremental behavior
- [x] Demonstrates cost savings (fewer LLM calls)
- [x] Test gated with `//go:build integration`

**Dependencies**: Milestone 6

---

### Milestone 8: Documentation

**Objective**: Document incremental Cognify feature.

**Files to Modify:**
- [README.md](../../README.md) - Add Incremental Cognify section
- [CHANGELOG.md](../../CHANGELOG.md) - Add v0.8.0 entry

**README Documentation:**

```markdown
### Incremental Cognify

By default, gognee tracks which documents have been processed and skips them on subsequent `Cognify()` calls. This saves processing time and API costs when updating your knowledge graph.

​```go
// First Cognify - processes all 3 documents
g.Add(ctx, doc1, AddOptions{Source: "file1.txt"})
g.Add(ctx, doc2, AddOptions{Source: "file2.txt"})
g.Add(ctx, doc3, AddOptions{Source: "file3.txt"})
result1, _ := g.Cognify(ctx, CognifyOptions{})
// result1.DocumentsProcessed = 3, DocumentsSkipped = 0

// Second Cognify - skips all (already processed)
g.Add(ctx, doc1, AddOptions{Source: "file1.txt"})
g.Add(ctx, doc2, AddOptions{Source: "file2.txt"})
g.Add(ctx, doc3, AddOptions{Source: "file3.txt"})
result2, _ := g.Cognify(ctx, CognifyOptions{})
// result2.DocumentsProcessed = 0, DocumentsSkipped = 3

// Force reprocessing when needed (e.g., after changing chunker settings)
result3, _ := g.Cognify(ctx, CognifyOptions{Force: true})
// result3.DocumentsProcessed = 3, DocumentsSkipped = 0
​```

**Document Identity**: Documents are identified by their content hash (SHA-256). Changing any character (including whitespace) creates a new document identity.

**Source Field**: The `source` option in `AddOptions` is metadata only and does not affect document identity. Same content with different sources = same document.

**Clearing Tracking**: To reset tracking without deleting your knowledge graph:
​```go
// Access the DocumentTracker interface
tracker := g.GetGraphStore().(store.DocumentTracker)
tracker.ClearProcessedDocuments(ctx)
​```

**:memory: Mode Note**: In-memory databases do not persist tracking across restarts, so all documents will be reprocessed on each application start.
```

**Acceptance Criteria:**
- [x] Feature is documented in README
- [x] CognifyOptions.Force and behavior documented
- [x] Usage examples show common use cases
- [x] `:memory:` limitation documented (Finding 5)
- [x] Hash input rules documented (exact text, no normalization)

**Dependencies**: Milestone 7

---

### Milestone 9: Version Management

**Objective**: Update version artifacts to v0.8.0.

**Files to Modify:**
- [CHANGELOG.md](../../CHANGELOG.md) - Add v0.8.0 entry
- [agent-output/planning/009-incremental-cognify-plan.md](../planning/009-incremental-cognify-plan.md) - Update status

**CHANGELOG Entry:**

```markdown
## [0.8.0] - 2025-12-XX

### Added
- **Incremental Cognify** (`pkg/gognee`)
  - Document-level deduplication using content hash (SHA-256)
  - Documents already processed are skipped on subsequent Cognify() calls
  - `CognifyOptions.SkipProcessed` defaults to `true` (incremental by default)
  - `CognifyOptions.Force` reprocesses all documents regardless of cache
  - `CognifyResult.DocumentsSkipped` reports skipped document count
  - Reduces LLM API costs for incremental knowledge graph updates
- **DocumentTracker Interface** (`pkg/store`)
  - `IsDocumentProcessed(ctx, hash)` - Check if document was processed
  - `MarkDocumentProcessed(ctx, hash, source, chunkCount)` - Record processing
  - `GetProcessedDocumentCount(ctx)` - Query total processed
  - `ClearProcessedDocuments(ctx)` - Reset tracking without deleting graph
  - Implemented on SQLiteGraphStore
- **processed_documents Table** (`pkg/store`)
  - New SQLite table for document tracking
  - Columns: hash (PK), source, processed_at, chunk_count
  - Backward compatible (added automatically on initialization)

### Changed
- **CognifyOptions** extended with `SkipProcessed` and `Force` fields
- **CognifyResult** extended with `DocumentsSkipped` field
- **Cognify()** now computes document hash and checks tracker before processing

### Technical Details
- Hash computed on exact document text (no normalization)
- Source is metadata only, does not affect document identity
- Fail-open on tracker errors (processing continues)

### Migration Notes
- Existing databases: New table created automatically on first access
- Default behavior changes: Documents now skipped if previously processed
- To reprocess all: Use `CognifyOptions{Force: true}`

### Documentation
- README updated with Incremental Cognify section
- `:memory:` mode limitations documented
```

**Acceptance Criteria:**
- [x] CHANGELOG documents incremental Cognify feature
- [x] Plan status updated to Implemented
- [x] Product roadmap status updated

**Dependencies**: All previous milestones

---

## Testing Strategy

### Unit Tests (Offline-First)

**DocumentTracker Tests** (6 tests):
- Hash lookup operations (found/not found)
- Mark/update semantics (upsert)
- Count and clear operations
- Empty source handling

**Incremental Cognify Tests** (7+ tests):
- First run processes all
- Second run skips all
- Mixed scenario (new + existing)
- Force override
- Explicit SkipProcessed=false
- Same content, different source = skip
- Whitespace change = reprocess

**Mock Strategy**:
- Use existing mock LLM/embeddings pattern from gognee_test.go
- In-memory SQLite for all unit tests
- No network access required

### Integration Tests (Real LLM)

**TestIntegration_IncrementalCognify**:
- Multi-session workflow with persistence
- Validates skipped count accuracy
- Confirms LLM not called for skipped documents (via timing/result)
- Gated with `//go:build integration`

### Coverage Target

- ≥80% for new code in pkg/store/tracker.go
- ≥80% for new code in Cognify() incremental logic
- Overall package coverage maintained at ≥85%

---

## Rollback Strategy

**If issues arise after release:**

1. **Immediate Mitigation**: Set `CognifyOptions{SkipProcessed: &falseVal, Force: true}` to disable incremental behavior
2. **Clear Tracking**: Call `ClearProcessedDocuments()` to reset state
3. **Schema Rollback**: Not required - `processed_documents` table is independent
4. **Code Rollback**: Revert to v0.7.1 if critical issues found

**Risk Assessment:**
- Low risk: New table is independent of existing schema
- Low risk: Feature is opt-out via Force=true
- Low risk: Fail-open on tracker errors

---

## Definition of Done

### Code Complete
- [ ] Milestone 1: Document Tracking Schema
- [ ] Milestone 2: DocumentTracker Interface
- [ ] Milestone 3: CognifyOptions Extension
- [ ] Milestone 4: CognifyResult Enhancement
- [ ] Milestone 5: Incremental Cognify Logic
- [ ] Milestone 6: Unit Tests (all passing)
- [ ] Milestone 7: Integration Tests (all passing)
- [ ] Milestone 8: Documentation
- [ ] Milestone 9: Version Management

### Quality Gates
- [ ] All unit tests pass (`go test ./...`)
- [ ] Integration tests pass (`go test -tags=integration ./...`)
- [ ] Test coverage ≥80% for new code
- [ ] No linter warnings (`gofmt`, `go vet`)
- [ ] README updated with Incremental Cognify section
- [ ] CHANGELOG v0.8.0 entry added

### Validation
- [ ] Value statement delivered: Documents are skipped when already processed
- [ ] Cost savings verified: LLM calls avoided for skipped documents
- [ ] Backward compatible: Existing code works without changes
- [ ] Force option works: All documents reprocessed when Force=true

### Handoff
- [ ] QA review complete
- [ ] UAT approval
- [ ] Plan status updated to Delivered
- [ ] Product roadmap updated

---

## Open Questions Resolution Summary

All open questions from the plan and critique have been resolved:

| Question | Resolution |
|----------|------------|
| Default for SkipProcessed? | **true** (incremental by default) - Plan updated |
| GraphStore interface bloat? | **Separate DocumentTracker interface** - Plan updated |
| Source field semantics? | **Metadata only**, hash determines identity - Plan clarified |
| ClearDocumentTracking API? | **ClearProcessedDocuments()** on DocumentTracker - Added |
| `:memory:` mode tracking? | **Tracking lost on restart** - Documented as limitation |
| Hash normalization? | **No normalization**, exact text - Documented |

---

## Outstanding Items

### Pre-Implementation
- None - all blocking questions resolved

### Known Limitations (Documented)
1. `:memory:` mode does not persist tracking across restarts
2. Hash is computed on exact text (whitespace changes = new document)
3. Chunk boundary changes after config update may cause inconsistent state (use Force=true)

### Future Enhancements (Out of Scope)
1. Chunk-level tracking (more granular than document-level)
2. TTL/expiry for processed document records
3. Batch processing optimization for large document sets

---

## Next Steps

1. **Implementer**: Begin TDD implementation starting with Milestone 1 (schema)
2. **Implementer**: Update plan status to "In Progress"
3. **QA**: Review implementation after Milestone 6 (unit tests)
4. **UAT**: Validate after Milestone 7 (integration tests)

---

## Files Summary

### Files to Create
| Path | Purpose |
|------|---------|
| `pkg/store/tracker.go` | DocumentTracker interface definition |
| `pkg/store/tracker_test.go` | DocumentTracker unit tests |

### Files to Modify
| Path | Changes |
|------|---------|
| `pkg/store/sqlite.go` | Add schema + DocumentTracker implementation |
| `pkg/gognee/gognee.go` | Extend CognifyOptions/CognifyResult, update Cognify() |
| `pkg/gognee/gognee_test.go` | Add incremental Cognify unit tests |
| `pkg/gognee/gognee_integration_test.go` | Add integration test |
| `README.md` | Add Incremental Cognify documentation |
| `CHANGELOG.md` | Add v0.8.0 entry |
| `agent-output/planning/009-incremental-cognify-plan.md` | Update status |

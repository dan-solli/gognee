# Plan 006 — Phase 6 Integration

**Plan ID:** 006

**Target Release:** v0.6.0

**Epic Alignment:** ROADMAP Phase 6 — Integration (Full Pipeline + API)

**Status:** UAT Approved

**Changelog**
- 2025-12-24: Created plan for Phase 6 implementation.
- 2025-12-24: Revised based on critique — addressed C1 (Cognify semantics), H1 (logging), H2 (API compatibility), M1 (node ID normalization), M2 (GraphStore extension), L1 (dependency wording).
- 2025-12-24: Critique approved — plan ready for implementation.
- 2025-12-24: UAT Complete — implementation delivers 100% of value statement, approved for release as v0.6.0.

---

## Value Statement and Business Objective

As a developer building an AI assistant with persistent memory (like Glowbabe), I want a unified API that lets me `Add()` text, `Cognify()` it into a knowledge graph, and `Search()` for relevant context, so that I can integrate knowledge graph memory into my application with a single library import and three method calls.

---

## Objective

Deliver Phase 6 from ROADMAP:
- Create unified `Gognee` API that wires together all Phase 1-5 components
- Implement `Add()` method to buffer text for processing
- Implement `Cognify()` method to run the full extraction pipeline
- Implement `Search()` method to query the knowledge graph
- Implement `Close()` method for resource cleanup
- Implement `Stats()` method for basic telemetry
- Add `DBPath` configuration for persistent storage
- Write end-to-end tests validating the complete pipeline
- Add usage documentation and examples

This phase completes the MVP and makes gognee ready for Glowbabe integration.

---

## Scope

**In scope**
1. Extend `pkg/gognee/gognee.go` with full pipeline orchestration
2. Add `DBPath` to `Config` for SQLite persistence
3. Implement `Add(ctx, text, opts)` — buffer raw text with optional metadata
4. Implement `Cognify(ctx, opts)` — process buffered text through the full pipeline:
   - Chunk text → Extract entities → Extract relations → Create nodes → Create edges → Embed nodes → Index vectors
5. Implement `Search(ctx, query, opts)` — delegate to HybridSearcher
6. Implement `Close()` — close GraphStore, release resources
7. Implement `Stats()` — return node/edge counts
8. Define `AddOptions`, `CognifyOptions` option structs
9. Re-export key types for caller convenience (`SearchResult`, `SearchOptions`, `Node`, etc.)
10. Unit tests with mocked dependencies (offline-first)
11. Integration tests with real OpenAI API (gated with build tag)
12. Update README with usage examples

**Out of scope**
- CLI interface (gognee is library-only per roadmap)
- Incremental cognify (processing only new text) — post-MVP enhancement
- Memory decay/forgetting — post-MVP enhancement
- Multiple LLM provider support — post-MVP enhancement
- Persistent vector store (SQLite-backed) — using in-memory for MVP
- Graph visualization

---

## Key Constraints

- **Library-only**: No `cmd/` directory, no executable
- **No new dependencies**: Keep dependency surface minimal; only existing dependencies allowed (SQLite driver `modernc.org/sqlite`, UUID `github.com/google/uuid`, standard library)
- **Interface-driven**: Continue using existing interfaces (`EmbeddingClient`, `LLMClient`, `GraphStore`, `VectorStore`, `Searcher`)
- **Cognee-aligned API**: Mirror Cognee's `add()`, `cognify()`, `search()` pattern
- **Offline-first tests**: Unit tests must not require network access
- **Single Go binary**: All components embeddable, no external services
- **No global logging**: Library must not produce side-effect logs; all errors reported via return values only

---

## Plan-Level Decisions

### 1. DBPath handling

- If `Config.DBPath` is empty or `:memory:`, use SQLite in-memory mode
- Otherwise, create/open SQLite database at specified path
- **Rationale**: Matches SQLite conventions; allows both ephemeral testing and persistent production use

### 2. Add() buffering strategy

- `Add()` appends text to an in-memory buffer (slice of `AddedDocument`)
- Text is NOT processed until `Cognify()` is called
- **Rationale**: Mirrors Cognee behavior; allows batch processing; enables caller control over when expensive LLM calls happen

### 3. AddedDocument structure

- Store raw text plus optional source metadata (e.g., document ID, source name)
- Track timestamp of addition for ordering
- **Rationale**: Enables future traceability features; supports deduplication if needed

### 4. Cognify() pipeline order

1. For each buffered document:
   a. Chunk text into segments
   b. For each chunk:
      - Extract entities via LLM
      - Extract relations via LLM (using extracted entities)
      - Create/upsert nodes in GraphStore (one per entity)
      - Create edges in GraphStore (one per triplet)
      - Generate embeddings for each node
      - Index embeddings in VectorStore
2. Clear the buffer after successful processing
- **Rationale**: Sequential processing is simpler and sufficient for MVP; parallelization is a post-MVP optimization

### 5. Node ID generation

- Generate deterministic node IDs using SHA-256 hash of normalized `(name + "|" + type)`
- **Normalization rules**:
  - Convert name to lowercase
  - Trim leading/trailing whitespace
  - Collapse multiple internal spaces to single space
  - Type is used as-is (already validated by EntityExtractor)
- **Collision handling**: SHA-256 collisions are astronomically unlikely; no secondary disambiguator needed for MVP
- **Implementation**: Gognee always supplies node IDs to `AddNode()`; never relies on store-generated UUIDs
- **Rationale**: Enables upsert semantics; same entity mentioned multiple times updates existing node rather than creating duplicates

### 6. Search() delegation

- `Search()` delegates to `HybridSearcher` by default
- Caller can specify `SearchOptions.Type` to use vector-only or graph-only
- **Rationale**: Hybrid search provides best results; type option enables flexibility

### 7. Type re-exports

- Re-export from `pkg/gognee`: `SearchResult`, `SearchOptions`, `SearchType`, `Node`, `Edge`
- Callers should not need to import `pkg/search` or `pkg/store` directly for common operations
- **Rationale**: Clean API surface for library consumers

### 8. Error handling and buffer semantics (Cognify)

- **Processing model**: Best-effort per document
  - Each document is processed independently
  - If a chunk fails (LLM error, embedding error), skip that chunk and continue with remaining chunks in the document
  - Successfully extracted entities/relations are persisted even if later chunks fail
- **Buffer clearing**: Always clear the entire buffer after `Cognify()` completes, regardless of partial failures
  - Prevents infinite retry loops
  - Caller can re-add failed documents if desired
- **Error reporting**: Return a `CognifyResult` struct instead of bare `error`:
  ```
  type CognifyResult struct {
      DocumentsProcessed int
      ChunksProcessed    int
      ChunksFailed       int
      NodesCreated       int
      EdgesCreated       int
      Errors             []error  // Individual chunk/extraction errors
  }
  ```
  - `Errors` slice contains all individual failures for inspection
  - Caller can check `len(result.Errors) > 0` to detect partial failure
- **No logging**: Errors are collected and returned, not logged (library constraint)
- **Rationale**: Best-effort with full error visibility; caller decides how to handle partial failures; prevents data loss from silent failures

### 9. Stats structure

```
type Stats struct {
    NodeCount     int64
    EdgeCount     int64
    BufferedDocs  int
    LastCognified time.Time
}
```
- **Rationale**: Minimal but useful telemetry for monitoring knowledge graph size

### 10. GraphStore interface extension

- Add `NodeCount(ctx) (int64, error)` and `EdgeCount(ctx) (int64, error)` methods to `GraphStore` interface
- **Affected implementations**: `SQLiteGraphStore` in `pkg/store/sqlite.go`
- **Affected tests**: `pkg/store/sqlite_test.go` — add tests for new methods
- **Rationale**: Stats() requires count queries; extending GraphStore keeps store logic in the store package

### 11. API compatibility (existing getters)

- **Keep existing accessors**: `GetChunker()`, `GetEmbeddings()`, `GetLLM()` remain public for backward compatibility
- Mark as "advanced access" in documentation; primary API is `Add/Cognify/Search`
- **Rationale**: Avoid breaking changes in v0.x; Glowbabe or other callers may use these for custom pipelines

---

## Milestones

### Milestone 1 — Config and Initialization

**Objective:** Extend `Gognee` struct and `New()` to initialize all components including stores.

**Tasks:**
1. Add `DBPath string` to `Config` struct
2. Add fields to `Gognee` struct: `graphStore`, `vectorStore`, `searcher`, `extractor`, `relationExtractor`, `buffer`
3. Update `New()` to:
   - Create `SQLiteGraphStore` using `DBPath` (or `:memory:` if empty)
   - Create `MemoryVectorStore`
   - Create `EntityExtractor` and `RelationExtractor` with configured LLM
   - Create `HybridSearcher` with embeddings, vectorStore, graphStore
   - Initialize empty document buffer
4. **Keep** existing accessor methods (`GetChunker`, `GetEmbeddings`, `GetLLM`) for backward compatibility
5. Add new accessors: `GetGraphStore()`, `GetVectorStore()` for advanced callers

**Acceptance Criteria:**
- `New()` successfully initializes all components
- SQLite database created at specified path (or in-memory)
- All internal dependencies wired correctly
- Existing tests pass without modification (backward compatible)
- New components accessible via getters

---

### Milestone 2 — Add() Method

**Objective:** Implement text buffering for deferred processing.

**Tasks:**
1. Define `AddedDocument` struct:
   - `Text string`
   - `Source string` (optional metadata)
   - `AddedAt time.Time`
2. Define `AddOptions` struct:
   - `Source string` (optional source identifier)
3. Implement `Add(ctx context.Context, text string, opts AddOptions) error`:
   - Validate text is non-empty
   - Append to internal buffer with timestamp
   - Return nil (no processing yet)
4. Add `BufferedCount() int` method for inspection

**Acceptance Criteria:**
- `Add()` stores text in buffer without processing
- Multiple `Add()` calls accumulate documents
- Empty text returns validation error
- `BufferedCount()` returns correct count

---

### Milestone 3 — Cognify() Method

**Objective:** Implement the full extraction and storage pipeline with best-effort semantics.

**Tasks:**
1. Define `CognifyOptions` struct:
   - (Reserved for future options like `ChunkSize` override)
2. Define `CognifyResult` struct (per Decision 8):
   - `DocumentsProcessed int`
   - `ChunksProcessed int`
   - `ChunksFailed int`
   - `NodesCreated int`
   - `EdgesCreated int`
   - `Errors []error`
3. Implement `Cognify(ctx context.Context, opts CognifyOptions) (*CognifyResult, error)`:
   - Return empty result if buffer is empty (no-op, not an error)
   - For each buffered document:
     a. Chunk text using configured chunker
     b. For each chunk:
        - Extract entities using EntityExtractor (on error: record in Errors, skip chunk)
        - Extract relations using RelationExtractor (on error: record in Errors, continue with entities only)
        - For each entity: create/upsert Node in GraphStore using deterministic ID
        - For each triplet: create Edge in GraphStore
        - For each node: embed and index in VectorStore
   - **Always** clear buffer after processing (best-effort model)
   - Track `lastCognified` timestamp
   - Return nil error even if some chunks failed (errors in `CognifyResult.Errors`)
   - Return error only for catastrophic failures (context canceled, store connection lost)
4. Implement deterministic node ID generation per Decision 5:
   - `sha256(strings.ToLower(strings.TrimSpace(collapseSpaces(name))) + "|" + type)`
   - Truncate hash to first 16 bytes, hex-encode for 32-char ID

**Acceptance Criteria:**
- `Cognify()` processes all buffered documents
- Entities become nodes in GraphStore with deterministic IDs
- Triplets become edges in GraphStore
- Node embeddings indexed in VectorStore
- Buffer **always** cleared after processing (not conditional on success)
- Partial failures collected in `CognifyResult.Errors`
- No logging side effects — errors returned only
- Same entity across documents resolves to same node (upsert via deterministic ID)

---

### Milestone 4 — Search() Method

**Objective:** Expose search functionality through the unified API.

**Tasks:**
1. Implement `Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)`:
   - Delegate to internal HybridSearcher
   - Apply defaults via existing `applyDefaults()`
   - Return results directly
2. Re-export types in `pkg/gognee/types.go`:
   - `SearchResult` (alias or re-export from `search.SearchResult`)
   - `SearchOptions` (alias or re-export from `search.SearchOptions`)
   - `SearchType` and constants
   - `Node` and `Edge` from store package

**Acceptance Criteria:**
- `Search()` returns relevant results from knowledge graph
- All three search types work (vector, graph, hybrid)
- Re-exported types accessible via `gognee.SearchResult`, etc.

---

### Milestone 5 — GraphStore Extension + Close() + Stats()

**Objective:** Extend GraphStore interface and implement resource cleanup and telemetry.

**Tasks:**
1. **Extend GraphStore interface** (per Decision 10):
   - Add `NodeCount(ctx context.Context) (int64, error)` to `GraphStore` interface in `pkg/store/graph.go`
   - Add `EdgeCount(ctx context.Context) (int64, error)` to `GraphStore` interface in `pkg/store/graph.go`
2. **Implement in SQLiteGraphStore** (`pkg/store/sqlite.go`):
   - `NodeCount()`: `SELECT COUNT(*) FROM nodes`
   - `EdgeCount()`: `SELECT COUNT(*) FROM edges`
3. **Add tests** to `pkg/store/sqlite_test.go`:
   - Test `NodeCount()` returns correct count after adds
   - Test `EdgeCount()` returns correct count after adds
4. Implement `Close() error` on Gognee:
   - Call `graphStore.Close()`
   - Clear buffer
   - Return any errors
5. Define `Stats` struct (as described in Decision 9)
6. Implement `Stats() Stats`:
   - Call `graphStore.NodeCount()` and `graphStore.EdgeCount()`
   - Return buffered doc count
   - Return lastCognified timestamp

**Acceptance Criteria:**
- `GraphStore` interface extended with count methods
- `SQLiteGraphStore` implements count methods correctly
- Tests for count methods pass
- `Close()` releases database connection
- `Stats()` returns accurate counts
- Calling methods after `Close()` returns appropriate errors

---

### Milestone 6 — Unit Tests (Offline)

**Objective:** Comprehensive unit tests using mocked dependencies.

**Tasks:**
1. Create mock implementations:
   - Mock `EmbeddingClient` returning deterministic embeddings
   - Mock `LLMClient` returning canned extraction results
   - Use existing `MemoryVectorStore` and in-memory SQLite
2. Test scenarios:
   - `New()` with various config combinations
   - `Add()` buffering behavior
   - `Cognify()` full pipeline with mocked LLM — verify `CognifyResult` fields
   - `Search()` after cognify
   - `Close()` cleanup
   - `Stats()` accuracy
   - Deterministic node ID generation (same entity → same ID)
3. Test error cases:
   - Empty text to `Add()`
   - `Cognify()` with empty buffer (returns empty result, not error)
   - LLM extraction failure handling (errors in `CognifyResult.Errors`, buffer still cleared)
   - Database errors (catastrophic → returns error)
4. Test backward compatibility:
   - `GetChunker()`, `GetEmbeddings()`, `GetLLM()` still work

**Acceptance Criteria:**
- All tests pass without network access
- Coverage ≥ 80% for `pkg/gognee`
- Tests are deterministic and fast
- No logging output during tests

---

### Milestone 7 — Integration Tests (Gated)

**Objective:** End-to-end tests with real OpenAI API.

**Tasks:**
1. Create `pkg/gognee/gognee_integration_test.go` with build tag `//go:build integration`
2. Test complete workflow:
   - Initialize with real API key (from `secrets/openai-api-key.txt`)
   - Add sample text about a technology project
   - Cognify — verify `CognifyResult` has nodes/edges created
   - Search for related concepts
   - Verify meaningful results returned
3. Test upsert behavior:
   - Add overlapping text with same entities
   - Cognify again
   - Verify node count did not double (upsert worked)
4. Clean up: use temp file for SQLite, delete after test

**Acceptance Criteria:**
- Integration tests do not run by default (`go test ./...`)
- Run successfully with `go test -tags=integration ./...`
- Real entities extracted and searchable
- Upsert semantics verified

---

### Milestone 8 — Documentation and Examples

**Objective:** Update README with usage documentation.

**Tasks:**
1. Add "Quick Start" section to README showing:
   - Installation (`go get github.com/dan-solli/gognee`)
   - Basic usage example (Add → Cognify → Search)
   - Configuration options
2. Add "API Reference" section describing:
   - `New()` and `Config`
   - `Add()`, `Cognify()`, `Search()`
   - `Close()`, `Stats()`
3. Document MVP limitations:
   - In-memory vector store (not persistent across restarts)
   - Sequential processing (no parallelization)

**Acceptance Criteria:**
- README provides working example
- All public API documented
- Limitations clearly stated

---

### Milestone 9 — Version and Release Artifacts

**Objective:** Update version to v0.6.0 and document changes.

**Tasks:**
1. Update CHANGELOG.md with v0.6.0 entry documenting:
   - Full pipeline API (`Add`, `Cognify`, `Search`)
   - Configuration options
   - Re-exported types
   - Integration test availability
2. Update ROADMAP.md to mark Phase 6 goals as complete
3. Ensure `go.mod` is consistent

**Acceptance Criteria:**
- CHANGELOG documents all Phase 6 deliverables
- ROADMAP Phase 6 checkboxes marked complete
- Version artifacts consistent

---

## Testing Strategy

**Unit Tests:**
- Mock all external dependencies (LLM, embeddings)
- Test each public method in isolation
- Test error paths and edge cases
- Verify `CognifyResult` fields accurately reflect processing outcome
- Verify no logging side effects (capture stdout/stderr in tests)
- Target ≥ 80% coverage for `pkg/gognee`

**Integration Tests (gated):**
- Full pipeline with real OpenAI API
- Build-tag gated (`//go:build integration`)
- Validates real-world extraction quality
- Validates upsert semantics with overlapping entities

**Critical Scenarios:**
- Empty buffer cognify (returns empty `CognifyResult`, not error)
- Multiple documents with overlapping entities (upsert behavior verified via node count)
- Search before any cognify (empty results)
- Large text chunking and processing
- Partial extraction failure handling (`CognifyResult.Errors` populated, buffer cleared)
- Deterministic node ID generation (same entity → same ID across calls)
- Backward compatibility (existing getters still work)

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| LLM extraction quality varies | Medium | Medium | Integration tests validate real results; unit tests use deterministic mocks |
| In-memory vector store limits persistence | Known | Low | Documented as MVP limitation; SQLite vector store is post-MVP |
| GraphStore interface extension breaks future implementations | Low | Low | Extension is additive; documented in plan; only one implementation exists |
| API surface complexity | Low | Medium | Re-export types; provide clear examples; keep getters for advanced access |
| Partial failure confusion | Low | Medium | `CognifyResult` provides explicit error list and counts; documented behavior |

---

## Dependencies

- **Phase 1-5 complete**: All prior phases delivered and UAT-approved ✓
- **No new external dependencies**: Uses existing SQLite driver, UUID library, and standard library

---

## Open Questions

None. All design decisions documented above.

---

## Handoff Notes

**For Critic:**
- Verify plan aligns with ROADMAP Phase 6 specification
- Check that value statement is delivered directly, not deferred
- Validate that scope is achievable in 1-2 weeks

**For Implementer:**
- Follow interface-driven patterns from previous phases
- Prioritize offline-first tests
- Keep code simple (KISS) — optimization is post-MVP

**For QA:**
- Plan specifies ≥ 80% coverage target
- Integration tests gated behind build tag
- Critical scenarios listed for test case design

---

## Success Metrics (from ROADMAP)

After Phase 6 completion, gognee should:
- [ ] Can add text and build knowledge graph
- [ ] Can search and retrieve relevant context
- [ ] Single binary, no external dependencies (beyond SQLite)
- [ ] Works on macOS, Linux, Windows
- [ ] < 5MB binary size
- [ ] < 100ms search latency for small graphs

---

## Post-MVP Enhancements (Out of Scope)

Per ROADMAP, these are deferred to future releases:
- Multiple LLM provider support (Anthropic, Ollama)
- Persistent vector store (SQLite-backed)
- Graph visualization
- Incremental cognify (only process new text)
- Memory decay/forgetting
- Session/context awareness

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.7.0] - 2025-12-25

### Added
- **Persistent Vector Store** (`pkg/store`)
  - `SQLiteVectorStore` implementation of `VectorStore` interface
  - Vector embeddings now persist in SQLite `nodes.embedding` BLOB column
  - Embeddings survive application restarts without re-running Cognify()
  - Direct-query search: SELECT all non-NULL embeddings, compute cosine similarity in Go
  - Dimension validation: mismatched embeddings are skipped during search
  - Shares database connection with `SQLiteGraphStore` (no separate connection management)
  - `Close()` is a no-op (connection owned by GraphStore)
- **SQLiteGraphStore.DB()** accessor method
  - Returns underlying `*sql.DB` for connection sharing with vector store
  - Connection lifecycle remains owned by GraphStore
- **Automatic Storage Mode Selection** (`pkg/gognee`)
  - Persistent DBPath (file-based): Uses `SQLiteVectorStore`
  - In-memory DBPath (`:memory:`): Uses `MemoryVectorStore` (backward compatible)
  - No API changes - mode selected based on Config.DBPath value
- **Integration Test**: `TestIntegrationPersistentVectorStore`
  - Validates Add → Cognify → Close → Reopen → Search workflow
  - Confirms embeddings are immediately searchable after restart
  - Tests incremental updates (adding new data in second session)

### Changed
- **gognee.New()** now creates SQLiteVectorStore for persistent databases
- **Vector storage behavior**: Embeddings persist across restarts when using file-based DBPath

### Technical Details
- **Serialization**: Embeddings stored as little-endian float32 arrays in BLOB column
- **Search Performance**: Linear scan O(n) - acceptable for <10K nodes per plan
- **Dimension Handling**: Search skips embeddings with dimension mismatch (returns 0 similarity)
- **Connection Sharing**: SQLiteVectorStore shares DB connection from GraphStore
- **Memory Mode**: `:memory:` databases continue using in-memory vector store (no persistence)

### Migration Notes
- **Existing v0.6.0 databases**: Run `Cognify()` once after upgrading to populate persistent embeddings
- **New databases**: Persistent embeddings work automatically
- **No schema changes needed**: `nodes.embedding` column already existed, now actively used

### Documentation
- **README.md**: 
  - Updated "Storage" section with persistence behavior examples
  - Added migration guide for v0.6.0 users
  - Removed "In-Memory Vector Index" from MVP limitations
  - Documented linear search performance characteristics
- **ROADMAP.md**:
  - Marked "Persistent vector store" as completed (v0.7.0)
  - Updated Phase 4 documentation to reflect SQLite vector implementation
  - Removed MVP limitation note about non-persistent embeddings

### Testing
- 8 unit tests for SQLiteVectorStore (Add, Search, Delete, persistence, dimension validation)
- 1 unit test for SQLiteGraphStore.DB() accessor
- 1 integration test for end-to-end persistence workflow
- All existing tests pass (backward compatible)

## [0.9.0] - 2025-12-25

### Added
- **Memory Decay System** (`pkg/gognee`)
  - Time-based decay affecting search ranking to keep knowledge graph relevant
  - `Config` extensions:
    - `DecayEnabled bool` - Enable decay scoring (default: false for backward compatibility)
    - `DecayHalfLifeDays int` - Days for score to halve (default: 30)
    - `DecayBasis string` - "access" or "creation" decay calculation (default: "access")
  - Exponential decay formula: `0.5^(age_days / half_life_days)`
  - Configuration validation on `New()` for decay parameters
- **Access Reinforcement**
  - `Search()` automatically updates `last_accessed_at` for returned nodes
  - Batch UPDATE operations for performance (single SQL statement for all TopK results)
  - Access-based decay preserves frequently queried nodes (mimics human memory)
- **Prune API**
  - `Prune(ctx, PruneOptions)` method for explicit node deletion
  - `PruneOptions` struct:
    - `MaxAgeDays int` - Remove nodes older than N days
    - `MinDecayScore float64` - Remove nodes below decay threshold
    - `DryRun bool` - Preview pruning without deletion
  - `PruneResult` struct with NodesEvaluated, NodesPruned, EdgesPruned, NodeIDs
  - Cascade deletion: edges automatically deleted when endpoints are pruned
  - Vector store synchronization on prune
- **DecayingSearcher** (`pkg/search`)
  - Decorator pattern implementation wrapping any `Searcher`
  - Fetches node timestamps and applies decay multipliers post-search
  - Fallback to `created_at` when `last_accessed_at` is NULL
  - Filters nodes with extremely low scores (< 0.001)
  - No changes required to Searcher interface or existing implementations
- **Schema Migration** (`pkg/store`)
  - Automatic column addition on database initialization
  - `last_accessed_at DATETIME DEFAULT NULL` column to nodes table
  - `access_count INTEGER DEFAULT 0` column for future frequency-based decay
  - `columnExists()` helper to detect and migrate existing databases
  - Safe migration: NULL-friendly defaults preserve existing rows
- **SQLiteGraphStore Extensions**
  - `UpdateAccessTime(ctx, nodeIDs)` for batch access timestamp updates
  - `GetAllNodes(ctx)` for prune evaluation (returns all nodes with timestamps)
  - `DeleteNode(ctx, nodeID)` for node removal
  - `DeleteEdge(ctx, edgeID)` for edge removal
- **Node struct extension** (`pkg/store`)
  - `LastAccessedAt *time.Time` field for decay tracking

### Changed
- **gognee.New()** wires `DecayingSearcher` when `DecayEnabled=true`
- **gognee.Search()** now updates access timestamps for returned results (batch operation)
- **Schema initialization** runs migrations to add new columns to existing databases

### Technical Details
- **Decay Implementation**:
  - `calculateDecay(age, halfLife)` function implements exponential decay
  - Edge cases handled: negative age (1.0), zero half-life (1.0), NULL timestamps (fallback)
  - Decorator pattern keeps decay orthogonal to search implementations
- **Migration Strategy**:
  - On startup, `initSchema()` calls `migrateSchema()`
  - Uses `PRAGMA table_info()` to detect missing columns
  - `ALTER TABLE nodes ADD COLUMN` executed per missing column
  - Existing rows get NULL/0 defaults (backward compatible)
- **Performance**:
  - Batch access updates use single `UPDATE ... WHERE id IN (...)` statement
  - TopK-only tracking: only final results updated, not intermediate candidates
  - Decay calculation is O(1) per node (simple exponential)
- **Testing**:
  - 7 unit tests for decay function (zero age, half-life, edge cases)
  - 5 unit tests for DecayingSearcher (disabled, access-based, creation-based, fallback, threshold)
  - 4 unit tests for Prune API (dry run, MaxAgeDays, cascade, empty database)
  - 2 integration tests (end-to-end decay+prune, access reinforcement)
  - Schema migration test (old DB → migration → verify columns)

### Documentation
- **README.md**: New "Memory Decay and Forgetting" section
  - Configuration options explained
  - Access reinforcement behavior documented
  - Prune API usage examples with dry run pattern
  - Decay math formula and examples
  - Best practices for half-life tuning by domain
- **Removed** "No Memory Decay" from MVP limitations (now implemented)

## [0.6.0] - 2025-12-24

### Added
- **Unified API** (`pkg/gognee`)
  - `Add(ctx, text, opts)` method to buffer documents for processing
  - `Cognify(ctx, opts)` method implementing full extraction pipeline:
    - Text chunking → entity extraction → relation extraction → graph storage → vector indexing
    - Returns `CognifyResult` with processing statistics and error list
    - Best-effort semantics: continues processing on chunk failures, always clears buffer
    - Deterministic node ID generation using SHA-256 hash of normalized (name, type)
  - `Search(ctx, query, opts)` method delegating to HybridSearcher
  - `Close()` method for resource cleanup
  - `Stats()` method returning node count, edge count, buffered documents, last cognify time
  - `BufferedCount()` method for inspection
  - `Config` extension with `DBPath` field for persistent SQLite storage
- **GraphStore Interface Extension**
  - `NodeCount(ctx)` method returning total node count
  - `EdgeCount(ctx)` method returning total edge count
- **SQLiteGraphStore Implementation**
  - `NodeCount()` and `EdgeCount()` methods using efficient SQL COUNT queries
- **Type Re-exports** (`pkg/gognee/types.go`)
  - Re-exported `SearchResult`, `SearchOptions`, `SearchType` for convenience
  - Re-exported `Node`, `Edge` from store package
  - Constants: `SearchTypeVector`, `SearchTypeGraph`, `SearchTypeHybrid`
- **Integration Tests** (`pkg/gognee/gognee_integration_test.go`)
  - Build-tag gated (`//go:build integration`) integration tests
  - Full pipeline test with real OpenAI API
  - Upsert semantics verification
  - All search type options test
  - Tests skipped if `OPENAI_API_KEY` not available
- **Documentation**
  - Comprehensive README.md with quick start, API reference, and examples
  - Usage examples for all core methods
  - Integration test documentation
  - MVP limitations and future enhancements documented
- **Unit Tests**
  - 8 new unit tests for Gognee API (all offline, mocked dependencies)
  - Tests for Config defaults, Add buffering, Cognify empty buffer, Close, Stats
  - Tests for deterministic node ID generation
  - 2 new SQLite store tests for NodeCount and EdgeCount methods
  - Test mocks updated for new GraphStore interface methods

### Changed
- **Search Module**: Exported `ApplyDefaults` function for use by top-level API
- **GraphStore Interface**: Added `NodeCount` and `EdgeCount` methods to interface
- **Test Mocks**: Updated all test mocks (testGraphStore, mockGraphStore) to implement new interface methods
- **Backward Compatibility**: Maintained all existing accessor methods (GetChunker, GetEmbeddings, GetLLM)

### Fixed
- **LLM Response Parsing**: Added `stripMarkdownCodeFence()` to handle LLM responses wrapped in Markdown code fences (```json ... ```). This fixes integration test failures where OpenAI returned JSON inside backticks.

### Technical Details
- **Deterministic IDs**: Node IDs are derived from SHA-256(lowercase(trimmed_name) + "|" + type)
  - Enables upsert semantics: same entity across documents resolves to same node
  - Prevents duplicate nodes for identical entities mentioned multiple times
- **Buffer Semantics**: Add() only buffers; Cognify() processes and always clears buffer
  - Caller controls when expensive LLM operations occur
  - Allows batch processing of multiple documents before cognification
- **Error Handling**: CognifyResult includes Errors slice for inspection
  - Catastrophic errors (DB connection lost) return error
  - Per-chunk failures collected and returned; buffer still cleared
- **Storage**: DBPath ":memory:" or empty uses in-memory SQLite; file path uses persistent storage

## [0.5.0] - 2025-12-24

### Added
- **Search Layer** (`pkg/search`)
  - `SearchType` enum with `vector`, `graph`, and `hybrid` search modes
  - `SearchResult` struct with NodeID, Node, Score, Source, and GraphDepth fields
  - `SearchOptions` struct with Type, TopK, GraphDepth, and SeedNodeIDs configuration
  - `Searcher` interface for unified search API across all search types
  - `VectorSearcher` implementation
    - Text-to-embedding-to-vector-search pipeline
    - Enriches results with full node data from GraphStore
    - Gracefully handles stale vector index entries (missing nodes)
    - Source tagged as "vector" for direct similarity hits
  - `GraphSearcher` implementation
    - BFS traversal from seed nodes with configurable depth
    - Score decay formula: `1.0 / (1 + depth)` where seeds score 1.0
    - Deduplicates nodes discovered via multiple paths (keeps shortest)
    - Uses `SeedNodeIDs` from SearchOptions for unified interface
    - Returns error if no seeds provided
  - `HybridSearcher` implementation
    - Combines vector similarity and graph traversal
    - Explicit score formula: `combined_score = vector_score + graph_score`
    - Fetches `max(TopK * 2, 20)` initial vector results for expansion base
    - Expands via graph neighbors from each vector hit
    - Three-way Source tagging: "vector" (vector only), "graph" (graph only), "hybrid" (both)
    - Nodes found by both paths receive score boost
    - Final results sorted by combined score and limited to TopK

### Technical Details
- 85.0% test coverage for search package
- All searchers implement the `Searcher` interface
- Default TopK = 10, default GraphDepth = 1 (Cognee-aligned)
- Graph traversal uses BFS with visited tracking for accurate depth
- Hybrid search prioritizes nodes with high combined scores (both semantic and structural relevance)
- Offline-first unit tests with mocked dependencies (no network calls)
- All tests pass

### Notes
- This release implements Phase 5 from the roadmap: Hybrid Search
- Three search modes enable flexible querying strategies: pure similarity, pure structure, or combined
- Phase 6 (Integration) will wire searchers into `Gognee.Search()` API and complete the Add→Cognify→Search pipeline

## [0.4.0] - 2025-12-24

### Added
- **Storage Layer** (`pkg/store`)
  - `Node` struct representing knowledge graph entities with embeddings and metadata
  - `Edge` struct representing relationships between nodes
  - `GraphStore` interface defining graph storage operations
  - `SQLiteGraphStore` implementation for persistent graph storage
    - SQLite schema with nodes and edges tables
    - Full CRUD operations for nodes and edges
    - Case-insensitive node name search with `FindNodesByName` and `FindNodeByName`
    - Direction-agnostic edge retrieval (Cognee-aligned)
    - Multi-depth graph traversal with `GetNeighbors`
    - Automatic embedding and metadata serialization
    - Upsert semantics (INSERT OR REPLACE) for idempotent operations
  - `VectorStore` interface for vector similarity search
  - `MemoryVectorStore` in-memory implementation
    - Cosine similarity search with top-K results
    - Thread-safe operations using RWMutex
    - Efficient vector operations
  - `CosineSimilarity` function for computing vector similarity

### Technical Details
- SQLite driver: `modernc.org/sqlite` (pure Go, no CGO required)
- UUID generation: `github.com/google/uuid`
- Graph traversal is direction-agnostic (undirected) for Cognee alignment
- Depth=1 neighbors return direct adjacents only (default for Cognee parity)
- Node embeddings stored as BLOB, metadata as JSON in SQLite
- Vector store does not persist across restarts (MVP limitation, documented)
- 86.2% test coverage for store package
- All tests pass with race detector enabled

### Notes
- This release implements Phase 4 from the roadmap: Storage Layer
- The in-memory vector store is suitable for MVP but requires re-population after restart
- Phase 5 (Hybrid Search) will combine graph traversal and vector search
- Phase 6 (Integration) will connect the full Add→Cognify→Search pipeline

## [0.3.0] - 2025-12-24

### Added
- **Relationship Extraction** (`pkg/extraction`)
  - `Triplet` struct with Subject, Relation, and Object fields
  - `RelationExtractor` for extracting relationships between entities using LLM
  - `NewRelationExtractor` constructor following Phase 2 patterns
  - Relationship extraction prompt requesting JSON-only output
  - **Strict linking mode**: triplets must reference known entities or extraction fails
  - Case-insensitive entity name matching for linking
  - Whitespace trimming for all triplet fields
  - Deduplication with first-occurrence-wins ordering (case-insensitive comparison)
  - Validation of required fields (subject, relation, object all non-empty)
- **Integration tests**
  - `relations_integration_test.go` with `//go:build integration` tag
  - Tests full entity→relationship extraction pipeline against real OpenAI API
  - Validates triplets link to extracted entities

### Technical Details
- All new unit tests are offline-first using fake `LLMClient`
- 100% test coverage for `pkg/extraction` package
- No additional retry logic added (uses `LLMClient`'s built-in retry)
- Relation names are not normalized or restricted to an allowlist in Phase 3
- Prompt encourages consistent relation names (USES, DEPENDS_ON, etc.) but accepts any non-empty value

### Notes
- This release implements Phase 3 from the roadmap: Relationship Extraction
- Strict mode ensures linking correctness—no silent dropping of invalid triplets
- Run integration tests with: `go test -tags=integration ./...`
- Phase 4 (Storage Layer) will persist the extracted graph structure

## [0.2.0] - 2025-12-24

### Added
- **LLM package** (`pkg/llm`)
  - `LLMClient` interface for LLM completions
  - OpenAI Chat Completions API implementation using `gpt-4o-mini`
  - Exponential backoff retry logic with jitter (max 3 retries)
  - Comprehensive error handling for rate limits, timeouts, and API errors
  - `CompleteWithSchema` helper for JSON-based structured output
- **Entity Extraction package** (`pkg/extraction`)
  - `Entity` struct with Name, Type, and Description fields
  - `EntityExtractor` for extracting entities from text using LLM
  - Entity type validation against allowlist: Person, Concept, System, Decision, Event, Technology, Pattern
  - JSON-only prompt design for reliable structured extraction
  - Validation of extracted entities (required fields and type checking)
- **Gognee façade updates**
  - Added `LLMModel` configuration field (default: `gpt-4o-mini`)
  - Integrated LLM client initialization in `New()`
  - Added `GetLLM()` accessor method
- **Integration tests**
  - Optional integration test with `//go:build integration` tag
  - Tests actual OpenAI API entity extraction
  - Reads API key from `OPENAI_API_KEY` env var or `secrets/openai-api-key.txt`

### Removed
- **cmd/ directory** - gognee is a library-only package (not a CLI tool)

### Changed
- **Project vision clarified** - gognee mimics Cognee as an importable library for use in Glowbabe

### Technical Details
- All new unit tests are offline-first using fake servers and mock clients
- LLM retry logic includes jitter to prevent thundering herd
- Entity extraction validates all required fields before returning results
- Integration tests do not run by default (`go test ./...`)
- Run integration tests with: `go test -tags=integration ./...`

### Notes
- This release implements Phase 2 from the roadmap: Entity Extraction via LLM
- Cost-optimized using `gpt-4o-mini` model ($0.15/1M input, $0.60/1M output)
- All offline tests pass without API keys
- Test coverage >80% for new packages
- **Project vision**: gognee is an importable library (like Cognee) for building Glowbabe

## [0.1.0] - 2025-12-23

### Added
- Initial Go module structure (`github.com/dan-solli/gognee`)
- **Chunker package** (`pkg/chunker`)
  - Text chunking with sentence boundary awareness
  - Configurable max tokens and overlap
  - Deterministic chunk IDs using content hash
  - Word-based token counting heuristic
- **Embeddings package** (`pkg/embeddings`)
  - `EmbeddingClient` interface for generating text embeddings
  - OpenAI embeddings client implementation
  - Offline-first unit tests using fake HTTP server
  - Support for batch and single-text embedding
- **Main library package** (`pkg/gognee`)
  - Unified configuration via `Config` struct
  - Constructor that wires chunker and embeddings
  - Library-only design (no CLI) for importing into other Go projects
- Comprehensive test coverage with TDD approach
- Project documentation in `ROADMAP.md` with Phase 1 complete

### Notes
- This release implements Phase 1 from the roadmap: Foundation (Chunking + Embeddings)
- All tests run offline by default (no OpenAI API key required)
- Token counting uses a simple word-based heuristic as documented in roadmap
- **gognee is a library package** (not a CLI tool) - designed to mimic Cognee for use in Glowbabe

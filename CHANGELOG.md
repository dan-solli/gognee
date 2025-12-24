# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

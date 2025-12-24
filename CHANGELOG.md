# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

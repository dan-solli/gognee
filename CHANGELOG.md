# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
- Placeholder CLI entrypoint at `cmd/gognee/main.go`
- Comprehensive test coverage with TDD approach
- Project documentation in `ROADMAP.md` with Phase 1 complete

### Notes
- This release implements Phase 1 from the roadmap: Foundation (Chunking + Embeddings)
- All tests run offline by default (no OpenAI API key required)
- Token counting uses a simple word-based heuristic as documented in roadmap

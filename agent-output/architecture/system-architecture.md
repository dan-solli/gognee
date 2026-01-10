# gognee - System Architecture

**Last Updated**: 2026-01-10
**Architecture Owner**: Architect mode agent
**Status**: Current state documented; reconciled against implemented Memory CRUD

## Changelog
| Date | Change | Rationale |
|------|--------|-----------|
| 2026-01-02 | Created initial system architecture doc and diagram | Repo had plans/implementations but no architecture SSOT; required before Epic 8.1 planning |
| 2026-01-10 | Reconciled architecture vs. implemented Memory CRUD (v1.0.0+) and follow-up releases | Architecture docs must reflect what IS, not what was planned |

## Purpose
`gognee` is an importable Go library that provides persistent knowledge-graph-backed memory for AI assistants. It is designed to be embedded into downstream apps (notably Glowbabe).

## High-Level Architecture

### Public Facade
- **pkg/gognee**: `Gognee` is the primary API surface (`New`, `Add`, `Cognify`, `Search`, `Prune`, `Close`, `Stats`).

### Pipeline Components
- **Chunking**: `pkg/chunker` splits input text into token-ish chunks with overlap.
- **Embeddings**: `pkg/embeddings` provides `EmbeddingClient` (OpenAI implementation).
- **LLM**: `pkg/llm` provides `LLMClient` (OpenAI implementation).
- **Extraction**: `pkg/extraction` extracts entities and relations via LLM (JSON-only prompts).

### Storage
- **Graph**: `pkg/store` `SQLiteGraphStore` implements `GraphStore` over SQLite.
  - Tables: `nodes`, `edges`, `processed_documents`.
  - Nodes store: name/type/description/metadata + `embedding` BLOB.
- **Memories (first-class CRUD)**: `pkg/store` `SQLiteMemoryStore` implements `MemoryStore`.
  - Tables: `memories`, `memory_nodes`, `memory_edges`.
  - Purpose: persist structured memory payloads and provenance links to derived nodes/edges.
- **Vector**: `pkg/store` provides `VectorStore`.
  - In `:memory:` mode: `MemoryVectorStore`.
  - In file DB mode: `SQLiteVectorStore` (stores embeddings in `nodes.embedding`; linear-scan cosine similarity in Go).

### Search
- **Hybrid Search**: `pkg/search` `HybridSearcher` embeds the query then combines vector similarity + graph expansion.
- **Decay Decorator**: `pkg/search` `DecayingSearcher` optionally applies time-based decay scoring without changing underlying searchers.

## Runtime Flows

### Add(text)
- Buffers documents in-memory (`AddedDocument`) without calling LLMs.

### Cognify(opts)
- For each buffered document:
  - Optionally skips processing via incremental caching (document hash tracked in `processed_documents`).
  - Chunk → entity extraction → relation extraction.
  - Upserts nodes/edges in SQLite using deterministic IDs:
    - Node ID derived from (name, type)
    - Edge ID derived from (source_node_id, relation, target_node_id)
  - Persists embeddings on nodes (either in-memory vector store or SQLite `nodes.embedding`).

### Search(query, opts)
- Embeds query.
- Vector search returns top-K node IDs.
- Graph expansion retrieves neighbors.
- Optional decay scoring (based on access or creation time).
- Access reinforcement: node access timestamps updated on retrieval (currently implemented by casting to `*SQLiteGraphStore` for best-effort batch update).

### Memory CRUD (v1.0.0+)
- `AddMemory(input)`: persists a MemoryRecord, cognifies it, and links provenance (memory → nodes/edges).
- `ListMemories(opts)`: pagination-oriented listing over MemoryRecords.
- `GetMemory(id)`: returns a MemoryRecord.
- `UpdateMemory(id, updates)`: updates payload, re-cognifies, updates provenance, and triggers GC.
- `DeleteMemory(id)`: removes provenance links and deletes unreferenced artifacts via GC, then deletes the MemoryRecord.

### Prune(opts)
- Evaluates nodes against decay criteria and/or max age.
- Deletes nodes and cascades edge deletions (store-level delete helpers).

## Data Boundaries

### SQLite (Source of Truth)
- `nodes`: entity nodes + optional embedding BLOB; includes access tracking columns (`last_accessed_at`, `access_count`) via schema migration.
- `edges`: relations between nodes.
- `processed_documents`: document hash cache used for incremental processing.
- `memories`: first-class memory payloads (topic/context/decisions/rationale/metadata).
- `memory_nodes`, `memory_edges`: provenance tables linking MemoryRecords to derived artifacts.

### Non-Persisted
- Add buffer is memory-only until Cognify.

## Dependencies
- SQLite driver: `modernc.org/sqlite`.
- UUID generation: `github.com/google/uuid`.

## Quality Attributes
- **Determinism**: nodes/edges use stable IDs and upsert semantics to avoid duplication.
- **Offline-first testing**: unit tests do not require network; integration tests are build-tagged.
- **Minimal dependency surface**: Go stdlib + SQLite.

## Problem Areas / Design Debt
- **Partial API cohesion**: `SQLiteGraphStore` contains capabilities used via concrete casts that are not part of `GraphStore`.
- **Attribution vs. aggregation**: shared nodes/edges are upserted/aggregated across memories; delete/update correctness is at the artifact-existence level, but “subtracting” contributions (e.g., descriptions) is not modeled.
- **Transactional boundaries**: CRUD-like operations on derived graph data are not modeled as atomic units at the API/store boundary.
- **Performance ceiling (vector search)**: SQLiteVectorStore uses linear scan cosine similarity in Go; may require indexing/approximate search if corpus grows.

## Decisions (Architectural)

### D-001: SQLite is persistence substrate
- **Choice**: SQLite tables for nodes/edges and document tracking.
- **Consequence**: Simple deployment; requires careful schema migration and transaction design.

### D-002: Embeddings persisted on nodes
- **Choice**: store embedding BLOB in `nodes.embedding` (SQLiteVectorStore uses linear scan).
- **Consequence**: Fewer tables; acceptable performance for small graphs; easy cascade delete via node deletion.

### D-003: Search extensibility via decorator
- **Choice**: time-based decay implemented as `DecayingSearcher` wrapping existing searcher.
- **Consequence**: avoids interface churn; enables feature flags.

### D-004: First-class memories with provenance-first semantics
- **Choice**: implement MemoryRecord persistence plus provenance mapping tables (`memory_nodes`, `memory_edges`) to enable safe delete/update and GC.
- **Alternatives**: per-memory subgraph duplication; memory-centric search graph.
- **Consequence**: delete/update is correct for artifact existence; content-level “contribution subtraction” remains future work.

## Roadmap Readiness: First-Class Memory CRUD (Implemented)
First-class memory CRUD is implemented (v1.0.0+) and is now architectural fact. The original architectural constraints are captured in:
- `agent-output/architecture/011-memory-crud-architecture-findings.md`

Remaining readiness / hardening opportunities:
- Add integration tests for end-to-end CRUD with real LLMs under an `integration` gate.
- Consider an explicit GC API (manual GC trigger) if downstreams need it.
- Consider improvements to vector search performance as corpus scales.
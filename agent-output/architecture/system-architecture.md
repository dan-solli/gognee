# gognee - System Architecture

**Last Updated**: 2026-01-02
**Architecture Owner**: Architect mode agent
**Status**: Current state documented; Epic 8.1 assessed (planning gated)

## Changelog
| Date | Change | Rationale |
|------|--------|-----------|
| 2026-01-02 | Created initial system architecture doc and diagram | Repo had plans/implementations but no architecture SSOT; required before Epic 8.1 planning |

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

### Prune(opts)
- Evaluates nodes against decay criteria and/or max age.
- Deletes nodes and cascades edge deletions (store-level delete helpers).

## Data Boundaries

### SQLite (Source of Truth)
- `nodes`: entity nodes + optional embedding BLOB; includes access tracking columns (`last_accessed_at`, `access_count`) via schema migration.
- `edges`: relations between nodes.
- `processed_documents`: document hash cache used for incremental processing.

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
- **No first-class memory records**: the system persists derived graph artifacts but not the original structured memory payload.
- **Provenance gap**: derived nodes/edges are not attributable to a specific input document/memory.
- **Transactional boundaries**: CRUD-like operations on derived graph data are not modeled as atomic units at the API/store boundary.
- **Interface drift**: `SQLiteGraphStore` contains methods used via concrete casts (e.g., `DB()`, `UpdateAccessTime`, delete helpers) that are not part of `GraphStore`.
- **Shared artifacts**: deterministic node/edge IDs imply nodes/edges are shared across inputs; delete/update semantics require reference tracking to avoid data loss.

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

## Roadmap Readiness: Epic 8.1 (First-Class Memory CRUD)
Epic 8.1 introduces a new *entity type* in the domain model: a stable, user-facing **MemoryRecord** (topic/context/decisions/rationale/metadata) with browse/edit/delete.

This epic is **not implemented yet**. Architectural requirements and constraints are captured in:
- `agent-output/architecture/011-memory-crud-architecture-findings.md`

Key readiness notes:
- Must add persistence for original memory payloads (new table).
- Must add provenance mapping from memory → derived nodes/edges.
- Must provide transactional update/delete semantics.
- Must define shared-node deletion behavior (reference counting vs per-memory subgraph).
- Decision (2026-01-03): pursue provenance-first design (provenance tables + refcounts) as the primary path for Epic 8.1.
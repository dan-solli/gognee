# 011 - Epic 8.1 Memory CRUD - Architecture Findings

**Date**: 2026-01-02
**Handoff Context**: Glowbabe requests a Memory Browser requiring browse/edit/delete of stored memories. Current gognee persists only derived nodes/edges/embeddings and does not preserve structured memory payloads.
**Outcome Summary**: Epic 8.1 is feasible, but planning MUST incorporate provenance + transactional semantics. The largest open risk is deletion/update correctness in the presence of shared entities/relations.

## Verdict
**APPROVED_WITH_CHANGES**

Decision (2026-01-03): Proceed with **provenance-first** approach for Epic 8.1. Plan 011 must include provenance mapping and reference-count semantics as the primary path (no “rebuild-only” shortcut).

Planning can proceed only if the plan explicitly covers the required architectural constraints below.

## What Exists Today (Current Constraints)
- gognee persists **entities** as `nodes` and **relations** as `edges` in SQLite.
- Incremental Cognify tracks document hashes in `processed_documents` (not user-facing).
- Vector search is node-based; embeddings are stored in `nodes.embedding` (SQLiteVectorStore).
- There is **no persisted record** of the original user memory payload (topic/context/decisions/rationale/metadata) and no stable “memory id”.
- Node IDs are deterministic by (name,type) and edge IDs are deterministic by (source_id,relation,target_id), so derived artifacts are inherently shared across inputs.

## Architectural Requirements for Epic 8.1

### R-1: First-class memory persistence
Introduce a persisted **MemoryRecord** separate from derived graph artifacts.

Minimum schema (conceptual):
- `memories(id, topic, context, decisions_json, rationale_json, metadata_json, created_at, updated_at, version, doc_hash, source)`

Notes:
- `doc_hash` should be content-hash of the canonicalized memory payload (topic+context+decisions+rationale) for dedup and update detection.
- `version` enables forward schema evolution.

### R-2: Provenance mapping (memory → derived artifacts)
To support safe delete/update, gognee must track which derived nodes/edges were produced from which memory.

Recommended normalized mapping:
- `memory_nodes(memory_id, node_id, created_at)` (PK: memory_id,node_id)
- `memory_edges(memory_id, edge_id, created_at)` (PK: memory_id,edge_id)

This enables:
- delete/update of a memory without scanning the full graph for inferred matches
- reference counting semantics to decide when nodes/edges can be deleted

### R-3: Transactional semantics
Memory CRUD operations must be atomic:
- `AddMemory`: insert memory → cognify → link provenance (commit) OR rollback
- `UpdateMemory`: replace memory payload → recompute derived links → update provenance atomically
- `DeleteMemory`: remove provenance links → delete derived rows that become unreferenced → delete memory row (commit) OR rollback

This requires an explicit transaction boundary in the store layer.

### R-4: Shared-node/shared-edge deletion behavior (the big risk)
**Problem**: current entity nodes are deduplicated across all inputs. Deleting a memory must not delete nodes/edges still referenced by other memories.

Required behavior:
- A node/edge can be physically deleted only when it has **zero remaining provenance references**.

This implies delete needs a “garbage collection” phase:
- delete rows from `memory_nodes/memory_edges`
- delete `nodes/edges` where NOT EXISTS any mapping rows

### R-5: Search must surface memory provenance
Glowbabe needs to open a memory record from search results.

Minimum acceptable:
- Search results include `memory_ids` (or a “primary” memory_id) for each returned node.

Note: current `SearchResult` contains only node information (no provenance), so this requires either (a) enriching search results post-query using provenance tables, or (b) adding a new search result type/API dedicated to memory retrieval.

Recommended:
- Return `[]memory_id` per node, sorted deterministically (e.g., most recently updated memory first).

## Design Options Considered (Orphan/Shared Semantics)

### Option A (Recommended): Shared entity graph + provenance tables
- Keep deduplicated entity nodes/edges.
- Add provenance mapping and reference counting.
- Delete/update is correct with respect to *existence* of derived artifacts.

Tradeoffs:
- Node/edge attributes (e.g., `description`) are still aggregated/upserted; delete may not perfectly “subtract” contributions from a specific memory.

### Option B: Per-memory subgraph (IDs include memory_id)
- Create distinct nodes/edges per memory.
- Delete is trivial (delete by memory_id).

Tradeoffs:
- Graph grows rapidly; loses dedup; search quality likely worse due to duplicates; higher storage cost.

### Option C: Memory-centric search graph (memory nodes as primary retrieval unit)
- Represent each MemoryRecord as a node; link entities as neighbors.
- Search returns memory nodes; entity graph becomes supporting structure.

Tradeoffs:
- Larger change to search semantics and public API; likely best long-term for “deleting a memory removes it from recall”.

## Required Integration Points

### Store layer
Introduce a new interface boundary:
- `MemoryStore` for CRUD (separate from `GraphStore` to maintain cohesion).
- Store layer MUST support transactions for CRUD operations.

Do NOT require downstream callers to manage `*sql.DB` directly.

### Public API surface
- Add memory CRUD methods on `Gognee` (preferred for downstreams).
- Keep existing `Add/Cognify/Search` API intact.

## Non-Goals for v1.0.0
- Full-text search in `ListMemories` (pagination-only confirmed).
- Perfect “contribution subtraction” of entity descriptions during delete (may require node contribution modeling).

## Decisions Needed (Architect + UAT)
1. **Delete semantics expectation**: Is it sufficient that the *memory record* is deleted and unreferenced nodes/edges are removed, even if shared entity nodes retain aggregated description text?
   - If UAT requires strict “delete removes from recall”, Option C becomes preferred.

## Planning Gate (must be in Plan 011)
Plan 011 MUST include:
- schema changes + migrations
- provenance mapping tables
- transaction design
- explicit delete/update semantics with reference counting
- search provenance surfacing strategy

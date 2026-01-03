# Value Statement and Business Objective
Glowbabe needs a Memory Browser with reliable browse/edit/delete of stored memories. Gognee currently indexes text via Add→Cognify, producing derived graph nodes and embeddings but not a first-class memory record. To support user-facing memory CRUD, gognee must expose stable memory entities (with original payload) and synchronized graph/vector updates.

## Status
Active

## Changelog
| Date | Change | Author |
|------|--------|--------|
| 2026-01-02 | Initial analysis drafted | Analyst |

## Objective
Identify gaps in gognee needed for Glowbabe's Memory Browser (Epic 6.1) and outline a PR-ready change set for gognee to add first-class memory CRUD with consistent graph/vector synchronization.

## Context (What I looked at)
- Glowbabe backend adapter ([backend/internal/gognee/adapter.go](../../backend/internal/gognee/adapter.go#L1-L200), [backend/cmd/glowbabe/main.go](../../backend/cmd/glowbabe/main.go#L1-L240))
- Glowbabe types ([backend/internal/types/types.go](../../backend/internal/types/types.go#L1-L140))
- Current gognee integration: `storeMemory` formats `Topic: <topic>\n\n<context>` → `gognee.Add` → `gognee.Cognify` → `gognee.Search` for retrieval.
- Existing gognee APIs available in module version `v0.0.0-20251225074511-c2f91941581d` (Add, Cognify, Search, Stats, Prune, graph store CRUD, but no first-class memory records).

## Root Cause (why current plan is blocked)
- **No first-class memory entity**: Gognee does not persist the raw memory payload (topic/context/decisions/rationale/metadata) or a stable memory ID. Only derived graph nodes/edges and embeddings are stored.
- **Search-only surface**: Retrieval uses `Search` results (graph nodes) mapped back to `Source` and `Description`, losing any structured fields beyond topic/context formatting.
- **No CRUD hooks**: There is no way to list, get, update, or delete a specific stored memory; only graph node deletion (Prune/DeleteNode) exists, which is not equivalent to deleting a user memory.
- **Re-indexing not defined**: Updating context/topic would require re-chunking, re-embedding, and rewriting graph edges; gognee lacks an API for this.

## Methodology
- Code inspection of Glowbabe backend adapter and RPC surface.
- Review of gognee usage in Glowbabe and known gognee capabilities (Add/Cognify/Search/Stats/Prune, graph store CRUD).
- Mapping required Memory Browser flows (list/get/edit/delete) to current gognee surface to identify gaps.

## Findings (Fact vs. Hypothesis)
- **Fact**: Glowbabe stores memories by sending formatted text to `gognee.Add` then `Cognify`; only topic/context survive as free text in graph nodes. Decisions/rationale/metadata are dropped. (adapter.go StoreMemory)
- **Fact**: Retrieval returns `Search` results as `MemoryItem` with topic from `Source`, context from `Node.Description`, and node metadata. No memory ID is exposed. (adapter.go RetrieveMemory)
- **Fact**: Gognee provides graph-level CRUD (GetNode, DeleteNode, GetAllNodes, Prune) but no API for list/get/update/delete of the original document/memory payload.
- **Hypothesis**: Editing or deleting a memory via graph node operations would leave embeddings and related nodes inconsistent because the original document-to-node mapping is not tracked.
- **Hypothesis**: Introducing a document/memory table with stable IDs and mapping to generated node/vector IDs would allow safe CRUD and reindexing.

## Recommendations (for gognee PR)
1) **Add first-class Memory/Document model**
   - Introduce a `MemoryRecord` (or `Document`) persisted table with fields: `id (uuid)`, `topic`, `context`, `decisions[]`, `rationale[]`, `metadata (json)`, `created_at`, `updated_at`, `version`, `doc_hash`, `chunk_count`, `node_ids[]`, `vector_ids[]`.
   - Store raw payload before chunking; keep `doc_hash` for dedup and update detection.

2) **CRUD API surface (library)**
   - `AddMemory(ctx, MemoryInput) (*MemoryResult, error)` — store record, run chunking/embedding/cognify, return memoryID and stats.
   - `ListMemories(ctx, ListOptions{offset, limit, query?}) ([]MemorySummary, error)` — paginated summaries (topic, preview, created/updated, decision count).
   - `GetMemory(ctx, id string) (*MemoryRecord, error)` — full payload + linkage (node/vector IDs).
   - `UpdateMemory(ctx, id string, updates MemoryUpdate) (*MemoryResult, error)` — re-chunk/re-embed/re-cognify; update record and replace affected nodes/vectors atomically.
   - `DeleteMemory(ctx, id string) error` — delete record, remove associated nodes/edges/vectors; cascade safely.

3) **Reindexing semantics**
   - On update: recompute hash; if content changes, re-run chunking/LLM extraction; delete/rewrite old nodes/edges/vectors tied to this memory; update access timestamps appropriately.
   - On delete: remove nodes/edges/vectors linked to memory; ensure vector store is pruned; consider orphan edge handling.

4) **Data linkage**
   - Track which graph nodes and vector entries originate from a memory (e.g., store `memory_id` on nodes/edges or maintain a join table).
   - Ensure `Search` results can surface `memory_id` to the caller so Glowbabe can open the correct memory in the browser.

5) **Backward compatibility & migration**
   - Migration path: hydrate legacy graph data by parsing `Topic: X\n\nContext` nodes into v1 `MemoryRecord` with null/empty decisions/rationale; mark as legacy if mapping incomplete.
   - Version field on stored records for future schema evolution.

6) **Performance & limits**
   - Provide pagination in `ListMemories`; default limit 50.
   - Consider optional search filter (topic/substring) at list level to reduce client-side filtering overhead.

7) **API ergonomics**
   - Expose typed results for `MemorySummary` and `MemoryRecord` with stable IDs.
   - Keep existing `Add`/`Cognify` API for backward compatibility; mark memory APIs as the preferred path for applications needing CRUD.

## Proposed next steps
- Draft gognee PR implementing the Memory CRUD surface (items 1–4) plus minimal migration support (5). Keep scope to library changes; Glowbabe can then consume the new APIs for Epic 6.1.
- Define test matrix in gognee: unit tests for CRUD, reindexing after update, cascade delete, legacy hydration; integration test that round-trips add→list→get→update→search→delete.

## Open Questions
- How should decisions/rationale be serialized—simple JSON arrays or a richer typed structure? (Recommend JSON arrays, versioned record.)
- Should `ListMemories` support full-text search server-side, or is pagination + client filter sufficient for v1?
- Do we need partial update semantics (patch) or only full replace? (Prefer partial but re-embed on any context/topic change.)

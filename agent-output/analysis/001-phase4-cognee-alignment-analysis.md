# 001-phase4-cognee-alignment-analysis

## Value Statement and Business Objective
Ensure Phase 4 storage behaviors stay consistent with Cognee’s semantics so downstream Glowbabe users get predictable graph traversal and lookup results (undirected neighbor discovery, single-hop defaults, and clear name matching), reducing integration friction and surprises.

## Status
Planned

## Changelog
- 2025-12-24: Initial Cognee-alignment analysis for Phase 4 storage plan.
- 2025-12-24: Incorporated into Plan 004; status set to Planned.

## Objective
Compare the drafted Phase 4 storage plan with Cognee’s actual graph behaviors (edges/neighbors/name matching) and flag alignment gaps to adjust the plan before implementation.

## Context
The current plan in agent-output/planning/004-phase4-storage-layer-plan.md defines `GetEdges` as outgoing-only and `GetNeighbors` as recursive traversal with depth. Cognee’s adapters (Neo4j/Neptune/Kuzu) use undirected, single-hop queries for edges and neighbors and exact name matching. We need to reconcile these differences before coding.

## Methodology
- Reviewed Cognee graph adapters and interface: `cognee/infrastructure/databases/graph/*/adapter.py` and `graph_db_interface.py` (Neo4j/Neptune/Kuzu).
- Focused on `get_edges`, `get_neighbors`, and name matching queries; inspected `get_nodeset_subgraph` for traversal defaults.
- Compared findings against the Phase 4 plan tasks and decisions.

## Findings (facts vs. hypotheses)
- Fact: Cognee’s `get_edges(node_id)` returns all incident edges (incoming and outgoing) using `MATCH (n)-[r]-(m)`, i.e., undirected, single-hop.
- Fact: Cognee’s `get_neighbors(node_id)` returns all adjacent nodes, undirected, single-hop; no multi-depth traversal in this API.
- Fact: Cognee subgraph helpers (`get_nodeset_subgraph`) start from named nodes, include their direct neighbors (undirected), and only edges whose endpoints are inside the primary+neighbor set; effectively depth=1 expansion.
- Fact: Name matching is exact (case-sensitive) equality on `name`; duplicate names are not disambiguated in queries.
- Hypothesis: If we keep recursive `GetNeighbors` with depth>1, we diverge from Cognee; we should either constrain to depth=1 by default or document the divergence.

## Root Cause
The Phase 4 draft mirrored the ROADMAP interface but assumed directed edges for retrieval and added recursive traversal, while Cognee’s actual implementation treats edge discovery as undirected and shallow. This creates potential behavioral mismatches for downstream Glowbabe integration.

## Recommendations
1) Align `GetEdges` with Cognee: return all incident edges (both directions) for the node; document that it is direction-agnostic.
2) Align `GetNeighbors`: default to depth=1, undirected adjacency; if supporting deeper traversal, treat it as an explicit extension (documented) and keep depth=1 as default for parity.
3) Clarify `FindNodeByName`: Decision: keep case-insensitive matching (UX over strict Cognee parity); document the divergence and define deterministic tie-breaking if multiple matches exist.
4) Note subgraph parity: if we later add nodeset/subgraph helpers, mirror Cognee’s pattern (primary set + direct neighbors, undirected edges only inside that set).

## Open Questions — Resolved
- For case-insensitive `FindNodeByName`, ambiguity is now handled by returning all matches via `FindNodesByName`; the single-return helper errors on ambiguity.
- Depth>1 traversal is supported as an explicit extension; default is depth=1 for Cognee parity.

# Plan 003 — Phase 3 Relationship Extraction

**Plan ID:** 003

**Target Release:** v0.3.0

**Epic Alignment:** ROADMAP Phase 3 — Relationship Extraction

**Status:** UAT Approved

**Changelog**
- 2025-12-24: Created plan for Phase 3 implementation.
- 2025-12-24: Revised per critique: strict mode, constructor, clarified validation/ordering.
- 2025-12-24: Implementation started.
- 2025-12-24: Implementation complete. All milestones delivered.
- 2025-12-24: UAT approved. Release v0.3.0 ready.

---

## Value Statement and Business Objective
As a developer embedding gognee into Glowbabe (Flowbaby-like assistant), I want gognee to extract relationships between previously extracted entities, so that the knowledge graph can represent meaningful edges (triplets) and later support graph traversal + hybrid search.

---

## Objective
Deliver Phase 3 from ROADMAP:
- Relationship extraction prompt
- Triplet extraction (subject, relation, object)
- Link relationships to the extracted entity set
- Handle cases where entities aren’t found (robust behavior, no crashes)

This phase should remain library-only (no CLI) and stay aligned with the existing Phase 2 patterns: JSON-only output, offline-first unit tests, interface-driven design.

---

## Scope

**In scope**
1. Implement `pkg/extraction/relations.go`:
   - `Triplet` struct
   - `RelationExtractor` struct
   - `Extract(ctx, text, entities)` method using `LLMClient`
2. Add relationship extraction prompt returning JSON-only output
3. Parse/validate triplets and link them to the known entity set
4. Offline unit tests for relationship extraction (fake `LLMClient`)
5. Optional: gated integration test mirroring Phase 2 approach (build tag)

**Out of scope**
- Graph/SQLite storage (Phase 4)
- Vector store (Phase 4)
- Hybrid search (Phase 5)
- High-level `Gognee.Add/Cognify/Search` orchestration (Phase 6)
- Any CLI surface (explicitly out)

---

## Key Constraints
- Library-only: no `cmd/` directory, no executable concerns
- No Python
- Keep dependency surface minimal (stdlib + existing deps only)
- Unit tests must not require network access or an OpenAI API key
- LLM must return JSON-only responses that can be unmarshaled reliably
- Reuse existing retry/backoff in `pkg/llm` (no duplicate retry layers unless required)

---

## Plan-Level Decisions (to remove ambiguity)

1. **Linking behavior when entities don’t match:**
   - Extracted triplets MUST have non-empty `Subject`, `Relation`, and `Object`.
   - **Strict mode (chosen):** if a triplet’s subject or object does not match any provided entity name, extraction MUST fail with a clear error (no silent dropping).
   - Rationale: failing fast makes linking correctness explicit and prevents silently losing relationships.
   - Note: reevaluate strict mode if integration tests show this is too brittle.

2. **Entity-name matching rule (linking):**
   - Use case-insensitive comparison against entity names.
   - Treat exact-string match after trimming whitespace as the baseline rule.
   - Rationale: reduces brittle failures from capitalization and minor spacing.

3. **Relation name policy:**
   - Do not enforce a strict allowlist of relation names in Phase 3.
   - Encourage consistent relation names via prompt (“USES”, “DEPENDS_ON”, …) but accept any non-empty value.
   - Do not normalize relation names (e.g., uppercasing) in Phase 3.
   - Rationale: strict allowlists/normalization can be added later; Phase 3 focuses on extraction + linking.

4. **Deduplication:**
   - Deduplicate identical triplets `(Subject, Relation, Object)` returned by the LLM.
   - Preserve stable ordering by keeping the first occurrence and removing later duplicates.
   - Rationale: keeps downstream storage clean and deterministic.

5. **Retry encapsulation:**
   - Retry/backoff remains encapsulated in the `LLMClient` implementation.
   - `RelationExtractor` MUST NOT add an additional retry layer.

---

## Open Questions — Resolved

1. **Target release versioning:** v0.3.0 confirmed for Phase 3.
2. **Strictness vs permissiveness:** strict mode is preferred at this point. Reevaluation trigger: if integration tests show strict mode causes unacceptable brittleness.

---

## Plan (Milestones)

### Milestone 1 — Relation Structures + Extractor
**Objective:** Implement the Phase 3 API surface in `pkg/extraction`.

**Tasks**
1. Create `pkg/extraction/relations.go` with:
   - `Triplet` struct (`subject`, `relation`, `object` JSON tags)
   - `RelationExtractor` that depends on `LLMClient`
   - `Extract(ctx, text, entities)` method
   - `NewRelationExtractor(llmClient)` constructor (match `NewEntityExtractor` pattern)
2. Define and store the relationship extraction prompt template as a constant in the same file.

**Acceptance criteria**
- New extraction API compiles and is usable without additional packages.
- Prompt explicitly requests JSON-only output.

---

### Milestone 2 — Triplet Parsing + Validation + Linking
**Objective:** Make relationship extraction robust and predictable.

**Tasks**
1. Parse LLM response as `[]Triplet` using `CompleteWithSchema`.
2. Validate each triplet:
   - non-empty subject/relation/object
   - trim whitespace
3. Link triplets to the provided entity set:
   - case-insensitive match to `Entity.Name`
   - in strict mode, return an error if any subject or object is not found
4. Deduplicate triplets.
5. Preserve ordering after deduplication (first occurrence wins).

**Acceptance criteria**
- Unknown subject/object does not crash extraction and returns a clear error.
- Returned triplets reference only known entities (enforced by strict linking).
- Output is deterministic (trimmed, deduped, stable ordering with first-occurrence-wins).

---

### Milestone 3 — Offline Unit Tests
**Objective:** Lock in behavior with offline-first tests.

**Tasks**
1. Create `pkg/extraction/relations_test.go` using a fake `LLMClient`.
2. Cover:
   - happy path (multiple triplets)
   - empty list response
   - malformed JSON response
   - missing subject/object/relation (error)
   - unknown subject/object (error; strict linking)
   - duplicate triplets (deduped)
   - whitespace/case variations

**Acceptance criteria**
- `go test ./...` passes offline.
- New code has strong coverage (target: >80% for new files/packages).

---

### Milestone 4 — Optional Gated Integration Test
**Objective:** Provide a manual validation path against the real OpenAI API.

**Tasks**
1. Add `pkg/extraction/relations_integration_test.go` under `//go:build integration`.
2. Read API key from `OPENAI_API_KEY` or `secrets/openai-api-key.txt`.

**Acceptance criteria**
- Integration tests do not run by default.
- Can be run explicitly: `go test -tags=integration ./...`.

---

### Milestone 5 — Version and Release Artifacts
**Objective:** Update release artifacts for v0.3.0.

**Tasks**
1. Add CHANGELOG.md entry for `v0.3.0` documenting Phase 3 deliverables.
2. Ensure ROADMAP references remain consistent with “library-only” positioning.

**Acceptance criteria**
- Changelog clearly documents relationship extraction capabilities.

---

## Validation
- `go test ./...`
- `go test ./... -cover` (expect strong coverage for new package code)
- `go vet ./...` (optional but recommended)

---

## Risks & Mitigations
- **LLM returns entities not in provided list:** mitigate by strict linking + clear error return; reevaluate if integration tests show unacceptable brittleness.
- **LLM returns malformed JSON:** mitigate by strict JSON-only prompt + clear error propagation.
- **Low-quality/overly generic relations:** mitigate by prompt tuning first; model upgrades later if needed.

---

## Handoff Notes
- Phase 3 builds directly on Phase 2 outputs (`[]Entity`) and existing `LLMClient`.
- Keep relationship extraction independent of storage/search so Phase 4 can choose the best persistence strategy.

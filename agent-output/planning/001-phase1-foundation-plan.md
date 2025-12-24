# Plan 001 — Phase 1 Foundation (Revised)

**Plan ID:** 001

**Target Release:** v0.1.0 (proposed)

**Epic Alignment:** ROADMAP Phase 1 — Foundation (Chunking + Embeddings)

**Status:** QA Complete

**Changelog**
- 2025-12-23: Created plan and incorporated roadmap critique items (cgo policy, testability of OpenAI calls, token counting + chunk IDs, persistence caveats).
- 2025-12-23: Implementation completed - all milestones delivered, tests passing.
- 2025-12-24: QA complete - all tests passing; overall coverage at 89.8% (CLI placeholder excluded).

---

## Value Statement and Business Objective
As a developer embedding `gognee` into an AI assistant, I want a reliable Phase 1 foundation (chunking + embeddings with stable interfaces), so that later phases (extraction, storage, search) can be implemented without reworking core boundaries.

---

## Objective
Deliver Phase 1 from [ROADMAP.md](ROADMAP.md):
- Go module/project skeleton matching intended layout
- Deterministic chunking with overlap and basic “token-ish” counting
- Embedding client interface + OpenAI implementation
- Tests that run offline by default (no required network/API key)

This plan also resolves contradictions identified in the roadmap critique that would otherwise block stable implementation.

---

## Scope
**In scope**
1. Create initial Go project structure (`cmd/`, `pkg/`), with Phase 1 packages.
2. Implement `pkg/chunker` and tests.
3. Implement `pkg/embeddings` (interface + OpenAI implementation) and tests.
4. Add minimal `pkg/gognee` façade if needed to tie Phase 1 together.
5. Documentation updates necessary to remove critical contradictions.

**Out of scope**
- Entity/relationship extraction (Phase 2–3)
- Storage/search (Phase 4–6)
- CLI implementation beyond a placeholder entrypoint

---

## Key Constraints (from repo + roadmap)
- Single Go binary/library approach; avoid unnecessary services.
- No Python.
- External dependency policy is ambiguous: roadmap says “no external deps beyond SQLite” but later lists `cobra` and `google/uuid`.
  - Treat this as an **OPEN QUESTION** requiring explicit decision (see below).
- cgo is **allowed** (confirmed by user), even if pure-Go alternatives remain preferred for portability.

---

## Decisions Incorporated from Critique
1. **cgo policy:** Allowed. If a pure-Go SQLite driver is preferred later, it’s an optimization choice, not a rule.
2. **External API tests:** Unit tests must not require network or `OPENAI_API_KEY`.
3. **Chunk IDs + token estimation:** Must be deterministic and documented; avoid hand-wavy “~500 tokens” without defining the approximation.
4. **Persistence caveat:** If Phase 1/4 uses in-memory vector store initially, document that full persistence is not complete until vectors are stored in SQLite.

---

## OPEN QUESTION (Blocking)
1. **Dependency policy clarification:**
   - Does “no external deps beyond SQLite” mean *no third-party Go modules* (except SQLite driver), or “no external services”?
   - This affects whether we can use `cobra`/`google/uuid`.

**Default assumption if not clarified:**
- For Phase 1, use only the Go standard library plus (optionally) a SQLite driver when Phase 4 begins. Avoid `cobra` and third-party UUID libs until this is answered.

---

## Plan (Milestones)

### Milestone 0 — Resolve roadmap contradictions (doc-level)
**Objective:** Ensure roadmap guidance does not conflict with agreed constraints.

**Tasks**
1. Update the “Technical Decisions” section in [ROADMAP.md](ROADMAP.md) to reflect:
   - cgo is allowed
   - `modernc.org/sqlite` may be recommended but not mandated
2. Add a short note clarifying offline-first tests for external APIs (OpenAI).
3. Add a short note clarifying token counting approximation for chunking.

**Acceptance criteria**
- ROADMAP no longer discourages cgo as a rule.
- Roadmap explicitly states unit tests must not depend on live OpenAI calls.

---

### Milestone 1 — Go module + skeleton
**Objective:** Establish the initial repository layout that future phases will build on.

**Tasks**
1. Add `go.mod` and baseline module structure.
2. Create directories and placeholder files consistent with Phase 1 deliverables:
   - `pkg/chunker/`
   - `pkg/embeddings/`
   - `pkg/gognee/`
   - (optional placeholder) `cmd/gognee/`

**Acceptance criteria**
- `go test ./...` runs and discovers packages (even if minimal initially).
- Layout matches the roadmap’s intended structure.

---

### Milestone 2 — Chunker implementation (offline, deterministic)
**Objective:** Produce stable chunk boundaries with overlap and deterministic IDs.

**Tasks**
1. Implement `Chunk` + `Chunker` API as described in Phase 1.
2. Define a deterministic `Chunk.ID` strategy (e.g., content hash + index) and document it.
3. Define “token-ish” counting strategy used for chunk sizing (e.g., word/rune heuristic) and document it.

**Acceptance criteria**
- Given the same input, chunking output is deterministic (same boundaries + IDs).
- Overlap behavior is validated.
- Chunk sizing respects configured limits under the chosen token approximation.

---

### Milestone 3 — Chunker tests (TDD)
**Objective:** Lock in behavior and prevent regressions.

**Tasks**
1. Add unit tests for:
   - sentence boundary behavior
   - overlap behavior
   - edge cases (empty input, very short input, very long input)

**Acceptance criteria**
- Tests pass locally with `go test ./...`.
- Tests do not require any network access.

---

### Milestone 4 — Embeddings interface + OpenAI implementation (offline tests by default)
**Objective:** Provide an `EmbeddingClient` boundary with a real OpenAI implementation.

**Tasks**
1. Implement `EmbeddingClient` interface.
2. Implement OpenAI client with configurable model and API key.
3. Implement request/response parsing with clear error surfaces.
4. Provide unit tests using an in-process fake server (no live OpenAI calls).
5. (Optional) Provide a separately gated integration test that runs only when explicitly enabled (e.g., via build tag or env flag).

**Acceptance criteria**
- Unit tests do not require `OPENAI_API_KEY`.
- OpenAI implementation correctly handles:
  - non-200 responses
  - invalid JSON
  - empty input
  - partial failures (if applicable)

---

### Milestone 5 — Minimal façade (optional)
**Objective:** Provide a small entrypoint for downstream callers without committing to Phase 6 API.

**Tasks**
1. Add `pkg/gognee` minimal constructor wiring for chunker + embeddings if it helps usage.

**Acceptance criteria**
- Does not pre-empt Phase 6 API design.
- Keeps boundaries interface-driven.

---

### Milestone 6 — Version and release artifacts
**Objective:** Ensure planned release artifacts exist and match the target release.

**Tasks**
1. Add/update a `CHANGELOG.md` entry for v0.1.0.
2. Ensure any version references (if introduced) align to `v0.1.0`.

**Acceptance criteria**
- Repo contains a clear changelog entry describing Phase 1 deliverables.

---

## Validation (high level)
- Local verification commands:
  - `gofmt ./...`
  - `go test ./...`

---

## Risks & Mitigations
- **Token counting ambiguity** → mitigate by explicitly documenting the approximation and testing behavior, not “true tokens”.
- **External dependency policy ambiguity** → mitigate by keeping Phase 1 stdlib-only unless clarified.
- **OpenAI API drift** → mitigate with offline unit tests + clearly gated integration test.

---

## Handoff Notes
- This plan intentionally avoids Phase 2+ design decisions.
- If dependency policy is clarified to allow third-party modules, revisit whether to adopt `cobra` and a UUID lib later.

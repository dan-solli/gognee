# Plan 002 — Phase 2 Entity Extraction

**Plan ID:** 002

**Target Release:** v0.2.0

**Epic Alignment:** ROADMAP Phase 2 — Entity Extraction via LLM

**Status:** Complete

**Changelog**
- 2025-12-24: Created plan for Phase 2 implementation.
- 2025-12-24: Implementation started.
- 2025-12-24: Implementation completed; tests passing.

---

## Value Statement and Business Objective

As a developer building an AI assistant with persistent memory, I want to extract named entities from text chunks using an LLM, so that I can later construct a knowledge graph with meaningful nodes representing concepts, people, decisions, and systems.

---

## Objective

Deliver Phase 2 from [ROADMAP.md](../../ROADMAP.md):
- LLM client interface + OpenAI implementation (cheapest viable: `gpt-4o-mini`)
- Entity extraction with structured JSON output
- Exponential backoff retry logic with jitter (max 3 retries)
- Offline-first unit tests (fake HTTP server); optional gated integration test

---

## Scope

**In scope**
1. Implement `pkg/llm` package with `LLMClient` interface and OpenAI implementation
2. Implement `pkg/extraction/entities.go` with `EntityExtractor`
3. Entity extraction prompt returning JSON-only output
4. Robust error handling (retry with backoff, clear errors on failure)
5. Offline unit tests for both LLM client and entity extractor
6. Wire LLM client into the `pkg/gognee` façade

**Out of scope**
- Relationship/triplet extraction (Phase 3)
- Storage layer (Phase 4)
- CLI enhancements

---

## Model Selection (Cost Optimization)

| Component | Model | Rationale |
|-----------|-------|-----------|
| Embeddings | `text-embedding-3-small` | Already configured in Phase 1; cheapest embedding model ($0.02/1M tokens) |
| LLM (entity extraction) | `gpt-4o-mini` | Cheapest viable completion model with strong JSON output ($0.15/1M input, $0.60/1M output); sufficient for structured extraction |

If `gpt-4o-mini` produces subpar entity extraction results, escalate to `gpt-4o` or consider structured outputs (JSON mode).

---

## Key Constraints

- Single Go binary; no Python
- Unit tests must not require network or API key
- Retry logic: exponential backoff, max 3 retries
- JSON-only output from LLM (prompt engineering)
- Interface-driven design for testability

---

## Plan-Level Decisions (to remove ambiguity)

1. **LLM schema helper method**: Implement the roadmap interface method `CompleteWithSchema(ctx context.Context, prompt string, schema any) error` and use it for entity extraction.
   - Rationale: keeps Phase 2 aligned with the roadmap while remaining stdlib-only (marshal/unmarshal based).
2. **Entity type validation**: Validate extracted `Entity.Type` against the roadmap allowlist: `[Person, Concept, System, Decision, Event, Technology, Pattern]`.
   - Behavior: If any entity has an unknown type, return an error (fail fast). This keeps downstream graph schemas consistent.
3. **Prompt storage**: Store the entity extraction prompt as a package-level constant in `pkg/extraction/entities.go` (no external prompt files for Phase 2).
4. **Structured output mode**: Phase 2 relies on prompt discipline + JSON parsing/validation (no dependency on OpenAI-specific JSON modes). Optional OpenAI response-format usage can be revisited if JSON quality is subpar.

---

## Plan (Milestones)

### Milestone 1 — LLM Client Interface + OpenAI Implementation

**Objective:** Provide an `LLMClient` boundary with OpenAI Chat Completions implementation.

**Tasks**
1. Create `pkg/llm/client.go` with `LLMClient` interface:
   - `Complete(ctx, prompt) (string, error)`
   - `CompleteWithSchema(ctx, prompt, schema) error` (roadmap-aligned helper)
2. Create `pkg/llm/openai.go` with `OpenAILLM` struct:
   - Configurable model (default: `gpt-4o-mini`)
   - Configurable base URL for testing
   - Request/response parsing for Chat Completions API
3. Implement exponential backoff retry logic **with jitter** (initial delay 1s, max 3 retries, backoff factor 2)
4. Handle error surfaces:
   - Non-200 responses
   - Rate limiting (429)
   - Invalid JSON response
   - Context cancellation

**Acceptance criteria**
- `LLMClient` interface defined
- OpenAI implementation compiles and handles error cases
- Retry behavior includes jitter and caps at 3 retries
- No external dependencies beyond stdlib

---

### Milestone 2 — LLM Client Tests (Offline)

**Objective:** Lock in LLM client behavior with offline tests.

**Tasks**
1. Add unit tests using `httptest` fake server:
   - Successful completion
   - Empty response handling
   - Non-200 response handling
   - Invalid JSON response
   - Context cancellation
   - Retry behavior (verify retries on 500/429)

**Acceptance criteria**
- Tests pass with `go test ./...`
- No network access required

---

### Milestone 3 — Entity Extractor Implementation

**Objective:** Extract structured entities from text using LLM.

**Tasks**
1. Create `pkg/extraction/entities.go`:
   - `Entity` struct: `Name`, `Type`, `Description`
   - `EntityExtractor` struct with `LLM LLMClient` field
   - `Extract(ctx, text) ([]Entity, error)` method
2. Implement entity extraction prompt (JSON-only output) as a constant in `entities.go`
3. Parse JSON response into `[]Entity` and validate:
   - required fields present (non-empty Name/Type/Description)
   - `Type` is within the allowlist: `[Person, Concept, System, Decision, Event, Technology, Pattern]`
4. Return clear error if extraction fails (do not silently skip)

**Acceptance criteria**
- Entity struct matches roadmap spec
- JSON parsing handles malformed LLM output gracefully
- Unknown entity types fail fast with a clear error message
- Clear error messages on failure

---

### Milestone 4 — Entity Extractor Tests (Offline)

**Objective:** Validate entity extraction with controlled LLM responses.

**Tasks**
1. Create fake `LLMClient` implementation for testing
2. Add unit tests:
   - Successful extraction with multiple entities
   - Empty entity list (valid JSON, no entities)
   - Malformed JSON response
   - LLM error propagation
   - Unknown entity type returns error

**Acceptance criteria**
- Tests pass offline
- Edge cases covered

---

### Milestone 5 — Façade Integration

**Objective:** Wire LLM client and entity extractor into the gognee façade.

**Tasks**
1. Add `LLMModel` field to `Config` (default: `gpt-4o-mini`)
2. Initialize `OpenAILLM` in `New()`
3. Add `GetLLM()` accessor
4. Optionally add convenience method for entity extraction

**Acceptance criteria**
- Façade exposes LLM client
- Config propagates model selection

---

### Milestone 6 — Optional Integration Test (Gated)

**Objective:** Provide a real-API integration test for manual validation.

**Tasks**
1. Create integration test file with build tag `//go:build integration`
2. Read API key from environment or file
3. Test actual entity extraction on sample text

**Acceptance criteria**
- Integration test does NOT run by default
- Runs successfully when explicitly enabled: `go test -tags=integration ./...`

---

### Milestone 7 — Version and Release Artifacts

**Objective:** Update version references for v0.2.0.

**Tasks**
1. Add CHANGELOG.md entry for v0.2.0
2. Update any version references

**Acceptance criteria**
- Changelog documents Phase 2 deliverables

---

## Testing Strategy

**Unit tests (mandatory)**
- LLM client: fake HTTP server responses
- Entity extractor: fake LLM client returning controlled JSON

**Integration tests (optional, gated)**
- Real OpenAI API call with `//go:build integration` tag

**Coverage target**
- New packages should have >80% statement coverage

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| LLM returns malformed JSON | Validate JSON parsing; return clear error; consider retry with adjusted prompt |
| Rate limiting | Exponential backoff with jitter |
| Model produces low-quality entities | Monitor results; escalate to `gpt-4o` if needed |
| API key exposure | Keep in `secrets/` folder, add to `.gitignore` |

---

## API Key Handling

The API key is stored at `secrets/openai-api-key.txt`. For integration tests:
- Read from environment variable `OPENAI_API_KEY` first
- Fall back to reading from file if env var not set
- Never commit the key to version control

Confirm `secrets/` is in `.gitignore`.

---

## Success Metrics

- `go test ./...` passes offline
- Entity extraction produces valid `[]Entity` from sample text
- Retry logic verified via test
- Coverage ≥80% for new packages

---

## Handoff Notes

- Phase 2 builds on Phase 1 interfaces
- Entity extraction output will feed into Phase 3 (relationship extraction)
- If `gpt-4o-mini` output quality is inconsistent, revisit prompt wording first; OpenAI-specific structured output options can be considered later as an optimization.


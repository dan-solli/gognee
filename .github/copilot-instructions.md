# Copilot instructions for `gognee`

```markdown
# Copilot instructions for `gognee`

## Purpose & Big picture
- `gognee` is an importable Go library that provides persistent memory via a SQLite-backed knowledge graph plus vector (embedding) search. It is intentionally a library (no `cmd/`) meant to be embedded by callers such as `Glowbabe`.
- Core responsibilities live in `pkg/`: chunking (`pkg/chunker`), embeddings (`pkg/embeddings`), LLM integration (`pkg/llm`), extraction (`pkg/extraction`), storage (`pkg/store`), search (`pkg/search`), and the façade `pkg/gognee`.

## Key files & components (quick map)
- Main façade: [pkg/gognee/gognee.go](pkg/gognee/gognee.go)
- Storage / graph model: [pkg/store/graph.go](pkg/store/graph.go)
- Vector store implementations: [pkg/store/vector.go](pkg/store/vector.go)
- Chunking: [pkg/chunker/chunker.go](pkg/chunker/chunker.go)
- Embeddings interface + impls: [pkg/embeddings](pkg/embeddings)
- LLM client and prompts: [pkg/llm](pkg/llm)
- Extraction (entities/relations): [pkg/extraction](pkg/extraction)
- Search algorithms (vector/graph/hybrid): [pkg/search](pkg/search)

## Why the layout / design decisions
- Library-only: consumers import `pkg/gognee` — there is no CLI. Keep API backwards-compatible.
- SQLite is the single persistent backend (see `go.mod`: `modernc.org/sqlite`), so persistence and vector storage are co-located in the DB.
- Interfaces are used at boundaries (`EmbeddingClient`, `LLMClient`, `GraphStore`) so implementations (mocks, in-memory, SQLite-backed) are swappable.

## Developer workflows (commands you will use)
- Run unit tests: `go test ./...` (unit tests must not require network).
- Run a single package: `go test ./pkg/store -run TestName`
- Format code: `gofmt -w .` (or `gofmt` via your editor).
- Use build tags / env for integration tests that hit external services — these must be gated (see `.github/skills/testing-patterns/SKILL.md`).

## Tests & integration rules
- TDD is required: add a failing test placed next to the package (e.g., `pkg/chunker/chunker_test.go`).
- Networked/integration tests must be gated behind an `integration` build tag or env var. Unit tests should use fakes/mocks (see existing tests in `pkg/*_test.go`).

## Secrets and environment
- Real API keys should come from the environment: `OPENAI_API_KEY` is expected. A repository file `secrets/openai-api-key.txt` exists for local dev only — do NOT commit real keys.

## Common patterns & conventions (project-specific)
- Persist embeddings in SQLite alongside nodes (see `pkg/store/vector.go`). For in-memory mode use the `MemoryVectorStore`.
- Extraction prompts and returned shapes must be strictly JSON arrays of objects (`Entity`, `Triplet`) as defined in `ROADMAP.md` — callers expect machine-parseable JSON.
- Follow the `ROADMAP.md` phases: pick a phase and implement only that phase's deliverables for clarity and reviewability.

## Integration points to watch
- SQLite via `modernc.org/sqlite` (pure-Go driver). Expect DB schema in `pkg/store`.
- OpenAI (or other LLM/embedding providers) are abstracted via interfaces in `pkg/embeddings` and `pkg/llm` — inject mocks for tests.

## How an AI agent should operate here
- Start by reading `ROADMAP.md` and the phase planning files in `agent-output/planning/` to understand the intended API shape.
- Use the interfaces in `pkg/*` to discover extension points; prefer writing unit tests before implementation.
- When changing the storage schema, update SQL schema and add migration notes in `agent-output/implementation/` and tests that validate persistence.

## Where to find development guidance and process artifacts
- Roadmap & API shapes: [ROADMAP.md](ROADMAP.md)
- Testing & memory-contract skills: `.github/skills/memory-contract/SKILL.md` and `.github/skills/testing-patterns/SKILL.md`
- Implementation notes and design rationale: `agent-output/implementation/` and `agent-output/architecture/`.

## Quick checklist for PRs by an AI agent
- Add/modify tests first; run `go test ./...` and ensure no network calls in unit tests.
- Update `pkg/` code without adding new external services.
- If adding embedding/LLM behaviour, add a mock client under `pkg/*/test` and an `integration`-tagged test for real API calls.
- Update `agent-output/implementation/` with a short note summarizing the change and the roadmap phase.

``` 

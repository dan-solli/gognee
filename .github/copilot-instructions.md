# Copilot instructions for `gognee`

## Big picture
- `gognee` is an **importable Go library package** that mimics [Cognee](https://github.com/topoteretes/cognee) (Python) with **persistent memory**, **knowledge graph relations**, and **vector + hybrid search**.
- **Not a CLI tool** - it's a library designed to be imported and used by other Go projects (specifically **Glowbabe**, which will mimic Flowbaby using gognee instead of cognee).
- Primary constraints from [ROADMAP.md](../ROADMAP.md): **library-only**, **no CLI**, **single Go binary**, **no Python**, **no external deps beyond SQLite**.

## Source of truth
- Treat [ROADMAP.md](ROADMAP.md) as the authoritative spec for architecture, phases, and key APIs.
- Agent process conventions live in `.github/skills/` (notably `memory-contract` and `testing-patterns`).

## Intended project layout (follow the roadmap)
- **No CLI**: gognee is a library package only (no `cmd/` directory needed)
- Library packages under `pkg/` (examples in the roadmap):
  - `pkg/chunker` (token-ish chunking + overlap)
  - `pkg/embeddings` (interface + OpenAI implementation)
  - `pkg/llm` (interface + OpenAI implementation)
  - `pkg/extraction` (entities + relations)
  - `pkg/store` (SQLite graph + vector storage)
  - `pkg/gognee` (high-level library façade - main entry point for importers)

## Project-specific patterns to follow
- Prefer **interfaces** at boundaries (`EmbeddingClient`, `LLMClient`, `GraphStore`) as sketched in [ROADMAP.md](ROADMAP.md), so implementations can swap without rewriting callers.
- Keep the dependency surface “boring”: standard library first; SQLite is the only planned external dependency.
- Match the JSON shapes in the roadmap for extraction (`Entity`, `Triplet`) and keep prompts returning **ONLY valid JSON**.

## Dev workflow expectations (repo-specific)
- **TDD is mandatory for this repo** (humans + AI): write the failing test first, then implement (see `.github/skills/testing-patterns/SKILL.md`).
- AI agents should follow the Flowbaby memory contract when available (see `.github/skills/memory-contract/SKILL.md`).
- If/when a Go module is added, keep formatting/tests conventional (`gofmt`, `go test ./...`) and colocate tests next to packages as shown in the roadmap (e.g., `pkg/chunker/chunker_test.go`).

## SQLite + cgo
- `cgo` is allowed in this project (e.g., using a `database/sql` SQLite driver that requires it).

## Before implementing code
- Confirm which roadmap phase is being implemented and implement only the deliverables listed for that phase.
- Do not introduce additional services (no external DBs/queues) unless the roadmap is updated explicitly.

# QA Checklist (gognee)

This folder contains QA reports and supporting artifacts (coverage profiles, screenshots if ever needed).

## Required contents per QA report

Each QA report in this folder should include:

- **Plan Reference**: Link/path to the plan in `agent-output/planning/`
- **QA Status**: One of: Test Strategy Development / Awaiting Implementation / Testing In Progress / QA Complete / QA Failed
- **Changelog**: Date, handoff, request, summary
- **Timeline**: Start/end timestamps and final status

## Execution evidence (minimum)

- Unit test command and PASS/FAIL
- Coverage command and total coverage line
- Integration tests (if present) should be explicitly reported as:
  - PASS, or
  - SKIPPED (with gating reason), or
  - FAIL (with failure summary)

## Artifact conventions

- Coverage profiles: `agent-output/qa/<plan-id>-cover.out`
- Optional HTML coverage: `go tool cover -html=<cover.out> -o agent-output/qa/<plan-id>-coverage.html`

## Notes

- Unit tests must be offline-first.
- Integration tests should be gated and should not run by default.

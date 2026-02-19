# Residuals Ledger

Tracks residual risks, limitations, technical debt, and deferred work identified during implementation and validation phases. Each entry must have a unique ID, severity, owner assignment, and target for resolution.

---

## Active Residuals

### RES-2026-001: DecayingSearcher.Search() Logging Not Implemented

| Field | Value |
|-------|-------|
| **ID** | RES-2026-001 |
| **Severity** | Medium |
| **Owner** | TBD (Planner) |
| **Target** | Next Release / Backlog |
| **Identified** | 2026-02-19 |
| **Plan Reference** | Plan 023 - Memory Decay Structured Logging |
| **Status** | Open |

**Description:**
Plan 023 Milestone 9 specified "Implement DecayingSearcher Logging" with per-node evaluation at DEBUG level. Implementation delivered only infrastructure (SetLogger() method, logger field) but no actual logging in the Search() method. Placeholder tests exist at `pkg/search/decay_logging_test.go`.

**Impact:**
- Developers cannot observe real-time decay score calculations during search operations
- Prune logging IS fully functional; startup config logging IS functional
- Core user value (observing prune decisions) is delivered; search observability is missing

**Mitigation:**
- Prune operations (the more critical path for debugging data deletion) are fully logged
- Infrastructure is in place - future implementation requires only adding LogAttrs calls

**Resolution Path:**
Create follow-up plan to implement DecayingSearcher.Search() logging using existing placeholder tests and SetLogger() infrastructure.

---

### RES-2026-002: Test Coverage Below 80% Target

| Field | Value |
|-------|-------|
| **ID** | RES-2026-002 |
| **Severity** | Low |
| **Owner** | TBD (QA) |
| **Target** | Backlog |
| **Identified** | 2026-02-19 |
| **Plan Reference** | Plan 023 - Memory Decay Structured Logging |
| **Status** | Open |

**Description:**
Plan 023 success criteria specified ≥80% test coverage for logging code paths. Actual coverage:
- pkg/gognee: 68.4% (target: 80%)
- pkg/search: 78.0% (target: 80%)

**Impact:**
- Logging is additive code with explicit nil-checks - core logic already covered by existing tests
- Coverage shortfall is primarily due to DecayingSearcher.Search() logging not being implemented (see RES-2026-001)

**Mitigation:**
- All implemented logging paths are covered by tests
- Nil-safety tests verify no panics across all code paths

**Resolution Path:**
Will be addressed when RES-2026-001 is resolved (implementing Search() logging will add coverage).

---

## Closed Residuals

(None)

---

## Severity Definitions

| Severity | Criteria | SLA |
|----------|----------|-----|
| Critical | Security vulnerability, data loss risk, production outage | IMMEDIATE (1h) |
| High | Core value not delivered, major functionality broken | SAME-DAY (4h) |
| Medium | Partial value delivery, missing non-critical feature | Next Release |
| Low | Technical debt, test gaps, documentation | Backlog |

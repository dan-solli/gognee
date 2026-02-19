# 023 - Memory Decay Structured Logging Plan

**Plan ID**: 023
**Target Release**: v1.6.0
**Epic Alignment**: Epic 10.1: Memory Decay Observability
**Status**: UAT Approved

## Changelog
| Date | Change |
|------|--------|
| 2026-02-18 | Initial plan created from discovery documents |
| 2026-02-19 | Implementation complete - all 11 milestones delivered |
| 2026-02-19 | QA validated - all tests pass, security compliance confirmed |
| 2026-02-19 | UAT Approved - value delivery confirmed with 2 residuals (RES-2026-001, RES-2026-002) |

---

## Value Statement and Business Objective

> **As a** Glowbabe integrator debugging memory decay behavior,
> **I want to** observe decay configuration, condition evaluation, and prune operations via structured logs,
> **so that** I can correlate decay behavior with search/prune results and diagnose issues without exposing sensitive memory content.

---

## Objective

Add structured logging to the memory decay subsystem using Go's standard `log/slog` package, following the established `WithMetricsCollector()` / `WithTraceExporter()` injection pattern. Logging is purely opt-in: nil logger means zero logging overhead.

---

## Success Criteria (from Roadmap)

- [ ] Injectable `WithLogger(*slog.Logger)` method on Gognee; nil = no logging
- [ ] Startup: log decay config at INFO level
- [ ] DecayingSearcher: log per-node evaluation at DEBUG level
- [ ] Prune: log options at INFO, per-item evaluation at DEBUG, summary at INFO
- [ ] Structured slog Attrs (no string interpolation)
- [ ] Zero overhead when Logger is nil (no allocations, no formatting)
- [ ] ≥80% test coverage for logging code paths

---

## Security Requirements (MANDATORY)

Per [memory-decay-logging-security-review.md](../security/memory-decay-logging-security-review.md):

**MUST NOT Log (Block on Violation)**:
| Field | Risk |
|-------|------|
| `MemoryRecord.Topic` | HIGH - user content |
| `MemoryRecord.Context` | HIGH - knowledge storage |
| `MemoryRecord.Decisions` | HIGH - business reasoning |
| `Node.Name` | MEDIUM-HIGH - extracted entities |
| `Node.Description` | HIGH - extracted content |
| `Config.OpenAIKey` | CRITICAL - credentials |
| Error messages with content | HIGH - information leakage |

**Safe to Log**:
| Field | Notes |
|-------|-------|
| IDs (memory_id, node_id, edge_id) | UUIDs are opaque |
| Status enums | Active, Superseded, etc. |
| Timestamps | CreatedAt, UpdatedAt, LastAccessedAt |
| Counts | AccessCount, NodesPruned, etc. |
| Config values | DecayEnabled, HalfLifeDays, etc. |
| Boolean decisions | shouldPrune, filtered, etc. |

---

## Assumptions

1. Go 1.21+ is required (per go.mod: 1.25.4) — `log/slog` is available
2. Existing `WithMetricsCollector()` and `WithTraceExporter()` patterns are the model
3. Test coverage ≥80% measured via `go test -cover`
4. Entity name leak in [pkg/extraction/entities.go](pkg/extraction/entities.go) is remediated as part of this work (per security finding 2.2)
5. DecayingSearcher is internal to gognee — adding logger parameter is acceptable

---

## Plan

### Milestone 1: Tests First — Logger Infrastructure (TDD)

**Objective**: Write tests for logger injection and nil-safety before implementation.

**Files to Create/Modify**:
- [pkg/gognee/gognee_test.go](pkg/gognee/gognee_test.go) — add logger injection tests

**Test Cases**:
1. `TestWithLogger_NilSafe` — calling methods with nil logger produces no panic
2. `TestWithLogger_Injection` — `WithLogger()` returns same instance (fluent pattern)
3. `TestWithLogger_PropagatesLogging` — when logger is set, logs are emitted (use test handler)
4. `TestDecayConfigLogging` — decay config is logged at INFO on NewWithClients when logger present
5. `TestNoLogAllocationWhenNil` — benchmark confirming zero allocs when logger is nil

**Test Helper Pattern**:
```go
// ILLUSTRATIVE ONLY - shows test capture pattern
type captureHandler struct {
    records []slog.Record
    mu      sync.Mutex
}
func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.records = append(h.records, r)
    return nil
}
```

**Acceptance Criteria**:
- [ ] Tests compile and fail (no implementation yet)
- [ ] Tests cover: nil logger, logger injection, log capture, no-content-in-logs

---

### Milestone 2: Implement Logger Field and WithLogger()

**Objective**: Add logger field to Gognee struct and WithLogger() injection method.

**Files to Modify**:
- [pkg/gognee/gognee.go](pkg/gognee/gognee.go)

**Tasks**:
1. Add field to Gognee struct: `logger *slog.Logger`
2. Add `WithLogger(logger *slog.Logger) *Gognee` method matching existing pattern
3. Initialize to `nil` in `NewWithClients()` (comment: "Set via WithLogger")
4. Add internal nil-safe helper (if desired) or use nil-check pattern

**Nil-Check Pattern (Recommended — matches existing metrics/trace pattern)**:
```go
// ILLUSTRATIVE ONLY
if g.logger != nil {
    g.logger.LogAttrs(ctx, slog.LevelInfo, "message", attrs...)
}
```

**Acceptance Criteria**:
- [ ] M1 tests pass
- [ ] `WithLogger()` exists and is fluent-chainable
- [ ] No functional change to existing behavior when logger is nil

---

### Milestone 3: Tests First — Decay Config Logging at Startup

**Objective**: Write tests for startup config logging before implementation.

**Files to Modify**:
- [pkg/gognee/gognee_test.go](pkg/gognee/gognee_test.go)

**Test Cases**:
1. `TestNewWithClients_LogsDecayConfigWhenLoggerSet` — verify INFO log emitted with decay config attrs
2. `TestNewWithClients_NoLogWhenLoggerNil` — verify no log when logger nil
3. `TestNewWithClients_LogAttrsAreSecure` — verify no content fields in logged attributes

**Expected Log Attributes** (all safe):
| Attribute | Source |
|-----------|--------|
| `decay_enabled` | `cfg.DecayEnabled` |
| `half_life_days` | `cfg.DecayHalfLifeDays` |
| `decay_basis` | `cfg.DecayBasis` |
| `access_frequency_enabled` | `cfg.AccessFrequencyEnabled` |
| `reference_access_count` | `cfg.ReferenceAccessCount` |

**Acceptance Criteria**:
- [ ] Tests compile and fail (no implementation yet)
- [ ] Tests verify correct attributes, correct level (INFO)

---

### Milestone 4: Implement Startup Decay Config Logging

**Objective**: Log decay configuration at INFO level when Gognee is created.

**Files to Modify**:
- [pkg/gognee/gognee.go](pkg/gognee/gognee.go) — in constructor after config initialization

**Logging Point**: After defaults are applied in `NewWithClients()`, before returning:
```go
// ILLUSTRATIVE ONLY
if g.logger != nil {
    g.logger.LogAttrs(ctx, slog.LevelInfo, "decay config initialized",
        slog.Bool("decay_enabled", cfg.DecayEnabled),
        slog.Int("half_life_days", cfg.DecayHalfLifeDays),
        slog.String("decay_basis", cfg.DecayBasis),
        slog.Bool("access_frequency_enabled", cfg.AccessFrequencyEnabled),
        slog.Int("reference_access_count", cfg.ReferenceAccessCount),
    )
}
```

**Note**: `NewWithClients()` doesn't have context — either:
- Use `context.Background()` for startup log, OR
- Log on first operation (less preferred)

**Decision**: Use `context.Background()` — startup logging is acceptable.

**Acceptance Criteria**:
- [ ] M3 tests pass
- [ ] Log emitted only when logger is non-nil
- [ ] All attributes are from safe-to-log list

---

### Milestone 5: Tests First — Prune Operation Logging

**Objective**: Write tests for Prune() logging before implementation.

**Files to Modify**:
- [pkg/gognee/gognee_test.go](pkg/gognee/gognee_test.go)

**Test Cases**:
1. `TestPrune_LogsStartAtInfo` — verify INFO log at prune start with options
2. `TestPrune_LogsPerMemoryAtDebug` — verify DEBUG logs for memory evaluation (capture shows memory_id, status, decision)
3. `TestPrune_LogsPerNodeAtDebug` — verify DEBUG logs for node evaluation
4. `TestPrune_LogsSummaryAtInfo` — verify INFO log at completion with counts
5. `TestPrune_NoLogWhenLoggerNil` — verify no logs when logger nil
6. `TestPrune_NoContentInLogs` — verify no Topic, Context, Name, Description in any log

**Expected Log Attributes**:
| Event | Level | Attributes |
|-------|-------|------------|
| prune started | INFO | dry_run, max_age_days, min_decay_score, prune_superseded |
| memory evaluated | DEBUG | memory_id, status, retention_policy, pinned, decision |
| node evaluated | DEBUG | node_id, age_days, decay_score, decision |
| prune complete | INFO | memories_evaluated, memories_pruned, nodes_evaluated, nodes_pruned, edges_pruned, duration_ms |

**Acceptance Criteria**:
- [ ] Tests compile and fail
- [ ] Tests verify no sensitive data (Topic, Context, Name, Description) appears in logs

---

### Milestone 6: Implement Prune Operation Logging

**Objective**: Add structured logging to Prune() method.

**Files to Modify**:
- [pkg/gognee/gognee.go](pkg/gognee/gognee.go) — Prune() function

**Log Points**:
1. **Prune start** (INFO): After defaults applied, before any evaluation
2. **Memory evaluation** (DEBUG): Inside memory evaluation loop
3. **Node evaluation** (DEBUG): Inside node evaluation loop  
4. **Prune complete** (INFO): After all operations, include timing

**Timing**: Capture `startTime := time.Now()` at start, compute `durationMs` at end.

**Acceptance Criteria**:
- [ ] M5 tests pass
- [ ] ≥80% coverage for Prune() logging code paths
- [ ] Zero overhead verified when logger is nil

---

### Milestone 7: Tests First — DecayingSearcher Logging

**Objective**: Write tests for DecayingSearcher logging before implementation.

**Files to Create/Modify**:
- [pkg/search/decay_test.go](pkg/search/decay_test.go)

**Test Cases**:
1. `TestDecayingSearcher_LogsDisabledPassthrough` — when decay disabled, DEBUG log indicates passthrough
2. `TestDecayingSearcher_LogsNodeEvaluation` — verify DEBUG log per node with decay_score, decision
3. `TestDecayingSearcher_LogsRetentionPolicy` — when retention policy applied, log the policy half-life
4. `TestDecayingSearcher_LogsFilteredNode` — when node filtered (score < 0.001), log the filtering
5. `TestDecayingSearcher_NoLogWhenLoggerNil` — verify no logs when logger nil
6. `TestDecayingSearcher_NoContentInLogs` — verify no Node.Name, Node.Description in logs

**Expected Log Attributes**:
| Event | Level | Attributes |
|-------|-------|------------|
| decay disabled passthrough | DEBUG | — |
| node timestamp fetch failed | WARN | node_id, error_code (not full error message) |
| retention policy applied | DEBUG | node_id, policy, half_life_days |
| node evaluated | DEBUG | node_id, decay_score, heat_multiplier, filtered |
| node filtered | DEBUG | node_id, score |

**Acceptance Criteria**:
- [ ] Tests compile and fail
- [ ] Tests use mock logger to capture records

---

### Milestone 8: Propagate Logger to DecayingSearcher

**Objective**: Add logger capability to DecayingSearcher and wire it from Gognee.

**Files to Modify**:
- [pkg/search/decay.go](pkg/search/decay.go) — add logger field and SetLogger() method
- [pkg/gognee/gognee.go](pkg/gognee/gognee.go) — propagate logger in WithLogger()

**Tasks**:
1. Add `logger *slog.Logger` field to DecayingSearcher struct
2. Add `SetLogger(logger *slog.Logger)` method to DecayingSearcher
3. In Gognee.WithLogger(), propagate to DecayingSearcher:
   ```go
   // ILLUSTRATIVE ONLY
   if ds, ok := g.searcher.(*search.DecayingSearcher); ok {
       ds.SetLogger(logger)
   }
   ```

**Acceptance Criteria**:
- [ ] DecayingSearcher has logger field
- [ ] Logger propagates from Gognee to DecayingSearcher
- [ ] M7 tests pass

---

### Milestone 9: Implement DecayingSearcher Logging

**Objective**: Add structured logging to DecayingSearcher.Search().

**Files to Modify**:
- [pkg/search/decay.go](pkg/search/decay.go)

**Log Points**:
1. **Decay disabled passthrough** (DEBUG): Early return when `!d.enabled`
2. **Node timestamp fetch failed** (WARN): On graphStore.GetNode() error
3. **Retention policy applied** (DEBUG): When policy override used
4. **Node evaluated** (DEBUG): After decay calculation, before filter
5. **Node filtered** (DEBUG): When score < 0.001

**Acceptance Criteria**:
- [ ] M7 tests pass
- [ ] ≥80% coverage for DecayingSearcher logging code paths
- [ ] Verify zero overhead with nil logger (benchmark)

---

### Milestone 10: Remediate Entity Name Leak (Security Fix)

**Objective**: Fix existing security issue in pkg/extraction/entities.go that logs entity names.

**Files to Modify**:
- [pkg/extraction/entities.go](pkg/extraction/entities.go) — line ~97

**Current (INSECURE)**:
```go
log.Printf("gognee: entity %q has unrecognized type %q, normalizing to Concept", entity.Name, entity.Type)
```

**Fix Options**:
1. Remove log entirely (simplest)
2. Log type only: `"entity with unrecognized type %q, normalizing to Concept", entity.Type`
3. Integrate with injectable logger (future work)

**Decision**: Option 2 — log type only (safe). Full logger integration is out of scope for this plan.

**Acceptance Criteria**:
- [ ] No `entity.Name` in any log statement
- [ ] Type normalization still logged for debugging
- [ ] Tests verify no content leakage

---

### Milestone 11: Update Version and Release Artifacts

**Objective**: Prepare v1.6.0 release.

**Files to Modify**:
- [CHANGELOG.md](CHANGELOG.md)
- [README.md](README.md) — document logging configuration

**CHANGELOG Entry**:
```markdown
## [1.6.0] - YYYY-MM-DD

### Added
- **Structured logging for memory decay**: Injectable `WithLogger(*slog.Logger)` for observability
  - Startup: decay config logged at INFO level
  - Prune: operation options (INFO), per-item evaluation (DEBUG), summary (INFO)
  - DecayingSearcher: per-node evaluation at DEBUG level
- Zero overhead when logger is nil (no allocations, no formatting)

### Fixed
- **Security**: Entity type normalization log no longer includes entity names

### Security
- Logging follows strict data classification: IDs, timestamps, counts, status enums are safe; Topic, Context, Name, Description are NEVER logged
```

**README Update**: Add section on logging configuration:
```markdown
### Logging (Optional)

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug, // or LevelInfo for less verbose
}))

g, _ := gognee.New(cfg)
g.WithLogger(logger)
```
```

**Acceptance Criteria**:
- [ ] CHANGELOG updated with all additions and security note
- [ ] README documents logging usage
- [ ] Version updated to v1.6.0

---

## Testing Strategy

**Test Types**:
- **Unit Tests**: All logging code paths tested with mock log handler
- **Security Tests**: Assert no sensitive fields (Topic, Context, Name, etc.) in captured logs
- **Benchmark Tests**: Verify zero allocations when logger is nil

**Coverage Target**: ≥80% for new logging code

**Test Patterns**:
1. Use `captureHandler` to capture log records for assertion
2. Search captured records for forbidden strings
3. Use `testing.AllocsPerRun()` for zero-alloc verification

---

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Content leakage in logs | Medium | High | Security tests that grep for forbidden strings; code review |
| Performance overhead | Low | Medium | Nil-check before logging; benchmark tests |
| Breaking API change | Low | Low | `WithLogger()` is additive; DecayingSearcher is internal |
| Log verbosity in production | Medium | Low | Per-item logs at DEBUG only; recommend INFO for production |

---

## Residuals Reconciliation

**Residuals Ledger**: No `agent-output/process-improvement/residuals-ledger.md` file exists.

**Action**: No residuals to reconcile. This plan creates new observability infrastructure without addressing prior technical debt entries.

**Note**: If residuals ledger is created before implementation, re-check for relevant entries.

---

## Dependencies

1. Discovery documents reviewed and incorporated:
   - [023-memory-decay-structured-logging-architecture-findings.md](../architecture/023-memory-decay-structured-logging-architecture-findings.md) — APPROVED
   - [004-decay-logging-observability-analysis.md](../analysis/004-decay-logging-observability-analysis.md) — patterns confirmed
   - [memory-decay-logging-security-review.md](../security/memory-decay-logging-security-review.md) — APPROVED_WITH_CONTROLS

2. No external dependencies added — uses Go stdlib `log/slog`

---

## Open Questions

**None** — All architecture and security decisions resolved in discovery phase.

---

## Handoff Notes

1. **TDD Approach**: Milestones alternate tests-first (odd) and implementation (even)
2. **Security First**: M10 (entity name leak fix) is REQUIRED per security review
3. **Coverage**: Run `go test -cover ./pkg/gognee ./pkg/search` and verify ≥80%
4. **Zero-Alloc**: Include benchmark test to verify nil-logger path allocates nothing

---

*Plan created: 2026-02-18*
*Ready for Critic review*

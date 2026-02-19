# Architecture Findings: Structured Logging for Memory Decay Subsystem

**ID**: 023
**Date**: 2026-02-18
**Status**: PRE-PLANNING REVIEW COMPLETE
**Verdict**: APPROVED

## Changelog
| Date | Handoff | Context | Outcome |
|------|---------|---------|---------|
| 2026-02-18 | User → Architect | Pre-planning review for adding structured logging to decay subsystem | Architecture findings documented; APPROVED with recommendations |

---

## 1. Executive Summary

Adding structured logging to Gognee's memory decay subsystem is architecturally sound and follows established patterns. The recommendation is to use Go's standard `log/slog` package with caller-injected loggers, matching the existing `WithMetricsCollector()`/`WithTraceExporter()` pattern.

**Key Recommendations:**
- Use `*slog.Logger` directly (no custom interface)
- Inject via `WithLogger(*slog.Logger)` method
- Default to no-op (nil check) when logger not provided
- Log levels: Debug (config), Info (actions), Warn (anomalies)

---

## 2. Current State Analysis

### 2.1 Existing Injection Patterns

Gognee already uses a consistent pattern for optional infrastructure:

```go
// In Gognee struct
metricsCollector  metrics.Collector // Optional
traceExporter     tracepkg.Exporter // Optional

// Injection via builder methods
func (g *Gognee) WithMetricsCollector(collector metrics.Collector) *Gognee
func (g *Gognee) WithTraceExporter(exporter tracepkg.Exporter) *Gognee

// Usage pattern (nil-check at call site)
if g.metricsCollector != nil {
    g.metricsCollector.RecordOperation(ctx, "cognify", status, durationMs)
}
```

This pattern is idiomatic for Go libraries: optional by default, caller-injected, nil-safe.

### 2.2 Trace vs Logging Distinction

The existing trace infrastructure ([pkg/trace/](pkg/trace/)) serves a different purpose:

| Aspect | Trace (existing) | Logging (proposed) |
|--------|------------------|-------------------|
| **Purpose** | Performance timing/spans | Operational observability |
| **Format** | JSON Lines file | Structured log records |
| **Granularity** | Per-operation aggregates | Per-event detail |
| **Consumer** | Performance analysis tooling | Operators, debugging |
| **When** | Opt-in per operation | Always (when logger set) |

Logging complements tracing; they are not redundant.

### 2.3 Go Version Compatibility

`go.mod` specifies Go 1.25.4 - well beyond Go 1.21 where `log/slog` was added. No compatibility concerns.

---

## 3. Recommendation: Use `log/slog` Directly

### 3.1 Why `*slog.Logger` Over Custom Interface

| Option | Pros | Cons |
|--------|------|------|
| **Custom interface** | Library controls contract | Forces callers to adapt/wrap; more code |
| **`*slog.Logger` directly** | Zero adaptation for callers; stdlib; widely adopted | Couples to stdlib (Go 1.21+) |

**Recommendation**: Use `*slog.Logger` directly.

Rationale:
1. Go 1.21+ is already required (current: 1.25.4)
2. `slog.Handler` interface allows complete customization (JSON, text, custom backends)
3. Callers using any logging library can adapt via slog handlers (zap, zerolog, logrus all have slog adapters)
4. Reduces gognee maintenance burden - no custom interface to document/version

### 3.2 Injection Pattern

```go
// In Config struct - NO change (loggers are runtime, not config)

// In Gognee struct
logger *slog.Logger  // Optional structured logger

// Injection method
func (g *Gognee) WithLogger(logger *slog.Logger) *Gognee {
    g.logger = logger
    return g
}
```

### 3.3 Nil-Safe Logging Helper

To avoid repetitive nil checks:

```go
// Internal helper (not exported)
func (g *Gognee) log() *slog.Logger {
    if g.logger == nil {
        return slog.New(discardHandler{})  // or use a package-level noop logger
    }
    return g.logger
}

// Usage
g.log().Info("prune started", "dryRun", opts.DryRun)
```

Alternative (simpler): just nil-check at each site, matching existing metricsCollector pattern.

---

## 4. Injection Points Analysis

### 4.1 `gognee.New()` / `NewWithClients()`

**Current**: No logging.

**Proposed Logging Events**:
| Event | Level | Attributes |
|-------|-------|------------|
| Config applied | Debug | `decayEnabled`, `halfLifeDays`, `basis`, `accessFrequencyEnabled` |
| Config validation failed | Warn | `field`, `value`, `error` |

**Example**:
```go
if g.logger != nil {
    g.logger.LogAttrs(ctx, slog.LevelDebug, "decay config applied",
        slog.Bool("enabled", cfg.DecayEnabled),
        slog.Int("halfLifeDays", cfg.DecayHalfLifeDays),
        slog.String("basis", cfg.DecayBasis),
    )
}
```

### 4.2 `DecayingSearcher`

**Current**: No logging, silently skips nodes on error.

**Proposed Logging Events**:
| Event | Level | Attributes |
|-------|-------|------------|
| Decay disabled, passthrough | Debug | - |
| Node timestamp fetch failed | Warn | `nodeID`, `error` |
| Retention policy applied | Debug | `nodeID`, `policy`, `halfLifeDays` |
| Node filtered (score < 0.001) | Debug | `nodeID`, `score` |

**Architectural Note**: `DecayingSearcher` currently has no access to a logger. Two options:

**Option A (Recommended)**: Pass logger to `NewDecayingSearcher()`:
```go
func NewDecayingSearcher(
    underlying Searcher,
    graphStore store.GraphStore,
    memoryStore store.MemoryStore,
    enabled bool,
    halfLifeDays int,
    basis string,
    accessFrequencyEnabled bool,
    referenceAccessCount int,
    logger *slog.Logger,  // NEW
) *DecayingSearcher
```

**Option B**: Add `WithLogger()` to `DecayingSearcher`.

Option A is preferred because it matches the constructor-injection pattern and avoids exposing internal types to callers.

### 4.3 `Prune()`

**Current**: Silent operation, errors collected but not logged.

**Proposed Logging Events**:
| Event | Level | Attributes |
|-------|-------|------------|
| Prune started | Info | `dryRun`, `maxAgeDays`, `minDecayScore`, `pruneSuperseded` |
| Memory evaluated | Debug | `memoryID`, `status`, `retentionPolicy`, `pinned`, `decision` |
| Memory pruned | Info | `memoryID`, `reason` |
| Node evaluated | Debug | `nodeID`, `ageDays`, `decayScore`, `decision` |
| Node pruned | Info | `nodeID`, `reason` |
| Prune complete | Info | `memoriesEvaluated`, `memoriesPruned`, `nodesEvaluated`, `nodesPruned`, `durationMs` |

---

## 5. Log Level Recommendations

| Level | Use Case | Examples |
|-------|----------|----------|
| **Debug** | Config details, per-item evaluation, internal state | "decay config applied", "node evaluated", "retention policy applied" |
| **Info** | Significant actions, operation boundaries | "prune started", "node pruned", "prune complete" |
| **Warn** | Anomalies, recoverable errors, unexpected states | "node fetch failed", "memory evaluation error" |
| **Error** | Operation failures (rare - most errors return to caller) | Logged by caller, not gognee |

**Guideline**: Gognee should NOT log errors that it returns - callers decide how to handle those. Only log errors it handles/ignores internally.

---

## 6. Risks and Mitigations

### 6.1 Risk: Logging Performance Overhead

**Concern**: Debug logging in hot paths (DecayingSearcher.Search) could impact latency.

**Mitigation**:
1. Use `Logger.Enabled(level)` check before constructing expensive log messages
2. Default to no logger (zero overhead)
3. Recommend Info level for production, Debug only for troubleshooting

### 6.2 Risk: Log Verbosity / Noise

**Concern**: Per-node/per-memory logging during large prune operations could generate excessive logs.

**Mitigation**:
1. Per-item evaluation at Debug level only
2. Aggregated summaries at Info level
3. Consider sampling for very large operations (future enhancement)

### 6.3 Risk: Breaking API Change

**Concern**: Adding logger parameter to `NewDecayingSearcher()` is a breaking change.

**Mitigation**:
- `DecayingSearcher` is in `pkg/search` (internal to library use)
- Only `Gognee` facade is the public API; internal wiring changes are acceptable
- Alternatively, add optional parameter via functional options pattern

### 6.4 Risk: Sensitive Data in Logs

**Concern**: Memory content, user queries could appear in logs.

**Mitigation**:
1. Never log memory content, queries, or user data
2. Log only IDs, counts, configuration, and operational metadata
3. Follow the trace infrastructure pattern: IDs only, no payloads

---

## 7. Implementation Guidance

### 7.1 Suggested Struct Changes

```go
// pkg/gognee/gognee.go
type Gognee struct {
    // ... existing fields ...
    logger *slog.Logger  // Optional structured logger
}

// pkg/search/decay.go
type DecayingSearcher struct {
    // ... existing fields ...
    logger *slog.Logger  // Optional (nil = no logging)
}
```

### 7.2 Wire-up in NewWithClients

```go
// Pass logger to DecayingSearcher
searcher = search.NewDecayingSearcher(
    baseSearcher,
    graphStore,
    memoryStore,
    cfg.DecayEnabled,
    cfg.DecayHalfLifeDays,
    cfg.DecayBasis,
    cfg.AccessFrequencyEnabled,
    cfg.ReferenceAccessCount,
    nil,  // logger - will be set via WithLogger
)

// ... but DecayingSearcher doesn't expose logger setter, so either:
// 1. Create DecayingSearcher after WithLogger is called (lazy init), or
// 2. Store logger ref and propagate in WithLogger
```

**Recommended Pattern**: Store logger on Gognee, propagate to DecayingSearcher via interface or method:

```go
func (g *Gognee) WithLogger(logger *slog.Logger) *Gognee {
    g.logger = logger
    // Propagate to decay searcher if it accepts logger
    if ds, ok := g.searcher.(*search.DecayingSearcher); ok {
        ds.SetLogger(logger)
    }
    return g
}
```

### 7.3 Avoid Config Struct Pollution

Do NOT add logger to Config:
- Config is for serializable/persistent settings
- Logger is a runtime dependency, not configuration
- Matches existing pattern (metricsCollector, traceExporter not in Config)

---

## 8. Testing Implications

1. **Unit tests**: Use `slog.New(slog.NewTextHandler(io.Discard, nil))` or nil
2. **Assertion tests**: Create a test handler that captures records for verification
3. **No network**: Logging is local-only, no test isolation concerns

---

## 9. Decision Summary

| Question | Decision |
|----------|----------|
| Custom interface vs slog? | Use `*slog.Logger` directly |
| Where to inject? | `WithLogger(*slog.Logger)` method on Gognee |
| Default behavior? | nil = no logging (zero overhead) |
| Levels for decay events? | Debug (evaluation), Info (actions), Warn (anomalies) |
| Config struct change? | None - logger is runtime, not config |
| Breaking changes? | No - internal wiring only; public API unchanged |

---

## 10. Files Affected (Anticipated)

| File | Change Type |
|------|-------------|
| [pkg/gognee/gognee.go](pkg/gognee/gognee.go) | Add logger field, WithLogger(), logging calls in Prune() |
| [pkg/search/decay.go](pkg/search/decay.go) | Add logger field, SetLogger(), logging in Search() |
| [pkg/gognee/gognee_test.go](pkg/gognee/gognee_test.go) | Test with/without logger |
| [pkg/search/decay_test.go](pkg/search/decay_test.go) | Test logging output |

---

## 11. Verdict

**APPROVED**

Adding structured logging via `log/slog` with caller-injected loggers is architecturally sound. It:
- Follows established gognee patterns (metricsCollector, traceExporter)
- Uses Go stdlib, minimizing dependency surface
- Provides operational visibility without breaking API
- Has clear level semantics
- Introduces no blocking concerns

**Recommended Next Steps**:
1. Planner creates implementation plan with milestones
2. First milestone: Add logger field + WithLogger() to Gognee
3. Second milestone: Logging in Prune() 
4. Third milestone: Propagate logger to DecayingSearcher + logging there

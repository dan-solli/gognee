Value Statement and Business Objective
Gognee needs a clear, low-overhead logging pattern for decay observability so integrators can correlate decay behavior with search/prune results without exposing sensitive memory content.

Status: Planned

**Planned In**: [023-memory-decay-structured-logging.md](../planning/023-memory-decay-structured-logging.md)

Changelog
- 2026-02-18: Initial analysis and recommendations.
- 2026-02-18: Status → Planned. Incorporated into Plan 023.

Objective
Provide recommended Go logging patterns for decay observability, including logger injection, metadata, and a safe default null logger, aligned with existing Gognee patterns.

Context
- Gognee currently supports optional metrics and tracing via setter methods on the main struct, not via functional options or a config field ([pkg/gognee/gognee.go](pkg/gognee/gognee.go#L1)).
- Tracing defines an interface and provides a no-op exporter when the path is empty, keeping default behavior silent ([pkg/trace/interface.go](pkg/trace/interface.go#L1), [pkg/trace/exporter.go](pkg/trace/exporter.go#L1)).
- Metrics exposes a small interface with a concrete Prometheus implementation and documentation for a no-op default (via build tags or alternative build) ([pkg/metrics/interface.go](pkg/metrics/interface.go#L1), [pkg/metrics/metrics.go](pkg/metrics/metrics.go#L1)).

Root Cause
Decay behavior is currently observable only indirectly (e.g., through search results and pruning side effects), and there is no first-class library logging pattern in Gognee to expose decay computations, thresholds, and reasons in a structured, low-risk way.

Methodology
- Read the main configuration and construction flow to see how dependencies are injected.
- Reviewed tracing and metrics packages for existing observability patterns and safe defaults.

Findings (Fact)
- Gognee uses constructor + setter methods for optional subsystems (metrics, trace), rather than a unified functional options pattern ([pkg/gognee/gognee.go](pkg/gognee/gognee.go#L1)).
- Trace provides a no-op exporter by default (empty path), enabling safe opt-in observability ([pkg/trace/exporter.go](pkg/trace/exporter.go#L1)).
- Metrics uses a narrow interface and avoids payload leakage by design/tests ([pkg/metrics/metrics_test.go](pkg/metrics/metrics_test.go#L1)).

Findings (Hypothesis)
- For Go 1.21+ libraries, `log/slog` is the most idiomatic baseline for structured logging and integrates with `context.Context` for trace correlation.
- A default no-op logger is preferred for libraries to avoid unexpected stdout logging and to keep performance predictable.

Recommendations
1) Logging pattern (Go 1.21+): use `log/slog` and accept `*slog.Logger` injection.
- Rationale: `slog` is standard, structured, and supports context-aware logging via `LogAttrs` for trace correlation.
- Avoid global loggers; keep the logger on the `Gognee` instance.

2) Logger injection pattern aligned with Gognee:
- Prefer a setter method `WithLogger(*slog.Logger)` to match `WithMetricsCollector` and `WithTraceExporter`.
- Optionally also allow `Config.Logger` so users can set it at construction time.

Code example (recommended):
```go
// config field (optional)
// Logger provides structured logging (nil uses NoOp logger).
Logger *slog.Logger

// setter method (matches existing pattern)
func (g *Gognee) WithLogger(logger *slog.Logger) *Gognee {
    if logger == nil {
        g.logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
        return g
    }
    g.logger = logger
    return g
}
```

3) Default NoOp logger pattern:
- Use a discard handler for a true no-op, consistent with trace’s noop default.

Code example:
```go
var noopLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
```

4) Decay log metadata (structured attributes):
- Core identifiers: `operation` (search, prune, decay), `op_id` (correlation id), `component` (decay).
- Configuration: `decay_enabled`, `decay_basis`, `half_life_days`, `access_frequency_enabled`, `reference_access_count`.
- Computation inputs: `last_access_at`, `created_at`, `access_count`, `elapsed_days`.
- Computation outputs: `decay_score_before`, `decay_score_after`, `heat_multiplier`.
- Decision metadata: `threshold`, `prune_reason`, `pruned`, `dry_run`, `retention_policy`.
- Safe identifiers only: `memory_id`, `node_id`, `edge_id` (no content fields).

5) Logging levels and volume control:
- `Debug`: per-memory decay calculation events (high volume).
- `Info`: summary events per operation (counts, thresholds, timing).
- `Warn`: configuration anomalies or unexpected decay basis.
- `Error`: storage failures, invalid data preventing decay computation.

Example summary log:
```go
logger.LogAttrs(ctx, slog.LevelInfo, "decay.summary",
    slog.String("operation", "prune"),
    slog.Int("nodes_evaluated", nodes),
    slog.Int("nodes_pruned", pruned),
    slog.Float64("min_decay_score", minScore),
    slog.String("decay_basis", cfg.DecayBasis),
)
```

Open Questions
- Should logger injection be config-only or both config and `WithLogger` to align with existing setter patterns?
- Should decay logging be integrated with trace spans (e.g., reuse `op_id` in both trace and logs)?
- What is the acceptable log volume for per-memory decay logs in production, and should sampling be built in?

Notes for Planner Handoff
- Existing observability patterns in Gognee strongly favor opt-in behavior with a no-op default for silent operation.
- The logger should follow the same privacy posture as metrics and tracing: no memory content or user payload in logs.

# Security Review: Memory Decay Logging Implementation

**Review Type**: Targeted Code Review  
**Scope**: Logging implementation for memory decay subsystem  
**Date**: 2026-02-18  
**Status**: COMPLETE  
**Verdict**: APPROVED_WITH_CONTROLS

---

## Executive Summary

Adding logging to the memory decay subsystem is **APPROVED** with mandatory controls. The primary risks involve inadvertent exposure of sensitive memory content (Topics, Context, Decisions, Rationale) through logs. Secondary concerns relate to library stdout pollution and GDPR compliance.

---

## 1. Data Classification & Logging Guidance

### 1.1 MUST NOT Log (Critical - Block on Violation)

| Data Field | Risk | Reason |
|------------|------|--------|
| `MemoryRecord.Topic` | HIGH | User-generated, may contain PII, project names, sensitive subjects |
| `MemoryRecord.Context` | HIGH | Free-form text, primary knowledge storage - highly sensitive |
| `MemoryRecord.Decisions` | HIGH | Business-critical reasoning, may reveal strategic information |
| `MemoryRecord.Rationale` | HIGH | Similar to decisions - reasoning chains |
| `Node.Name` | MEDIUM-HIGH | Entity names extracted from user content |
| `Node.Description` | HIGH | Descriptions from knowledge extraction |
| `Config.OpenAIKey` | CRITICAL | API credentials - never log |
| `MemoryRecord.Metadata` (values) | HIGH | Arbitrary user-supplied data |
| `SupersessionRecord.Reason` | MEDIUM-HIGH | User-provided text |
| `MemoryRecord.PinnedReason` | MEDIUM-HIGH | User-provided text |
| `MemoryRecord.Source` | MEDIUM | May reveal file paths, URLs, or internal systems |

### 1.2 Safe to Log (Approved for Logging)

| Data Field | Notes |
|------------|-------|
| `MemoryRecord.ID` | UUIDs are opaque identifiers |
| `Node.ID` | UUIDs are opaque identifiers |
| `Edge.ID`, `Edge.SourceID`, `Edge.TargetID` | UUIDs only |
| `MemoryRecord.Status` | Enum values: Active, Superseded, Archived, Pinned |
| `MemoryRecord.RetentionPolicy` | Enum values: permanent, decision, standard, ephemeral, session |
| `MemoryRecord.Version` | Integer counter |
| `MemoryRecord.AccessCount` | Integer counter |
| `MemoryRecord.AccessVelocity` | Decimal statistic |
| Timestamps: `CreatedAt`, `UpdatedAt`, `LastAccessedAt`, `PinnedAt` | ISO timestamps |
| `MemoryRecord.DocHash` | SHA-256 hash, not reversible |
| `PruneOptions.*` | Configuration values only |
| `PruneResult.*` | Aggregate counts and ID lists |
| `Config.DecayEnabled`, `Config.DecayHalfLifeDays`, `Config.DecayBasis` | Boolean/integer/enum |
| `Config.DBPath` | File path (review for path traversal in other contexts) |
| Count/aggregate statistics | `NodeCount`, `EdgeCount`, `MemoriesEvaluated` |

### 1.3 Conditional (Log with Redaction/Truncation)

| Data Field | Guidance |
|------------|----------|
| `Node.Type` | Safe - enum values (Person, Concept, System, etc.) |
| `Edge.Relation` | Safe - relationship type enum |
| `Metadata` (keys only) | Keys may be safe; log key count or key names only, never values |
| Error messages | Sanitize to remove content - use error codes or generic messages |

---

## 2. Critical Security Findings

### Finding 2.1: Library Context - Stdout Pollution Risk

**Severity**: MEDIUM  
**Status**: OPEN

**Issue**: Gognee is a library (`pkg/gognee`) meant to be embedded by consuming applications. Using `log.Printf()` writes to global stdout/stderr which may:
- Pollute consumer application logs
- Expose sensitive information in production logs the consumer doesn't control
- Interfere with structured logging systems (JSON loggers, etc.)

**Current pattern observed**:
```go
// pkg/extraction/entities.go:97
log.Printf("gognee: entity %q has unrecognized type %q, normalizing to Concept", entity.Name, entity.Type)
```
This logs `entity.Name` which violates Section 1.1.

**Recommendation**: Implement a configurable logger interface:

```go
// pkg/gognee/logger.go
type Logger interface {
    Debug(msg string, fields ...any)
    Info(msg string, fields ...any)
    Warn(msg string, fields ...any)
    Error(msg string, fields ...any)
}

// NoopLogger for default (silent library behavior)
type NoopLogger struct{}
func (NoopLogger) Debug(msg string, fields ...any) {}
func (NoopLogger) Info(msg string, fields ...any)  {}
func (NoopLogger) Warn(msg string, fields ...any)  {}
func (NoopLogger) Error(msg string, fields ...any) {}

// Config addition
type Config struct {
    // ...existing fields...
    Logger Logger // Optional; defaults to NoopLogger
}
```

**Rationale**: Libraries should be silent by default. Let consumers opt-in to logging and choose their logging implementation.

---

### Finding 2.2: Existing Log Statement Leaks Entity Names

**Severity**: HIGH  
**Status**: OPEN - REQUIRES REMEDIATION

**Location**: [pkg/extraction/entities.go](pkg/extraction/entities.go#L97)

**Issue**: The existing log statement leaks entity names:
```go
log.Printf("gognee: entity %q has unrecognized type %q, normalizing to Concept", entity.Name, entity.Type)
```

**Recommendation**: Remove or redact:
```go
// Option 1: Log only the type (safe)
log.Printf("gognee: entity with unrecognized type %q, normalizing to Concept", entity.Type)

// Option 2: Use configurable logger (preferred)
g.logger.Warn("entity type normalized", "original_type", entity.Type, "target_type", "Concept")
```

---

### Finding 2.3: Memory Decay Logging Best Practices

**Severity**: INFO (advisory for new implementation)

**Recommended log points for decay/prune operations**:

```go
// SAFE - Config initialization
logger.Info("decay config initialized", 
    "enabled", cfg.DecayEnabled,
    "half_life_days", cfg.DecayHalfLifeDays,
    "decay_basis", cfg.DecayBasis,
    "access_frequency_enabled", cfg.AccessFrequencyEnabled)

// SAFE - Prune operation start
logger.Info("prune started",
    "dry_run", opts.DryRun,
    "max_age_days", opts.MaxAgeDays,
    "min_decay_score", opts.MinDecayScore,
    "prune_superseded", opts.PruneSuperseded)

// SAFE - Prune operation complete
logger.Info("prune completed",
    "nodes_evaluated", result.NodesEvaluated,
    "nodes_pruned", result.NodesPruned,
    "edges_pruned", result.EdgesPruned,
    "memories_evaluated", result.MemoriesEvaluated,
    "superseded_memories_pruned", result.SupersededMemoriesPruned,
    "duration_ms", elapsed.Milliseconds())

// CONDITIONAL - ID logging (only at Debug level, opt-in)
logger.Debug("memory evaluated for prune",
    "memory_id", memory.ID,               // OK: UUID
    "status", summary.Status,             // OK: enum
    "retention_policy", memory.RetentionPolicy,  // OK: enum
    "access_count", memory.AccessCount,   // OK: integer
    "will_prune", shouldPrune)            // OK: boolean

// NEVER LOG:
// - memory.Topic, memory.Context, memory.Decisions
// - node.Name, node.Description
// - Any user-provided text fields
```

---

## 3. GDPR & Compliance Considerations

### 3.1 Data Retention in Logs

**Issue**: If memory IDs are logged alongside timestamps, and those logs are retained by consumers, it creates an audit trail that may be subject to GDPR Right to Erasure.

**Recommendation**:
1. Document that memory IDs in logs do not contain PII themselves
2. Recommend consumers implement log rotation/retention policies
3. Consider adding a `LogAnonymize` config option that uses hashed identifiers:
   ```go
   anonymizedID := sha256.Sum256([]byte(memory.ID))[:8]
   logger.Debug("memory evaluated", "memory_id_hash", hex.EncodeToString(anonymizedID))
   ```

### 3.2 Cross-Border Data Transfer Consideration

If logs containing memory IDs are sent to third-party log aggregation services (Datadog, Splunk Cloud, etc.), ensure data processing agreements cover this. Memory IDs alone are not PII, but correlating them with stored content could reveal personal data.

---

## 4. Implementation Recommendations

### 4.1 Structured Logging (Priority: HIGH)

Use structured logging with key-value pairs rather than format strings:

```go
// BAD: Format string interpolation
log.Printf("Pruned %d nodes: %v", count, nodeIDs)

// GOOD: Structured fields
logger.Info("prune completed", "nodes_pruned", count, "node_ids", nodeIDs)
```

**Rationale**: Structured logs are easier to filter, redact, and process. They also prevent accidental injection of sensitive data through %v formatting.

### 4.2 Log Levels (Priority: MEDIUM)

| Level | Use For |
|-------|---------|
| ERROR | Operation failures requiring attention |
| WARN | Degraded operations, recoverable issues |
| INFO | Significant state changes (prune complete, config loaded) |
| DEBUG | Per-item processing, detailed diagnostics |

**Default level**: INFO (or silent/NoopLogger for library mode)

### 4.3 Sampling for High-Volume Operations (Priority: LOW)

If logging individual memory evaluations, consider sampling:

```go
// Only log every Nth item to avoid log flooding
if i%100 == 0 {
    logger.Debug("prune progress", "evaluated", i, "total", len(allMemories))
}
```

---

## 5. Security Checklist for Implementation

- [ ] **No content logging**: Verify no `Topic`, `Context`, `Decisions`, `Rationale`, `Name`, `Description` in any log statement
- [ ] **Configurable logger**: Implement `Logger` interface with `NoopLogger` default
- [ ] **Structured logging**: Use key-value pairs, not format strings
- [ ] **Log level control**: Provide level configuration
- [ ] **Test coverage**: Unit tests that verify no sensitive data in log output (capture log buffer, assert no content strings)
- [ ] **Documentation**: Add logging configuration to README with security guidance

---

## 6. Remediation Tracking

| Finding | Severity | Status | Owner | Deadline |
|---------|----------|--------|-------|----------|
| 2.1 Stdout pollution | MEDIUM | OPEN | Implementer | Pre-release |
| 2.2 Entity name leak | HIGH | OPEN | Implementer | Immediate |
| Logger interface | MEDIUM | OPEN | Implementer | Pre-release |

---

## Verdict

**APPROVED_WITH_CONTROLS**

The logging implementation may proceed with the following mandatory controls:

1. **BLOCK**: Do not log any fields listed in Section 1.1 (MUST NOT Log)
2. **REQUIRED**: Implement configurable logger interface (Finding 2.1)
3. **REQUIRED**: Remediate existing entity name leak (Finding 2.2)
4. **RECOMMENDED**: Follow structured logging patterns (Section 4.1)

---

*Security review by: Security Agent*  
*Review methodology: OWASP Logging Cheat Sheet, GDPR Article 17, Defense-in-Depth*

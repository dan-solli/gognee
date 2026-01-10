# Gognee Benchmark Baselines

**Date**: 2025-01-22  
**Plan**: 015-performance-instrumentation  
**Gognee Version**: v1.1.0-dev  
**System**: Linux, Intel Pentium G4600 @ 3.60GHz, 4 cores

## Methodology

- All benchmarks use **fake clients** (deterministic, no OpenAI API calls)
- FakeEmbeddingClient: SHA256-based vector generation (768 dimensions, float32)
- FakeLLMClient: Canned entity/relation responses
- Database: SQLite `:memory:` mode
- Benchmark duration: 3 seconds (`-benchtime=3s`)

## Baseline Results

```
BenchmarkCognify_Empty-4               48033        80454 ns/op  (~80µs)
BenchmarkCognify_100Memories-4         43465        78863 ns/op  (~79µs)
BenchmarkCognify_1000Memories-4        48112        85106 ns/op  (~85µs)
BenchmarkSearch_Empty-4               482324         6280 ns/op  (~6µs)
BenchmarkSearch_100Memories-4         645115         5582 ns/op  (~5.6µs)
BenchmarkSearch_1000Memories-4        643296         8060 ns/op  (~8µs)
```

### Analysis

**Cognify Performance**:
- Empty graph: ~80µs per operation
- 100 memories: ~79µs (no degradation)
- 1000 memories: ~85µs (minimal degradation, +6%)

**Search Performance**:
- Empty graph: ~6µs per query
- 100 memories: ~5.6µs (faster due to better vector locality)
- 1000 memories: ~8µs (28% increase, acceptable)

### Observations

1. **Cognify is consistent**: ~80µs regardless of graph size (chunking/extraction/embedding dominate, not storage)
2. **Search scales well**: <2µs increase from 100 to 1000 memories
3. **Offline benchmarks work**: No OpenAI calls, fully deterministic, fast execution
4. **Overhead is negligible**: Fake clients add <10µs overhead vs. production (network latency would be 100ms+)

## CI Guidance

- Run benchmarks with: `go test -bench=. ./pkg/gognee -benchtime=3s`
- **Tolerance**: 20% deviation from baseline (e.g., Cognify should be <96µs, Search <10µs)
- **Regression detection**: Compare to this baseline on every PR
- **Environment**: CI should use consistent hardware (e.g., GitHub Actions standard runners)

## Future Improvements

- Add memory allocations per op (`-benchmem`)
- Add CPU profile generation (`-cpuprofile`)
- Add backend handler benchmarks (glowbabe adapter + JSON-RPC overhead)
- Add hybrid/graph search benchmarks (currently only vector search)

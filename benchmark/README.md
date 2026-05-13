# assetmin Benchmark Suite

Performance benchmarks for the compile-and-invoke SSR asset extraction mechanism.

## Overview

These benchmarks measure the performance of the new typed CSS migration mechanism that replaces AST-based extraction with compile-and-invoke (`go run`).

**Quick Links:**
- [Performance Comparison](PERFORMANCE_COMPARISON.md) — Detailed AST vs Compile-and-Invoke comparison
- [Benchmark Results](results/) — Previous benchmark run results 

**Key Metrics:**
- **Cold extraction**: First-time compilation of all modules (~300-500ms expected)
- **Warm extraction**: Cached results when modules haven't changed (~5-10ms expected)  
- **Cache effectiveness**: Hash-based caching prevents re-compilation

## Running Benchmarks

### Run all benchmarks
```bash
go test -bench=. -benchmem -benchtime=10s ./benchmark
```

### Run specific benchmark
```bash
go test -bench=BenchmarkExtractSSRAssets_SingleModule -benchtime=10s ./benchmark
```

### Run with detailed timing
```bash
go test -bench=. -benchmem -benchtime=30s -v ./benchmark
```

## Benchmarks Included

### 1. **BenchmarkExtractSSRAssets_SingleModule**
Measures extraction time for a single component module with simple CSS.

**What it tests:**
- Module discovery
- Combined main.go generation  
- Single `go run` compilation and execution
- JSON parsing

**Expected results:**
- Cold run: ~350-450ms (Go compiler)
- Cached runs: ~5-15ms (cache lookup)

### 2. **BenchmarkExtractSSRAssets_ThreeModules**
Measures extraction with three component modules and tests cache effectiveness.

**What it tests:**
- Discovery of multiple modules
- Combined compilation of all modules in one `main.go`
- Amortized compilation cost (all modules compiled together)
- Cache hits on subsequent calls

**Expected results:**
- First call: ~400-500ms (compilation amortized across 3 modules)
- Cached calls: ~5-15ms

### 3. **BenchmarkExtractSSRAssets_LargeCSS**
Measures extraction with larger CSS code (~500 lines) to test:
- JSON marshaling overhead
- CSS parsing/validation
- Memory allocation patterns

**Expected results:**
- Cold run: ~350-450ms
- Cached: ~5-15ms

## Understanding Results

### Sample Output
```
BenchmarkExtractSSRAssets_SingleModule-8           10      1234567890 ns/op         12345 B/op       123 allocs/op
```

Breaking down the output:
- **10 iterations** completed in the benchmark run
- **1.23 seconds** average per operation (cold compilation + execution)
- **12,345 bytes** allocated per operation
- **123 allocations** per operation

### Interpreting Performance

| Scenario | Expected Time | Notes |
|----------|---|---|
| **Cold extraction** (first run) | 300-500ms | Go compiler cold start + all modules compiled together |
| **Warm extraction** (cached) | 5-15ms | Cache lookup + JSON parsing only |
| **Incremental change** | 300-500ms | Any module change invalidates cache, forces recompile |
| **Many modules** (7+) | 400-600ms | Single `go run` compilation amortizes the cost |
| **Per-module old way** | 300ms × N | Old AST extraction required N compilations |

## Performance Benefits

The compile-and-invoke mechanism offers **significant advantages**:

### vs. Old AST-based extraction
- **Single compilation**: One `go run` for all modules, not one per module
- **7 components**: ~5× faster cold path (1 compile vs 7 compiles)
- **Cache**: Warm cache makes it negligible (~5-10ms)

### How caching works
1. **Hash computation**: MD5 hash of all `.go` files in all modules
2. **Cache key**: Hash set of modules
3. **Invalidation**: If ANY `.go` file changes, cache invalidates
4. **Hit rate**: Typical development sees 95%+ cache hits after first build

## Example: Production Scenario

A typical web app with **7 components**:

```
Old AST approach:
  button:  300ms (AST parse)
  card:    300ms (AST parse)
  form:    300ms (AST parse)
  dialog:  300ms (AST parse)
  search:  300ms (AST parse)
  nav:     300ms (AST parse)
  footer:  300ms (AST parse)
  ─────────────────────
  Total:  2,100ms per build

New compile-and-invoke:
  First run:  450ms  (all 7 modules compiled once)
  Subsequent: 5-10ms (cache hit, just JSON lookup)
  ─────────────────────
  Typical: ~450ms first build, then ~5-10ms incrementally
```

## Development Tips

### Profiling
To profile where time is spent:
```bash
go test -bench=. -benchtime=1s -cpuprofile=cpu.prof ./benchmark
go tool pprof cpu.prof
```

### Memory analysis
```bash
go test -bench=. -benchtime=1s -memprofile=mem.prof ./benchmark
go tool pprof mem.prof
```

### Checking cache behavior
The cache is global per-project-root. To see cache effectiveness:
1. Run benchmarks once (cache populates)
2. Run again (should see much faster times on repeated calls)
3. Modify a module file and run again (cache invalidates)

## When to Re-run Benchmarks

- **After optimization changes** to the extraction mechanism
- **After upgrading Go** (compiler performance can change)
- **When adjusting hash computation** or caching strategy
- **Performance regression testing** during development

## CI/CD Integration

To track performance over time, add to CI:

```bash
go test -bench=. -benchmem -benchtime=10s -json ./benchmark > benchmarks.json
# Store benchmarks.json in artifact storage for trend analysis
```

## Future Improvements

Potential optimizations to benchmark:
- [ ] Parallel module compilation (if beneficial)
- [ ] Incremental compilation (cache individual modules)
- [ ] Streaming JSON parsing (reduce memory)
- [ ] Binary cache format (faster than JSON)

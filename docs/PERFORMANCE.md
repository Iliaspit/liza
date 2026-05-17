# Performance Guide

Operational reference for Liza's performance features: lock metrics, state caching, file system watching, and tuning.

## Lock Metrics

Optional performance monitoring for blackboard lock operations. Disabled by default.

### When to Enable

- Diagnosing slow agent performance or command execution
- Detecting lock contention with multiple agents
- Investigating timeout errors
- Profiling system behavior under load

### Interpreting Metrics

| Metric | Healthy | Warning | Problem |
|--------|---------|---------|---------|
| avg_acq | < 10ms | 10-100ms | > 100ms |
| max_acq | < 50ms | 50-500ms | > 500ms |
| avg_hold | < 500ms | 500ms-2s | > 2s |
| max_hold | < 2s | 2-5s | > 5s |

**High acquisition times** (avg_acq > 100ms): Lock contention. Multiple agents/processes competing. Reduce parallelism or operation frequency.

**High hold times** (avg_hold > 2s): Long-running operations under lock. Review code holding lock, consider moving computation outside lock.

**High max with low avg**: Occasional spikes (may be acceptable). Check for periodic batch operations or stuck processes.

### Usage

```go
bb := db.New(".liza/state.yaml")
bb.EnableMetrics()

// ... perform operations ...

recorder := bb.GetMetricsRecorder()
stats := recorder.GetStats()
fmt.Println(stats.String())
// Output: Lock statistics: count=42, avg_acq=5ms, avg_hold=120ms, max_acq=50ms, max_hold=500ms

// Clear between runs
recorder.Clear()

// Disable when done
bb.DisableMetrics()
```

Overhead is negligible (~48 bytes/op, ~1-2us/op) but disable in long-running production.

## State Caching

Liza uses **mtime-based caching** to avoid redundant YAML parsing.

### How It Works

1. First `Read()` parses state.yaml and caches result
2. Subsequent `Read()` checks file mtime first
3. If mtime unchanged, returns cached state (~10-50us vs ~1-5ms)
4. If mtime changed, re-parses and updates cache

**Cache coherence across processes**: Process A modifies -> mtime changes -> Process B reads -> detects change -> re-parses. No stale reads (assuming filesystem semantics).

### Gotchas

- **mtime granularity**: 1 second on ext3/FAT32. Rapid modifications within same second may not invalidate cache. Liza uses atomic rename, which updates mtime immediately.
- **External modifications**: Editors with auto-save, Dropbox/iCloud syncing `.liza/` will thrash the cache.
- **Cache miss on every read-after-write**: If every `Read()` follows a `Modify()`, caching doesn't help.

## File System Watching

Event-driven state change notification using `fsnotify`.

### Polling vs Watching

| | Polling | Watching |
|---|---------|---------|
| Latency | 0-30s (15s mean) | 0-50ms |
| CPU idle | Wakes every 30s | Zero |
| Response | 30-600x slower | Immediate |

All agents use a hybrid approach: event-driven primary, 30s polling fallback.

### Implementation Details

**Debouncing**: 50ms delay coalesces rapid events. Atomic rename creates multiple fsnotify events (CREATE, WRITE, RENAME) -- debouncing reduces 3-5 events per `Modify()` to 1.

**Directory watching**: Watches the `.liza/` directory, not state.yaml directly. Atomic rename creates a new inode; watching the file directly would miss it.

**Cross-platform**: Linux (inotify), macOS (FSEvents), Windows (ReadDirectoryChangesW), BSD (kqueue). Network filesystems (NFS, SMB) may not support notifications -- polling fallback recommended.

## Performance Tuning

### Configuration Parameters

| Parameter | Default | Trade-off |
|-----------|---------|-----------|
| `heartbeat_interval` | 60s | Lower = faster crash detection, more writes |
| `lease_duration` | 1800s (30min) | Lower = faster crash recovery, more renewals |
| `doer_max_wait` | 18000s (5hr) | Lower = agents exit faster when idle |
| Lock timeout | 10s (code) | Lower = fail fast, may false-positive on slow systems |

### Tuning Profiles

**Short tasks** (<10 min): `heartbeat: 30, lease: 900, max_wait: 600`

**Long tasks** (30min-5hr): `heartbeat: 60, lease: 3600, max_wait: 18000`

**Network filesystems**: `heartbeat: 90, lease: 2700` (and increase lock timeout in code)

### Lock Hold Time Optimization

**Bad** -- expensive computation under lock:
```go
bb.Modify(func(s *models.State) error {
    result := computeExpensiveMetrics(s)
    s.Sprint.Metrics = result
    return nil
})
```

**Good** -- compute outside, update under lock:
```go
state, _ := bb.Read()
result := computeExpensiveMetrics(state)
bb.Modify(func(s *models.State) error {
    s.Sprint.Metrics = result
    return nil
})
```

**Batch operations**: Multiple `bb.Modify()` calls in a loop should be a single `bb.Modify()` with the loop inside.

### Parallel Agent Scaling

**Optimal**: 1 planner, 1-3 coders, 1-2 reviewers.

**Contention indicators**: avg lock acquisition > 100ms, agents waiting frequently, high CPU on filesystem ops. **Solution**: reduce agents rather than adding more.

## Benchmarking

### Commands

```bash
go test -bench=. ./internal/db/                          # All benchmarks
go test -bench=BenchmarkRead ./internal/db/               # Specific benchmark
go test -bench=. -benchmem ./internal/db/                 # With memory stats
go test -bench=. -benchtime=10s -count=5 ./internal/db/   # Multiple runs
```

### Targets

| Operation | Target | Acceptable | Poor |
|-----------|--------|------------|------|
| Read (cached) | < 100us | < 500us | > 1ms |
| Read (uncached) | < 5ms | < 20ms | > 50ms |
| Modify (small) | < 10ms | < 50ms | > 100ms |
| Modify (large) | < 50ms | < 200ms | > 500ms |
| Lock acquire | < 10ms | < 100ms | > 500ms |

### Regression Detection

```bash
go test -bench=. ./internal/db/ > baseline.txt
# ... make changes ...
go test -bench=. ./internal/db/ > current.txt
benchstat baseline.txt current.txt
```

Acceptable variance: +/-10% run-to-run. Investigate if >20% slower.

# 61 - Flock-Only Lock Authority

## Context and Problem Statement

Liza state and log locks could appear stale based on PID metadata while the kernel lock was still unavailable. Operators saw lock errors involving dead-looking PIDs, sandbox-translated PIDs, and lock files with no obvious `lsof` holder. Removing lock files sometimes helped, but it could also race a live command such as `liza validate` that recreated or held the lock.

PID metadata was not a reliable ownership authority in the environments Liza runs in. Sandboxes and PID namespaces can make PID files misleading. A lock file can also exist while a live operation is legitimately active.

## Considered Options

1. **Continue PID-based stale lock recovery** - use lock PID files to decide whether a lock is stale.
2. **Make PID namespace-aware cleanup smarter** - attempt to interpret PID metadata across sandbox boundaries.
3. **Use kernel `flock` as the only authority** - keep owner metadata for diagnostics, but never decide acquisition from PID files.

## Decision Outcome

Chose **Option 3**: kernel `flock` acquisition is the only authority for lock ownership.

### Architecture

- `internal/filelock` no longer uses legacy PID metadata to decide whether lock acquisition should proceed.
- Owner metadata remains best-effort diagnostic information.
- If the kernel lock remains unavailable, Liza reports timeout/contention.
- Troubleshooting guidance no longer treats PID-file deletion as normal stale-lock recovery.

### Rationale

The kernel lock is the source of truth. PID metadata is useful for humans, but it is not reliable enough to drive mutation safety. No PID namespace-aware cleanup alternative was pursued; making metadata interpretation smarter would still leave correctness dependent on non-authoritative evidence.

### Consequences

**Positive:**
- Avoids deleting or bypassing locks based on misleading PID files.
- Makes lock semantics match the operating system primitive actually protecting state.
- Reduces unsafe manual cleanup guidance.

**Limitations accepted:**
- Operators lose an automatic stale-PID cleanup path.
- Some lock timeouts that previously looked "stale" are now treated as real contention until the kernel lock is available.
- Diagnostics can identify likely owners but cannot override the lock decision.

**Extends:** ADR-0022 (Concurrency Hardening) - lock ownership now follows the same principle as CAS merges: state safety is decided by authoritative primitives, not inferred metadata.

---
*Reconstructed from commit a05eb261 and liza-run-issues.md (2026-05-18)*

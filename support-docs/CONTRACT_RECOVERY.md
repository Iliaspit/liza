# Contract Recovery Protocols

Read this document when entering RESET, detecting source conflict, reaching the
three-failure tool threshold, or encountering a partial multi-file edit. CORE
defines the stop triggers and authority; this document supplies the response.

## RESET Protocol

Before returning to IDLE, report the interrupted task, state, touched files,
rule broken, cause, and why it was missed. Give the permitted options with
rationale (Tier 0 excludes Resume). Pairing awaits the human; Multi-Agent logs
to the blackboard, sets BLOCKED, and awaits the supervisor.

## Source Contradiction Protocol

When sources conflict (specs vs code, tests vs type hints):
```
⚠️ SOURCE CONFLICT
[Source 1] says: [X] at [location]
[Source 2] says: [Y] at [location]
Options: (1) Proceed with Source 1 — [rationale] (2) Proceed with Source 2 — [rationale] (3) Flag for resolution
```
Never silently choose when sources conflict.

## Tool Failure Protocol

After 3 consecutive failures on same operation:
```
⚠️ TOOL RELIABILITY ISSUE
Operation: [what] | Failures: [count] | Pattern: [summary]
Options: (R)etry differently, (S)kip with implications, (P)ause
```

## Batch Rollback

If multi-file change fails partway:
```
⚠️ BATCH PARTIAL FAILURE
Completed: [files] ✅ | Failed: [file] ❌ | Not attempted: [files]
Options: (R)ollback, (F)ix and continue, (P)ause
```
Never leave repository in inconsistent partial-change state without acknowledgment.

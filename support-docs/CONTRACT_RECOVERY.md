# Contract Recovery Protocols

Read this document when entering RESET, detecting source conflict, reaching the
three-failure tool threshold, encountering a partial multi-file edit, or
recovering from context pressure/reset, a plan-to-execution transition, or
drift. CORE defines triggers and authority; this document supplies response.

## Context Recovery and Continuity

On context reset, plan-to-execution transition, or first degraded recall,
enter Working Set before acting. Re-read CORE Tier 0–1 and state machine,
current task intent/validation, GUARDRAILS.md if present, the selected mode
annex's re-read list, and the active skill SKILL.md. On first degradation,
announce `"⚠️ WORKING SET — Context pressure. Re-reading mode essentials.
Tier 2-3 best-effort."` If insufficient, enter Kernel: Pairing asks
`"Context severely degraded. (C)heckpoint, (R)eset fresh?"`; Multi-Agent
checkpoints to blackboard and self-terminates for supervisor restart.

At state transitions or after extended time, Pairing asks
`"Drift check: Still on [task]? Key constraint: [X]. (Confirm or correct)"`;
Multi-Agent re-reads the blackboard task and verifies checkpoint alignment.
Use `specs/`, `docs/`, and `lessons/` as durable memory: read current state,
perform one atomic task, write updated state. Identify affected docs before
changes. Subagents return partial results on context pressure instead of
using Working Set or Kernel.

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

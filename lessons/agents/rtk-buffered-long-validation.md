---
title: "RTK-buffered long foreground validation"
trigger: "When a long RTK-wrapped foreground validation returns a session ID but no incremental output"
keywords: [RTK, make test, write_stdin, session_id, buffered output]
date: 2026-08-21
---

## Context

Repository-wide validation such as `rtk make test` can run for several minutes.
RTK may buffer successful package output while the execution tool exposes the
still-running process through a session ID.

## Failure Mode

Repeated empty foreground waits can look like a stalled command even while tests
are progressing. Interrupting the session solely because no output was emitted
turns a healthy validation run into an incomplete one and discards its final exit
status.

## Solution

Treat waits on one execution session as observation of the original foreground
command, not as validation retries. Continue waiting within the tool's documented
foreground mechanism, provide periodic status updates when required, and interrupt
only when there is independent evidence of a stall or a configured timeout expires.
Do not launch a duplicate validation command to obtain visible output.

## References

- [Project guardrails](../../GUARDRAILS.md)
- [Worktree build prerequisites](worktree-build-prerequisites.md)

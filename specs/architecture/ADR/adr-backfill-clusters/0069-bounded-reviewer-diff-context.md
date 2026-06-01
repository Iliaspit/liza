# Cluster 0069 - Bounded Reviewer Diff Context

## Commit Set
- `aa598803` - fix(prompts): bound reviewer diff reads

## Intent Hypothesis
Prevent review prompts from causing unbounded diff reads under context pressure.

## Architectural Signals
- Reviewer instructions changed from broad diff reads to name-only/stat-first inspection
- Code-review skill updated to match the bounded protocol
- Role spec updated with reviewer context-handling behavior
- Prompt rendering tests enforce targeted commands

## Classification
Skipped by user selection. Keep this as a prompt protocol refinement under existing context-management and reviewer-prompt decisions rather than generating a standalone ADR.

## Candidate Decision Date
2026-05-25

## Status
Skipped.

## Confidence
0.70 (medium)

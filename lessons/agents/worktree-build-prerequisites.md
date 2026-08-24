---
title: "Sync embedded assets before Go builds in Liza worktrees"
trigger: "When running `go build` or `go test` in a Liza worktree"
keywords: [go build, go test, make sync-embedded, internal/embedded, go:embed, worktree]
date: 2026-04-12
---

## Context

Liza embeds contracts and skills from `internal/embedded/` at build time. In a worktree, those embedded copies can lag behind the repo masters after branch switches or edits.

## Failure Mode

Bare `go build` can fail in `internal/embedded` when embedded assets are stale. Agents then debug the compile error instead of refreshing the generated copies.

For tests, the stronger repo rule still applies: use the Make targets rather than bare `go test ./...`. The stale-embedded failure mode is the reason that rule exists.

## Solution

For routine full-suite validation, run:

`make test`

This target intentionally omits race and coverage instrumentation. During final
pre-commit/merge validation, every change set MUST also pass once with:

`make test-race`

`make test-fast` is available for short-mode iteration, and `make coverage`
produces the package-local coverage report. CI wiring for `make test-race` and
`make coverage` is currently deferred; the existing workflow's `make test` step
does not replace the mandatory final race run.

If a worktree build or test failure points to stale embedded assets, sync them from the worktree root:

`make sync-embedded`

If the shell is not at the worktree root, run:

`make -C <worktree-root> sync-embedded`

## References

- [REPOSITORY.md](../../REPOSITORY.md)
- [internal/embedded/consistency_test.go](../../internal/embedded/consistency_test.go)
- [specs/architecture/ADR/0031-configurable-post-worktree-command.md](../../specs/architecture/ADR/0031-configurable-post-worktree-command.md)

# Git and Exploratory Operations Protocol

Read before Git state changes, selective commits, or temporary repository-state
experiments. CORE's approval and destructive-operation gates still apply.

## Git Protocol

**File State Clarity:** "Pending changes" = working tree + index. When referencing files, specify version read (pending/HEAD/index) if ambiguous.

**Read-Only Operations (always permitted):**
- `git status`, `git diff`, `git log`, `git show`, `git branch` (list), `git blame`, `git ls-files`, `git grep`

**State-Modifying Operations (require approval/checkpoint):**
- `git commit`, `git push`, `git merge`, `git rebase`, `git reset`, `git checkout` (branch switch)

**Requires Checkpoint (noting HEAD movement):**
- `git bisect` — state known-good SHA, test command, and that HEAD will move
- `git stash` — state reason and confirm stash list before/after

**Before Operations:** State current branch, flag uncommitted changes.

**Commit Message Standard (all `git commit` operations):**
- MUST follow Conventional Commits: `type(scope): short summary` (scope optional; `!` for breaking change)
- MUST include a body with both why and what of the change
- **Breaking changes:** `!` after type/scope AND `BREAKING CHANGE:` footer stating what breaks and migration path

**Selective Commits (committing specific files while preserving other changes):**
Leave untracked files untouched and restore the original staged state afterward:

1. `git stash`
2. `git checkout stash -- <files-to-commit>`
3. `git add <files> && git commit`
4. `git stash pop --index`

**NEVER** use `git commit -- <pathspec>` with other uncommitted changes — it can discard them.

**Renames/Moves:** Always use `git mv`, never `mv`. Plain `mv` breaks history tracking.

**Merge Conflicts:** Never auto-resolve. Present conflict, require explicit resolution approval.

**Unrelated Working Tree Changes:** Changes outside current task scope are not owned by the agent. Surface: `"⚠️ Unrelated change detected in [file]"`, do NOT revert/stash/modify, await direction. Reverting unowned files has same approval requirements as `git reset --hard`.

## Exploratory Operations Protocol

Operations that temporarily modify repo state must restore it exactly.

1. **Snapshot:** `git status --short`, `git branch --show-current`, `git stash list`
2. **Scope minimally:** prefer `git show <commit>:<file>` over checkout
3. **Restore** before reporting results; verify snapshot matches
4. **Interruption:** next action MUST be restoration before any other work

**Invariant:** Repo state after = state before. Violation is Tier 2.

## Pull Requests

PR title MUST follow Conventional Commits.

For non-trivial changes, synthesize the body from task context, specs/issues,
existing behavior, diff, and validation evidence. Prefer these sections when
relevant: Summary, Problem / Why, Existing Context, Approach, Change Map,
Reviewer Focus, Validation, Risks / Rollback, and Not in Scope. Reference
specs/issues in Summary or Problem when present. For trivial changes, use a
compact body, but still include why and validation.

## GitHub

Codex: DO NOT use `codex_apps.github`.

Use `gh` (GitHub CLI) for GitHub issues, PRs, releases, and GitHub API queries
when repository context and authentication are available. Prefer `gh` over raw
`curl` calls to GitHub APIs.

For any GitHub write that sends Markdown body text (issue/PR descriptions,
comments, reviews, releases), write the intended Markdown to a temp file and
pass that file to `gh` with `--body-file` or through a JSON payload file. Do
not stream the body through stdin. After writing, read the body back with
`gh api` and verify one or more unique exact phrases from the intended
Markdown before claiming success.

DO NOT use `gh pr edit --body-file -` or generate API JSON through stdout
redirected from `rtk jq`; both patterns can produce empty or truncated bodies
while reporting success.

DO NOT probe `gh pr edit` syntax by appending `--help` to a partially formed
edit command. Run `gh pr edit --help` as a standalone command before
constructing the state-changing command.

---
name: bash-policy
description: Generate and curate Claude-oriented bash-policy project rules. Use to run bash-policy export/report, review .bash-policy-candidates.yaml from Claude settings, update .bash-policy.yaml, normalize command-shape identities, or validate bash-policy configuration.
---

# Bash Policy

## Core Workflow

1. Find the repo root and inspect current policy state.
   - Use `git status --short -- .bash-policy.yaml .bash-policy-candidates.yaml` before edits.
   - Treat `.bash-policy.yaml` as curated source of truth; `.bash-policy-candidates.yaml` is generated evidence.
   - Keep `policy-artifact-root` distinct from `safe-root`: the artifact root is the durable project root where `.bash-policy.yaml`, dry-run logs, and candidates live; the safe root is the active checkout/worktree used for path validation.
   - For interactive curation, use an absolute `--policy-artifact-root`. Do not rely on hook cwd or an auxiliary worktree root.

2. Generate or refresh Claude candidates.
   - Bash-policy curation is mostly for Claude permission telemetry.
   - Use this exact export shape by default:

     ```bash
     bash-policy export --provider claude --policy-artifact-root "$repo_root" --claude-settings .claude/settings.json
     ```

   - If `.claude/settings.json` is missing, ask or locate the intended Claude settings file before guessing.
   - `bash-policy export` regenerates `.bash-policy-candidates.yaml` from current sources; previous candidate-file contents are not policy state.
   - Export sources are aggregated dry-run evidence and optional Claude settings. Claude settings contribute `Bash(...)` permission-family candidates; dry-run evidence contributes command-shape candidates.
   - Use `bash-policy report --provider claude --policy-artifact-root "$repo_root"` when frequency, proposed status, or examples from `.bash-policy-dry-run.jsonl` would improve curation.
   - Generated evidence artifacts are not intended policy deliverables: `.bash-policy-candidates.yaml`, `.bash-policy-dry-run.jsonl`, `.bash-policy-dry-run.jsonl.lock`, and `.bash-policy-dry-run.jsonl.lock.owner.json` should be ignored or worktree-excluded.

3. Curate command-shape candidates.
   - Add runtime `command-shape` rules only for safe, read-only inspection commands.
   - A command-shape rule may use `decision: allow`, `decision: deny`, or `decision: manual`.
   - Configured command-shape rules override built-in defaults, but never override the non-overridable safety floor: secret exposure, credential paths, safe-root escape, verified bypasses, unsupported redirections, dynamic shell forms, and destructive irreversible operations must not become auto-allowed.
   - Prefer a built-in profile over a custom rule when the current evaluator already allows a representative command. Export should omit built-in-covered shapes; if a candidate appears anyway, behavior-check before adding a redundant rule.
   - Prefer normalized placeholders:
     - `<safe-path>` for a single normalized safe path.
     - `<safe-path>...` for one or more final path operands.
     - `<pattern>` for non-sensitive search patterns.
     - `<number>` and `<fields>` for numeric and field-list values.
     - `<safe-glob>` for constrained non-sensitive filename globs such as `*.go` in supported option-value positions.
     - `<safe-pathspec>` when the engine emits pathspec-shaped operands.
   - Generalize command shapes whenever the broader rule is still safe. Replace specific single-path or literal-value rules with placeholder or terminal-variadic forms, then remove narrower rules covered by the general rule.
   - Do not copy highly specific literals such as project regexes, command names, timestamps, local absolute paths, or generated line ranges when a placeholder rule captures the intent.
   - Do not keep literal glob tokens such as `*.go` or `*.py` in command-shape identities. For git pathspec operands, including `git grep ... -- *.go` and `git diff ... -- *.go`, expect current bash-policy to normalize safe extension globs to `<safe-path>` and cover repeated operands with `<safe-path>...`.
   - For grep filename filters, use `--include=<safe-glob>` only when current validation/evaluation supports it. Do not rewrite include globs as `--include=<pattern>`; if `<safe-glob>` is unsupported in the installed binary, omit the rule and leave that command manual.
   - For sed read-range forms, prefer normalized rules such as `sed -n <number>,<number>p <safe-path>...` over literal line ranges and filenames when current evaluation emits that shape. Do not add sed rules for `-e`, `--expression`, `-f`, `--file`, `-i`, or `--in-place` forms; those can execute commands, read scripts, or write files and must remain manual.
   - Do not emit display summaries, transcript fragments, or truncation markers such as standalone `...` as policy identities. Export should canonicalize full legacy summaries when possible and omit unreconstructable truncated identities.
   - Curate compound command evidence as leaf command-shape rules. Do not add whole-compound rules for `;`, `&&`, `||`, or `|`; every leaf must pass the safety floor and resolve through a built-in, configured command-shape rule, or admissible Claude setting.
   - `rtk` is a wrapper. Candidate and policy identities should use the wrapped command shape when recoverable, not an `rtk`-prefixed rule.
   - Never add rules containing `<redacted>` or secret-looking values.
   - Keep state-changing commands manual unless the user explicitly approves a narrow rule. Be skeptical of candidates like `python3 <safe-path> ...`, `git add`, `git commit`, package managers, broad shell wrappers, and tool invocations that mutate coordination state.

4. Curate permission-family candidates.
   - In `.bash-policy.yaml`, `permission-family` entries are export bookkeeping only:

     ```yaml
     - kind: permission-family
       identity: Bash(grep:*)
       status: resolved
     ```

   - They suppress already-reviewed Claude `Bash(...)` families in future candidate exports; they do not authorize runtime commands by themselves.
   - Distinguish this from Claude settings: user-global or repo-local `permissions.deny` entries can act as runtime deny inputs, and admissible `permissions.allow` entries can act as runtime allow inputs after the safety floor and concrete command-shape checks pass. Repo-local settings prevail over exact normalized conflicts with user-global settings.
   - `Bash(...:*)` identities legitimately contain `*`; do not treat that as a glob command-shape problem.
   - Narrow command-shape rules do not resolve a broad permission-family candidate. For example, `Bash(gh:*)` remains unresolved until `.bash-policy.yaml` records `permission-family` `Bash(gh:*)` as `resolved`.
   - Read-only permission families already covered by built-in profiles should be omitted from unresolved candidates; unresolved non-built-in families should be resolved only after review, not promoted wholesale.
   - Recoverable `rtk`-wrapped Claude permission families should normalize to the inner family before comparing built-in coverage or adding a resolution.

5. Validate and behavior-check.
   - Always run:

     ```bash
     bash-policy validate --policy-artifact-root "$repo_root"
     ```

   - Check for accidental glob command-shapes:

     ```bash
     rg -n 'identity: .*\*\.' .bash-policy.yaml
     ```

     No runtime command-shape identity should match. Permission-family `Bash(...:*)` entries are fine.

   - For non-trivial normalization, evaluate representative commands:

     ```bash
     printf '%s' '{"command":"git grep -n foo -- internal/file.go"}' |
       bash-policy evaluate --provider claude --mode on \
       --policy-artifact-root "$repo_root" --safe-root "$repo_root"
     ```

   - Confirm expected `allow`/`manual` behavior from actual output, not from the rule text alone.
   - Re-run export after curation when useful; accepted command-shape rules and resolved permission families should disappear from `.bash-policy-candidates.yaml`.
   - If activation or init is part of the task, preserve existing `on`, `dry-run`, or `off` activation and ensure generated provider hook commands pass explicit `--policy-artifact-root` and `--safe-root` values.

6. Report clearly.
   - State how candidates were generated, which sources were merged, what was intentionally omitted, and validation results.
   - Mention whether `.bash-policy.yaml` is staged, unstaged, or both, without changing staging unless requested.

## CLI Reference

- `bash-policy export --provider claude --policy-artifact-root "$repo_root" --claude-settings .claude/settings.json`
  Regenerate `.bash-policy-candidates.yaml` from dry-run evidence and Claude settings. Use this as the default candidate-generation command for Claude-oriented curation.

- `bash-policy report --provider claude --policy-artifact-root "$repo_root"`
  Summarize `.bash-policy-dry-run.jsonl` with counts, proposed statuses, and examples. Use before curation when frequency or examples matter.

- `bash-policy validate --policy-artifact-root "$repo_root"`
  Validate `.bash-policy.yaml` schema and rule identities. Run after every edit.

- `bash-policy evaluate --provider claude --mode on --policy-artifact-root "$repo_root" --safe-root "$repo_root"`
  Evaluate one hook payload from stdin. Use with `printf '%s' '{"command":"..."}' | ...` for behavior checks.

- `bash-policy init --provider claude --policy-artifact-root "$repo_root"`
  Install or repair Claude hook configuration. Preserve existing activation state; do not run during policy curation unless hook setup is explicitly in scope.

- `bash-policy activation on|dry-run|off --provider claude --policy-artifact-root "$repo_root"`
  Change Claude hook activation. `dry-run` logs redacted telemetry without changing provider behavior; `on` may emit provider decisions. Do not change activation unless explicitly requested.

- `bash-policy codex-readiness [--json]`
  Check Codex readiness when Codex integration is in scope. For this skill, Codex is secondary; Claude candidate export is the default path.

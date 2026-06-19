---
name: gandalf-review
description: Run an OmniAgentBot-style local adversarial QA loop after implementation. Use when the user asks for gandalf review, guardian/gatekeeper review loops, adversarial review until approval, automatic fix-and-review cycling, or local replacement for repeatedly pushing PR fixes and asking OmniAgentBot/GitHub reviewers again.
---

# Gandalf Review

Run review -> fix -> re-review until both reviewers approve, while archiving each run and producing aggregate metrics.

This skill is a local QA loop. It does not replace the active contract: approval gates, safety stops, credential rules, destructive-operation rules, and validation requirements still apply.

# Metrics Archive

Use the helper for every run:

```bash
python3 skills/gandalf-review/scripts/gandalf_metrics.py --root ~/.liza/gandalf-review <command>
```

Archive layout:

- `~/.liza/gandalf-review/runs/<run-id>/metrics.jsonl` — event stream for one run.
- `~/.liza/gandalf-review/runs/<run-id>/summary.md` — human recap for one run.
- `~/.liza/gandalf-review/runs/<run-id>/artifacts/` — review and fix artifacts by iteration.
- `~/.liza/gandalf-review/index.jsonl` — global aggregate, one JSON object per run.
- `~/.liza/gandalf-review/aggregate.md` — human aggregate summary.

If `GANDALF_REVIEW_EXPORT_CMD` is set, the helper runs it after each event with these environment variables:

- `GANDALF_REVIEW_EVENT_PATH`
- `GANDALF_REVIEW_INDEX_PATH`
- `GANDALF_REVIEW_AGGREGATE_PATH`
- `GANDALF_REVIEW_RUN_SUMMARY_PATH`
- `GANDALF_REVIEW_RUN_DIR`
- `GANDALF_REVIEW_RUN_ID`

Keep this export generic. Do not add Slack-specific behavior here; a Slack forwarder can consume the paths above.

Do not archive secrets. If a finding contains sensitive data, redact before recording the artifact.

# Reviewer Runtime

For small local QA runs, use Codex ACP Fast Mode for the primary reviewer when available. Create a task-local Codex home instead of mutating global `~/.codex/config.toml`:

```bash
python3 skills/gandalf-review/scripts/gandalf_codex_fast_home.py \
  --output-dir <run-dir>/codex-fast-home
```

Use the returned `CODEX_HOME` and `CODEX_MODEL` for the primary Codex ACP review command. The helper writes `model_reasoning_effort = "minimal"` and symlinks auth without reading the auth file.

Use the stronger/default reviewer mode when any of these are true:

- The diff touches auth, security, money, data loss, migrations, or public APIs.
- A prior fast review missed a blocker.
- The user explicitly asks for a deep review.

Keep the secondary adversarial reviewer independent from the primary reviewer. Do not pass the primary review's hidden reasoning; pass only the primary artifact, diff, validation evidence, and unresolved blockers.

# Pre-open PR Hook

Invite users to run Gandalf before opening a pull request. Git has no native "pre-open PR" hook, so use one of these opt-in wrappers:

- A `gh pr create` wrapper that runs Gandalf first and opens the PR only after `APPROVED`.
- A `pre-push` hook when the team accepts local review before pushing.
- A project script such as `scripts/pr-ready` that runs tests, Gandalf, and then prints the `gh pr create` command.

The hook should:

1. Refuse to run on a dirty index unless the user explicitly asks to include uncommitted changes.
2. Start a Gandalf run against the merge base or target branch.
3. Run the review loop until `APPROVED` or `BLOCKED`.
4. Print the run summary path and aggregate path.
5. Open or suggest the PR only after approval.

Do not hide the archive. The hook may suppress loop noise, but it must leave the full run in `~/.liza/gandalf-review/`.

# Git Commit Hygiene

When operating inside a Git repository, create one fix commit per iteration so each repair step is recoverable and reviewable:

```bash
git add <changed-files>
git commit -m "fix(gandalf): iteration <n> repair"
```

Record the created commit on the fix event:

```bash
python3 skills/gandalf-review/scripts/gandalf_metrics.py record \
  --run-id <run-id> \
  --kind fix_finished \
  --iteration <n> \
  --duration-kind fix \
  --duration-ms <milliseconds> \
  --commit <fix-commit-sha> \
  --summary "<what changed and how it was validated>"
```

After both reviewers approve, squash the iteration commits into one clean final commit for the branch:

```bash
python3 skills/gandalf-review/scripts/gandalf_squash.py \
  --base-ref <base-ref> \
  --message "<final conventional commit message>"
```

Preserve the metrics archive exactly; do not rewrite `metrics.jsonl`, review artifacts, or summaries to pretend there was only one iteration.

If the worktree contains unrelated user changes, do not squash or stage them. Stop and ask for scope clarification.

# Start

1. Establish review scope from the current task, branch, staged diff, pending diff, or PR URL.
2. Record the run:

```bash
python3 skills/gandalf-review/scripts/gandalf_metrics.py start \
  --repo <repo-name-or-url> \
  --branch <current-branch> \
  --base-ref <base-ref> \
  --goal "<one-line QA goal>"
```

Capture the returned `run_id`.

3. Set `iteration = 1`.

# Default Budgets

Initial defaults are calibrated from recent `omni3ai/omni` OmniAgentBot PR review comments sampled on 2026-06-19: 8 PRs, 22 bot responses, median response latency about 4 minutes, p75 about 5 minutes, max observed about 15 minutes, and max observed paired rounds 7.

- Minimum review rounds: 1 full primary + adversarial round. Do not run extra rounds after both approve.
- Soft convergence checkpoint: after 3 iterations, re-review only unresolved blocker questions and new P0-P2 regressions.
- Hard iteration cap: 7 iterations.
- Slow review warning: record a warning when one review pass exceeds 10 minutes.
- Review timeout/default blocker threshold: 25 minutes without a completed review pass.
- Fix timing has no fixed cap; record actual duration and block only when the same finding repeats without progress.

# Review Progress Bars

Use `scripts/gandalf_progress.py` when a review command is expected to take long enough that terminal feedback helps. The bar advances based on the normal review time and caps at 99% until the command exits; completion writes 100%.

Example for a primary review with a 4-minute expected duration:

```bash
python3 skills/gandalf-review/scripts/gandalf_progress.py \
  --label "primary review" \
  --expected-ms 240000 \
  --stdout-file <primary-review.md> \
  --stderr-file <primary-review.err> \
  -- <review-command> <args>
```

Use separate bars and expected durations for the primary and adversarial reviewers. Do not use progress bars for fix implementation; fixes vary by defect and should be reported with regular status messages and measured after completion.

# Review Loop

For each iteration:

1. Run the primary review.
   - Use the code-review skill when reviewing code changes.
   - Prioritize P0-P2: security, correctness, data integrity, missing behavior tests.
   - Output:

```markdown
**Verdict:** Approved | Changes requested

**Findings**
- Blocking findings, or "No blocking findings."

**Validation**
- What was checked.

**Residual risk**
- Remaining risk or "Low."
```

2. Record the primary review artifact:

```bash
python3 skills/gandalf-review/scripts/gandalf_metrics.py record \
  --run-id <run-id> \
  --kind primary_review_finished \
  --iteration <n> \
  --reviewer primary \
  --verdict "<Approved|Changes requested>" \
  --duration-kind review \
  --duration-ms <milliseconds> \
  --summary "<short finding summary>" \
  --content-file <primary-review.md> \
  --artifact-name primary-review.md
```

3. Run the adversarial review.
   - Challenge the primary review, not just the code.
   - Look for missed high-impact issues, unsupported findings, missing validation, and scope drift.
   - Output:

```markdown
**Adversarial verdict:** Approved | Changes requested

**Challenge**
- What the primary pass missed, overstated, or got right.

**Action**
- The next blocking action, or "No action required."
```

4. Record the adversarial artifact with `--kind adversarial_review_finished --reviewer adversarial`.

5. If both verdicts approve, run final validation, record validation duration, finish with `APPROVED`, and stop.

6. If either verdict requests changes:
   - Extract only blocking findings and material concerns.
   - Do not send nits, style preferences, or already-fixed findings into the fix pass.
   - Apply the smallest fix that addresses root cause.
   - Update or add tests when behavior changed.
   - Validate the changed path with realistic inputs.
   - Commit the fix before the next review pass when working in Git.
   - Record a fix artifact:

```bash
python3 skills/gandalf-review/scripts/gandalf_metrics.py record \
  --run-id <run-id> \
  --kind fix_finished \
  --iteration <n> \
  --duration-kind fix \
  --duration-ms <milliseconds> \
  --commit <fix-commit-sha> \
  --summary "<what changed and how it was validated>" \
  --content-file <fix-summary.md> \
  --artifact-name fix-summary.md
```

7. Increment `iteration` and repeat with a narrowed re-review:
   - Round 2 checks previous blockers, new fix diff, and new P0-P2 regressions only.
   - Round 3+ requires a specific unresolved question. Do not broaden into a fresh review.

# Stop Conditions

Stop and finish with `BLOCKED` if any condition applies:

- The same finding repeats after two fix attempts without new evidence.
- Fixing one reviewer blocker reintroduces another blocker.
- The loop reaches 7 iterations.
- Validation cannot exercise the changed behavior.
- The review requires credentials, external systems, destructive operations, or user decisions not available locally.

Record the block:

```bash
python3 skills/gandalf-review/scripts/gandalf_metrics.py finish \
  --run-id <run-id> \
  --final-verdict BLOCKED \
  --summary "<short recap>" \
  --blocker "<specific blocker>"
```

# Finish

On approval:

```bash
python3 skills/gandalf-review/scripts/gandalf_metrics.py finish \
  --run-id <run-id> \
  --final-verdict APPROVED \
  --summary "<iterations, main fixes, validation, residual risk>"
```

Rebuild aggregate when needed:

```bash
python3 skills/gandalf-review/scripts/gandalf_metrics.py aggregate
```

Use Iteration Ledger as the default final user recap:

```markdown
# Gandalf Review Ledger

Final verdict: <APPROVED|BLOCKED|STOPPED>
Total elapsed: <duration>
Total review: <duration>
Total fix: <duration>
Total validation: <duration>

| Iteration | Primary | Adversarial | Fix | Commit | Result |
|---:|---|---|---|---|---|
| 1 | Changes requested | Changes requested | <fix summary> | <sha> | Re-review |
| 2 | Approved | Approved | None | n/a | APPROVED |

Validation:
- `<command>` -> <result>

Artifacts:
- Run summary: `<summary.md path>`
- Aggregate: `<aggregate.md path>`
- Index: `<index.jsonl path>`
```

Final user recap must include:

- Final verdict.
- Iteration count.
- Total review time and fix time.
- Key fixes made.
- Validation run.
- Paths to `summary.md`, `aggregate.md`, and `index.jsonl`.

Do not dump every review note unless the user asks. The archive contains the detail.

# Production Lessons

Apply these hardening lessons from dogfooding this skill on itself:

- Treat archive data as untrusted input. A corrupt historical run must not break future `start`, `record`, `finish`, or `aggregate` operations.
- Quarantine corrupt runs as `CORRUPT`; do not count them as approved, blocked, or completed.
- Record wrapper/tool failures separately from reviewer verdicts when a useful verdict artifact still exists.
- Keep reviewer prompts narrow after the first failed round. Round 2+ should verify prior blockers, the fix diff, and new P0-P2 regressions only.
- Preserve detailed iteration history in the archive even when Git history is squashed for a clean PR.

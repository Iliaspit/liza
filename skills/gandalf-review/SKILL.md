---
name: gandalf-review
description: Run a local adversarial QA loop after implementation. Use when the user asks for gandalf review, guardian/gatekeeper review loops, adversarial review until approval, automatic fix-and-review cycling, or local PR-readiness review before asking GitHub reviewers again.
trigger: User asks for Gandalf review, guardian/gatekeeper review loops, adversarial review until approval, automatic fix-and-review cycling, or local PR-readiness review.
---

# Gandalf Review

Run primary reviewer sub-agent -> fix -> adversarial reviewer sub-agent -> re-review until both reviewers approve, while archiving each run and producing aggregate metrics.

This skill is a local QA loop that spawns reviewer sub-agents. The main agent coordinates scope, fixes, validation, metrics, and user-facing decisions. The reviewer sub-agents produce review artifacts only.

This skill does not replace the active contract: approval gates, safety stops, credential rules, destructive-operation rules, and validation requirements still apply.

# Main Agent Instructions

## Metrics Archive

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

`start`, `record`, and `finish` update the current run summary and replace that run's index entry. Use `aggregate` when you need a full rebuild across historical runs or to quarantine corrupt historical data.

If `GANDALF_REVIEW_EXPORT_CMD` is set, the helper runs it after each event and automatically sets these environment variables for that exporter. Do not set them manually:

- `GANDALF_REVIEW_EVENT_PATH`
- `GANDALF_REVIEW_INDEX_PATH`
- `GANDALF_REVIEW_AGGREGATE_PATH`
- `GANDALF_REVIEW_RUN_SUMMARY_PATH`
- `GANDALF_REVIEW_RUN_DIR`
- `GANDALF_REVIEW_RUN_ID`

Keep this export generic. Do not add Slack-specific behavior here; a Slack forwarder can consume the paths above.

Do not archive secrets. If a finding contains sensitive data, redact before recording the artifact.

## Reviewer Runtime

Use a fast local reviewer for the primary pass when the change is low risk, then use an independent adversarial reviewer to challenge that result. Keep provider-specific runtime configuration in provider-specific subsections; the general rule is speed for low-risk first pass, independence for the second pass, and stronger review mode for high-risk changes.

## Codex ACP Fast Mode

For small local QA runs with Codex available, use Codex ACP Fast Mode for the primary reviewer. Create a task-local Codex home instead of mutating global `~/.codex/config.toml`:

```bash
python3 skills/gandalf-review/scripts/gandalf_codex_fast_home.py \
  --output-dir <run-dir>/codex-fast-home
```

Use the returned `CODEX_HOME` and `CODEX_MODEL` for the primary Codex ACP review command. The helper writes `model_reasoning_effort = "minimal"` and symlinks auth without reading the auth file.

Codex `model_reasoning_effort` values are `minimal`, `low`, `medium`, `high`, and `xhigh`; `xhigh` is model-dependent. Use `minimal` only for bounded, low-risk review passes. Escalate to `medium`, `high`, or `xhigh` when the review touches higher-risk code or repeatedly misses blockers.

The generated task-local Codex config intentionally sets `approval_policy = "never"`, `sandbox_mode = "workspace-write"`, and network access for autonomous local review. Keep that posture task-local, archive only redacted artifacts, and do not copy secrets into the run directory.

Use the stronger/default reviewer mode when any of these are true:

- The diff touches auth, security, money, data loss, migrations, or public APIs.
- A prior fast review missed a blocker.
- The user explicitly asks for a deep review.

Keep the secondary adversarial reviewer independent from the primary reviewer. Do not pass the primary review's hidden reasoning; pass only the primary artifact, diff, validation evidence, and unresolved blockers.

## Pre-open PR Gate

Invite users to run Gandalf before opening a pull request. Git has no native "pre-open PR" hook, and this skill must not assume a wrapper script exists. Use one of these paths:

- A direct user prompt such as "Use gandalf-review against this branch before I open the PR."
- An existing project-approved `gh pr create` wrapper that runs Gandalf first and opens the PR only after `APPROVED`.
- An existing `pre-push` hook when the team accepts local review before pushing.
- A new project script only when the user explicitly asks to create one as a separate implementation task.

The gate should:

1. Refuse to run on a dirty index unless the user explicitly asks to include uncommitted changes.
2. Start a Gandalf run against the merge base or target branch.
3. Run the review loop until `APPROVED` or `BLOCKED`.
4. Print the run summary path and aggregate path.
5. Open or suggest the PR only after approval.

Do not hide the archive. A wrapper may suppress loop noise, but it must leave the full run in `~/.liza/gandalf-review/`.

## Git Commit Hygiene

When operating inside a Git repository, follow the repository commit protocol. Get the required approval/checkpoint before commit, reset, or squash operations. Commit subjects must use Conventional Commits, and final commits must include a body explaining why and what changed.

Create one fix commit per iteration so each repair step is recoverable and reviewable:

```bash
git add <changed-files>
git commit \
  -m "fix(gandalf): iteration <n> repair" \
  -m "Why: <why this repair is needed>

What: <what changed and how it was validated>"
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
  --message "<final conventional commit subject>" \
  --body "Why: <why the final change is needed>

What: <what the final change contains>"
```

For longer messages, write the full Conventional Commit message to a file and pass `--message-file <path>`.

Preserve the metrics archive exactly; do not rewrite `metrics.jsonl`, review artifacts, or summaries to pretend there was only one iteration.

If the worktree contains unrelated user changes, do not squash or stage them. Stop and ask for scope clarification.

## Start

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

## Default Budgets

Initial defaults are calibrated from recent local adversarial PR review loops sampled on 2026-06-19: 8 PRs, 22 reviewer responses, median response latency about 4 minutes, p75 about 5 minutes, max observed about 15 minutes, and max observed paired rounds 7.

- Minimum review rounds: 1 full primary + adversarial round. Do not run extra rounds after both approve.
- Soft convergence checkpoint: after 3 iterations, re-review only unresolved blocker questions and new P0-P2 regressions.
- Hard iteration cap: 7 iterations.
- Slow review warning: record a warning when one review pass exceeds 10 minutes.
- Review timeout/default blocker threshold: 25 minutes without a completed review pass.
- Fix timing has no fixed cap; record actual duration and block only when the same finding repeats without progress.

## Review Progress Bars

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

# Reviewer Sub-Agent Instructions

Reviewer sub-agents only review. They do not edit files, commit, resolve GitHub threads, mutate metrics, or make user-facing decisions. The main agent passes them the diff, relevant source context, validation evidence, prior review artifact when applicable, and unresolved blockers.

Primary reviewer sub-agent:

- Use the code-review skill when reviewing code changes.
- Prioritize P0-P2: security, correctness, data integrity, missing behavior tests.
- Return the required primary review artifact.

Adversarial reviewer sub-agent:

- Challenge the primary review, not just the code.
- Look for missed high-impact issues, unsupported findings, missing validation, and scope drift.
- Return the required adversarial review artifact.

# Review Loop

For each iteration:

1. Run the primary review.
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

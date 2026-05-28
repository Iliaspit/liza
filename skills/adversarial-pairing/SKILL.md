---
name: adversarial-pairing
description: Coordinate Pairing-mode doer/reviewer sessions through a Markdown blackboard. Use when the user invokes /adversarial-pairing with role and blackboard-path arguments or asks multiple pairing agents to coordinate plan review, implementation, staged code review, and follow-up review rounds without Liza multi-agent mode.
---

# Invocation

Use as:

```text
/adversarial-pairing <role> <blackboard-path>
```

`role` is `doer` or `reviewer`. `blackboard-path` is the authoritative Markdown blackboard file for this session. It may be untracked and must not be committed unless the user explicitly asks. Do not use sibling or similarly named blackboards as fallbacks when the provided path is missing.

# Operating Model

This skill creates a lightweight Pairing-mode coordination loop. The blackboard is shared coordination state; the human remains the approval authority.

The YAML frontmatter is machine-pollable state. The Markdown body is durable context: goal, evidence, plans, reviewer comments, decisions, validation output, and review rounds.

Poll only the frontmatter every 60 seconds while waiting for work. Read the Markdown body only when the polled state indicates this agent has work to do.

For doers, submitting an artifact for review is not task completion. After moving to `ANALYSIS_SUBMITTED`, `PLANNING_SUBMITTED`, `RED_TEST_SUBMITTED`, `CODE_SUBMITTED`, or `FOLLOWUP_REVIEW`, the doer MUST continue frontmatter-only polling until reviewer verdicts require action or `phase` becomes terminal.

For reviewers, startup registration is not task completion. After registering or confirming an existing agent entry, continue frontmatter-only polling until a reviewable phase appears or `phase` becomes terminal.

Stop polling when `phase` is terminal: `COMMITTED`, `BLOCKED`, or `STOPPED`.

# Blackboard Ownership

- The doer owns phase transitions.
- Each agent owns only its own `agents.<id>` entry.
- Reviewers append review notes to the Markdown body and update only their own agent entry, plus the reviewer-owned claim transitions listed below.
- Reviewer-owned blackboard writes authorized by this skill do not require an additional human approval gate. They still MUST follow the lock protocol and field ownership rules.
- Before any write, re-read the blackboard and verify the revision or round being edited is still current.
- Every blackboard write MUST go through the sidecar OS lock acquired by `skills/adversarial-pairing/scripts/blackboard_write.py`, including agent self-registration, status updates, review verdicts, Markdown body appends, and phase transitions. Do not lock for polling.
- Never overwrite another agent's comments or status.
- If two writes conflict or the file changed unexpectedly, stop and ask the user how to reconcile.

# Frontmatter Contract

The blackboard frontmatter is the coordination contract. It must be valid YAML and must include these fields when a new blackboard is created:

```yaml
---
phase: DRAFT
work_type: feature
rca_required: false
red_test_required: false
required_reviewers: []
plan_revision: 0
analysis_revision: 0
red_test_round: 0
code_review_round: 0
phase_updated_at: "YYYY-MM-DDTHH:MM:SSZ"
worktree: null
agents:
  doer:
    role: doer
    status: DRAFT
    last_seen: null
    reviewed_analysis_revision: null
    analysis_verdict: null
    reviewed_plan_revision: null
    plan_verdict: null
    reviewed_red_test_round: null
    red_test_verdict: null
    reviewed_code_round: null
    code_verdict: null
---
```

Agent IDs should be stable and human-readable. If absent, register yourself by adding an entry for your role after asking the user if identity is ambiguous.

Worktree rules:

- Before entering `CODING`, the doer must create or select a dedicated git worktree and set `worktree` to its absolute path.
- If `worktree` is null before `CODING`, ask the user for the worktree path or approval to create one.
- Do not implement in the main checkout unless the user explicitly approves a no-worktree workflow.
- Once `worktree` is set, run all implementation, staging, validation, and review diff commands from that path.
- Reviewers must run code review diff commands from `worktree`.
- The blackboard may remain outside the worktree and untracked; agents access it by the provided `blackboard-path`.

Reviewer registration:

- If invoked as a reviewer and no agent entry exists for you, choose a stable human-readable ID and self-register under `agents.<id>`. Self-registration is a reviewer-owned blackboard write authorized by this skill; do not ask for additional human approval unless identity is ambiguous. Self-registration MUST follow the lock protocol.
- Do not add yourself directly to `required_reviewers` unless the user explicitly names you as required.
- Before the first reviewable artifact is submitted, the doer snapshots registered reviewer agents into `required_reviewers`.
- Once populated, `required_reviewers` changes only by explicit user instruction.

Allowed values:

- `phase`: one of the phases listed in `Phases`.
- terminal `phase`: `COMMITTED`, `BLOCKED`, or `STOPPED`.
- `work_type`: `feature`, `debugging`, `refactor`, `docs`, `spike`, or another explicit user-defined type.
- `rca_required`, `red_test_required`: booleans.
- `role`: `doer` or `reviewer`.
- `status`: `DRAFT`, `IDLE`, `WAITING`, `WORKING`, `REVIEWING`, `APPROVED`, `CHANGES_REQUESTED`, `BLOCKED`, or `STOPPED`.
- verdict fields: `APPROVED`, `CHANGES_REQUESTED`, `COMMENT`, or `null`.

Field ownership:

- The doer owns `phase`, counters, `work_type`, gate booleans, `required_reviewers`, and `worktree`, except reviewer-owned claim transitions listed below.
- Each agent owns only its own `agents.<id>` status, timestamps, reviewed counters, and verdicts.
- The doer MUST NOT modify reviewer-owned `agents.<id>` entries, including `status`, `last_seen`, reviewed counters, or verdicts. Doer writes must preserve reviewer entries exactly.
- Reviewers write verdict fields only for their own agent entry.
- Reviewers may move `ANALYSIS_SUBMITTED -> REVIEWING_ANALYSIS`, `PLANNING_SUBMITTED -> REVIEWING_PLAN`, `RED_TEST_SUBMITTED -> REVIEWING_RED_TEST`, and `CODE_SUBMITTED -> REVIEWING_CODE` when claiming review work.
- Reviewers may move `CODE_CHANGES_REQUESTED -> FOLLOWUP_REVIEW` only after the doer has updated `code_review_round` and submitted follow-up changes.
- Any agent may set `phase: STOPPED` after a direct user stop or abort instruction.
- Any agent may set its own `status: BLOCKED`; setting global `phase: BLOCKED` requires a concrete workflow blocker in the Markdown body.

Lock protocol:

- Agents MUST use `skills/adversarial-pairing/scripts/blackboard_write.py` for blackboard writes. Prepare the complete intended file content in a temporary file, then run:

```bash
python3 skills/adversarial-pairing/scripts/blackboard_write.py \
  --path <blackboard-path> \
  --content-file <tmp-content-file> \
  --operation <short-operation-name> \
  --expect-sha256 <sha256-read-before-edit>
```

- For creating a missing doer-authorized blackboard, use `--create-if-missing` instead of `--expect-sha256`.
- `--expect-sha256` is the SHA-256 of the uncommitted working-tree file content read before editing; it is not a git commit hash. Use it for every write to an existing blackboard.
- The helper acquires an OS file lock on `<blackboard-path>.lock`, re-reads the blackboard under lock, verifies the expected SHA-256, atomically replaces the Markdown file, and releases the lock. If it reports stale content or lock timeout, re-read and restart the write from current state.
- Do not use Edit/Write tools to manually acquire, release, or modify in-document `lock` or `locked_by` fields. New blackboards do not include in-document lock fields; lock authority is `<blackboard-path>.lock` plus `<blackboard-path>.lock.owner.json` diagnostics.
- Lock all writes and phase transitions through the helper. Do not lock for polling.
- Use UTC ISO-8601 timestamps ending in `Z`.
- If one logical action updates both frontmatter and Markdown body, perform both updates in the same helper write. Do not split a verdict/status update and its explanatory notes across separate writes.
- Within a logical write, update the Markdown body first and the frontmatter status/phase last in the prepared content. A status or phase must never advertise completed/submitted/reviewed work before the corresponding worklog, artifact, or review notes are present in the same committed file content.
- Writing without the helper is a protocol violation. Stop and report the violation if it happens; do not make a second unlocked write to repair it.

Completion predicates:

- RCA approved: every `required_reviewers` entry has `analysis_verdict: APPROVED` and `reviewed_analysis_revision` equal to current `analysis_revision`.
- RCA changes requested: any required reviewer has `analysis_verdict: CHANGES_REQUESTED` for current `analysis_revision`.
- Plan approved: every required reviewer has `plan_verdict: APPROVED` and `reviewed_plan_revision` equal to current `plan_revision`.
- Plan changes requested: any required reviewer has `plan_verdict: CHANGES_REQUESTED` for current `plan_revision`.
- Red test approved: every required reviewer has `red_test_verdict: APPROVED` and `reviewed_red_test_round` equal to current `red_test_round`.
- Red test changes requested: any required reviewer has `red_test_verdict: CHANGES_REQUESTED` for current `red_test_round`.
- Code approved: every required reviewer has `code_verdict: APPROVED` and `reviewed_code_round` equal to current `code_review_round`.
- Code changes requested: any required reviewer has `code_verdict: CHANGES_REQUESTED` for current `code_review_round`.
- Missing or stale required reviewer records mean the review is incomplete; do not advance to an approved or changes-requested phase yet.

# Phases

| Phase | Meaning |
|-------|---------|
| `DRAFT` | Blackboard exists but the doer has not started planning. |
| `ANALYZING` | Doer is performing root cause analysis before planning. |
| `ANALYSIS_SUBMITTED` | RCA is ready for adversarial review. |
| `REVIEWING_ANALYSIS` | Reviewers are checking evidence, falsifiability, and root cause quality. |
| `ANALYSIS_CHANGES_REQUESTED` | RCA needs revision before planning. |
| `ANALYSIS_APPROVED` | RCA is approved; planning may begin. |
| `PLANNING` | Doer is preparing a plan. |
| `PLANNING_SUBMITTED` | Plan revision is ready for reviewer review. |
| `REVIEWING_PLAN` | One or more reviewers are reviewing the submitted plan. |
| `PLAN_CHANGES_REQUESTED` | Reviewer comments require a revised plan. |
| `PLAN_APPROVED` | Reviewers have no blocking plan comments. |
| `RED_TESTING` | Doer is writing a failing test or reproduction for the approved diagnosis. |
| `RED_TEST_SUBMITTED` | Red test has been shown failing for the expected reason and is ready for review. |
| `REVIEWING_RED_TEST` | Reviewers are checking that the test proves the bug, not the implementation. |
| `RED_TEST_CHANGES_REQUESTED` | Red test needs revision before fix implementation. |
| `RED_TEST_APPROVED` | Red test is approved; fix implementation may begin. |
| `CODING` | Doer is implementing the approved plan. |
| `CODE_SUBMITTED` | Doer has staged the candidate changes for review. |
| `REVIEWING_CODE` | Reviewers are reviewing the current staged diff. |
| `CODE_CHANGES_REQUESTED` | Doer is addressing review comments in unstaged changes. |
| `FOLLOWUP_REVIEW` | Reviewers are reviewing the unstaged follow-up diff. |
| `READY_TO_COMMIT` | Reviewers have no remaining blocking comments. |
| `COMMITTED` | User-approved commit is complete; doer has proposed rebase, merge into the base branch, and post-merge worktree deletion. |
| `BLOCKED` | Work cannot proceed without user input. |
| `STOPPED` | User asked to stop or abort the workflow. |

# Transition Rules

The doer submits artifacts for review. Reviewers may mark the global phase as `REVIEWING_*` when claiming work, but both `*_SUBMITTED` and the matching `REVIEWING_*` phase remain reviewable until all required reviewers have recorded current verdicts.

Do not submit a reviewable artifact while `required_reviewers` is empty unless the user explicitly approves a no-review workflow.

The doer resolves requested changes and resubmits. Increment the relevant counter on each submission or resubmission:

- `analysis_revision` for RCA artifacts.
- `plan_revision` for plans.
- `red_test_round` for red tests or reproductions.
- `code_review_round` for the initial staged code submission and each follow-up review submission.

Counter increments, Markdown artifact updates, and phase transitions to `*_SUBMITTED` or `FOLLOWUP_REVIEW` must happen in the same helper write.

For submissions and resubmissions, write the worklog/artifact body content before changing `phase`, counters, or agent status in the same transaction. Other agents may observe the file immediately after the short-lived lock releases, so the released file must never contain a readiness/status signal without its matching body evidence.

Doer submission and resubmission writes MUST NOT reset, normalize, or otherwise edit reviewer-owned agent fields. If the doer accidentally changes reviewer-owned fields, stop and report the ownership violation; do not perform a doer-side repair of those fields unless the user gives an explicit ownership-waiver repair instruction naming the exact fields.

If follow-up review requests changes, move back to `CODE_CHANGES_REQUESTED`; repeat until code is approved or the workflow stops.

`work_type` describes the default workflow shape. `rca_required` and `red_test_required` are independent gates. `work_type: debugging` should normally set both to `true`, but other work types may enable either gate explicitly.

When the user asks to stop or abort the workflow, move `phase` to `STOPPED`. All agents stop polling terminal phases.

# Startup Protocol

On invocation:

- If invoked as the doer and the blackboard file does not exist, create it only when the user asked the doer to create it.
- Create the exact `blackboard-path` provided by the user. Do not search for, read, or continue from sibling blackboards as substitutes for a missing target path.
- New blackboard creation is a write and MUST use `blackboard_write.py --create-if-missing` with the complete initial file content. If the file appears during creation, re-read it instead of overwriting it.
- If invoked as the doer and the blackboard file is missing but the user did not ask for creation, stop and ask whether to create it.
- If invoked as a reviewer and the blackboard file does not exist, fail fast: report that the doer must create the blackboard before reviewers start, then stop instead of creating or polling.

# Doer Protocol

Doer sessions MUST remain active while `phase` is non-terminal. If the doer is waiting for reviewers after submitting an artifact, keep polling frontmatter instead of stopping after submission, reporting `WAITING`, or summarizing review-wait state as completion. Stopping before a terminal phase or direct user stop is a protocol violation.

Mandatory human gates:

1. Before leaving `DRAFT` for `ANALYZING` or `PLANNING`, ask the user for approval to begin from the blackboard contents.
2. Before `ANALYZING -> ANALYSIS_SUBMITTED`, show the RCA and ask the user for approval to submit it for adversarial review.
3. Before `PLANNING -> PLANNING_SUBMITTED`, show the plan and ask the user for approval to submit it for adversarial review.

No human approval is required for `RED_TESTING -> RED_TEST_SUBMITTED`; submitting the red test is part of the approved debugging workflow.

For `work_type: debugging`, use the debugging skill and treat RCA as a distinct artifact before planning when `rca_required: true`.

Do not enter `PLANNING` from `ANALYSIS_SUBMITTED`; enter planning only from `ANALYSIS_APPROVED` unless the user explicitly waives RCA approval.

After all reviewers approve the plan, ask the user before coding unless the previous approval explicitly included permission to proceed after reviewer approval.

When `red_test_required: true`, do not implement the fix until a red test or reproduction has failed for the expected reason and reviewers have approved it. If no practical red test exists, ask the user to approve an alternate validation path.

During coding, follow the normal Pairing-mode approval and validation rules. Stage only when the user asks or when the approved workflow explicitly says to stage for review.

Do not begin addressing review comments until all required reviewers have completed the current review round.

When all reviewers complete a review round and changes are requested, stage the reviewed changes before making follow-up edits. This preserves the already-reviewed baseline in the index so follow-up edits are isolated in the unstaged diff for the next review round. No user intervention is required for this workflow-specific staging step; outside this workflow scope, the normal Pairing-mode git policy applies.

When addressing code-review comments, do not stage follow-up changes. Reviewers use staged diff for the first pass and unstaged diff for follow-up passes.

After a user-approved commit is complete, the doer MUST propose the next integration step to the user: rebase the worktree onto the base branch, merge the worktree back into that base branch, then delete the worktree after the merge succeeds. Do not rebase, merge, or delete the worktree automatically; those git state changes require explicit user approval.

# Reviewer Protocol

Reviewer sessions remain active while `phase` is non-terminal. If no reviewable artifact exists yet, keep polling frontmatter instead of stopping after registration, reporting `WAITING`, or summarizing idle state as completion.

Reviewer verdict submission is one logical write: append the review notes to the Markdown body and update the reviewer frontmatter fields under the same lock acquisition, then release `lock`.

Do not ask for additional human approval before reviewer-owned claim transitions, review-note appends, or reviewer verdict/status updates authorized by this protocol.

When `phase` is `ANALYSIS_SUBMITTED` or `REVIEWING_ANALYSIS`, review the RCA artifact, not the fix plan. Check root-cause quality, evidence, contradictions, falsifiability, and whether the reported failure would become impossible if this cause were fixed. Record the revision reviewed in `reviewed_analysis_revision` and set `analysis_verdict`.

When `phase` is `PLANNING_SUBMITTED` or `REVIEWING_PLAN`, review the latest plan revision. Record the revision reviewed in `reviewed_plan_revision` and set `plan_verdict`.

When `phase` is `RED_TEST_SUBMITTED` or `REVIEWING_RED_TEST`, review the test or reproduction. Check that it fails on current code for the expected reason, would pass if the bug were fixed, tests behavior rather than implementation, and does not corrupt existing expectations. Record the round reviewed in `reviewed_red_test_round` and set `red_test_verdict`.

When `phase` is `CODE_SUBMITTED` or `REVIEWING_CODE`, review staged changes by default:

```bash
git diff --cached --name-only
git diff --cached --stat
git diff --cached
```

Record the round reviewed in `reviewed_code_round` and set `code_verdict`.

When `phase: FOLLOWUP_REVIEW`, review unstaged follow-up changes by default:

```bash
git diff --name-only
git diff --stat
git diff
```

Record the round reviewed in `reviewed_code_round` and set `code_verdict`.

Use the `code-review` skill for code review passes. Label every review with the reviewed target: plan revision, staged diff round, or unstaged follow-up round.

# Blackboard Template

When creating a blackboard from a prompt, create both the frontmatter and Markdown body in one file. Include debugging-only body sections only when `work_type: debugging`, `rca_required: true`, or `red_test_required: true`.

Keep this template in sync with `Frontmatter Contract`; it is intentionally duplicated so agents can bootstrap a blackboard without assembling fields from multiple sections.

```markdown
---
phase: DRAFT
work_type: feature
rca_required: false
red_test_required: false
required_reviewers: []
plan_revision: 0
analysis_revision: 0
red_test_round: 0
code_review_round: 0
phase_updated_at: "YYYY-MM-DDTHH:MM:SSZ"
worktree: null
agents:
  doer:
    role: doer
    status: DRAFT
    last_seen: null
    reviewed_analysis_revision: null
    analysis_verdict: null
    reviewed_plan_revision: null
    plan_verdict: null
    reviewed_red_test_round: null
    red_test_verdict: null
    reviewed_code_round: null
    code_verdict: null
---

# Adversarial Pairing Blackboard

## Goal

## Evidence

## Plan Revisions

## Plan Reviews

## Implementation Notes

## Code Review Rounds

## Validation

## Decisions
```

# Markdown Body

Include `Root Cause Analysis` and `Red Tests` only when `work_type: debugging` or the corresponding gates are enabled. For non-debugging work, omit empty debugging-only sections.

Append; do not rewrite history except to fix obvious formatting before reviewers have acted on it.

# Stop Conditions

Stop and ask the user if:

- The blackboard phase and Markdown body contradict.
- Frontmatter is missing required fields or contains invalid enum values.
- Reviewable phase has empty `required_reviewers` and no explicit user-approved no-review workflow.
- Required reviewer records are missing, stale, or contradictory for the current revision or round.
- A reviewer reviewed an obsolete artifact revision or round.
- The doer needs to transition through a mandatory human gate.
- The current diff scope is ambiguous: staged, unstaged, or full pending state.
- The blackboard path appears to point outside the intended repository or worktree.
- A write would overwrite another agent's state or comments.

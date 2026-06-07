# Adversarial Pairing

Adversarial Pairing is the middle step between ordinary Pairing mode and the
full Liza MAS. Use it when one agent should implement and multiple reviewers
should challenge the work, but a full autonomous sprint would be too heavy.

It runs multiple Pairing-mode sessions against a shared Markdown blackboard. Each
agent runs in its own dedicated interactive terminal: one terminal for the doer,
and one separate terminal for each reviewer. The typical setup is one doer and
several reviewers on different models, so review disagreement exposes
model-specific blind spots. By default, the human remains the approval authority;
the blackboard coordinates doer/reviewer state, submitted artifacts, review
notes, validation output, and decisions.

The doer session is the only coding/control session and still stops at normal
Pairing approval gates before state changes unless started with `yolo`.
Reviewer sessions run autonomously: they read the blackboard, inspect submitted
artifacts, and write review notes or verdicts without asking the human for each
review action.

Use it as:

```text
/adversarial-pairing <role-or-reviewer-id> <blackboard-path> [yolo]
```

`role-or-reviewer-id` is `doer`, `reviewer`, or `reviewer-<id>`. Use
`reviewer-<id>` when you want the agent to receive both its reviewer role and
the stable ID it should use when registering in the blackboard. Start the doer
first so it can create or initialize the blackboard, then start reviewer
sessions against the same path:

```text
/adversarial-pairing doer .liza/adversarial/retry-client.md
/adversarial-pairing reviewer-codex .liza/adversarial/retry-client.md
/adversarial-pairing reviewer-claude .liza/adversarial/retry-client.md
```

Use `yolo` only on the doer session when you want the doer to proceed through
doer-side human approval gates without pausing:

```text
/adversarial-pairing doer .liza/adversarial/retry-client.md yolo
```

`yolo` does not waive reviewer approvals, validation, stop conditions,
merge-conflict handling, or user stop instructions.

When the blackboard file does not exist, or exists but does not yet contain the
goal, include a short goal paragraph in the doer session's first message along
with the `/adversarial-pairing doer <blackboard-path>` command. The doer records
that goal in the blackboard before planning so reviewer sessions share the same
task frame.

The blackboard path may be untracked and should not be committed unless you
explicitly want it preserved. During coding, the doer normally uses a dedicated
git worktree recorded in the blackboard; reviewers review the staged or
unstaged diff for the current review round.

Typical flow:

1. The doer records the goal, evidence, and plan in the blackboard.
2. Reviewers, usually on different models, challenge the submitted plan and record verdicts.
3. After plan approval and human approval to code, the doer implements. In `yolo` mode, the doer treats the human approval step as delegated by invocation.
4. The doer submits the candidate diff for code review.
5. Reviewers request changes or approve. Follow-up rounds continue until approval.
6. After reviewer approval, the doer commits, rebases, merges, then deletes the dedicated worktree and merged topic branch. Without `yolo`, the doer asks before those git state changes; with `yolo`, it proceeds unless a stop condition applies.

For debugging work, the blackboard can require explicit root-cause analysis and
red-test gates before implementation. That gives you the MAS-style discipline of
diagnosis review and failing-test review without handing the whole task to the
autonomous pipeline.

Best for: high-stakes Pairing-mode changes, complex debugging, architectural
edits that need a second agent's review, and situations where the full MAS would
be disproportionate.

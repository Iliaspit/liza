# Liza Support Reference

Troubleshooting reference for Liza multi-agent executions.
This file is written to `.liza/SUPPORT.md` during `liza init`.

## Diagnostic Commands

```bash
liza status                        # Dashboard: goal, sprint, agents, task summary
liza get tasks                     # All tasks with current state
liza get tasks --format table      # Tabular view
liza get agents                    # Registered agents and lease status
liza get agents --zombies          # Live liza agent supervisors missing from state
liza clear-agent-degraded <id>     # Clear role-capacity health after manual recovery
liza validate                      # Check blackboard against invariants
liza validate --skip-process-checks # Offline/archive validation only
liza analyze                       # Circuit breaker pattern detection
```

`liza status --format json` and `liza get agents --format json` include `process_status_source` and `process_status_detail` for agents; status also includes them for phase-handoff blockers. Use these fields when a task appears assigned but the process state is ambiguous. On Linux, procfs identity checks distinguish a matching Liza supervisor from a dead or PID-reused/mismatched process; when process identity is unavailable, active leases are treated conservatively.
PID-based `process_status` is local to the namespace running the command. In containers, sandboxes, or SSH/host boundary situations, a live host agent can appear as `process_status: stopped` because its PID is not observable from the current namespace. Do not recover from `process_status: stopped` alone in that environment; recent heartbeat, active lease, and growing `.liza/agent-outputs/` are stronger liveness evidence. Treat contradictory signals as ambiguous and inspect `process_status_source` / `process_status_detail` before recovery.
Agent health is separate from lifecycle status. A degraded agent epoch remains visible in status/get-agents health fields and does not count as repair-agent-pool capacity until it is cleared or a newer successful claim proves capacity. If the agent process exits and unregisters, the health marker stays visible as degraded capacity context for repair/status output.
Live zombie-agent detection currently requires Linux procfs. On hosts without procfs, `liza validate` warns and skips the live-process check, while `liza get agents --zombies` reports that scanning is unavailable.

## Recovery Commands

```bash
liza recover-task <task-id>        # Release claims + preserve/reattach coherent worktree/branch
liza recover-task <task-id> --fresh # Explicitly discard worktree/branch and reset non-blocked task to initial
liza recover-agent <agent-id>      # Release claim + remove worktree + delete agent
liza release-claim <task-id>       # Granular: release claim only
liza clear-stale-review-claims     # Clear all expired review leases
liza delete agent <id>             # Remove agent from state
liza delete task <id>              # Remove task from state
```

## System Control

```bash
liza pause                         # Pause all agents (sets CHECKPOINT)
liza resume                        # Resume or advance sprint (see Sprint Lifecycle)
liza stop                          # Abort system
liza sprint-checkpoint             # Force checkpoint (halt + summary)
liza replan [task-id]              # Invalidate planner output, create new planning task
liza proceed <task-id> <transition> # Create child tasks for next role-pair
```

## Pipeline Structure

Tasks flow through role-pairs organized in sub-pipelines. The project's frozen pipeline is in `.liza/pipeline.yaml` — inspect it for actual role-pairs, transitions, and state names.

Transitions with `trigger: manual` are human gates; `trigger: auto` transitions run without one. Use `liza status` to see currently available manual transitions.

### Transition Cardinalities

Each transition in `.liza/pipeline.yaml` has a `cardinality`:

| Cardinality | Behavior |
|-------------|----------|
| `per-subtask` | One child task per `output[]` entry |
| `one-to-one` | Single child task from parent |
| `many-to-one` | All sibling tasks in cohort must reach approved, then one child linking all parents |

### `liza proceed`

Creates child tasks based on a completed task's `output[]` and the transition's cardinality. After proceed, run `liza resume` to start the next sprint.

The source task may be either at the transition's configured source state or already at `MERGED`.
`MERGED` is treated as satisfying the transition precondition because `liza proceed` operates from sprint-terminal source tasks and does not change the source task's status.

```bash
liza proceed <task-id> <transition-name>
```

Transition names appear under `transitions:` within each sub-pipeline and under the top-level `pipeline-transitions:` (for cross-subpipeline transitions) in `.liza/pipeline.yaml`.

## Task State Machines

Every role-pair in `.liza/pipeline.yaml` defines its own state names under `states:`. The generic flow is:

```
initial → executing → submitted → reviewing → approved (sprint-terminal)
               │ ↑                      ↓
               │ └───── rejected ──────┘
               └──→ BLOCKED
```

Cross-pair states (not pair-specific):
- **BLOCKED** — Cannot proceed; see `blocked_reason` and `blocked_questions`
- **SUPERSEDED** — Replaced by tasks in `superseded_by`, or completed externally with no replacements (terminal)
- **ABANDONED** — Killed by orchestrator (terminal)
- **MERGED** — Merged to integration branch (terminal, coding pair only)
- **INTEGRATION_FAILED** — Merge conflict or test failure (coding pair only)

To find the actual state names for a role-pair, check `role-pairs.<name>.states` in `.liza/pipeline.yaml`. Some pairs define extra states (e.g. `partially-approved`, `reviewing-2` for quorum review, or `clean` for no-issues-found).

Supervisors automatically block a still-owned executing task when the child provider process makes no observable progress for `config.agent_progress_timeout` seconds. Observable progress is task state movement, worktree HEAD/status movement including untracked files, or provider stdout/stderr output. This prevents a stale `WORKING` agent from holding a task indefinitely when its provider process stalls. The watchdog cancels the provider and waits for it to exit before cleaning the worktree.

## Sprint Lifecycle

```
IN_PROGRESS → CHECKPOINT → COMPLETED → (new sprint) IN_PROGRESS
```

### `liza resume` behavior depends on sprint state:

| Sprint State | Condition | Effect |
|--------------|-----------|--------|
| CHECKPOINT | Not all tasks terminal | Back to IN_PROGRESS (resume current sprint) |
| CHECKPOINT | All tasks terminal | Mark COMPLETED |
| COMPLETED | — | Archive sprint, create new one, execute pipeline transitions |

**Two-step advance:** To move from one pipeline phase to the next, run `liza resume` twice: first marks COMPLETED, second archives and advances.
Approved transition-source output can make a task sprint-terminal before it is integrated. The second `liza resume` refuses to advance/archive until approved planning output is merged; run `liza wt-merge <task-id>` first.

### Checkpoint Actions

When a sprint checkpoints (status: CHECKPOINT), hard checkpoints pause all agents. Transition
checkpoints (`PLANNING_COMPLETE`, `MANY_TO_ONE_READY`) pause orchestrator transition execution only;
doer/reviewer agents may continue already-available work in the current sprint. The human decides:

| Action | Command | When |
|--------|---------|------|
| Accept & resume | `liza resume` | Satisfied with planner output or fan-in readiness, continue |
| Amend & replan | Edit plan, commit, `liza replan` | Want to change planner output |
| Pipeline transition | `liza proceed <task-id> <transition>` | Create child tasks from output or a ready cohort (auto-done by `liza resume` in batch) |
| Pause for manual work | (no command) | Make manual changes first |
| Abort | `liza stop` | Stop entirely |

### Replanning

When a transition checkpoint fires, the human reviews proposed downstream work before child tasks are created. `PLANNING_COMPLETE` means planner `output[]` represents the proposed task breakdown; `MANY_TO_ONE_READY` means a fan-in cohort is ready to create its consolidated child task.

```bash
# Typical replan workflow
# 1. Find planner output files — check the task's output[] in state.yaml
liza get tasks                         # identify the planning task
# 2. Edit the planner's output docs (e.g. specs/plan.md, specs/stories/*.md)
vim specs/plan.md                      # amend planner deliverables
# 3. If scope changed, also align upstream docs that fed the planner
#    (e.g. specs/goals/*.md, specs/epic.md) so inputs match outputs
git add -A && git commit -m "amend plan"
# 4. Replan
liza replan                            # auto-detects the planning task
liza replan <task-id>                  # or specify task ID explicitly
```

Replan invalidates the old task's output (preserved for audit, marked superseded), creates a new planning task with the same role-pair and spec, and returns the sprint to IN_PROGRESS. Multiple replans increment: `<task-id>-replan-1`, `<task-id>-replan-2`, etc.

### Auto-Resume

By default, checkpoints require manual `liza resume`. Auto-resume skips these gates:

- At init: `liza init --auto-resume "Goal"`
- At runtime: TUI `y` key toggles on/off

When enabled, agents auto-call `liza resume` on CHECKPOINT or COMPLETED. Use `liza pause` for a hard stop (never auto-resumed).

## Agent Review Cycles

### Doer: Submit → Await → Handle

```
liza submit-for-review → liza await-verdict → handle result
```

- **REJECTED**: Fix issues, resubmit (session stays alive — no cold restart)
- **ALREADY_TRANSITIONED**: Verdict was recovered after the task moved onward; follow `safe_action` (`stop` means exit without more worktree commands, `revise` means you still own it)
- **APPROVED** / **TERMINAL** with `safe_action: stop`: Exit normally; do not run more worktree commands because merge cleanup may remove the task worktree
- **NEW_ATTEMPT** / **TIMEOUT** / **ABORTED**: Exit normally

### Reviewer: Verdict → Await → Re-review

```
liza submit-verdict REJECTED → liza await-resubmission → review new changes
```

- **RESUBMITTED**: Review again (session stays alive)
- **TERMINAL** / **TIMEOUT** / **ABORTED**: Exit normally

## Agent Log Analysis

Agent logs (`.liza/agent-outputs/`) are the primary diagnostic tool.

**LLM-assisted** — use `/liza-logs` in any pairing agent session to cross-correlate logs, diagnose patterns, and propose fixes.

**CLI analyzer** (stdlib Python 3.12+):
```bash
python3 ~/.liza/skills/liza-logs/scripts/analyze-log.py .liza/agent-outputs/*.txt
```

**Browser analyzer** — drag-and-drop visual charts:
```bash
open ~/.liza/skills/liza-logs/tools/liza-session-analyzer.html   # or xdg-open on Linux
```

## state.yaml

Key task fields:
- `status` — current state
- `assigned_to` — which agent holds the task (doer)
- `reviewing_by` — which agent is reviewing
- `lease_expires` / `review_lease_expires` — when the claim expires
- `base_commit` — review diff base; for submitted/reviewing tasks with a worktree, the merge-base of `review_commit` and the configured integration branch
- `review_commit` — commit submitted for review; must match task worktree HEAD
- `merge_commit` — commit on integration branch after merge
- `iteration` — doer iteration count
- `review_cycles_current` / `review_cycles_total` — rejection count
- `blocked_reason` / `blocked_questions` — why the task is stuck
- `repair_request` — optional complete orchestrator-only repair request captured when the blocker is a state transition the assigned agent cannot perform (`operation`, `target`, `command`, `evidence`, `validation`)
- `rejection_reason` — reviewer feedback on rejection
- `depends_on` — task IDs that must be terminal before this task is claimable
- `output[]` — structured output entries (used by `liza proceed` to create child tasks)
  - `output[].depends_on` — sibling output indexes resolved during `proceed`
  - `output[].task_depends_on` — existing concrete task IDs copied to generated child tasks
  - `output[].destructive_db` — requires non-empty validation, with every command starting `LIZA_ALLOW_DESTRUCTIVE_DB=1 ` or `env LIZA_ALLOW_DESTRUCTIVE_DB=1 `; copied only to per-subtask children
- `history[]` — timestamped event log per task

Key agent fields:
- `id` — e.g. `coder-1`, `code-reviewer-2`
- `role` — runtime role name
- `status` — STARTING, IDLE, WORKING, REVIEWING, WAITING, HANDOFF
- `lease_expires` — agent registration expiry
- `current_task` — task ID being worked on

### Modifying state.yaml

**Golden rule:** Never round-trip state.yaml through `yaml.dump` or any YAML serializer. The file is owned by Go's `yaml.v3` library. Python/Ruby serializers change indentation, timestamp formatting, and block scalar representation in incompatible ways.

**Safe mutation methods (preference order):**

1. **CLI commands** — `liza unblock-task`, `liza retarget-dependency`, `liza cancel-task`, `liza supersede-task`, `liza release-claim`, `liza recover-task`, etc. Always prefer these.
2. **Line-level text edits** — For changes the CLI doesn't support (e.g., output-entry dependency repair, setting a status the CLI rejects). Use `liza pause` first to stop heartbeat updates, then back up before touching anything:

```bash
cp .liza/state.yaml .liza/state.yaml.bak
```

Edit with line-level operations:

```python
with open('.liza/state.yaml', 'r') as f:
    lines = f.readlines()
# Modify specific lines by index
lines[N] = lines[N].replace('OLD_VALUE', 'NEW_VALUE')
# Or insert lines
lines.insert(N, '      depends_on:\n')
# Write atomically
import tempfile, os
with tempfile.NamedTemporaryFile('w', dir='.liza', suffix='.yaml', delete=False) as tmp:
    tmp.writelines(lines)
    tmp_path = tmp.name
os.rename(tmp_path, '.liza/state.yaml')
```

3. **After any manual edit:**
   - `diff .liza/state.yaml.bak .liza/state.yaml` — review exactly what changed
   - `liza validate` — check invariants
   - `liza update-sprint-metrics --json` — triggers Go to normalize formatting (only works if the file still parses)
   - Timestamps must be ISO 8601 UTC (`YYYY-MM-DDTHH:MM:SSZ`). Generate: `date -u +%Y-%m-%dT%H:%M:%SZ`
   - Once satisfied, remove the backup: `rm .liza/state.yaml.bak`

### Known Gotchas

- **`|N` block scalars**: Go writes `|4` for multi-line fields (e.g. `rejection_reason`). If the file was round-tripped through Python YAML, these become unparseable. Fix: repair the broken scalars with line-level text edits (e.g. convert `|4` back to `|4` with correct indentation), then `liza validate`. Once parseable, `liza update-sprint-metrics` will normalize formatting.
- **Timestamps**: Python's `yaml.dump` converts `2026-04-14T14:29:31Z` to `2026-04-14 14:29:31+00:00`. Go rejects this. Never round-trip timestamps through a YAML library.
- **Concurrent writes**: Agents and CLI write concurrently. Use `liza pause` before manual edits, or write atomically (temp file + `os.rename`).
- **Field names**: SUPERSEDED tasks require `rescope_reason` (not `superseded_reason`). Check `liza validate` for correct field names.
- **Status constraints**: `liza supersede-task` works from BLOCKED, REJECTED, or any pipeline-declared initial state. Without replacements, pass `--recoverability-command "<single-line command>"` to record the operator audit command before branch/worktree cleanup; do not include secrets. For other states, edit state.yaml directly.
- **Dependency edits**: Use `liza retarget-dependency <task-id> <old-dep-id> <new-dep-ids> --reason "..."` for one non-terminal task's direct `depends_on` edge. It does not repair `output[].task_depends_on`; use line-level ops only for dependency metadata the CLI still does not support.
- **Holding a task from review**: Add a `depends_on` on the task that should be reviewed first — the system enforces ordering. Alternatively, set status to the pre-review state.

## Agent Exit Codes

| Code | Meaning | Supervisor action |
|------|---------|-------------------|
| 0 | No more work for this role | Stop supervisor |
| 42 | Graceful abort (context exhaustion, lease lost, pause) | Restart immediately |
| Other | Crash | Restart with backoff |

Exit 42 with `handoff_pending: true` on the task means context exhaustion — the restarted agent reads handoff notes and continues.

## Common Failure Patterns

### Lease defaults

- Lease duration: 30 minutes
- Heartbeat interval: 60 seconds
- If lease expires, task becomes reclaimable

### Stuck task (stale lease)
**Symptom**: Task in executing or reviewing state but agent is gone.
**Diagnosis**: `liza get tasks` — check `lease_expires` is in the past (see Lease defaults above).
**Fix**: `liza recover-task <task-id>` or `liza release-claim <task-id>`.

### Agent crash loop
**Symptom**: Supervisor keeps restarting, agent exits non-zero repeatedly.
**Diagnosis**: Check agent output logs in `.liza/agent-outputs/` and the bootstrap prompt in `.liza/agent-prompts/` (what the agent was told to do).
**Fix**: After 5 restarts without progress, supervisor auto-blocks the task. Check `blocked_reason`. May need `liza recover-task` then manual investigation.

### BLOCKED task
**Symptom**: Task in BLOCKED state, agents skip it.
**Diagnosis**: Read `blocked_reason`, `blocked_questions`, `depends_on`, and optional `repair_request` in state.yaml. A `BLOCKED` alert is raised when a task blocks; if the orchestrator assesses but cannot resolve it, an `UNRESOLVED BLOCKED` alert is raised.
**Fix**: If the blocker was another task, the blocked task should list it in `depends_on` so the orchestrator wakes when that task changes. If the wrong direct dependency edge is the blocker, use `liza retarget-dependency <id> <old-dep-id> <new-dep-id[,new-dep-id]> --reason "..."`; the task remains BLOCKED until explicitly unblocked or assessed. If the blocker was repaired and every `depends_on` target is directly MERGED, use `liza unblock-task <id> --reason "..."` to make the task claimable again, or add `--assign-to <doer-agent-id>` for a direct-resume fast path.
If the task has a preserved worktree and integration moved while it was blocked, use `liza unblock-task <id> --rebase-on <integration-branch> --reason "..."`. Tracked worktree changes require `--allow-dirty`, which rebases with Git autostash; untracked files that would be overwritten are refused. Submit/merge conflicts move tasks to `INTEGRATION_FAILED`; unblock-time rebase conflicts remain `BLOCKED` with fresh repair metadata so the preserved worktree can be repaired and unblocked again.
Supersede/cancel operations rewrite active downstream dependencies first; stale edges to SUPERSEDED or ABANDONED tasks must not remain on active tasks. Otherwise use `liza supersede-task <id> [replacements] --reason "..."` to replace with new tasks, `liza supersede-task <id> --reason "..." --recoverability-command "liza recover-task <id>"` to mark completed externally with no replacements, or `liza recover-task <id>` to reset.

### Integration failure
**Symptom**: Task in INTEGRATION_FAILED state.
**Diagnosis**: Merge conflict between task worktree and integration branch.
**Fix**: A coder can claim it (`integration_fix: true`). The worktree is preserved for conflict resolution.

### Sprint stuck at CHECKPOINT
**Symptom**: All agents idle, sprint in CHECKPOINT.
**Diagnosis**: `liza status` — check checkpoint trigger.
**Fix**: `liza resume` to continue, or `liza proceed` + `liza resume` to advance to next pipeline phase.

### Orphaned worktree
**Symptom**: `.worktrees/task-N/` exists but task is terminal.
**Diagnosis**: `liza validate` will flag this.
**Fix**: `liza wt-delete <task-id>`.

### Ghost agent
**Symptom**: Agent registered in state.yaml but process is dead or the registered PID now belongs to a different process.
**Diagnosis**: `liza get agents` — check `process_status`, `process_status_source`, `process_status_detail`, and lease expiry. Watch alerts report active-lease registered agents whose PID is dead or mismatched; unknown process identity is treated conservatively as still live.
**Fix**: `liza recover-agent <agent-id>` or `liza delete agent <id>`.

### Zombie agent process
**Symptom**: A live `liza agent` process is not registered in `state.yaml` and can collide with registered agents claiming the same work.
**Diagnosis**: `liza get agents --zombies` or `liza validate`.
**Fix**: Inspect the PID and stop the stale process. `liza validate --skip-process-checks` is only for archived/offline state validation where host process state is irrelevant.

### Provider quota exhausted
**Symptom**: All agents using a provider (e.g. Claude) have stopped. System mode is still RUNNING, sprint still IN_PROGRESS. Signal file `.liza/provider-quota-exhausted-<provider>` exists.
**Diagnosis**: `ls .liza/provider-quota-exhausted-*` or check `.liza/alerts.log` for `PROVIDER QUOTA EXHAUSTED`. `PROVIDER QUOTA SPAWN BLOCKED` means a spawn was attempted while the quota signal was still set; delete the flag file or run `liza pause` then `liza resume` before spawning again.
**Fix**: `liza pause` then `liza resume` — pause transitions RUNNING → PAUSED, resume clears quota signals and restarts the sprint. Then restart agents. (`liza resume` alone fails because the system is still RUNNING, not PAUSED.)

### Provider unavailable
**Symptom**: Agents for a provider stop before doing useful work, often after startup/session errors such as Codex failing to access `~/.codex/sessions`. Signal file `.liza/provider-unavailable-<provider>` exists.
**Diagnosis**: `ls .liza/provider-unavailable-*` or check `.liza/alerts.log` for `PROVIDER UNAVAILABLE`. `PROVIDER UNAVAILABLE SPAWN BLOCKED` means a spawn was attempted while the provider-unavailable signal was still set. Also inspect `.liza/agent-outputs/*.err` for provider startup errors.
**Fix**: Repair the provider environment first (for Codex, ensure the agent process can access `~/.codex/sessions`), then run `liza pause` and `liza resume` to clear provider-unavailable signals before restarting agents.

### Provider audit degraded
**Symptom**: Agent work may complete, but `.liza/agent-outputs/*.err` or `.liza/alerts.log` contains `PROVIDER AUDIT DEGRADED`, for example Codex `failed to record rollout items: thread ... not found`.
**Impact**: Treat task state and explicit task outputs as the source of truth. The provider transcript or rollout audit trail may be incomplete for the affected session.
**Diagnosis**: Upgrade or retest the provider CLI first. Then inspect `.liza/agent-outputs/*.err`, `.liza/alerts.log`, task state, and blackboard outputs before relying on the session transcript.
**Fix**: A single occurrence does not stop workers. Repeated occurrences across agents are recorded as `provider_audit_degraded` anomalies and can trip `liza analyze` as systemic observability degradation. Raw provider events are not stored in `state.yaml`; inspect `.liza/agent-outputs/` for full transcript evidence. If an older state contains raw provider JSON in anomaly messages, run `liza migrate` to scrub those fields while keeping the anomaly record.
After a circuit-breaker trigger is reviewed and `liza resume` clears it (`status: OK` and `current_trigger: null`), those historical anomalies are acknowledged. Future `liza analyze` and `liza tui` checks only count anomalies with timestamps after the latest cleared `TRIGGERED` history entry; reports include a suppressed-count line instead of mixing acknowledged anomalies into current evidence.

### Circuit breaker

`liza analyze` detects systemic patterns. Supervisor auto-triggers on:

| Condition | Action |
|-----------|--------|
| Agent crash loop (3× in 5min) | Supervisor stops the agent |
| Blackboard validation fails | All agents pause |
| Submit/merge integration branch conflict | Task set to INTEGRATION_FAILED |
| Unblock-time `--rebase-on` conflict | Task remains BLOCKED with fresh repair metadata |
| Circuit-breaker pattern in anomalies | CIRCUIT_BREAKER_TRIPPED mode, sprint CHECKPOINT, reports written |

## Validation Invariants

`liza validate` checks these (among others):
- Tasks in executing states must have `assigned_to`, `worktree`, and valid `lease_expires`
- Tasks in reviewing states must have `reviewing_by`, `review_lease_expires`, and `review_commit`
- Tasks in submitted states must have `review_commit`
- Tasks in rejected states must have `rejection_reason`
- BLOCKED tasks must have `blocked_reason` and `blocked_questions`
- MERGED tasks must not have `worktree`
- No two agents assigned to the same task
- Tasks in initial/draft states cannot have `assigned_to`

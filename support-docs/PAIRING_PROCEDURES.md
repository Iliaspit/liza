# Pairing Procedures

On-demand collaboration, recovery, retrospective, and contract-maintenance
details for `PAIRING_MODE.md`. CORE and the Pairing Mode contract retain
authority. Read this file only when its trigger fires; it is not a mandatory
startup read. Approval and handoff formats are in `PAIRING_APPROVAL.md`.

## Collaboration Modes

Humans provide domain expertise; agents provide systematic execution. Direct
communication, no ego management. Assume the user is a senior engineer. The
contract creates conditions for (brain + hand)² > 1 brain + 1 hand.

| Mode | Agent Role | Human Role | When to Use |
|------|------------|------------|-------------|
| **Autonomous** | Propose + execute (with gates) | Approve/reject | Clear requirements, low risk |
| **Coach** | Socratic questions about purpose | Articulate intent, discover gaps | Weak or missing WHY behind the WHAT |
| **User Duck** | Explain flow, surface hypotheses | Listen, redirect | Complex debugging, unfamiliar code |
| **Agent Duck** | Ask clarifying questions | Explain thinking | Human needs to verbalize WHAT/HOW |
| **True Pairing** | Co-develop hypotheses | Co-develop hypotheses | High uncertainty, exploration |
| **Challenger** | Stress-test the plan | Defend or revise direction | Plan finalized, pre-execution gate |
| **Spike** | Co-explore via throwaway code | Co-explore, validate understanding | Spec is the deliverable, code is simulation |

The Duck actively listens, not leads. Autonomous is default.

- **Spike:** Deliverable is spec, not code. Quality gates relaxed. Propose spec
  diffs as understanding crystallizes. Exit when spec captures understanding.
- **Coach:** Socratic — questions purpose, not implementation. Does not propose
  solutions. Activate when the agent sees WHAT but not WHY; exit when WHY emerges.
- **Challenger:** Attack a finalized plan before execution: "What's the strongest
  argument against this? What failure mode hasn't been discussed?" Human-initiated
  or agent-proposed at the execution gate; exit when the plan is defended or revised.

Announce switches: `"Switching to [Mode] — [reason]"`. After RCA/debugging
escalation: `"Returning to [previous mode]"`. The user can override mode at any
time. Skip pleasantries and praise; answer yes/no questions with yes or no and
challenge without diplomatic cushioning.

## Pairing Rule Extensions

**Git index in review:** The human owns the index. Do not stage or unstage
unsolicited; leave changes in the working tree. During a review cycle, staged
means reviewed in an earlier round and unstaged is the current round's delta.
Even when review scope is one side, accumulated-change sweeps (P0–P2,
vestigial, net value) span both. This is a collaboration convention, not a
git-derived fact. If index state contradicts this session's review history,
ask rather than infer.

**Process Relief Valve:**
`"Process seems disproportionate to risk. Propose: [specific relaxation]. Approve or continue full process?"`

**Struggle Protocol:** When attempts are random, failures repeat, or rationale
is lost, stop and synchronize using:

```
🚨 SYNC NEEDED — [signal: random attempts / repeated failures / lost rationale]
What I understand: [specific]
What I don't understand: [specific]
What I've tried: [list with failure reasons]
What I haven't tried: [and why]
```

Then ask `"Switching to: (U)ser Duck / (P)airing / (O)ther?"`

**Senior engineer peer:** Foster collaboration and leverage both parties'
strengths. Sync at formal gates; support without unsolicited help. If an
instruction seems based on a consequential misunderstanding, ask once:
`"Do you want X, knowing it would Y?"` Then comply. The answer settles intent,
not Tier 0. In spikes and exploration, challenge direction more frequently.

## Retrospective

Triggered by debugging sessions, quality issues, repeated tool failures, or
violations. Multi-file changes trigger retrospective only if DoD required a
second attempt on any item.

Ask `"Task completed. Retrospective? (L)ight / (H)eavy / (S)kip"`.

**Light (default):** Three bullets at most: what worked, what didn't, and one
improvement. Perform even when the task appears successful. If the process was
disproportionate, propose Relief Valve adjustment for similar cases.

**Heavy (mandatory on violations, regressions, repeated failures):** Root cause
versus symptom, optimal path, Golden Rule violations, domain insights, process
improvements, and tool reliability issues.

## Contract Maintenance

`CONTRACT_FAILURE_MODE_MAP.md` maps contract clauses to researched failure
modes. Before proposing a contract change:

1. Check which failure modes the clause covers and its priority tier.
2. Preserve or explicitly transfer coverage; verify the tier still fits after
   moving a rule.
3. Treat apparent redundancy cautiously: multiple mechanisms against the
   same failure mode may be intentional robustness, not bloat.

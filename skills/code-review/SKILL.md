---
name: code-review
description: Code Review Protocol
---

Code review is risk mitigation, not gatekeeping.
The goal is catching issues the author couldn't see — and occasionally sharing a better pattern when it genuinely helps.

# Review Context

Before reviewing, establish context:
- **Scope:** Default to staged files (`git diff --cached --name-only`, then `git diff --cached --stat`, then targeted path diffs). For PRs or commits, inspect changed files and stats before reading targeted hunks. Only broaden if explicitly asked.
- **Local working tree:** For local reviews, triage unstaged and untracked files before review. If related but not staged, surface this explicitly and include only if the review target is "pending changes"; if unrelated, ignore; if unclear, ask before including.
- **Intent:** Check ticket/description (PRs) or ask the author (pending). If unclear, clarify before reviewing.
- **Initial scope:** Before reading the diff, record in one line what the change set out to do. This is the anchor for every later round: it bounds which *behavior* is required, not which code is inspected. Every submitted line is still reviewed for P0-P2 defects. Work beyond the initial scope is scope creep, not thoroughness — review it, then flag it `[overreach]`.
- **Timing:** Is now the right time for this functionality? Half-baked or premature additions warrant a `[question]`.
- **Approach:** For complex changes, was the approach discussed before implementation? Catch architectural misalignment early — complete rewrites are painful.
- **Diff-first:** Read bounded diff context before source files. Prefer name-only/stat first, then targeted path or hunk diffs. Only read source when a finding needs surrounding context. Never pre-read the entire codebase.
- **Size:** If >800 lines or >20 files, consider suggesting a split (PR) or incremental commits (pending). Large diffs hide bugs.
- **Large diffs:** If truncated, >800 lines, >20 files, >80K chars total, or >20K chars in one file, avoid unbounded full-diff reads; classify files and inspect targeted paths/hunks. Verify critical findings against source before tagging `[blocker]` or `[concern]`.
- **Reviewer limits:** If reviewing outside your expertise, say so. Make assumptions explicit.

# Review Modes

| Mode | Scope | When |
|------|-------|------|
| **Sanity** | Skim diff, obvious issues | Trivial changes, config, docs, low-risk |
| **Standard** | Full change set via targeted diffs, P0-P3 checklist, spot-check tests | Most changes — balanced cost/coverage |
| **Deep** | Targeted diffs + source context, all priorities, trace data flow | Security-sensitive, core architecture, unfamiliar domain |

Announce mode: `"Reviewing in [mode] because [reason]. Adjust?"`

Default to Standard. Escalate to Deep if review surfaces unexpected complexity.

**Start high level, work down:** Focus on design and structure first. Defer naming, comments, and style until high-level issues are resolved — low-level notes often become moot after refactoring.

# Review Hierarchy

Review in this order. Stop and flag blockers immediately.

| Priority | Category | Focus |
|----------|----------|-------|
| P0 | **Security** | Injection, auth bypass, secrets exposure, unsafe deserialization |
| P1 | **Correctness** | Does it do what it claims? Edge cases? Error paths? |
| P2 | **Data integrity** | Validation, transactions, idempotency, race conditions |
| P3 | **Architecture & Operability** | Coupling, contracts, backward compat, observability, rollback |
| P4 | **Performance** | Only if measurable impact — N+1, unbounded growth, hot paths |
| P5 | **Maintainability** | Readability, naming, complexity, test quality |
| P6 | **Style** | Only if egregious or violates established conventions |

**Attention budget:** P0-P2 (70%) catch most production incidents — prioritize these. P3-P4 (20%) architectural and performance. P5-P6 (10%) only when egregious.

# Review Checklist

Not exhaustive — a mental sweep, not a checkbox exercise.

**Security (P0):**
- [ ] No hardcoded secrets
- [ ] Input validated at boundaries
- [ ] No injection vectors (SQL, command, XSS)
- [ ] Auth/authz not weakened
- [ ] Sensitive data not logged or exposed
- [ ] Worst realistic misuse considered

**Correctness (P1):**
- [ ] Logic matches stated intent
- [ ] Edge cases handled (null, empty, boundary values)
- [ ] Error paths don't swallow failures silently
- [ ] Return values / exceptions match contract
- [ ] External calls/APIs verified to exist and behave as assumed
- [ ] Impact of code removal assessed (callers, dependencies)
- [ ] New behavior has tests; changed behavior has updated tests

**Data (P2):**
- [ ] Transactions wrap related mutations
- [ ] Concurrent access considered
- [ ] Migrations reversible or safe; online-safe (no long-running locks on large tables)

**Architecture & Operability (P3):**
- [ ] Respects existing patterns; no unnecessary coupling
- [ ] Public API changes intentional; backward compatibility considered
- [ ] Dependency additions justified; version changes assessed for breaking behavior
- [ ] Not relying on implicit/undocumented configuration
- [ ] Operational surface documented: new env vars, README/CHANGELOG, deployment steps for db/breaking changes
- [ ] Observability intact: logs actionable with debug context; metrics/tracing updated if behavior changed
- [ ] Feature flags/kill switches respected if applicable
- [ ] Rollback path exists (code + data)

**Performance (P4):**
- [ ] No N+1 queries or unbounded loops
- [ ] Hot paths not degraded
- [ ] No premature optimization — flag if complexity added for hypothetical gains

**Maintainability (P5):**
- [ ] Readable without author's context
- [ ] Names reveal intent
- [ ] Comments explain *why*, not *what* — if code needs explanation, simplify it
- [ ] TODOs have ticket references — naked TODOs become stale
- [ ] Complexity proportional to problem
- [ ] Tests validate intent, not implementation; would fail on regression
- [ ] Mock-heavy implementation-testing flagged

# Feedback Format

**Severity tags:**

| Tag | Meaning | Blocking? |
|-----|---------|-----------|
| `[blocker]` | Must fix before merge — security, correctness, data integrity | Yes |
| `[concern]` | Should address — architecture, significant maintainability | Discuss |
| `[suggestion]` | Consider — minor improvements, alternatives | No |
| `[question]` | Clarify — reviewer may be missing context | No |
| `[nit]` | Take or leave — style, naming preference | No |
| `[appraisal]` | Acknowledge — good pattern, notable improvement | No |
| `[overreach]` | Beyond initial scope, or a larger solution than the finding required — shrink or split | Yes, in corrective rounds |

**Structure:**
```
[tag] file:line — Brief issue

Why it matters: [impact if not addressed]
Suggestion: [concrete alternative, if any]
```
For `[nit]` and trivial `[suggestion]`: one-liner is fine.

On `[blocker]` and `[concern]`, add `Closure condition:` — the observable state required for approval. Any `Suggestion:` is advisory; the author chooses the implementation. For findings about a mismatch between code and docs/spec/description, name which side is authoritative — otherwise the response defaults to changing code. A stated impact assessment bounds the response: escalating above it needs the reviewer's explicit agreement.

**Tone:** Avoid "you" — focus on code, not coder. Use "we" or omit the subject. "You forgot to close the handle" → "File handle left open" or "Can we close the handle here?"

**Example:**
```
[blocker] auth/login.py:47 — SQL injection via username parameter

Why it matters: Allows auth bypass and data exfiltration
Suggestion: Use parameterized query: cursor.execute("SELECT ... WHERE user = %s", (username,))
```

**Repeated patterns:** Flag 2-3 occurrences, then ask the author to fix the pattern throughout.

# Review Anti-Patterns

**Don't:**
- Invent requirements or failure modes not implied by the stated goal
- Nitpick style without a style guide (let linters handle it)
- Suggest rewrites when the code works and is readable
- Block on "I would have done it differently" without concrete risk
- Miss security issues while debating naming
- Demand perfection — good enough ships, perfect doesn't
- Accept style changes mixed with functional changes — ask to split
- Expand scope to untouched lines — file a bug or fix it yourself
- Treat a larger fix as better compliance — a fix that grows the change set is a finding, not progress
- Review states that exist only because the fix was oversized — flag the oversized fix instead

**Do:**
- Ask "How would I solve this?" — use the difference to guide feedback
- Frame findings as questions when uncertain — you might be missing context
- Ask questions before demanding changes
- Distinguish preference from requirement
- Consider author's experience level — teach, don't gatekeep
- Offer alternatives as questions ("What about...?") — teaches without prescribing
- Treat your own confusion as signal — future maintainers will struggle too
- Document in code, not PR — future readers won't see PR discussions
- Before posting: Is it true? (opinion ≠ truth) Is it necessary? (no nagging, no ego) Is it kind? (no shaming)
- Acknowledge one good decision per substantial review when genuine (brief, no cheerleading)
- Surface low-probability edge cases as `[suggestion]` with risk assessment (likelihood, impact, failure mode) — don't suppress legitimate findings; document the tradeoff

# Approval Criteria

**Approve when:**
- No blockers remain
- Concerns addressed or explicitly accepted as tech debt (record in `TECH_DEBT.md`)
- Code is better than before (not perfect, better)
- You'd be comfortable debugging this at 2am
- `[suggestion]`/`[nit]` don't block — unblock progress while noting improvements
- "No notes" is valid — don't feel compelled to find something wrong

**Request changes when:**
- Blockers exist (P0-P2)
- `[overreach]` findings remain in a corrective round
- Significant concerns unaddressed without rationale
- Intent unclear and author hasn't clarified

**Comment without blocking when:**
- Only suggestions/nits remain
- Concerns acknowledged with reasonable deferral plan

# Re-Review Protocol

Reviews converge by bounding the change set, not by narrowing inspection. Every round reviews the current change set in full — the change set is what cannot grow.

**Every round:**

1. Reconcile each prior finding and classify it: **RESOLVED**, **ACCEPTED** (rationale accepted, no code change), **PARTIALLY ADDRESSED**, or **STILL PRESENT**. Thread resolution, acknowledgement, and outdated diff markers are not evidence.
2. Inspect the whole change set for P0-P2 every round. After round 1, only unresolved prior findings and new P0-P2 defects may block; new P3-P6 observations route to follow-up.
3. Check the change set against the initial scope. Undeclared growth is a finding; declared and justified growth is not. File count alone is a signal, not a verdict.
4. Report the ledger: `Round N — remaining X→Y, files A→B`.

Do not escalate methodology to match a prior reviewer. Review mode cannot exceed the original mode unless the change itself introduced new complexity or risk.

**Corrective commit review:**

A corrective commit answers findings. It is not an opportunity to improve the change.

- **Scope bound:** the corrective diff defaults to files named in the findings it answers, plus those files' tests. Growth beyond that is `[overreach]` unless declared and justified — as a scope extension in multi-agent mode, or in the round record in Pairing. Undeclared growth is the finding: name each file and ask for it to be split out or reverted.
- **Minimal-fix test:** for each finding, ask "what is the least change that makes this finding false?" If the submitted fix is materially larger, flag `[overreach]` and state the smaller fix. A new interface, dependency, migration, or abstraction introduced to answer a `[concern]` or `[suggestion]` is `[overreach]` by default.
- **Collapse before extending:** when a finding concerns code that exists only because of the chosen fix, ask first whether shrinking the fix eliminates the finding. If it does, flag `[overreach]` on the fix rather than opening a round on the states it invented. Reviewing a surface the fix created is how loops fail to converge.
- **Suggestions:** a `[suggestion]` is addressed when it is inside the initial scope, or is a no-behavior-change edit to a file already in the change set. Everything else routes to follow-up. Deferring a suggestion is safe; losing it is not.

Stop when no `[blocker]` or `[overreach]` remains, `[concern]` items are fixed or deferred with rationale, validation exercises the changed behavior, and no new P0-P2 issue was introduced. Remaining `[suggestion]`, `[nit]`, and low-risk `[question]` items do not justify another round.

**Divergence:** files growing while remaining findings do not fall is non-convergence. Stop, restate the original finding set, escalate to the author or human.

**Blast-radius proportionality:**

Review depth scales with blast radius, not with prior review depth.

- **Low blast radius:** dev tooling, internal scripts, config, docs. At most one Standard review plus one focused verification pass.
- **Medium blast radius:** production behavior without schema/auth/public API impact. Standard review plus focused re-reviews until blockers close.
- **High blast radius:** auth, security, data integrity, migrations, public API, production runtime. Deep review is allowed; later rounds still reconcile rather than re-derive.

**Anti-pattern:**

**Methodological arms race:** matching or exceeding a prior review's rigor to appear credible rather than because the change warrants it. This creates non-converging review loops. Prefer a smaller, evidence-focused re-review over a broader, more impressive one.

# Review Summary Format

**Compact** (Approve/Comment, zero blockers/concerns, ≤3 suggestions, ≤3 files, high confidence):
```
Review: [mode] — Approve
```

**Full** (everything else):
```
Review: [mode] — [verdict: Approve / Request Changes / Comment]

Blockers: [count or "None"]
Concerns: [count or "None"]
Suggestions: [count]

Overall: [1-2 sentence assessment]
Blast Radius: [Low: internal refactor | Medium: logic change | High: migration/public API]
Confidence: [high: thorough | medium: focused on key areas | low: quick pass]
Next step: [e.g., "Merge after minor suggestions" | "Ready for another look"]
```

# Mode-Specific Behavior

**Pairing (default):** All prompts apply. "Adjust?" allows human to override review mode.

**§BRAND_NAME_TITLE§ (multi-agent):** No interactive prompts.

| Pairing Prompt | §BRAND_NAME_TITLE§ Behavior |
|----------------|---------------|
| Mode announcement ("Adjust?") | Announce mode, no prompt |
| "Ask the author" / "Clarify" | Check task spec and blackboard; if still unclear, note as `[question]` |
| "Consider suggesting a split" | Note as `[concern]` — do not block review |
| `[overreach]` finding | Also log a `scope_deviation` anomaly |
| "Routes to follow-up" | Record in the verdict text — no anomaly. Reserve `debt_created` for a known deficiency retained in the implementation |
| Non-convergence (see Divergence) | Log `scope_deviation`, or `retry_loop` when the coder is cycling; `review_budget_exhausted` is planner-owned at 5 cycles — do not preempt it |

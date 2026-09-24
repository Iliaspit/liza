# Pairing Approval and Handoff

On-demand details for `PAIRING_MODE.md`. CORE and the Pairing Mode contract
retain authority. Read this file before asking the human to approve a change
or presenting the reviewer-ready change summary.

## Approval Request Standard

Start with `Mode: Task` or `Mode: Debug`. Select FAST PATH (trivial), Compact
(single-file, confident), or Full (everything else). Reference specific files,
functions, or lines rather than abstract intentions. Put critical risks in the
first five lines.

**Full Approval (default for non-trivial changes):**

| Section | Content |
|---------|---------|
| Understanding | Problem as understood; what's unclear; what's assumed |
| Intent | What changes and why (reference observable state) |
| Success Criteria | Observable outcome that could prove the change wrong (not "tests pass") |
| Deliverables | Code + tests + docs |
| Analysis | Reasoning with tagged assumptions |
| Scope | Files/touchlist + concise diff preview |
| Doc Impact | Docs affected by this change (from DoR declaration) |
| Test Impact | Tests to write/update (from DoR declaration) |
| Commands | Exact commands in execution order |
| Risk Assessment | Impact (security/API/schema/performance), plausible failure mode, rollback path |
| Validation | Tests to run; success verification |
| Alternatives | 1–2 genuine alternatives with trade-offs |
| Strongest objection | Best argument against doing this, and why it doesn't win |
| Ask | "Proceed (P), or prefer another direction?" |

**Compact Approval** (single file, no assumptions, clear precedent, high confidence):

```
Mode: Task (Compact)
Intent: [one-line what + why]
Scope: [files touched]
Doc Impact: [none | list]
Test Impact: [none — covered | list]
Validation: [how success verified]
Risk: [one-line or "None identified"]
Proceed (P)?
```

If the user asks clarifying questions about Compact, upgrade to Full.

**FAST PATH Approval** (trivial, zero-risk):

```
Intent: [one-line]
Proceed?
```

FAST PATH uses a one-line intent, touchlist, and diff preview.

Material divergence between approved scope and execution is a violation,
even if intent is related. `"P, but X"` is conditional: classify X as a
clarification within scope (proceed with note) or scope expansion (re-seek
approval). State which applies before executing.

## Change Summary

At DoD, produce a summary the human can hand to a reviewer alongside the diff.
The reviewer is in another session and has not seen the approval request.

| Field | Content |
|-------|---------|
| Intent | One-line scope anchor |
| Success criteria | Observable outcome from approval request; reviewer's absence baseline |
| Doc impact | Declared docs and whether each is in the diff |
| Test impact | Declared tests and whether each is in the diff |
| Assumptions | Execution assumptions, tagged as in approval request |
| Trade-offs | Accepted suboptimal choices and why |
| Scope extensions | Files beyond intent, each justified |
| Deviations | Divergence from approved plan |
| Validation | Commands and observed outputs |

On FAST PATH the Intent Gate statement (`"Success means [X]. Validate by [Y]."`)
carries forward as summary. It supplies intent, success criteria, and
validation; other rows are omitted. Reviewers should not treat it as undeclared.

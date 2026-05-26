# Epic EP-NNN: <Short Descriptive Title>

Status: draft/review/approved/superseded

---

# Part 1 — Intent Review

*For the intent owner. Verify that this epic captures what you meant before agents decompose it.*

## Promise

**Before this epic:** <what the product cannot do today, in plain language>

**After this epic:** <what becomes possible, in plain language>

## Capability Map

| Capability | Human-facing outcome | Source intent captured | Main exclusion |
|------------|---------------------|-----------------------|----------------|
| CAP-001 — <name> | <what the user gains> | <source section/problem ID> | <what this deliberately leaves out> |
| ... | | | |

## Interpretation Decisions

Non-obvious inferences made while translating source material into this epic.
Omit when the epic is a straightforward decomposition with no judgment calls.

| Source signal | Epic interpretation | Confidence | Verify? |
|---------------|---------------------|------------|---------|
| <quote or paraphrase from source> | <how this epic interprets it> | HIGH / MEDIUM / LOW | Yes / No |
| ... | | | |

## Review Questions

Targeted questions only the intent owner can answer. Not technical unknowns — those belong in
Open Questions within the execution contract.

- [ ] <question about intent, scope boundary, or ownership>
- [ ] ...

---

# Part 2 — Execution Contract

*For Story Writers and the Orchestrator. Operational detail for decomposition and implementation.*

## Goal
One sentence. What this epic achieves when fully delivered. Measurable at the product level.

## Context
Why this matters now. How it fits within the product vision. What problem it solves for which persona.
Keep it brief — the Orchestrator needs orientation, not a lecture.

## Personas
- **<Persona name>**: <one-line description of who they are, their environment, and what they care about>
- ...

## General Information

Applies to: the entire epic scope.

### References
- <ref-type>: <path or link> — <section/line range if applicable>
- ...

### Epic Dependencies
- <dependency on another epic, if any>
- ...

### Completion Criteria
The falsifiable condition that closes this epic. When all story ACs pass, this must be satisfied.
Observable outcome, not a direction.

<one or two sentences stating the condition — e.g., "Users can create, edit, and delete tasks
without data loss across sessions. Operators can monitor task volume and error rates in real time."
If you cannot write this yet, surface it as OQ-000-N.>

### Non-Functional Requirements
- NFR-000-1: <requirement> — architectural constraints, performance, security, observability,
  technology mandates, compatibility requirements, etc.
- ...

### Related External Components
Summary of all external components referenced by this epic:
- Component C-NNN - <component name>: <one-line role in this epic>
- ...

### Interfaces *(include only when this epic defines component boundaries)*
Summary of all external interfaces referenced by this epic:
- I-NNN-NNN - <interface name> (Interface NNN of Component C-NNN): <protocol/contract description>
- ...

### Out of Scope
Explicit list of what this epic does NOT cover. Adjacent capabilities the Story Writer must not absorb.

### Assumptions
Items where the vision material was ambiguous and you made a judgment call.
- **ASM-000-1**: <what you assumed> — *Why*: <reasoning> — Confidence: HIGH | MEDIUM | LOW

LOW confidence assumptions are blocking: the human must resolve them before story-writing begins.

### Open Questions
Questions that cannot be resolved by assumption. Must be answered before Story Writers begin work.
- **OQ-000-1**: <question> — *Impact if unresolved*: <what remains unbounded or contradictory>

---

## Capability CAP-001 - <capability name>

One sentence: what this capability delivers and for whom.

### References
- <ref-type>: <path or link> — <section/line range if applicable>
- ...

### Description
Two to four sentences max. Describe the user-facing behavior — what the persona can do that they
could not do before. Do not describe implementation. Do not write stories.

### Story Documents
The set of story documents this capability decomposes into. One story document per cohesive unit of
work a Story Writer can own.

| Story Doc | Title | Priority | Notes |
|-----------|-------|----------|-------|
| <path — assigned by Orchestrator or agreed with human> | <short title> | P1 / P2 / P3 | <dependency note or blank> |
| ... | | | |

### Depends on:
- Capability CAP-NNN - <name>: <why — what from that capability is required here>
- ...

### Out of Scope
What this capability explicitly excludes. Prevents Story Writer scope absorption.

### Assumptions
- **ASM-001-1**: <what you assumed> — *Why*: <reasoning> — Confidence: HIGH | MEDIUM | LOW

### Open Questions
- **OQ-001-1**: <question> — *Impact if unresolved*: <what the Story Writer cannot bound>

---

## Capability CAP-002 - <capability name>
...

### Depends on:
- Capability CAP-001 - <name>: <why>
- ...

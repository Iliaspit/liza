# 7 - TDD Enforcement in Multi-Agent System

> **Note**: ADR-0044 extends this decision to make TDD enforcement configurable
> per workspace. The default remains TDD-mandatory. See ADR-0044 for details.

## Context and Problem Statement

In pairing mode, a human reviews code and can catch when tests validate implementation rather than spec — TDD could be just a user preference. In multi-agent mode, there's no human in the short loop.

Early MAS runs revealed a failure mode: Coders would write tests that "technically satisfy" the `done_when` criteria by testing what the code does, not what the spec requires. Reviewers kept rejecting work, but the pattern persisted.

## Considered Options

1. **Separate test tasks** — Planner creates "implement X" and "add tests for X" as distinct tasks
2. **Tests optional** — Trust Coders to add tests when appropriate
3. **TDD mandatory** — Tests first, implementation second, within a single task

## Decision Outcome

Chose **Option 3**: TDD is mandatory for all code tasks in MAS.

### Rationale

**The interface wins over the implementation.** When tests are written first against `done_when` criteria, they encode the spec's intent. The implementation must satisfy that intent, not the reverse.

**Separate test tasks break TDD flow.** If "implement X" and "test X" are separate tasks, the Coder implements first, then a different Coder (or same Coder, new session) writes tests. Those tests naturally validate what exists rather than what was intended.

**Coders can't validate their own work without tests.** The contract requires validation before claiming completion. Without tests, validation is "I ran it and it seemed to work" — exactly the phantom fix pattern the contract prevents.

**No human to catch the inversion.** In pairing, the human notices when tests are backwards. In MAS, Reviewers can reject, but they see the same green tests the Coder saw. TDD structurally prevents the problem rather than relying on detection.

### Architecture

**Task structure enforces TDD:**
```yaml
- id: implement-hello-cli
  description: "Implement hello CLI per specs/vision.md"
  done_when: |
    - python -m hello prints "Hello, World!" to stdout and exits 0
    - python -m hello --name Alice prints "Hello, Alice!" to stdout and exits 0
    - Tests exist that verify both behaviors
    - All tests pass
  scope: |
    IN: hello/__init__.py, hello/__main__.py, tests/test_hello.py
    OUT: packaging, CI/CD, documentation
```

**Coder workflow:**
1. Read `done_when` criteria
2. Write tests that verify each criterion
3. Run tests (expect red)
4. Implement until tests pass
5. Submit for review

**Reviewer enforcement:**
- REJECT submissions without tests covering `done_when`
- Verify tests actually test the criteria, not just the implementation

**Exemptions:**
- Documentation-only tasks
- Config-only tasks
- Spec-only tasks
- No code = no tests required

**TDD Waiver (code tasks):**
Code tasks that don't change behavior (cosmetic fixes, comment edits, formatting)
can declare `tdd_not_required` with justification in the pre-execution checkpoint.
The Code Reviewer verifies the justification is legitimate. Submission without
test files is allowed when this waiver is present.

### Consequences

**Positive:**
- Tests encode spec intent, not implementation behavior
- Structural prevention of "tests that pass because they test what exists"
- Coders have concrete validation before submitting
- Reviewers have meaningful tests to verify against spec

**Limitations accepted:**
- Adds overhead for small changes (still worth it for integrity)
- Requires Planner to include tests in scope, not separate tasks

---
*Reconstructed from commit d45249e (2026-01-21)*

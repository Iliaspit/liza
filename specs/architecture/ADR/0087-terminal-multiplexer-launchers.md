# ADR-0087: Terminal-Multiplexer Launchers

## Status

ACCEPTED

## Context

Opening correctly configured interactive multi-agent terminal sessions manually
was error-prone and high-friction.

## Decision

Add Liza launch commands for WezTerm and CMUX that create MAS and
adversarial-pairing panes, select provider CLIs, and inject initial prompts.
Preserve the existing Liza orchestration model while the multiplexer starts the
interactive sessions.

## Consequences

- Operators can start supported multi-agent sessions consistently.
- Liza owns multiplexer-specific launch, prompt injection, and readiness
  behavior.
- Prompt injection may require timing configuration for slow-starting CLIs.
- Users without supported multiplexers continue to launch sessions manually.

## Alternatives Considered

No formal alternatives were considered beyond supporting WezTerm and CMUX.

---

Reconstructed from commit 836e33d4 (2026-06-11) and launcher documentation.
User context confirmed 2026-07-16.

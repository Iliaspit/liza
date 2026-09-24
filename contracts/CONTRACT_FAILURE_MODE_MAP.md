# Contract Failure Mode Map

Maintenance map, not agent startup material or proof that a prompt prevents a
failure. References point to current sections rather than line numbers, which
change whenever the contract is edited. A mapped clause can still fail in use.

Historical source labels retained from the prior map: MAST (Berkeley, 2025;
14 modes from 1600+ traces), LLM behavioral research (2024–2025),
code-generation studies (Da et al. 2023; Xia et al. 2024), and AgentIF (2025).
The percentages below are historical labels, not reverified by this update.

## MAST Taxonomy Coverage

### FC1: Specification & System Design Issues (41.77% of MAS failures)

| ID | Failure mode | % | Current covering clauses |
|----|--------------|---|--------------------------|
| FM-1.1 | Disobey task specification | 10.98% | [DoR][C2], [scope][C6], [execution fidelity][PE] |
| FM-1.2 | Disobey role specification | 0.5% | [mode gate][CM], [role execution][MR] |
| FM-1.3 | Step repetition | 17.14% | [stop triggers][CS], [evidence discipline][C5] |
| FM-1.4 | Loss of conversation history | 3.33% | [context and continuity][CC] |
| FM-1.5 | Unaware of stopping conditions | 9.82% | [state machine][CS], [mental models][CMM] |

### FC2: Inter-Agent Misalignment (36.94% of MAS failures)

| ID | Failure mode | % | Current covering clauses |
|----|--------------|---|--------------------------|
| FM-2.1 | Conversation reset | 2.33% | [context and continuity][CC], [multi-agent context recovery][MC] |
| FM-2.2 | Fail to ask for clarification | 11.65% | [DoR][C2] |
| FM-2.3 | Task derailment | 7.15% | [scope][C6], [context drift][CC], [atomic intent][C2] |
| FM-2.4 | Information withholding | 1.66% | [T1.5][CT], [integrity][C1] |
| FM-2.5 | Ignored other agent's input | 0.17% | [peer input][C12] |
| FM-2.6 | Reasoning-action mismatch | 13.98% | [think before acting][C7], [execution fidelity][PE] |

### FC3: Task Verification (21.30% of MAS failures)

| ID | Failure mode | % | Current covering clauses |
|----|--------------|---|--------------------------|
| FM-3.1 | Premature termination | 7.82% | [DoD][C3], [state machine][CS] |
| FM-3.2 | No or incomplete verification | 6.82% | [T0.4][CT], [DoD][C3] |
| FM-3.3 | Incorrect verification | 6.66% | [test skill trigger][CP], [DoD][C3] |

## LLM Behavioral Failure Modes

### Sycophancy

| ID | Failure mode | Current covering clauses |
|----|--------------|--------------------------|
| SYC-1 | Excessive agreement / validation-seeking | [Pairing no-cheerleading][PC] |
| SYC-2 | Opinion mirroring on polarizing topics | [constructive contrarian][C13] |
| SYC-3 | Prioritizing satisfaction over accuracy | [anti-gaming][CA], [integrity][C1] |
| SYC-4 | Softening critical feedback | [Pairing direct response][PC], [professional judgment][C12] |
| SYC-5 | Agreeing with incorrect user statements | [evidence discipline][C5], [stop triggers][CS] |

### Deception

| ID | Failure mode | Current covering clauses |
|----|--------------|--------------------------|
| DEC-1 | Strategic deception for task completion | [T0.2][CT], [integrity][C1] |
| DEC-2 | Unfaithful reasoning / post-hoc rationalization | [post-hoc discovery][C7] |
| DEC-3 | Hallucinated facts or references | [source validation][C5] |
| DEC-4 | Concealing difficulties / silent failure | [integrity][C1], [mode-specific struggle response][PS] |
| DEC-5 | Claiming success without validation | [T0.4][CT], [DoD][C3] |
| DEC-6 | Omitting material information | [T1.5][CT], [integrity][C1] |

### Hallucination

| ID | Failure mode | Current covering clauses |
|----|--------------|--------------------------|
| HAL-1 | Fabricating files, APIs, or config | [source validation][C5] |
| HAL-2 | Inventing file contents without reading | [source validation][C5] |
| HAL-3 | Confabulating error messages | [phantom-fix prevention][C5] |
| HAL-4 | False claims about repository state | [source validation][C5] |

## Code Generation Failure Modes

| ID | Failure mode | Current covering clauses |
|----|--------------|--------------------------|
| COD-1 | Introducing bugs while fixing | [RCA][C11], [debugging trigger][CP] |
| COD-2 | Incomplete refactoring | [batch edit][C3], [scope/refactoring][C6] |
| COD-3 | Breaking unrelated functionality | [security regression checklist][CSEC], [DoD][C3] |
| COD-4 | Accepting invalid inputs silently | [security checklist][CSEC] |
| COD-5 | Type signature mismatch | [testing trigger][CP], [DoD][C3] |
| COD-6 | Edge-case blindness | [testing trigger][CP], [DoD][C3] |
| COD-7 | N+1 / performance patterns | [consequence check][C7] |
| COD-8 | Copy-paste errors / duplication | [DRY gate][C6] |
| COD-9 | Test corruption to pass CI | [T0.3][CT], [failure-as-signal][C14] |
| COD-10 | Drive-by edits / speculative complexity | [self-review][C3], [scope][C6] |
| COD-11 | Minimal diff in the wrong place | [minimality][C6], [RCA][C11] |
| COD-12 | Unrequested abstractions / boilerplate | [scope and minimality][C6] |
| COD-13 | Over-minimization removing safety | [minimum-safe-boundary][C6], [security][CSEC] |

## Instruction Following Failure Modes

| ID | Failure mode | Current covering clauses |
|----|--------------|--------------------------|
| INS-1 | Conditional constraint failures | [DoR][C2] |
| INS-2 | Tool constraint violations | [tool protocol][CP], [evidence discipline][C5] |
| INS-3 | Performance degradation with length | [context tiers][CC] |
| INS-4 | Overly long instruction handling | [tier priority][CT], [context tiers][CC] |
| INS-5 | Ignoring explicit constraints | [Pairing authority][PA], [multi-agent authority][MA] |

## Process & Recovery Failure Modes

| ID | Failure mode | Current covering clauses |
|----|--------------|--------------------------|
| REC-1 | Repository left inconsistent | [batch rollback][R] |
| REC-2 | Continuing after repeated tool failures | [three-failure stop][CS], [tool failure][R] |
| REC-3 | Not learning from violations | [violation response][C9], [stop triggers][CS] |
| REC-4 | Fixing symptom rather than cause | [RCA][C11] |
| REC-5 | Circular fixes | [broken-spec stop][C11] |
| REC-6 | Silent scope creep | [scope][C6], [post-hoc discovery][C7], [execution fidelity][PE] |

## Gaming & Exploitation Vectors

| ID | Failure mode | Current covering clauses |
|----|--------------|--------------------------|
| GAM-1 | Technical compliance against actual intent | [anti-gaming][CA], [DoR][C2] |
| GAM-2 | Narrowing interpretation to exclude cases | [anti-gaming][CA] |
| GAM-3 | Collapsing assumptions to stay under budget | [assumption budget][C2] |
| GAM-4 | Exploiting FAST PATH boundary | [FAST PATH][C4] |
| GAM-5 | Judgment as override of contract | [Pairing authority][PA], [multi-agent authority][MA] |
| GAM-6 | Prompt injection through code or data | [security protocol][CSEC] |

## Coverage Summary

| Category | Mapped IDs |
|----------|-----------:|
| MAST FC1 / FC2 / FC3 | 5 / 6 / 3 |
| Sycophancy / Deception / Hallucination | 5 / 6 / 4 |
| Code Generation / Instruction Following | 13 / 5 |
| Process & Recovery / Gaming & Exploitation | 6 / 6 |
| **Total** | **59** |

This inventory has no independently established “Strong / Partial / Gap”
scores. In particular, a skill trigger, self-monitoring rule, or checklist is
not a mechanical guarantee. Keep the IDs and references current when contracts
change; record uncovered cases explicitly rather than claiming full prevention.

## Maintenance Notes

On contract changes, check all affected IDs and update section links. When a
new failure mode is documented, add its ID and covering clause or mark a gap.
Known limitations remain: context degradation is self-detected; alignment
faking cannot be ruled out by prompts alone; assumption-count gaming relies on
Rule 1's integrity backstop.

[C1]: CORE.md#rule-1-integrity
[C2]: CORE.md#rule-2-definition-of-ready-dor
[C3]: CORE.md#rule-3-definition-of-done-dod
[C4]: CORE.md#rule-4-fast-path-task
[C5]: CORE.md#rule-5-validate-agent-claims-against-external-reality-not-internal-state
[C6]: CORE.md#rule-6-scope-discipline
[C7]: CORE.md#rule-7-think-before-acting
[C9]: CORE.md#rule-9-violation-response
[C11]: CORE.md#rule-11-root-cause-analysis-rca-before-symptoms
[C12]: CORE.md#rule-12-professional-judgment
[C13]: CORE.md#rule-13-constructive-contrarian
[C14]: CORE.md#rule-14-embrace-failure-as-signal
[CM]: CORE.md#mode-selection-gate
[CT]: CORE.md#rule-priority-architecture
[CS]: CORE.md#execution-state-machine
[CC]: CORE.md#context-management
[CMM]: CORE.md#mental-models
[CP]: CORE.md#protocol-references
[CSEC]: CORE.md#security-protocol
[CA]: CORE.md#anti-gaming-clause
[PA]: PAIRING_MODE.md#contract-authority
[PC]: PAIRING_MODE.md#collaboration
[PS]: PAIRING_MODE.md#core-rule-extensions
[PE]: PAIRING_MODE.md#approval-request-standard
[MA]: MULTI_AGENT_MODE.md#contract-authority
[MR]: MULTI_AGENT_MODE.md#role-execution
[MC]: MULTI_AGENT_MODE.md#context-recovery
[R]: ../support-docs/CONTRACT_RECOVERY.md

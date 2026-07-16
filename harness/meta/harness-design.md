# Portable Agent Harness Design

> Amendment (2026-07-16): this plan records the original build and is kept as a historical receipt. The July 2026 restructure changed facts it cites: `docs/source-analysis.md` and this file now live under `meta/`, the always-loaded footprint is tracked by `scripts/audit-harness.sh` (no longer 411 words), and the harness has since gained orchestration, collaboration, verify, and refactor layers with their own commits as receipts. The obligation rows below describe the original scope only; an external Codex review (`meta/codex-critique.md`) correctly flagged that these DONE statuses are structural, not semantic, proof.

## Contract

- User outcome: a reusable, low-noise harness distilled from production engineering repositories, their work records, Descartes debugging practice, and the supplied harness-cleaning transcript.
- Success: the template keeps stable engineering invariants, loads specialist workflows only when triggered, includes Java runtime debugging and evidence-first step-back skills, and supplies hard checks for machine-verifiable rules.
- Non-goals: copying project-specific domain policy, benchmark thresholds, provider/model tuning, build commands, or historical incident ledgers into every new project.
- Runtime validation: local structural checks and skill validation only; no model, benchmark, or live debugger run is required to prove this template.

## Design Obligation Matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-CORE | CRITICAL | User request; transcript principles 1-4 | One concise always-loaded contract routes to one canonical home per concern and specialist detail loads on demand | `AGENTS.md` and `wow.md` | Root instruction files | `scripts/validate-harness.sh` | Validation: 411 always-loaded words; routed asset checks passed | DONE | None |
| OBL-DESIGN | HIGH | Reviewed design standards | Portable design guidance preserves ownership, boundaries, explicit state/failure, operability, focused proof, simplicity, and clean cutover | `docs/design/` | Design principles and obligation protocol | Link and required-section checks | Not applicable: documentation contract | DONE | None |
| OBL-STEPBACK | HIGH | Reviewed stop-loss learnings; user request | Stuck or expensive work triggers evidence-first diagnosis, cycle contracts, classifications, necessity gates, checkpoints, and stop-loss | `skills/take-a-step-back` | Skill and ledger template | Included skill validator plus structural checks | Not applicable until invoked in a project | DONE | Forward-test during first real stuck investigation |
| OBL-DEBUG | HIGH | Descartes sessions/runbook; user request | Java debugging is source-first, parity-checked, async-safe, cursor-based, causal, and cleans up shared state | `skills/debug-java` | Skill, recovery reference, preflight script | Included skill validator and preflight self-check | Not applicable until a JDWP target exists | DONE | Forward-test during first live Java diagnosis |
| OBL-HARD-CHECKS | HIGH | Transcript principle 5; project gate scripts | Machine-verifiable requirements are enforced by scripts instead of repeated prose | `scripts/` | Harness audit, validator, obligation gate | Positive and negative fixture checks | `scripts/validate-harness.sh` passed | DONE | None |
| OBL-ADAPT | MEDIUM | Transcript principle 6 | Project-specific commands and policies have explicit extension points without polluting the portable core | `docs/project-adaptation.md` | Adaptation guide and placeholders | Placeholder/link checks | Not applicable: template customization | DONE | Replace placeholders when adopting |
| OBL-TRACE | MEDIUM | User request; harness map principle | Source learnings and keep/remove decisions are traceable without loading into normal work | `docs/source-analysis.md` | Evidence synthesis and decision table | Presence/link check | Not applicable: analysis record | DONE | None |

## Implementation Steps

| Step | Status | Obligation ids | Target | Work | Verification |
| --- | --- | --- | --- | --- | --- |
| STEP-001 | DONE | OBL-TRACE, OBL-CORE | root docs | Write architecture, evidence synthesis, and compact entrypoints | Structural validation passed |
| STEP-002 | DONE | OBL-DESIGN, OBL-ADAPT | `docs/` | Distill stable guidance and customization boundary | Required assets present |
| STEP-003 | DONE | OBL-STEPBACK, OBL-DEBUG | `skills/` | Create triggered skills with progressive disclosure | Both skills valid |
| STEP-004 | DONE | OBL-HARD-CHECKS | `scripts/` | Add deterministic audit and validation | Positive and negative script tests passed |
| STEP-005 | DONE | all | full harness | Run checks and update proof/status | `scripts/validate-harness.sh` passed |

## Pre-Implementation Gate

- [x] Every critical/high obligation has a named owner.
- [x] Every critical/high obligation has a concrete code target.
- [x] Every critical/high obligation has a focused test target.
- [x] Implementation steps reference obligation ids.

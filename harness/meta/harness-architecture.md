# Harness Architecture

## Objective

Give the model the outcome, non-inferable facts, permission boundary, and finish line early. Load method detail only at the phase where it helps. Convert hard requirements into hard checks.

## Layers

| Layer | Purpose | Expected size | Failure if misused |
| --- | --- | --- | --- |
| Root contract | Stable behavior and routing | Small | Every task pays context cost; conflicts become global |
| Project rules | Concrete local facts and commands | Small/medium | Generic skills become coupled to one repository |
| Guidance | Canonical judgment standards | As needed | Long standards load before they are relevant |
| Skills | Fragile or specialist workflows | Triggered | Discovery collisions and oversized routes |
| Runtime adapters (`skills/*/agents/`) | Per-runtime subagent/invocation profiles for one neutral skill | Tiny static templates | A generator or spec framework appears before a second consumer exists |
| Optional skills | Specialists enabled per project at adaptation | Opt-in | Niche runtimes ship in every project's core |
| Meta (`meta/`) | Harness maintenance rationale and change gate | Never copied to projects | Rationale essays load as operating guidance |
| Deterministic controls | Enforce binary properties | Executable | Prose is mistaken for enforcement |
| Plans/ledgers | Task state, hypotheses, proofs | Task-local | Historical incidents become permanent prompt baggage |
| Receipts/artifacts | Audit what actually ran | Generated | Claims cannot be reproduced or diagnosed |

## One Rule, One Home, One Owner

Every control has one canonical owner. Other files link to it and may state the trigger, but must not paraphrase the full rule. When a lesson is promoted from a session:

1. Identify the job it performs and evidence that it matters.
2. Choose its owner and loading phase.
3. Replace existing copies with links or delete them.
4. Add a deterministic check if the property is machine-verifiable.
5. Record the decision in source analysis or a task receipt.

## Change Gate

The change gate questions are owned by `docs/project-adaptation.md` so they ship with every adopting project; the retro and correction-capture loops there are the gate's main consumers. Template changes answer the same gate; do not restate the questions here.

## Maintenance

Run `scripts/audit-harness.sh .` after meaningful harness changes. Review duplicated normative language, broken links, skill metadata size, placeholder leakage, and always-loaded word counts. Periodically retire controls whose original failure is obsolete or whose enforcement moved into code.

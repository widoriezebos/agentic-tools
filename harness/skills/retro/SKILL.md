---
name: retro
description: Run the harness retro, turning accumulated task receipts into instruction changes and reviewing the previous retro's changes against evidence to keep, amend, or revert them. Use when scripts/receipt.sh check reports a retro due or the human asks for one. Do not use for code changes (normal work) or for mid-task rule additions (forbidden by the contract).
---

# Retro

The harness improves itself the way improve mode improves code: falsifiable changes with expected effects, reviewed against evidence, reverted when they do not deliver. The retro is that loop's engine. Application is human-gated by design: the agent proposes, the human vetoes.

## Inputs

- Receipts since the last retro: `scripts/receipt.sh stats` for the period numbers, the raw lines for patterns.
- Git history for the same period, cross-checked against receipts: a `shipped` receipt followed by fix commits is hidden rework, and it counts as rework.
- The instruction ledger at `plans/instruction-ledger.md`: every change previous retros adopted, each with its expected effect.

## Step 1 — Review the Previous Changes First

Before proposing anything new, give every ledger row adopted at an earlier retro a verdict against this period's evidence:

- `KEPT`: the expected effect is visible, or the change is cheap and nothing contradicts it.
- `AMENDED`: directionally right but the wording or trigger needed adjustment; record the amendment as a new row.
- `REVERTED`: the expected effect did not appear, or ceremony notes name the change as a cost. Remove the instruction in this same retro.

A change with no supporting evidence after two consecutive reviews is reverted by default: keeping it requires an argument; reverting it does not.

## Step 2 — Mine the Period

Record the period numbers from `stats` in the ledger so retros stay comparable over time. Then look for patterns, never anecdotes: a skill that should have triggered but did not; verification repeatedly catching the same class of issue; corrections clustering around one convention; ceremony notes; skills and rules no receipt ever mentions.

## Step 3 — Propose

Each proposal passes the change gate in `docs/project-adaptation.md` and lands as a ledger row:

```markdown
| Id | Retro | Change | Owner doc | Evidence pattern | Expected effect | Review by | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
```

The expected effect must be falsifiable — "fewer rework receipts on refactor tasks", "correction X never repeats" — not aspirational ("better quality"). Route every change to its one owning document; prune with the same energy as adding. Flag lessons that look portable beyond this project as upstream proposals to the harness template.

## Step 4 — Apply and Close

Present verdicts and proposals for human veto — an accept/veto list, not an essay. Apply the accepted rows, update the ledger, then record the marker: `scripts/receipt.sh retro` with a one-line summary. A retro that forgets its marker breaks the cadence for the next one.

## Automation

Evidence collection, due-detection, and period stats are automatic (`scripts/receipt.sh`); wiring `scripts/receipt.sh check` into CI or the runtime's scheduler removes the last manual nudge. The apply step is deliberately not automatic: changing the instructions is a reserved decision the harness applies to itself.

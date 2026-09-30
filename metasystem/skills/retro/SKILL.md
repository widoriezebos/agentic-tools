---
name: retro
description: Run the metasystem retro, turning accumulated task receipts into instruction changes and reviewing the previous retro's changes against evidence to keep, amend, or revert them. Use when metasystem receipt status reports a retro due or the human asks for one. Do not use for code changes (normal work) or for mid-task rule additions (forbidden by the contract).
---

# Retro

The metasystem improves itself the same way improve mode improves code: changes carry testable expected effects, get reviewed against evidence, and get reverted when they do not deliver. The retro runs that loop. Applying changes is human-gated by design: the agent proposes, the human vetoes.

## Inputs

- Receipts this retro has not yet read: run `metasystem receipt status --uncovered --verbose` first. It prints exactly the ledger lines no earlier retro covered, in ledger order, and a token (without `--verbose` it summarizes each line); mine exactly those lines (never a position or a time) and keep the token for Step 4. `metasystem receipt status` gives the period numbers, including the critique-waiver count; the raw `critique_waived` plus `waiver_stream` fields give per-stream waiver patterns.
- Launch usage for the same period: in the receipt ledger used by `metasystem receipt status`, read the final `RETRO` row and use its second pipe-separated field as the previous retro's RFC3339 time. Run `metasystem work status --history --since <the previous retro's time> --json` and record these four numbers in the retro record: builds over the cap, compactions per build job, compacted reads, and calls above 200K context.
- Git history for the same period, cross-checked against receipts. A "shipped" receipt followed by fix commits is hidden rework, and it counts as rework.
- The instruction ledger at `memory/instruction-ledger.md`: every change previous retros adopted, each with its expected effect.

## Step 1: Review the Previous Changes First

Before proposing anything new, give every ledger row adopted at an earlier retro a verdict against this period's evidence:

- `KEPT`: the expected effect is visible in this period's evidence.
- `KEPT-UNPROVEN`: the change is cheap and nothing contradicts it, but no supporting evidence appeared. This is a tracked state, not an acquittal.
- `AMENDED`: directionally right but the wording or trigger needed adjustment. Record the amendment as a new row.
- `REVERTED`: the expected effect did not appear, or ceremony notes name the change as a cost. Remove the instruction in this same retro.

A row whose previous verdict was already `KEPT-UNPROVEN` and earns it again is reverted by default: two consecutive unsupported reviews end a rule. Keeping it requires an argument; reverting it does not.

## Step 2: Mine the Period

Record the period numbers from `stats` in the ledger so retros stay comparable over time. Then look for patterns, never single anecdotes: a skill that should have triggered but did not; verification repeatedly catching the same class of issue; corrections clustering around one convention; ceremony notes; skills and rules no receipt ever mentions.

Also weigh process against product for the period. When plan and ledger edits outgrow shipped code, or bookkeeping commits accumulate while executed verification declines, the process has started substituting records for evidence, and that inversion is itself a retro finding.

## Step 3: Propose

Each proposal passes the change gate in `docs/project-adaptation.md` and lands as a ledger row:

```markdown
| Id | Retro | Change | Owner doc | Evidence pattern | Expected effect | Review by | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
```

The expected effect must be testable: "fewer rework receipts on refactor tasks", "correction X never repeats". "Better quality" is not an expected effect. New rows enter with status `ADOPTED` and `Review by` set to the next retro; the Step 1 verdicts replace that status at review. Route every change to its one owning document, and prune with the same energy as adding. Flag lessons that look portable beyond this project as upstream proposals to the metasystem template.

## Step 4: Apply and Close

Present verdicts and proposals as a list the human can accept or veto item by item. Apply the accepted rows, update the ledger, then record the marker: `metasystem receipt retro SUMMARY --covered TOKEN` with a one-line summary and the token the `--uncovered` read under Inputs printed. The marker records exactly the lines that read covered; a line that arrived after the read stays uncovered for the next retro, and a stale token writes nothing and says to read again. A retro that forgets its marker breaks the cadence for the next one, and a marker without `--covered` covers nothing, so the evidence bound keeps every chain an unread receipt names.

With peer agents active, run retros at a quiet point on the integration branch. Accepted changes land through the normal review flow, agents mid-task finish under the rules they started with, and new rules apply from their next session.

## Automation

Due-detection and period stats are automatic: `metasystem receipt add` warns when a retro is due, and wiring `metasystem receipt status` into CI or the runtime's scheduler removes the last manual nudge. Evidence collection is not automatic. It is a contract duty of the agent being measured, kept honest by the git cross-check and the human spot-check, and a gap in receipts is itself a retro finding. The apply step is deliberately manual: changing the instructions is a reserved decision the metasystem applies to itself.

Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal landing-receipt-survives-records-drift)
Date: 2026-09-09

# Design brief: a landing receipt that survives an append to a register

## Your authority to author this design

You are dispatched to AUTHOR a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1b (Wido, 2026-09-09: "switch back to Claude using Fable
5.1 for designs now") and R-25 put design authoring on this lane. Author the
design; do not stop to report an authority conflict. Dispositions,
certification, the ledger and the receipt stay with the orchestrator.

## The defect, exactly

A landing that carries a test receipt is refused whenever one of two tracked
registers is appended to between the battery and the commit. On 2026-09-09
it refused the same landing eight times; one refusal discarded a
twenty-minute battery. The goal record is
`metasystem/plans/goals/landing-receipt-survives-records-drift.md`.

The two registers are `memory/receipts.log` and
`records/narrator-digest.log`. They are appended by background writers and by
the seat's turn-boundary hooks, at times the seat does not control.

## The mechanism as it stands — read these before deciding anything

- **The landing package already names both paths, once.**
  `metasystem/internal/landing/observe.go:779-783`: the carriage classifier
  treats exactly `memory/receipts.log` and `records/narrator-digest.log` as
  append-only registers (`appendOnly`, refusal code
  `register-carriage-not-append-only`). That is the existing policy about
  these files. Nothing else in the package consults it.
- **The receipt's posture reads.** `metasystem/internal/landing/receipt.go:248`
  `receiptPosture` returns two trees: `workspace.StagedTree()` (the real
  index) and `workspace.Snapshot("HEAD")` (the working-tree projection).
  `Snapshot` in `metasystem/internal/gittree/gittree.go:255` does
  `read-tree HEAD` into an isolated index, then `git add -A -- .` over the
  workspace, then `write-tree`: every tracked file's working bytes enter the
  projection, the two registers included. `StagedTree` in
  `metasystem/internal/gittree/snapshotscope.go:246` reads the real index only.
- **Where the reads happen, four times.** `CreateTestReceipt`
  (`receipt.go:55`) refuses at `:81` unless the supplied tree equals both
  postures of the real workspace BEFORE the run; verifies the isolated
  candidate's postures at `:103-108`; runs the command; then records the
  real workspace's postures AFTER as `Binding.IndexTreeAfter` and
  `Binding.WorktreeTreeAfter`. `readTestReceipt` (`receipt.go:260`) requires
  all four recorded bindings to equal the candidate tree (`:296-306`) and
  then re-reads the real postures at landing time and requires both to equal
  it (`:307-310`). A register appended DURING the battery breaks the after
  binding; a register appended between the receipt and the landing breaks
  the landing-time read. Both happened.
- **The candidate tree itself.** `metasystem/scripts/agents/land.sh:351-358`
  `staged_candidate_tree` is `git write-tree` on the real index, subtree by
  prefix. It reads the index only, so an unstaged register append does not
  move the candidate; it moves the projection the receipt compares against.
- **A fifth site refuses on the same drift for a different reason.**
  `land.sh`'s step named "verify clean after commit" and its earlier "stage
  caller paths" step refuse with `unstaged changes remain after staging;
  transport requires a clean tree after commit`. Today the seat's only
  recovery is `git checkout --` on both registers immediately before every
  attempt.
- **A filtering primitive exists.** `gittree.FilterTree`
  (`gittree.go:282`) rewrites a tree with named paths removed; its comment
  records why the mission wall's identity space excludes force-tracked
  bookkeeping. Every caller passes the mission's own ledger path
  (`missionrunner/wall.go:510` `missionLedgerRel`); there is no shared
  bookkeeping list, so do not go looking for one to reuse.

## What you must decide

1. **Where the register set is declared** so it is one policy, not two. The
   classifier at `observe.go:779` and the posture reads must read the same
   declaration. Say where it lives and what its name means.
2. **Which trees are filtered, and which are not.** The orchestrator's
   reading, which you may refute: filter the WORKING-TREE PROJECTION only, at
   every one of the receipt's posture reads, by `FilterTree` over the
   `Snapshot` result; leave the index tree and the candidate tree exact. Then
   an unstaged append to a register is invisible to the receipt; a staged
   change to anything is still caught by the index; an unstaged change to any
   NON-register path still moves the projection and still refuses. If you
   filter more than that, say what proof the landing loses.
3. **What the fifth site does with drifted registers.** After the commit the
   registers may still be dirty. Either the landing stages them as
   append-only carriage under the existing `:779` rule, or the cleanliness
   check tolerates exactly the register set. Say which and why; do not leave
   the seat's manual `git checkout --` as the answer.
4. **What the receipt records.** If the projection is filtered, the receipt
   must say so, so a reader of the JSON cannot mistake a filtered tree for a
   raw one. Decide the field, and whether the schema version moves.

## What must not change

- The index tree and the candidate tree stay exact. A receipt for tree T
  must still be unlandable against any index other than T.
- The append-only rule at `observe.go:779` stays; this design widens no
  carriage.
- A change to any tracked path outside the register set between battery and
  landing must still refuse, with the existing message.
- `Snapshot` and `StagedTree` keep their contracts for every other caller;
  filter at the landing's call sites, or add an option, but do not change
  what those two functions return by default.

## Fixtures — the design names them, the build proves them

Each with the smallest proving run and a two-minute ceiling:

- **Passing canary**: a receipt is created for tree T; a line is appended to
  `records/narrator-digest.log` in the real workspace; the landing with
  `--test-receipt` succeeds. On the untouched tree this must fail with the
  existing message `the index or working tree moved after the test receipt
  was created`.
- **During-the-battery canary**: the append happens while the command runs
  (the command itself can append); the receipt's after bindings still equal
  T and `readTestReceipt` accepts it.
- **Refusal canary**: the same, but the appended file is a tracked
  non-register path; the landing must still refuse with the existing
  message. This canary must pass before AND after; it exists to prove the
  exclusion did not widen.
- **Fifth-site fixture**: after commit, a dirty register does not fail the
  cleanliness step, per your decision in question 3.

`metasystem/scripts/agents/land-fixtures.sh` already builds receipted chain
landings (`full_chain_receipt` around `:746`); extend it rather than
inventing a bed.

## Deliverable

Write this new file: `metasystem/plans/landing-receipt-survives-records-drift-design.md`

Ground every claim in the tree; cite lines from the revision you read. State
plainly anything the code cannot answer rather than inventing it. Wall-clock
budget: 45 minutes. Design only; you implement nothing and run no bed.

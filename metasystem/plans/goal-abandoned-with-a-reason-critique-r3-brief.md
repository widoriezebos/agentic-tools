Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Review brief: third independent critique of the abandoned-goal design

FINDING IDS: chain-unique, continue the register's series at GAW-19.
Never F-n.

## What you are reading, and what this read is

`metasystem/plans/goal-abandoned-with-a-reason-design.md`, revision 3,
landed at commit 81995968, sha256
3cae4b54103cefe3ac3c48ecd8d4fd4e230f41f5eb528bbfbb6cff924d48a505, 1510 lines.
Confirm the sha before reading; if it differs, stop and say so.

Revision 3 folds the eleven findings of the second read, at
`metasystem/records/misc/goal-abandoned-with-a-reason-critique-r2.md`
(with the coordinator's disposition), under the fold brief
`metasystem/plans/goal-abandoned-with-a-reason-design-fold-r2-brief.md`.
The first read is at
`metasystem/records/misc/goal-abandoned-with-a-reason-critique-r1.md`.
The page's revision record names each finding and what moved; its section
14 lists what the designer says the code cannot answer.

This is the LAST prose read of this design. After it the page becomes the
build's specification and what is still open becomes named fixtures. So:
for every finding, say whether it changes what the builder would build
(material) or is something the build itself answers (name the fixture that
would settle it). A finding stated as a fixture the builder can write is
worth more to this chain than the same finding stated as prose. Read the
code at 81995968; verify every line number you rely on.

## First on your mandate

1. The mandatory binding (section 4a, the answer to GAW-08). A goal-bound
   chain root binds the landing's goal; a landing that names none or
   another refuses; a non-human actor must name a goal unless the ledger
   is Goal-free (`metasystem/internal/goal/verbs.go` 1655 to 1682,
   `Root.Free`). Attack the routes: `metasystem/scripts/agents/commit.sh`
   583 to 600 (`--push` with no fetch, rebase or second look), the two
   `land.sh` push routes and the recertified one
   (`metasystem/scripts/agents/land.sh` 495 to 596), and any other place
   that pushes a landing commit. Can the observation at commit time and the
   held check at push time disagree, for instance a chain whose root record
   has a null `goalId`? The designer counted 59 of 59 agent commits on
   origin/main carrying `Goal-Item`; is that the right evidence for the
   claim that no live route lands agent commits without a goal?

2. The held check at the configured endpoint (GAW-13). `held` now takes
   `--remote` and `--ref` and resolves the goal endpoint through
   `metasystem/internal/goal/txn.go` 46 to 61. Who passes those from
   `land.sh`, what happens under `--skip-transport`, and can a landing's
   remote and the ledger's remote still differ without a refusal?

3. The transaction (GAW-10). Every goal in the abandoned set is locked and
   refusal 7 compares the record's revision with what the projection read.
   Read `metasystem/internal/goalrevision/lock.go` and say whether the lock
   can be taken for an unclaimed goal as the page requires, and read
   `metasystem/scripts/agents/dispatch.sh` around 1497 to 1546 and 1812 to
   1824 and say whether a dispatch that holds the lock for a fresh claim
   and an abandon that wants the same lock can interleave into a published
   abandon with an admitted job.

4. The lost-checkout recovery (GAW-11). Revision 3 decided this itself: a
   new `goal edit --carried <successor>` option on abandoned records,
   under abandon's proof, bound by rule 4 to the newest abandon or edit line
   carrying `carried=`, refused on journal replay. `goal edit` is today a
   proof-less coordinator verb (`metasystem/cmd/metasystem/goalsync_mutations.go`);
   say whether a proof-requiring option inside it is sound, or whether a
   verb is the right shape, and what the validation must refuse.

5. Rollout (section 10). The engine-floor root History line and the check
   over the five consumed slot classes
   (`metasystem/internal/registry/slots.go` 17 to 35, 84 to 93). State
   plainly, from `metasystem/internal/goal/root.go` and
   `metasystem/internal/goal/file.go` around 1303 to 1421, whether an
   engine at cab73164 accepts a root history line with the verb word
   `engine-floor` and its keys; the second read did not report on it. The
   page also says the slot classifier has no production caller today; say
   what that means for the check.

6. The fixtures (section 13, twelve new or rewritten). For each, say
   whether it discriminates: would it fail on today's tree and pass on the
   built one, and does it drive the real route (`land.sh` itself on the
   three push routes, `metasystem/scripts/agents/land-fixtures.sh`)? The
   recertified-route leg names a recipe the bed may not be able to mint
   without a real critic chain (section 14); say whether it can.

7. The census (sections 3 and 5). Redo it at 81995968: every reader of
   `TreeGoals.Done` and every archive decision, against the table
   (`metasystem/internal/goal/reconcilepub.go` 360 to 379 joined in
   revision 3). Name anything still missing.

8. The fold. For each of GAW-08 to GAW-18: closed, narrowed or relabelled,
   one line each, with the section that does it.

## Then the rest

Read every other section as a first revision, including sections 2, 7, 8,
9, 11 and 12, which earlier reads did not reach in depth, and the
interaction with the parked landing-receipt design
(`metasystem/plans/landing-receipt-survives-records-drift-design.md`).

## What you may not do

Do not propose a different design; find where this one is wrong,
underspecified where the builder would have to guess, or unprovable by its
own fixtures. Your sandbox is read-only; write the register in your return
and the coordinator carries it. Wall-clock budget: 60 minutes.

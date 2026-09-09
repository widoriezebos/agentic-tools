Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal landing-receipt-survives-records-drift)
Date: 2026-09-09

# Fold brief: revision 2 of the receipt-survives-drift design

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1b and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict.

## What you are revising

`metasystem/plans/landing-receipt-survives-records-drift-design.md`,
revision 1, landed at 5a316c26. Revise it IN PLACE to revision 2, revision
line updated, a revision record at the end naming each finding and what
moved. The read is
`metasystem/records/misc/landing-receipt-survives-records-drift-critique-r1.md`:
five findings, all material, one critical. The coordinator's disposition
there is binding on this fold. Decisions 1 and 2 stand; Decisions 3 and 4
are what this fold revises.

## LRD-01, critical: the stash is shared, and the window is real

Withdraw `git rebase --autostash`. Two facts, both verified: every worktree
of this repository and every agent session on this machine resolve
the stash ref to the same common path, so a failed autostash re-apply lands
on a list other sessions push to and pop from; and an append that happens
after git snapshots the autostash and before its reset is in neither the
stash nor HEAD. The critic watched `records/narrator-digest.log` go from
clean to modified during its own read-only review.

Design the replacement against the two writers as they are:

- `metasystem/internal/narratordigest/digest.go` rewrites the whole file
  atomically under its own flock (read lines 91-169).
- `metasystem/internal/receipt/receipt.go` appends through an open
  descriptor with no flock (read around line 439).

The drift verb already knows the register paths. Specify capture, rebase and
restoration with private per-landing storage under the landing's own state
(not the stash, not a tracked path), and say for each writer what happens if
it writes before capture, between capture and reset, during the rebase, and
after restoration: which bytes survive, and how the landing proves it. Name
the merge rule you apply when the rebased tree and the captured bytes both
carry new lines, and cite the attribute or code that makes it true. A
fixture must run a concurrent register write during the landing and must
prove a sentinel stash entry pushed beforehand is still there afterwards.

## LRD-02, high: the union premise

If no merge of register bytes remains after LRD-01, remove the claim that
"content conflicts cannot occur" and the literal-line pin with it. If any
merge remains, the pin must read the EFFECTIVE attribute (`git check-attr`)
for both registers in the tree where restoration happens, and the design
must name the refusal when it is absent. Today both resolve to `merge=union`
from `metasystem/.gitattributes` lines 1-2.

## LRD-03, high: the two-engine crossover

Today `metasystem/scripts/agents/land.sh` line 14 selects the live checkout
binary and line 386 mints the receipt with it, while
`metasystem/scripts/agents/commit.sh` lines 297-319 build a separate engine
from the staged source and line 463 reads the receipt with that engine. The
seat-boot skew guard in `dispatch.sh` compares the live binary only with
committed HEAD, not staged source. So on the landing that lands THIS
change, a schema-1 receipt is minted and a schema-2 reader refuses it after
the battery, which is the defect this goal exists to remove. Choose a
cutover rule and specify its test. The three shapes the critic named: mint
with the candidate-built engine; require an explicit candidate rebuild
before receipt creation; or a narrowly proved compatibility read. Say why
the one you choose cannot reintroduce the silent raw-versus-filtered
misreading that made you refuse schema 1 in the first place.

## LRD-04, high: the rule table under --require-empty-index

Section 3a orders the blank-worktree rule before the register tolerance, so
`MM`, `TM`, `AM`, `RM` and `CM` on a register fall through to tolerance even
with `--require-empty-index`, and transport can start with index changes
that appeared after the proved commit. Reorder: with the flag, any non-blank
index column is staged drift before the register exception; without it, the
staged-candidate-plus-register-append case stays tolerated. `DM` is not an
ordinary porcelain-v1 state; the documented unmerged pairs already reach the
final rule. The fixture list at lines 468-472 must say which mode it asserts,
and the tests assert both.

## LRD-05, medium: the shell refusal canary cannot reach its refusal

After the preceding successful landing the index is clean, so appending only
`payload.txt` and landing `--staged-only` hits "the caller-selected staging
set is empty" (`land.sh` lines 320-323) before the drift check, and an
unchanged HEAD passes for the wrong reason. Rewrite the setup: stage a fresh
non-register candidate, mint its receipt, then introduce a separate unstaged
payload edit, and assert the exact "unstaged changes remain after staging"
refusal before any commit.

## What must not change

Decisions 1 and 2 as landed: one declaration of the register set; only the
working-tree projection filtered, index, candidate and commit trees exact;
the append-only rule at `metasystem/internal/landing/observe.go:779`; a
change to any non-register path between battery and landing still refuses.

## Constraints

Wall-clock budget: 45 minutes. Design only; no code, no bed. State plainly
anything the code cannot answer rather than inventing it.

Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal backlog-ordered-by-priority)
Date: 2026-09-09

# Fold brief: revision 2 of the conclusion-vs-compaction design

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to fix
it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1b (Wido, 2026-09-09: "switch back to Claude using Fable 5.1
for designs now") and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict. Dispositions,
certification, the ledger and the receipt stay with the orchestrator.

## What you are revising

`metasystem/plans/backlog-ordered-by-priority-conclusion-vs-compaction-design.md`,
revision 1, landed at 8ca43596. Revise it IN PLACE to revision 2: same file,
revision line updated, a revision record section added at the end saying what
changed and why, citing the finding ids. The parent specification is
`metasystem/plans/backlog-ordered-by-priority-design.md` revision 2 and is
not yours to change.

The read is `metasystem/records/misc/backlog-ordered-by-priority-conclusion-vs-compaction-critique-r1.md`.
Three findings, all accepted by the orchestrator, one material. Read the
coordinator disposition there; it is binding on this fold.

## BCC-01, the material one: the split parent

The page's question 1 rests on "under State done, the last done verb in
history is always the goal's own conclusion". That is false, and the
orchestrator confirmed it in the code before sending this:

- `metasystem/internal/goal/split.go:288-299`: the split parent gets
  `State = StateDone`, then `touch(parent, r, "split", targets)`, then
  `t.Done[parentID] = parent`. No `done` line is written.
- The same transaction, `split.go:301-316`, runs
  `compactDepartedPriorities(t.Live, ...)` and merges the compaction onto
  survivors with `mergePriorityEvent(..., "split", ...)`. So a survivor of a
  split carries a compaction line whose verb is `split`, and a survivor of a
  conclusion carries one whose verb is `done`: compaction lines carry the
  departing act's verb.
- The archive writers in the tree, enumerated by the orchestrator: `done` via
  `doneRequest` (`metasystem/internal/goal/verbs.go:1160`) and via reconcile
  (`metasystem/internal/goal/reconcilepub.go:352`, and `reconcilemap.go:276-280`
  maps a hand-edited `State: done` to a `done` row); `split` via
  `split.go:297`. `metasystem/internal/goal/migrate.go:440` also writes an
  archived record during legacy migration; say what history such a record can
  carry, because the reader will meet it.

The failing path: a ranked live goal survives a neighbour's conclusion
(receives a `done` compaction line at T1), later becomes a split parent
(receives `split` at T2, archived). State is done; the last `done` verb is T1,
the neighbour's. Both the proposed `concludedAt` and the separately guarded
`concludingEpoch` scan report T1.

What must survive: the State-first test (an archived record is the only kind
that can be concluded) and the section 3 writer boundary. Revise WHICH LINE
is the conclusion instant. Decide it from the enumerated writer set, not from
the verb `done` alone, and prove the property you rely on: that no compaction
line can be appended after the archive act, because every compaction loop
walks `t.Live`, and that `reopen` (`verbs.go` around 1340, sets
`StateQueued`) is the only way back. If you conclude the reader cannot be
made sound without a writer change, say so explicitly as a scope widening for
the orchestrator to raise; do not take it.

The canary: add the split-parent path to section 1.5, and it must FAIL on
the untouched tree with named output for the same reason the finding names,
next to the existing survivor-with-landing subtest. Fixtures for split exist
in the tree; cite the one you extend.

## BCC-02 and BCC-03, wording, folded with the above

- BCC-02: `APPROVAL_REQUIRED` cannot reach the frontier in production:
  `metasystem/internal/goal/file.go:495-504` and `ValidateApprovalRecord`
  reject an approved record without a budget or with a mismatched digest at
  parse, and `loadTree` fails before `Next` runs. Section 2.2's cause list
  and any fixture premised on that branch must say so. The production cause
  set of the Refused category is `GOAL_NORM_REFUSED`; keep the typed branch if
  you want it, but do not state or fixture the category as if the other
  cause were reachable.
- BCC-03: section 1.4 step 2 keeps `concludingEpoch`'s own backward scan with
  a duplicated State check while steps 3 and 4 route the other three readers
  through the helper. Either route all four or say plainly that one keeps its
  own scan and why. Whatever you choose must also carry the BCC-01 repair.

## What must not change

Everything section 3 lists as untouched stays untouched. The Refused
category decision (section 2.2) stands; this fold does not reopen it. No rank
is set, inferred or moved; `goal next` stays a read; one admission owner; no
record grammar change.

## Constraints

Wall-clock budget: 40 minutes. Design only; you implement nothing and run no
fixture bed; the orchestrator runs the beds. State plainly anything the page
and the code cannot answer rather than inventing it.

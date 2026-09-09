Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Fold brief: revision 3 of the abandoned-goal design

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1b and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict.

## What you are revising

`metasystem/plans/goal-abandoned-with-a-reason-design.md`, revision 2,
landed at 7ae27b7e. Revise IN PLACE to revision 3 with a revision record.
The read is
`metasystem/records/misc/goal-abandoned-with-a-reason-critique-r2.md`;
the coordinator's disposition there is binding, and the directions below
are its expansion. Eleven findings, all material, one critical, seven high.

This is the last prose revision. What is still open after its read becomes
named fixtures for the build, so where a rule can be stated as a fixture
that would fail today, state it that way.

## GAW-08, critical: the closure is voluntary

`land.sh` takes `--goal` as an option (line 71); the observation runs
`heldGoal` only when a goal was named
(`metasystem/internal/landing/observe.go` 276 to 279) and accepts a new
`records/` file without one (831 to 836); nothing on the page makes a
goal-bound chain name its goal. So a straggler's coordinator lands it
without `--goal`, no trailer is stamped, and `held` prints "no goal binding"
and exits 0.

Close it at the observation: when the chain root's record carries a goal
(the map read at 159 to 167 already has it), the landing's goal is that
goal; `--goal` absent refuses `goal-binding-missing`, `--goal` naming
another refuses `goal-binding-mismatch`. For a landing with no chain (a
direct fix), decide whether a non-human actor must name a goal; the
coordinator's recommendation is yes, every agent landing is goal work, and
`commit.sh` refuses a non-human actor without `Goal-Item`. A human commit
stays sovereign as revision 2 has it. Then re-derive section 4a's argument
with the binding mandatory and say which of the four call sites still
receives revision 0.

## GAW-09, high: the Machine trailer

`commit.sh` appends `Machine:` (552 to 558) without checking whether the
message already carries one, and it already scans for a caller-typed
`Goal-Item` (159 to 235). Specify the same scan for `Machine`, and have
`held` refuse `machine-trailer-malformed` on a missing or duplicate
`Machine` before it reads anything else. The exemption then keys off one
trailer the seat's own engine wrote.

## GAW-10, high: the lock guards only what the projection saw

Section 4 refusal 6 (lines 246 to 250) tolerates a goal that became
claimed after the human's read, and the lock (405 to 418) is taken only for
goals the projection saw claimed. Make refusal 6 compare the record's own
revision (its history length) with what the projection read, for every goal
in the abandoned set: any change refuses. Take the goal-revision lock for
every goal in the set at the revision the projection read, claimed or not,
so a claim landing between the read and the publish is what refusal 6
catches, and a dispatch admitted under that claim on this checkout finds the
abandon refused, not published. Say what happens if the lock cannot be
taken for an unclaimed goal, if that is how `goalrevision.Acquire` behaves
(`metasystem/internal/goalrevision/lock.go`).

## GAW-11, high: the fenced reopen and the lost checkout

State the limitation as what it is: a fenced abandoned goal is reopened
where `resume` would have run, at the claimant's checkout under its
enrolled terminal, because the proof and the batch live there
(`metasystem/internal/humanauthority/authority.go` 141 to 146,
`metasystem/internal/goal/stop.go` 82 to 86). Then name the recovery when
that checkout is gone: the goal is not reopened; a human opens a successor
and the abandoned record's `carried=` names it. Say in section 6 that this
is the contract's "reopenable by a human" bounded by where stop authority
can be proven, and put the bound in section 0.

## GAW-12, high: every push route runs held

Section 4a (465 to 475) says `held` runs after either rebase. The parked
landing-receipt design
(`metasystem/plans/landing-receipt-survives-records-drift-design.md` 503 to
515) prescribes a repair route that ends in a raw `git push`. State the
rule as a property of every route that pushes a landing commit, not of
`land.sh`'s current lines: no push without `held` at the parent. The
coordinator has recorded on the parked goal that its repair route must
call `held` when the two meet; you need only state the rule and the shared
files (the parked design touches more than `land.sh`; name them).

## GAW-13, high: the goal endpoint

Goal publication reads `goal.sync-remote` and `goal.sync-branch`
(`metasystem/internal/goal/txn.go` 46 to 60); `land.sh` pushes `origin`
and the checked-out branch (400 to 410). `held` resolves the goal endpoint
through the same function and refuses `endpoint-mismatch` when the landing's
remote and branch are not it. State the supported configuration (origin,
`refs/heads/main`) and that `--skip-transport` does not change which
endpoint the ledger is.

## GAW-14, -15, -16, -17, -18

- GAW-14: `metasystem/internal/goal/reconcilepub.go` 368 to 379 decides an
  archive race from `t.Done`; add it to the section 5 table with its
  outcome for an abandoned record.
- GAW-15: section 8 rule 4 binds every `Abandoned` field (by, at, reason,
  displaced, stopId, carried, and the event's target goal) to the History
  event it points to, after the approval-binding pattern in
  `metasystem/internal/goal/file.go` 577 to 632; add the mismatch and
  wrong-verb fixtures to section 13; state the validation that `stopId` and
  `carried` appear only on abandon lines.
- GAW-16: the abandonment-race fixture drives `land.sh` itself on each of
  the three push routes (normal, retry after a rejected push, recertified)
  and asserts the refusal from `held`; name
  `metasystem/scripts/agents/land-fixtures.sh` as part of the chain's proof
  even though it is not in the fast gate.
- GAW-17: order the refusals so that the input-only ones (grammar, blank
  or duplicate waiver reasons) come before any that read a tree; renumber
  and fix the fixture that assumed no tree is read.
- GAW-18: the floor check names its slot classes against
  `metasystem/internal/registry/slots.go` 17 to 35. The coordinator's
  recommendation: any unclosed claim whose engine build is below the floor
  refuses, and unknown liveness with an old build refuses too, because the
  cost of a wrong refusal is a retry and the cost of a wrong pass is a
  wedged seat. Add a fixture per class the check inspects.

## What must not change

Everything section "What must not change" of the round-1 fold brief listed,
plus revision 2's two mechanisms and the engine-floor gate.

## Constraints

Wall-clock budget: 45 minutes. Design only; no code, no bed. State plainly
anything the code cannot answer.

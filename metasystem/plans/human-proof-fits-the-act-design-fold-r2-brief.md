Working Mode: design
Orchestrator Identity: m1 (lineage main-1788940932-18533-7fa6c2, coordinator under goal human-proof-fits-the-act)
Date: 2026-09-09

# Fold brief: revision 2 of the human-proof design

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal design-prohibition-is-role-scoped exists to fix
it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1b (Wido, 2026-09-09: "switch back to Claude using Fable 5.1
for designs now") and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict. Dispositions,
certification, the ledger and the receipt stay with the orchestrator.

## What you are revising

`metasystem/plans/human-proof-fits-the-act-design.md`, revision 1, landed
at b7fb35a2d. Revise it IN PLACE to revision 2: same file, revision line
updated, a revision record section at the end saying what changed and why,
citing the finding ids. The goal record
`metasystem/plans/goals/human-proof-fits-the-act.md` is the contract and is
not yours to change.

The read is the register human-proof-fits-the-act-critique-r1.md and the
dispositions human-proof-fits-the-act-critique-r1-dispositions.md, both in
the records directory under misc, new and not yet committed. Nine
findings, all accepted by the orchestrator. The dispositions are binding
on this fold; read them first.

## The one that decides the page: HPA-01

Today an exported lineage plus --by reaches several human acts with no
proof at all: `metasystem/cmd/metasystem/goalsync_mutations.go` proves
human authority only when the lineage is absent; migrate and repair in
`metasystem/cmd/metasystem/goalsync_verbs.go` bypass syncReq; fleet
enrollment in `metasystem/internal/goal/approval.go` records no Authority;
`metasystem/internal/goal/accepted.go` checks only that a human name is
nonempty. Revision 2 opens with a POSITIVE INVENTORY of every human-only
entry point in the tree (the goal verbs, the brain verbs, session stop,
the process verbs, migrate, repair, fleet enrollment, the mission-runner's
adopt-disputed-tree) and states, for each, the proof it takes and where
that proof is checked, so that Actor.Human alone is never sufficient
anywhere. The exported-lineage bypass becomes the primary canary: a
scratch ledger, an agent shell with the lineage exported, --by on steal,
foreign release, classify-sweep, set-pin, reconcile, migrate and repair,
every one refused. Never run it on the live ledger; say so on the page.

## The grading, HPA-02, 03, 04

Replace the table with a transition matrix: every human verb, its
precondition, what the transition grants, its grade, and why. Fixed by
the dispositions: set-pin is enrolled (clearing lets every machine claim,
moving lets the destination claim); set-priority stays terminal; unpark
that restores Approved, set-arc that creates a claim, and
discharge-review-obligation are enrolled; arm, steward arm and restart,
brain withdraw and adopt-disputed-tree are enrolled because they reopen
execution, while their stopping twins stay terminal. Say which walk
`metasystem arm` uses today (process ancestry, not the enrollment) and
keep it. The mission-runner files enter section 8's scope.

## Bootstrap, HPA-05

Permit local enrollment before migration and defer the fleet-cutoff
publication until the backlog exists; migrate then takes the enrolled
grade. State the lifecycle in order.

## The relayed word, HPA-06

Name every deletion site: ArmTemporary's caller in
`metasystem/cmd/metasystem/steward_verbs.go`, the flag text in
`metasystem/cmd/metasystem/main.go` and `metasystem/internal/up/up.go`, and
the carry-forward in `metasystem/internal/steward/runner.go`. Decide that
re-arm keeps historical fields as inert provenance and never mints a
temporary authority generation from them.

## The refusals, HPA-07 and 08

Define one structured no-enrollment outcome shared by Prove and
ProveTerminal, mapped to the enroll-terminal command and tested. The
active-other-terminal refusal states, in words: no prior enrollment or
authority is needed; enrolling here retires the other terminal; retry
the refused command after. Where a human name cannot be derived until
goal human-goal-verbs-forgiving lands, the refusal says no complete
command is available rather than printing a placeholder.

## Canaries, HPA-09

The exported-lineage bypass is canary one and it is red today. Today's
agent refusals stay as regression guards, named as such. The dead
terminal recovery executes the printed command and verifies the old
terminal is retired. Inject a liveness probe into the fixture reader so
alive-versus-dead is deterministic. Every canary names the smallest run
that proves it.

## What must not change

The two-grade shape and the goal's rule. Decision 5's deferral to
breach-stop-wedges-seat. The TOTP idea stays out (Wido postponed it).
No rank, claim or approval semantics change beyond what the matrix
grades.

## Constraints

Wall-clock budget: 45 minutes. Design only; you implement nothing and run
no bed; the orchestrator runs the beds. State plainly anything the page
and the code cannot answer rather than inventing it.

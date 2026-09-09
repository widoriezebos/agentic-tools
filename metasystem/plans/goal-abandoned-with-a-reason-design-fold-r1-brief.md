Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Fold brief: revision 2 of the abandoned-goal design

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1b and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict.

## What you are revising

`metasystem/plans/goal-abandoned-with-a-reason-design.md`, revision 1,
landed at 03f94dfc. Revise IN PLACE to revision 2 with a revision record.
The read is
`metasystem/records/misc/goal-abandoned-with-a-reason-critique-r1.md`; the
coordinator's disposition there is binding on this fold. Seven findings, all
material, one critical, two high. Sections 2 (the state and its fields), 3
(the tree shape, with its census corrected), 7 (prune), 11 (doc) and 12 (the
specimens) stand unless a finding below reaches them.

## GAW-02, critical: a running job is not inert after abandonment

The page relied on dispatch admission and landing both requiring a claimed
goal. Admission does (`metasystem/internal/dispatch/admission.go` around
line 132, Live, claimed, exact revision). Landing does not, in the way that
matters: `heldGoal` in `metasystem/internal/landing/observe.go` (around
line 748) reads the goal's state and actor from the candidate's BASE tree at
observation time and binds no goal revision, and
`metasystem/scripts/agents/land.sh` runs commit.sh, then fetch, then the
transport rebase, then push, without observing again. A job that was
running when the goal was abandoned can therefore rebase its commit cleanly
above the abandonment and push it, while dropping out of the live goal's
budget projection (`metasystem/internal/goal/budget.go`) and returning
normally.

Specify the closure. Two mechanisms, and you may need both: the landing
observation binds the goal's revision it observed (the chain root's
GoalRevision is already on the job record; dispatch stamps it), and the
landing refuses at push time if the goal's state or revision at the rebased
base differs from what was observed, which means land.sh's post-rebase step
must re-run the observation or an equivalent cheaper check through the
engine, not a shell grep. Say what the running job's eventual return and its
terminal job record mean once the goal is abandoned, and what its budget
accounting does. Then say how the transition itself is atomic between
refusal 3's job-record read and the publish: name the lock (the lease
package's bounded lock is what advance and commit.sh use) or the
compare-and-swap.

Note this touches the landing package, which goal
landing-receipt-survives-records-drift is also changing in a parked chain
(the advance verb). Do not design around that chain's unlanded code; design
against the landing as it stands, and say in the revision record where the
two will meet.

## GAW-03, high: reopen must be a proven human act, and frozen authority is not cleared by provenance

The proposed reopen guard is `r.Actor.Human != ""`. In the lineage-bearing
path (`syncReqClassified` in `metasystem/cmd/metasystem/goalsync_mutations.go` around line 59, the actor built at line 117) a caller's
`--by` becomes `Actor.Human` without enrolled-terminal proof. Reopen from
abandoned must take the same proof `set-priority` takes
(`proveGoalHumanAuthority` in
`metasystem/cmd/metasystem/goalsync_mutations.go` around line 677). And a
reopen from a record that carries a frozen `StopFence` must not clear it on
the strength of a history line: either the reopen verifies the stop batch
exactly as `resume` does (`metasystem/internal/goal/stop.go`,
`VerifyStopBatchComplete`) before clearing, or the fence stays frozen on the
reopened queued record until a resume-like act clears it, with the at-rest
rule (section 8 rule 6) extended to say so. Choose, and say why the other is
wrong.

## GAW-04, high: the rollout cannot be a warning

The first abandon on the fleet ledger must refuse unless a precondition is
met. Two candidates, and you decide: a machine-checkable one — the
machine-wide supervision registry at the user's `~/.metasystem` names each
enrolled seat's engine build, but only on this machine, and the ledger has
no fleet binary registry — or a recorded fleet-wide ruling that a human
mints after every seat has rebuilt, which the verb reads from the ledger's
root record. Say what each can and cannot prove. A ruling-gated first use is
acceptable if the page says plainly that the machine cannot prove it.

## GAW-01, medium: the census, corrected

Add `runGoalSplit` (`metasystem/cmd/metasystem/goalsync_mutations.go`
line 1130) and the split parent precheck (`metasystem/internal/goal/split.go`
line 229) to the reader table with their classification. Withdraw "fails
closed" as stated and restate what the third map guarantees: a forgotten
reader degrades to "absent", never to "completed", and name that as the
residual risk.

## GAW-05, -06, -07, medium: the grammar and the two fixtures

- `--waive` refuses a blank reason and conflicting duplicate entries, before
  any edge is removed; add the refusals to section 4's ordered list.
- The dependency-reader canary places the record in `TreeGoals.Abandoned`
  and asserts the state is `StateAbandoned`.
- The Go specimen opens its `--carried` successor as a live goal before
  calling the verb, as the shell specimen does.

## What must not change

Done keeps its meaning and its counts. Abandoned is not a frontier category.
Stop proofs are never rewritten. No history is truncated. The dependency
rule's shape (re-point, waive with a recorded reason, or abandon the
dependent too; never a silent edge removal).

## Constraints

Wall-clock budget: 45 minutes. Design only; no code, no bed. State plainly
anything the code cannot answer.

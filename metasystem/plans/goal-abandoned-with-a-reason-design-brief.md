Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Design brief: a goal that will never be worked, and why

## Your authority to author this design

You are dispatched to AUTHOR a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1b (Wido, 2026-09-09: "switch back to Claude using Fable
5.1 for designs now") and R-25 put design authoring on this lane. Author the
design; do not stop to report an authority conflict. Dispositions,
certification, the ledger and the receipt stay with the orchestrator.

## The problem, in the goal's own words

The goal record is `metasystem/plans/goals/goal-abandoned-with-a-reason.md`;
its Intent and Next step are the contract and this brief only grounds them.
The ledger has five states (`metasystem/internal/goal/file.go` around line
355: queued, approved, claimed, parked, done) and no way to say "this goal
will never be worked, and here is why". The two moves available are both
dishonest: `goal done` with a conclusion saying it was not done, which
corrupts every count of completed work, or a park or a drop to the lowest
rank, which says nothing true.

Wido's shape (2026-09-08): move it out of the live set the way done goals
move; keep it out of consideration for implementation; retain it with its
reason rather than pruning it, because pruning loses history. A second
opinion from Codex, taken the same day, argued against a marker on `done`:
anything that changes completion counts, dependency satisfaction, retention
and reopening is a state whether or not it is named one; the code agrees.

## The two specimens, as they are today (re-read before you rely on this)

1. `metasystem/plans/goals/account-provenance.md`. It became unresumable
   through a machinery defect: breach-stopped, its authority record then
   advanced, and every exit verb refused (`release`, `done`, `park`,
   `detach`, `set-arc` via `clearClaimBinding` at
   `metasystem/internal/goal/verbs.go:276`; `set-budget` at `:731`; `steal`
   at `:1736`; `resume` because the stop batch bound an older revision). It
   held m1d's one claim slot, so m1d could take no work. It was released
   only by Wido's forced landing a68ed896, and m1d then parked it at
   2026-09-09T09:08Z with the reason "wedged specimen". It sits parked at
   3:116 today, meaning "never resume", which is the exact dishonesty this
   goal exists to remove. Its work is carried by
   `metasystem/plans/goals/account-provenance-carried.md` (queued, 2:3), so
   abandoning the record is honest while the work continues.
2. `metasystem/plans/metasystem-stop-design.md`, a design PAGE with no goal
   record, superseded by `metasystem/plans/metasystem-stop-verb-design.md`
   and still reported as open work by the end-of-turn scanner because its
   Next step says "critique once, then build". This is a page problem, not
   a state-machine problem; say so, and say what such a page needs (a
   settled or waiting line under the existing scanner rules) rather than
   widening this design to pages.

## Constraints verified in the code, each binding on the design

1. `depState` (`metasystem/internal/goal/verbs.go:1184`) returns done for
   EVERY id in the archive map, so an archived-with-a-marker goal would
   satisfy dependency edges as if completed. Only `done` may satisfy a
   dependency; that reader must be taught the difference.
2. `clearClaimBinding` (`:276`) refuses while a StopFence is set and `goal
   done` calls it, so abandonment routed through the completion path cannot
   touch a breach-stopped goal, which is the main case. Abandonment needs
   its own transition that preserves the stop evidence and invalidates the
   execution authority rather than clearing the fence; reopening must not
   become a way to erase a stop.
3. The record grammar is closed (`metasystem/internal/goal/file.go:1058`,
   "the record grammar is closed"; the unknown-field refusal above it), and
   `metasystem/internal/goal/validate.go:155-161` requires archived records
   to be done and live records not to be. A new state hard-fails an older
   binary; the rollout is ordered, and the design says in what order.
4. Prune (`verbs.go:1816`) retains the closure of live goals plus the newest
   N done, walked through blocked-by edges, and deletes the rest. It must
   become that closure plus abandoned goals AND their prerequisites;
   abandoned goals sit outside the newest-N allowance; abandonment must
   tolerate its own unfinished prerequisites.
5. Abandoning refuses while a live goal is blocked by it unless the same act
   re-points that dependency to a successor, waives it with a recorded human
   reason, or abandons the dependent too. Never silent edge removal: the
   defect is an unsatisfiable requirement, not a dangling reference.
6. Reopen (`verbs.go:1340`) returns a record to queued; abandoned reopens
   the same way, needing fresh approval, rank not restored, the abandonment
   event kept with reason, actor, time and revision.
7. `metasystem/docs/backlog-mechanism.md:105` says a goal that loses its
   justification "concludes"; that teaches the workaround. Separate the case
   where the want evaporated (concluding is honest) from the case where the
   want survives and the pursuit stops (abandoned).
8. The quota is one claim per machine (`validate.go:329`); an abandoned goal
   occupies no quota, no rank, no frontier category, and `goal next` never
   offers it. It is a human act at the enrolled terminal.

## What must not change

Done keeps its meaning and its counts. The frontier's categories (the
ordering design's) gain nothing; abandoned is not a frontier category, it is
out of the live set. The stop machinery's proofs are preserved, never
rewritten. No goal record's history is truncated.

## Deliverable

Write this new file: `metasystem/plans/goal-abandoned-with-a-reason-design.md`

The page names: the state and its record fields, the transition and its
refusals, what each existing reader does with an abandoned goal (depState,
prune, the frontier, the listing, the counts, the idle refusal), the reopen
path, the dependency rule, the rollout order under the closed grammar, the
doc amendment, and the fixtures: for each reader a canary that must FAIL on
the untouched tree for the reason named, plus the specimen: a fixture that
takes a breach-stopped claimed goal through abandonment and shows the stop
evidence intact, the quota freed and the dependency rule enforced. State
plainly anything the code cannot answer. Wall-clock budget: 45 minutes.
Design only; no code, no bed.

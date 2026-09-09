Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-09

# Follow-up 1 to slice 1: the three seams, answered; continue the build

You stopped after the parser, the validator and the third map (four files,
398 lines) and named three seams you would not join without a decision.
That was the right call under the brief. Here are the decisions. Continue
from your worktree; nothing you built is discarded.

## Seam 1: the engine build of another checkout

The page (section 10, check 3) says "the same resolution `supervise status
--repo` uses". That sentence is wrong about `status`:
`metasystem/cmd/metasystem/supervise.go` line 120 reports the executing
binary's `BuildStamp`, not the checkout's. The resolution the page means
exists on disk: every armed checkout's published supervision state carries
`engineBuild` (`metasystem/internal/supervise/disk.go`, the
`stateDocument` at line 109, field at 120, written from `BuildStamp` by
`PublishState` at 210 to 224), and `DiskCheckout` already reads that
document (`StateNamesSelf`, line 161, parses it from `statePath()`).

Decision: add one exported reader on `DiskCheckout`, `EngineBuild()
(string, error)`, that parses the state document the way `StateNamesSelf`
does and returns the `engineBuild` field; a missing or unreadable
document, or an empty field, is the error. Check 3 constructs a
`DiskCheckout` for each non-Free slot's checkout root and calls it. `dev`
and empty are offending lines exactly as the page says. Do not change
`supervise status`. Record in your return that the page's sentence about
`status` is inaccurate; the coordinator carries that to the register.

## Seam 2: the fixture seam between projection and mutation

The page says "a hook on the tree loader between projection and mutation
(as the existing competitor tests do)". There is no loader hook. What the
competitor tests use is `PublishRequest.BeforePush`
(`metasystem/internal/goal/txn_test.go` from line 170,
`TestLeaseRefusalOnMidflightCompetitor`): the competitor lands inside
`BeforePush` on the first attempt, the compare-and-swap loses, `Publish`
rebuilds and calls `Mutate` again on the new tip, and that second `Mutate`
is where refusal 7 fires because the tip's record revision no longer
matches what the projection read.

Decision: `abandonRequest` sets the request's `BeforePush` from an
unexported package variable in `metasystem/internal/goal/verbs.go`,
`abandonBeforePush func(attempt int) error`, nil in production, set by the
test and reset with `t.Cleanup`. The four cases of
`TestAbandonLocksEveryGoalInTheSetAndRefusesAnyRecordChange` (D claimed,
G's claim moved, G edited, nothing changed) land their competitor there.
The lock assertions (`LOCK_BUSY` on a concurrent `Acquire` while the verb
runs, released after `Publish` returns) are made from the same seam, which
runs while the locks are held. The dispatch-admission half of the D-claimed
case calls `EvaluateGoalRevisionAdmission` at the new claim revision from
the seam as well.

## Seam 3: eleven rules

Section 8 has eleven numbered rules. The brief's "twelve" was the
coordinator's miscount. Build the eleven.

## Continue

Everything else in the slice-1 brief stands: the `goal abandon` verb with
its eleven ordered refusals and its one-transaction effects, the lock for
every goal in the set, the readers of section 5 (row 13 excluded), prune,
recovery and the command table (minus `held` and `carry`), the
`engine-floor` verb with its root History line and rule 10, the registry
check, and every fixture the brief lists, including the shell scenario in
`metasystem/scripts/agents/goal-cli-fixtures.sh`. Run the gate the brief
names; report what the sandbox denied.

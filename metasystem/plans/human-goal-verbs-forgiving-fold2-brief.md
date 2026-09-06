Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal human-goal-verbs-forgiving)
Date: 2026-09-06

# Fold round two: chain hgvf-build1-20260906

The re-review critic (job hgvf-cc2c-20260906, reviewed tree
7b6b47ce21ecebeab18562b484c5c55b9f9f15c9) closed the three round-one
findings and left one material finding and one note. Its return is at
metasystem/artifacts/agents/hgvf-cc2c-20260906/rounds/1/return.json.
The contract is unchanged: metasystem/plans/human-goal-verbs-forgiving-design.md
and the goal record metasystem/plans/goals/human-goal-verbs-forgiving.md.
This is a follow-up on your own job: start from your round-two tree and
change only what the two items need.

# The change

1. HGV-07 (material), metasystem/cmd/metasystem/goalsync_mutations.go,
   the remedy helper for a completed box (completedBudgetRemedy or
   whatever you named it). Today its "the goal already carries that
   box" words line fires only when the goal's state is approved. But a
   compact box that completes to the standing box on a PARKED goal
   prints a run: line that exits 1 (approve answers nothing-to-do under
   the restored guard), and on a CLAIMED goal it prints a run: line that
   exits 1 (set-budget answers nothing-to-do for an unchanged tuple).
   And for an APPROVED goal whose standing approval is relayed or
   expired, the helper prints the words line where a proven
   re-approval with the same box would be a new act that succeeds. Make
   the helper mirror the engine's own no-op guard: the words line when
   the state is approved, parked or claimed AND the standing box equals
   the completed box AND (for approve routes) the standing approval is
   proven and not expired; the run: line otherwise. Prove each of the
   four cases the critic ran (parked same box, claimed same box,
   approved same box, approved same box under a relayed or expired
   approval) in the command package tests, and add the parked and
   claimed rows to the forgiving-human-refusals scenario of
   metasystem/scripts/agents/goal-cli-fixtures.sh with the twin
   comparison the helper already does.
2. HGV-08 (note), same file: in the resume verb's not-breach-stopped
   branch the goal view is bound with the goal's standing budget passed
   as the tier box. Bind the tier box (the norm for the tier) there as
   the other routes do; no printed command changes, say which test
   pins that.

Do not widen: no new verbs, no new flags, no change to authority or
history line formats; the file set stays the round-two set.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/goal/ ./internal/goalbudget/ ./cmd/metasystem/ -count=1`
green (the unchanged process-ownership test may stay red in the
sandbox; say so); `bash scripts/agents/goal-cli-fixtures.sh` green
(say so if the sandbox cannot run it; the orchestrator replays outside).

# Constraints

Wall-clock budget: 40 minutes; return before it ends even if something is
red, naming it. MECHANICAL reach (tier 2). Declare the boundary as every
file that differs from main. Gap rule: stop and report a gap with your
proposed contract written out.

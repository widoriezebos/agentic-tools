Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal human-goal-verbs-forgiving)
Date: 2026-09-06

# Fold round three: chain hgvf-build1-20260906

The round-three critic (job hgvf-cc3-20260906, reviewed tree
b05bf599340121c4e720d8a9ea13ff96dcdeb138) closed HGV-07 and HGV-08 and
left one low material finding, HGV-09; its return is at
metasystem/artifacts/agents/hgvf-cc3-20260906/rounds/1/return.json.
The seat's replay of the goal CLI fixture suite outside the sandbox
found one more defect in your round-three fixture rows, item 2 below,
which no sandbox run could reach. The contract is unchanged:
metasystem/plans/human-goal-verbs-forgiving-design.md and the goal
record metasystem/plans/goals/human-goal-verbs-forgiving.md. This is a
follow-up on your own job: start from your round-three tree.

# The change

1. HGV-09, metasystem/cmd/metasystem/goalsync_mutations.go, the
   completed-box remedy. A CLAIMED goal that is breach-stopped (it
   carries a stop fence) and is given a box that completes to its
   standing box must print the run: line (the keep form resumes it, a
   real act that succeeds), not the words line. One condition: a
   claimed goal with a stop fence takes the run line. Pin it in the
   command package test beside the four cases you added (the critic
   proved it on the standing-validation fixture goal: the words line
   came out, and the completed box then succeeded with outcome
   confirmed). Add the row to the forgiving-human-refusals scenario
   with a twin if the bed can hold a breach-stopped goal; the critic
   notes the bed already has one.
2. The seat's replay: scenario forgiving-human-refusals dies silently
   at the claimed-completion rows in
   metasystem/scripts/agents/goal-cli-fixtures.sh. The claim of
   claimed-completion is rejected with "claim requires the
   authenticated lease holder's positive claim epoch" because the
   scenario still holds direct-set-budget claimed from the alias rows
   and the fixture machine has one claim slot; the rejection went to
   /dev/null and set -e killed the child with nothing in its log.
   Fix: release direct-set-budget (and any other goal the scenario
   still holds) before the claimed-completion loop, the way the alias
   rows release alias-set-budget; and stop swallowing rejections in
   that loop: let the claim's output go to the log or check its
   outcome, so the next such failure names itself.
3. HGV-10 (note): the test named for HGV-08 pins the binding, not the
   printed command. Add one command-package test that drives the resume
   verb's not-breach-stopped branch with a standing box and asserts
   the printed run: line is the typed budget, unchanged.

Do not widen: no new verbs, flags or authority paths; the file set
stays the round-three set.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/goal/ ./internal/goalbudget/ ./cmd/metasystem/ -count=1`
green where the sandbox allows (name what it cannot run);
`bash scripts/agents/goal-cli-fixtures.sh` is replayed by the seat
outside the sandbox, so make item 2 right by reading the scenario's
claim and release order, not by guessing.

# Constraints

Wall-clock budget: 35 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 2). Declare the boundary as
every file that differs from main. Gap rule: stop and report a gap with
your proposed contract written out.

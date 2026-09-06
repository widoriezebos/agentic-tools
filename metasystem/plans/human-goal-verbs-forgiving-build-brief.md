Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal human-goal-verbs-forgiving)
Date: 2026-09-06

# Goal

Slice 2 of goal human-goal-verbs-forgiving (tier 2, approved by Wido at
his terminal 2026-09-06; the record
metasystem/plans/goals/human-goal-verbs-forgiving.md is the contract).
Build exactly what the landed design
metasystem/plans/human-goal-verbs-forgiving-design.md (revision 1)
says: the verb `goal budget` with the compact box form and the presets
norm and keep, the routing by goal state, the over-norm word folded
into a box typed at the enrolled terminal, the human's name on the
terminal enrollment record and the `--by` default from it, the one
refusal function that prints one sentence plus one command that would
have succeeded, and the fixtures of the design's section 6. The
design's section 7 is the scope: those files change, the listed things
do not.

The designer flagged four choices and the orchestrator ruled on each;
build them as ruled:

1. The approve transaction admits a parked goal with a box; the park
   stands and nothing else in that transaction moves.
2. `goal approve --budget box` maps to the `norm` preset (that is what
   it does in the tree today); `keep` is new behaviour.
3. The refusal rule covers the human-only verbs in
   metasystem/cmd/metasystem/goalsync_mutations.go and the shared
   refusals they pass through; the mixed and seat verbs keep their
   messages.
4. The design's size stands; no appendix.

# What binds

- The budget grammar lives in metasystem/internal/goalbudget/budget.go
  (ParseWorkingDuration, New); the tier box in
  metasystem/internal/config/budget.go; the norm check in
  metasystem/internal/goal/norm.go; the approve transaction in
  metasystem/internal/goal/approval.go; set-budget and resume in
  metasystem/internal/goal/verbs.go; the stop fence in
  metasystem/internal/goal/stop.go; the enrollment record and proof in
  metasystem/internal/humanauthority/authority.go; the verb table in
  metasystem/cmd/metasystem/main.go; the suite in
  metasystem/scripts/agents/goal-cli-fixtures.sh.
- Record formats and history verbs do not change (design section 7).
  Authority does not widen: the over-norm branch is reachable only by
  a real enrolled-terminal proof, never fixture-only, never a temporary
  word, never a channel proof; write the test that proves the
  exclusion before the branch.
- The enrollment record's new name field is optional on read so
  records already on disk parse and prove; say in the return what an
  engine built before this change does with a record carrying the
  field.
- The positional box token: settle how it is found among the flags so
  that `metasystem goal budget --id <goal> 1d/10/720m/1/3` and
  `metasystem goal budget 1d/10/720m/1/3 --id <goal>` both work, and a
  goal id can never be taken for a box; if the flag package makes one
  order impossible, say so in the return and keep the documented
  order.
- Source comments carry no round, finding or goal references.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/goal/ ./internal/goalbudget/ ./internal/config/ ./internal/humanauthority/ ./cmd/metasystem/ -count=1 -timeout 30m` green;
`bash scripts/agents/goal-cli-fixtures.sh` green with every new
scenario of the design's section 6 (say so if the sandbox cannot run
it; the orchestrator replays it outside).

# Constraints

Wall-clock budget: 120 minutes; return before it ends even if something
is red, naming it. Declare the boundary as every file that differs from
main. Gap rule: stop and report a gap with your proposed contract
written out; never fill it silently. Return per the version-2
implementer schema.

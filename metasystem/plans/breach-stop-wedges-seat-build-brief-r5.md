Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, follow-up under goal breach-stop-wedges-seat, round 5 of chain bsws-build1b-20260909)
Date: 2026-09-09

# Goal

Same goal, same contract: metasystem/plans/goals/breach-stop-wedges-seat.md.
Round 4 completed the fold of the first critique with no gaps. Since the
chain's base, trunk has moved by fifty-four commits and six of the chain's
files moved with it: metasystem/cmd/metasystem/goal.go and its priority test,
metasystem/internal/channel/report.go and its test,
metasystem/internal/goal/project.go and metasystem/internal/goal/turnverdict.go.
The dispatcher has rebased this chain's branch onto the current trunk before
handing you this round. This round is the merge-forward: prove the rebased
tree is whole, resolve nothing by hand that the rebase already resolved, and
change no behaviour.

# Facts

- The engine's own rebase plan for this chain reported the six overlapping
  paths above and no unmerged paths. If your worktree nevertheless holds
  conflict markers or an unmerged path, that is a gap: report it with the
  file named and stop.
- The trunk change most likely to touch your work: project.go gained an
  AdmissionRefusal type and a Refused list on NextVerdict, next to the Fenced
  list this chain added; goal next and the channel report gained readers of
  it. Both lists must survive side by side, each with its own comment, and
  SelectNext must still ignore Fenced.
- Nothing decided in rounds 1 to 4 is reopened. The fence law
  (clearClaimBinding, the breach-stop writer) and
  metasystem/internal/steward/delivery.go stay byte-identical to trunk.

# Decisions (the orchestrator's; decided, not open)

D1. No new behaviour. The only edits allowed are what the rebase requires
for the tree to compile and for every test in the chain's boundary to pass:
import order, a moved line, a renamed neighbour, a struct field that now sits
beside a new one.

D2. Re-run, on the rebased tree, the whole verification list below and
report each at evidence level ran. If any chain fixture fails on the rebased
tree because trunk changed a helper or a display line it asserts on, fix the
fixture to assert the same behaviour against the new text, and say exactly
what changed. Never weaken an assertion.

D3. Boundary: the chain's fourteen files as they stand after round 4
(metasystem/internal/goal: file.go, goalverbs.go, project.go, stop.go,
turnverdict.go, validate.go, breach_stop_quota_test.go,
servingprojection_test.go; metasystem/cmd/metasystem/goal.go and
goal_priority_test.go; metasystem/internal/channel/report.go and
channel_test.go; metasystem/internal/steward/openwork.go and
openwork_converted_test.go). Anything beyond is a gap, not a change.

D4. Source comments describe the application, never this round, a
critique, or a finding id.

# Verification

Required, run from the worktree and reported at evidence level ran:
go test ./internal/goal/ ./internal/steward/ ./internal/channel/
./cmd/metasystem/ (the goal package in full), then
metasystem/scripts/agents/go-gate.sh --fast, then
metasystem/scripts/agents/goal-cli-fixtures.sh. The two bed scenarios and
the one cmd test the sandbox cannot prove are known; the orchestrator
re-runs them outside the sandbox.

# Constraints

Wall-clock budget: 60 minutes. Return per the implementer schema with the
diff boundary listed. Gap rule: stop and report a gap; never fill it
silently. Never weaken or delete an existing test to pass.

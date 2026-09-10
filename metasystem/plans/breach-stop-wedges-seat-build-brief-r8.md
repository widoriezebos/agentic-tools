Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, follow-up under goal breach-stop-wedges-seat, round 8 of chain bsws-build1b-20260909)
Date: 2026-09-10

# Goal

Same goal, same contract: metasystem/plans/goals/breach-stop-wedges-seat.md.
Round 7 folded the dispatch-admission finding and its read found one thing:
trunk has since created its own metasystem/internal/dispatch/admission_test.go
(commit a633b7a08), so the chain's new file of that name collides and the
patch cannot land. This round is mechanical: move the chain's three
admission fixtures to a file name that does not collide, rebase onto current
trunk, change no behaviour, re-run. The dispatcher rebases the chain onto
current trunk before handing you this round; if the rebase leaves an
unmerged path other than the collision named here, report it and stop.

# Facts

- Trunk's metasystem/internal/dispatch/admission_test.go is not yours; it
  stays byte-identical. The chain's three fixtures
  (TestGoalAdmissionIgnoresSiblingFencedClaim,
  TestGoalAdmissionIgnoresMultipleSiblingFencedClaims,
  TestGoalRevisionAdmissionStillRefusesTheFencedGoal) and any helper they
  brought move to a new file beside it named admission_fenced_sibling_test.go (under
  internal/dispatch; it does not exist on trunk yet).
- Trunk's file may already define helpers with the same names as the
  chain's; if so, use trunk's and delete the chain's duplicate, changing no
  assertion.
- Nothing decided in rounds 1 to 7 is reopened. Production code does not
  change in this round.

# Decisions (the orchestrator's; decided, not open)

D1. The chain's admission fixtures live in the new file
admission_fenced_sibling_test.go under internal/dispatch; the chain no
longer adds or modifies admission_test.go.

D2. Boundary: that one new file, and the removal of the chain's own
admission_test.go from the chain's staged set. Anything beyond is a gap, not
a change.

D3. Source comments describe the application, never this round, a
critique, or a finding id.

# Verification

Required, run from the worktree and reported at evidence level ran:
go test ./internal/dispatch/ ./internal/goal/ ./internal/steward/
./internal/channel/ ./cmd/metasystem/, then
metasystem/scripts/agents/go-gate.sh --fast, then
metasystem/scripts/agents/dispatch-fixtures.sh and
metasystem/scripts/agents/goal-cli-fixtures.sh. Known sandbox-only reds the
orchestrator re-runs outside: the bed scenarios brain-human-word-refuses and
wrong-terminal, the dispatch-bed scenarios adapter-selftest cleanup and
brain-absent-node-proceeds, TestGroupOwnedLiveNonOwnerExitsNotOwned, and the
two command tests red on bare trunk on this host.

# Constraints

Wall-clock budget: 45 minutes. Return per the implementer schema with the
diff boundary listed. Gap rule: stop and report a gap; never fill it
silently.

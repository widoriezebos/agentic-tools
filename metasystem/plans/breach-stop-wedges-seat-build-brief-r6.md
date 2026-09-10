Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, follow-up under goal breach-stop-wedges-seat, round 6 of chain bsws-build1b-20260909)
Date: 2026-09-10

# Goal

Same goal, same contract: metasystem/plans/goals/breach-stop-wedges-seat.md.
Round 5 rebased the chain onto trunk with no behaviour change. The closing
independent read of round 5 found two material defects and one proof hole;
all three are accepted and this round folds them. Nothing decided in rounds
1 to 5 is reopened. The dispatcher rebases the chain onto current trunk
before handing you this round; if that leaves an unmerged path, report it
and stop.

# Facts

- The predicate is GoalFile.IsFencedClaim in metasystem/internal/goal/file.go.
  Every site below uses it; do not re-derive the three-field test.
- The quota rule's arc exception is in metasystem/internal/goal/validate.go,
  the block that begins "Quota: one claim per machine, tree-wide": members
  of ONE arc under one claimant count once. The resume pre-check added in
  round 4 in metasystem/internal/goal/stop.go (resumeRequest, after
  VerifyStopBatchComplete, before touch) must match that rule exactly.
- The fence law does not change: clearClaimBinding and the breach-stop
  writer stay byte-identical, and metasystem/internal/steward/delivery.go is
  not touched.

# Decisions (the orchestrator's; decided, not open)

D1 (BSW-08). In the resume pre-check in metasystem/internal/goal/stop.go,
skip the other claim when it sits in the same non-empty arc as the goal
being resumed (compare the Arc field; an empty arc never matches). The
refusal text and its placement do not change. Fixture in the chain's
own test file under internal/goal (breach_stop_quota_test.go, present in your
worktree, not yet on trunk): two goals claimed
together as one arc on mac-a through the arc claim path (the package's
tests already exercise ClaimArc), one breach-stopped; the human resume of
the stopped member is confirmed while the sibling stays live; and the
existing refusal case (a live claim in no arc, or in a different arc) still
refuses with the round-4 text.

D2 (BSW-09). In metasystem/cmd/metasystem/proof_run.go,
uniqueActiveProofGoal skips fenced claims (IsFencedClaim), so one stopped
claim beside one live claim resolves to the live one, and a machine holding
only a stopped claim resolves to no goal, with the same "pass --goal" refusal
text the function already uses for none. Apply the same skip in
resolveTestingGoal in metasystem/cmd/metasystem/test.go if it has its own
loop rather than calling the first. Fixtures in the command package beside
the existing syncedStoppedGoalFixture: (a) stopped plus live resolves to
the live goal; (b) stopped only refuses naming the stopped goal as fenced,
not as ambiguous. Give the "stopped only" refusal one plain sentence that
says the only claim here is breach-stopped and names the stop id.

D3 (BSW-10). In metasystem/internal/channel/channel_test.go, add the fixture
the critic named: two live claims in one arc plus a fenced claim on the
machine; the report shows the FENCED line and exactly two Next up lines.
Reverting the counter to the old length check must turn it red.

D4. Boundary: metasystem/internal/goal/stop.go and the chain's test file
breach_stop_quota_test.go beside it; metasystem/cmd/metasystem/proof_run.go,
metasystem/cmd/metasystem/test.go and their tests;
metasystem/internal/channel/channel_test.go. Anything beyond is a gap, not a
change.

D5. Source comments describe the application, never this round, a
critique, or a finding id.

# Verification

Required, run from the worktree and reported at evidence level ran:
go test ./internal/goal/ ./internal/steward/ ./internal/channel/
./cmd/metasystem/ (the goal package in full), then
metasystem/scripts/agents/go-gate.sh --fast, then
metasystem/scripts/agents/goal-cli-fixtures.sh. Known sandbox-only
failures the orchestrator re-runs outside: the two bed scenarios
brain-human-word-refuses and wrong-terminal, and in the command package
TestGroupOwnedLiveNonOwnerExitsNotOwned; two further command tests
(TestProcessClassifierDataFailureRepairsThenRetriesTheRequestedVerb and
TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker)
are red on bare trunk on this host and are not yours.

# Constraints

Wall-clock budget: 60 minutes. Return per the implementer schema with the
diff boundary listed. Gap rule: stop and report a gap; never fill it
silently. Never weaken or delete an existing test to pass.

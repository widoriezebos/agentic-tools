Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, follow-up under goal breach-stop-wedges-seat, round 7 of chain bsws-build1b-20260909)
Date: 2026-09-10

# Goal

Same goal, same contract: metasystem/plans/goals/breach-stop-wedges-seat.md.
Round 6 folded the second read. The closing read of round 6 found one
material defect: the chain lets this machine claim goal B beside a
breach-stopped goal A and points every surface at B, but dispatch admission
still refuses every dispatch for B while the same lineage holds the fenced
claim A. The wedge this goal exists to remove would move from claim to
dispatch. This round folds that one finding. Nothing decided in rounds 1 to
6 is reopened. The dispatcher rebases the chain onto current trunk first; an
unmerged path is a gap, report it and stop.

# Facts

- The predicate is GoalFile.IsFencedClaim in metasystem/internal/goal/file.go.
- EvaluateGoalAdmission in metasystem/internal/dispatch/admission.go has a
  machine-wide loop (near lines 128-145 on the chain base) over every claim
  this machine and lineage hold; for a fenced one it adds a refusal carrying
  the live stop reason, and the admission command exits 10. Separately, the
  per-goal revision admission (near line 215) refuses a dispatch against a
  goal whose own revision is breach-stopped. The second is right and stays.
- scripts/agents/dispatch.sh (near lines 670-683) reacts to exit 10 by
  recording REFUSED-BUDGET and running the breach-stop routes, which skip a
  fenced goal whose stop batch is complete, so the dispatch dies with "goal
  admission required breach-stop but supplied no stoppable route". Nothing in
  dispatch.sh changes in this round; the fix is upstream of it.

# Decisions (the orchestrator's; decided, not open)

D1 (BSW-12). In the machine-wide loop of EvaluateGoalAdmission, a claim
that IsFencedClaim does not produce a refusal when the dispatch is for
another goal. It still produces the refusal when the dispatch is for the
fenced goal itself (or leave that case entirely to the per-goal revision
admission, whichever the existing code makes the smaller change; say which
in your return). The refusal text for the fenced goal itself does not
change.

D2. Fixtures in the dispatch package beside EvaluateGoalAdmission's existing
tests: (a) live goal B and fenced goal A, both claimed on one machine by one
lineage, admission for B passes with no refusal; (b) admission for A itself
is still refused with the live stop reason; (c) a second fenced claim beside
B changes nothing for B.

D3. Boundary: metasystem/internal/dispatch/admission.go and its tests.
Anything beyond is a gap, not a change. metasystem/scripts/agents/dispatch.sh
is not touched.

D4. Source comments describe the application, never this round, a
critique, or a finding id. The one sentence to carry: a breach-stopped goal
is waiting on a human and must not keep the machine from working the next
item.

# Verification

Required, run from the worktree and reported at evidence level ran:
go test ./internal/dispatch/ ./internal/goal/ ./internal/steward/
./internal/channel/ ./cmd/metasystem/, then
metasystem/scripts/agents/go-gate.sh --fast, then
metasystem/scripts/agents/dispatch-fixtures.sh and
metasystem/scripts/agents/goal-cli-fixtures.sh. Known sandbox-only reds the
orchestrator re-runs outside: the bed scenarios brain-human-word-refuses and
wrong-terminal; in the command package
TestGroupOwnedLiveNonOwnerExitsNotOwned; and two command tests red on bare
trunk on this host (TestProcessClassifierDataFailureRepairsThenRetriesTheRequestedVerb,
TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker),
which are not yours. State which fail before your change.

# Constraints

Wall-clock budget: 60 minutes. Return per the implementer schema with the
diff boundary listed. Gap rule: stop and report a gap; never fill it
silently. Never weaken or delete an existing test to pass.

Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, follow-up under goal breach-stop-wedges-seat, round 4 of chain bsws-build1b-20260909)
Date: 2026-09-09

# Goal

Same goal, same contract: metasystem/plans/goals/breach-stop-wedges-seat.md.
Round 2 built the predicate (GoalFile.IsFencedClaim) at four sites. The
independent critic of round 2 found five material defects, every one of them
a place where "this machine's live work" is still decided without the
predicate, or where the fenced goal became invisible, or where the proof has
a hole. All five are accepted and this round folds them. Round 3 stopped,
correctly, on a contradiction in the orchestrator's own brief: D2 named the
resume mutation in verbs.go and D8 fenced the boundary around it, while the
mutation is resumeRequest in metasystem/internal/goal/stop.go. This round is
round 3 with that one error corrected; nothing else changes. Nothing decided
in rounds 1 and 2 is reopened.

# Facts

- The critic's findings, with the exact lines it read, are in the round
  packet you were given as the critique register; the orchestrator's
  dispositions below restate each one you must act on.
- Your round-2 return claimed consumers outside the package inherit the fix
  through SelectNext and ReadClaimableBudgetedWork. The critic proved that
  false for ServingProjection, goal next and the channel report. Do not
  rely on that claim again; grep for the sites.
- The fence law still does not change: clearClaimBinding and the breach-stop
  writer stay byte-identical. The resume change below is a pre-check that
  refuses earlier and in plain words; its outcome (rejected, fence intact) is
  the same as today's.

# Decisions (the orchestrator's; decided, not open)

D1 (BSW-01). ServingProjection in metasystem/internal/goal/goalverbs.go
chooses this machine's served goal. It must skip fenced claims (use
IsFencedClaim) and choose deterministically in backlog order
(OrderedOpenGoalIDs, as Next does), never by map iteration. With only a
fenced claim on the machine it serves no goal, exactly as if none were
claimed. Fixture: one fenced and one live claim on the machine; forty calls
all serve the live one.

D2 (BSW-02). In resumeRequest in metasystem/internal/goal/stop.go, in the
mutation, after VerifyStopBatchComplete has accepted the fence and before
touch and bindClaim, if the tree shows another live (unfenced) claim by the
same machine (any goal other than the one being resumed for which
IsFencedClaim is false and Claimed.Machine matches), refuse with this shape
and nothing cryptic:
"goal resume <id> refused: machine <m> already holds live claim <other>;
conclude, park or release <other> first, then resume <id>". Outcome
rejected; fence and claim untouched. Fixture: fenced A, live B on mac-a;
human resume of A is rejected with that text and A's fence still stands;
release B; resume A confirms. Reuse the existing resume-with-human-authority
shape from the package's tests (the critic's probe used it); if human
authority in tests needs the fixture-authority mechanism, use it.

D3 (BSW-03). Rename the round-2 helper fencedClaimDisplay to an exported
FencedClaimLines in metasystem/internal/goal/turnverdict.go and use it in
three places: the turn verdict (as now), the goal next command in
metasystem/cmd/metasystem/goal.go (one line per fenced claim printed before
the selection line, in every branch including "no claimable goal"), and the
channel report's Next up block in metasystem/internal/channel/report.go
(one line per fenced claim). The line text stays: FENCED <id>: breach-stopped
by <stopId> (<reason>); only goal resume, a human act, clears it; the queue is
open. Fixtures: goal next with a fenced claim and no ready work prints the
FENCED line and then its existing "no claimable goal" line; the report
builder with a fenced claim carries the line under Next up.

D4 (BSW-04). In metasystem/internal/steward/openwork.go, add the branch
for a machine whose only claim is fenced: same classification as today (no
live work owned), reason line "the only claim held here is breach-stopped:
<id> (stop <stopId>); it waits on a human resume, and the queue is open".
Fixture beside the existing open-work tests.

D5 (BSW-05). Add the fixture for the branch the change exists for: a live
claim and a fenced claim on the same machine; the verdict display carries
the live goal's current line and the FENCED line. Deleting the FENCED
append in that branch must turn this test red.

D6 (BSW-06). Give the three new display appends in turnverdict.go the same
nil guard the branch above them applies. Cosmetic; allowed because the
lines are touched.

D7 (BSW-07). Not this round. The steward's delivery health counting a
fenced claim's age is an inherited hazard the orchestrator has sent to the
backlog. Do not touch metasystem/internal/steward/delivery.go.

D8. Boundary: metasystem/internal/goal (goalverbs.go; stop.go, the resume
mutation only, nothing else in that file; turnverdict.go; and tests),
metasystem/cmd/metasystem/goal.go and its tests,
metasystem/internal/channel/report.go and its tests,
metasystem/internal/steward/openwork.go and its tests. verbs.go is not in
the boundary and clearClaimBinding stays byte-identical. Anything beyond is
a gap, not a change.

D9. Source comments describe the application, never this round, the
critique, or a finding id.

# Verification

Required, run from the worktree and reported at evidence level ran:
go test ./internal/goal/ ./internal/steward/ ./internal/channel/
./cmd/metasystem/ (the goal package in full), then
metasystem/scripts/agents/go-gate.sh --fast, then
metasystem/scripts/agents/goal-cli-fixtures.sh. State which fail before
your change. The orchestrator re-runs the bed outside the sandbox and proves
the live seat after landing.

# Constraints

Wall-clock budget: 90 minutes. Return per the implementer schema with the
diff boundary listed. Gap rule: stop and report a gap; never fill it
silently. Never weaken or delete an existing test to pass.

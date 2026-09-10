Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal breach-stop-wedges-seat)
Date: 2026-09-09

# Goal

Goal breach-stop-wedges-seat (approved by Wido 2026-09-06, appetite 1h).
Its record, metasystem/plans/goals/breach-stop-wedges-seat.md, is the
contract. In one sentence: a breach-stopped claim must stop wedging the
whole machine. Today the one-claim-per-machine quota counts a claim that
sits under a StopFence, and every verb that could clear that claim refuses
while the fence stands, so one budget breach freezes the seat until a human
resumes the breached goal. DONE, from the record: a breach-stopped goal does
not hold the quota slot, with the fence on RESUMING that goal preserved
exactly as it is.

# Facts

- The quota rule is in metasystem/internal/goal/validate.go, the block
  that begins with the comment "Quota: one claim per machine, tree-wide".
  It collects every live goal whose State is claimed and whose Claimed
  binding is set, groups by machine, and refuses a machine with more than
  one claim outside a single arc. It does not look at StopFence.
- The fence is enforced by clearClaimBinding in
  metasystem/internal/goal/verbs.go: every claim-clearing verb (release,
  park, done, unapprove, steal, and the arc forms) calls it, and it refuses
  while StopFence is set with "only goal resume may clear its launch
  fence". Resume is the one verb that clears a fence, and it is a human act.
  None of that changes.
- Live specimen, this week: goal account-provenance was breach-stopped on
  2026-09-07 while claimed by an m1d session that later died; m1d could not
  claim anything until a forced human landing cleared it on 2026-09-09.
  The refusal text was: "machine m1d claims account-provenance,
  landing-receipt-survives-records-drift: the quota is one claim per
  machine (one arc counts once)". The full account of the locked doors is
  metasystem/records/misc/account-provenance-resume-is-wedged.md; the
  original 2026-09-01 incident is metasystem/records/misc/idle-loss-2026-09-01.md.
- The existing test
  TestBreachStopFenceAndHumanResumeAreOneWayTransactions in
  metasystem/internal/goal/stop_test.go already fixtures a breach-stopped
  claim end to end (open, claim, CloseStop, then park and done refused).
  Its helpers (twoClones, seedLedger, verbReq, claimApprovedForTest,
  CloseStop, Project) are the ones to reuse.
- The ledger is shared by every machine and validated on every mutation
  and every fetch, so the validator's answer must be identical for every
  engine reading the same tree; the change is one predicate, not a new
  state.

# Decisions (the orchestrator's; decided, not open)

D1. Take the first branch of DONE, the narrowest one: a claim that sits
under a standing StopFence does not count toward the one-claim-per-machine
quota. In the quota block of validate.go, skip a goal whose StopFence is
non-nil when collecting claims by machine. Nothing else in validation
changes. The claim binding, the StopCapability and the StopFence all stay
on the fenced goal exactly as today; the goal still reads as claimed by the
dead or breached holder; resume is still the only verb that clears the
fence and still needs the human.

D2. Do not take the second branch (making release lawful on a stopped
claim). It would touch clearClaimBinding, which is the fence itself, and
the record's constraint is that the budget law stays intact and only the
quota interaction changes.

D3. Fixture, in metasystem/internal/goal, as a new test beside the one
named above: open and claim goal A on machine mac-a, breach-stop it through
CloseStop so it carries a StopFence, then open goal B and claim it on the
same machine mac-a; the claim must be confirmed, and the tree must
validate. Then assert the fence still holds on A: park and done on A are
still refused with "only goal resume may clear its launch fence". Then
assert the quota still bites where it should: with A fenced and B claimed,
a claim of a third goal C on mac-a is refused with the quota text. Give the
test a name that says what it proves in plain words.

D4. Boundary: exactly metasystem/internal/goal/validate.go and one new
test file (or new test functions in an existing test file) under
metasystem/internal/goal. Anything beyond that is a gap, not a change.

D5. If the quota block turns out to be duplicated elsewhere (a second
place that counts claims per machine, for instance in the claim verb's own
pre-check or in the steward's idle logic), do not change it; report it as
a gap with the file named. The orchestrator decides whether it is in
scope.

D6. Source comments describe the application, never this round, a
critique, or a finding id. The comment on the quota block should say why
a fenced claim does not count: a breach-stopped goal is waiting on a human
and must not keep the machine from taking the next item.

# Verification

Required, run from the worktree and reported at evidence level ran:
go test ./internal/goal/ (the whole package, including the new test and
TestBreachStopFenceAndHumanResumeAreOneWayTransactions unchanged), then
metasystem/scripts/agents/go-gate.sh --fast, then
metasystem/scripts/agents/goal-cli-fixtures.sh. State in the return
which of them fail against the tree before your change (the new test
should fail before and pass after; everything else should pass both
times). The orchestrator proves the live seat behaviour after landing;
do not claim it from the sandbox.

# Constraints

Wall-clock budget: 60 minutes. Return per the implementer schema with the
diff boundary listed. Gap rule: stop and report a gap; never fill it
silently. Never weaken or delete an existing test to pass.

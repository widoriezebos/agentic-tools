Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal breach-stop-wedges-seat)
Date: 2026-09-09

# Review brief: fresh independent critique of the final tree (chain bsws-build1b-20260909, work round bsws-build1b-20260909-r2)

FINDING IDS: chain-unique, BSW-01, BSW-02, ... never F-n.

Why this review exists: chain completion under DESTRUCTIVE-REACH requires
a fresh-context critic whose reviewed job is the final work round. This
change edits the shared ledger's validator, so every machine's engine must
agree with it. Round 1 stopped, correctly, on a scope gap: the one-claim
quota was not the only place a breach-stopped claim owns a machine; the
goal-next frontier, the turn verdict's Current goal and the idle branch
that hands a goal to the steward all did too. Round 2 built one predicate
and applied it at every site. This chain examines the final tree in a
fresh session and closes the build chain.

Round budget: 1 focused round. A finding is material only if it changes
what gets built and names the artifact it would change.

# What was built (the contract, in plain words)

The goal record is metasystem/plans/goals/breach-stop-wedges-seat.md. Its
DONE: a breach-stopped goal does not hold the machine's one claim slot,
with the fence on RESUMING that goal preserved exactly as it is. The
orchestrator's decisions, carried in the reviewed job's two briefs (in the
job packet you were given), were:

- One predicate, one owner: GoalFile.IsFencedClaim in
  metasystem/internal/goal/file.go (State claimed, Claimed set, StopFence
  set) means "a claim under a standing StopFence is not this machine's
  live work".
- Sites, all in metasystem/internal/goal: the quota block of validate.go
  skips fenced claims; Next in project.go routes them to a new
  NextVerdict.Fenced list that SelectNext ignores; convertedGoalFacts in
  turnverdict.go never makes one the Current goal; the idle branch in
  turnverdict.go therefore falls through to the queue head; the verdict
  display carries one FENCED line per fenced same-machine claim.
- Left alone on purpose: clearClaimBinding and resume in
  metasystem/internal/goal/verbs.go (the fence law), brainSummary (a
  human-facing held-claims count), legacyGoalFacts (the legacy world).
- Fixtures in the new test file the round added, breach_stop_quota_test.go
  under internal/goal in the reviewed tree (it exists only there until the
  chain lands): quota
  release with the fence preserved and park/done still refused and a third
  claim still refused; goal-next selection; current-goal suppression with
  the FENCED line and idle continuation to ready work.

The live specimen that motivated it, with every refusal verbatim:
metasystem/records/misc/account-provenance-resume-is-wedged.md.

# Mandate

Read the diff of the reviewed round against its base and the whole of
the four touched production files, then answer these, each with the file
and line you read:

1. The fence law is untouched. Confirm by diff that no line of
   clearClaimBinding, the resume mutation, or the breach-stop writer
   changed, and that the existing test
   TestBreachStopFenceAndHumanResumeAreOneWayTransactions in
   metasystem/internal/goal/stop_test.go is byte-identical.

2. Every place that decides "this machine's live work" goes through the
   predicate. Search the package and its consumers (metasystem/cmd/metasystem,
   metasystem/internal/report, the steward's code under
   metasystem/internal) for any remaining test of the shape "State is
   claimed and Claimed.Machine equals this machine" that feeds a
   scheduling, continuation or current-goal decision rather than a count.
   The builder's return claims outside consumers inherit the fix through
   SelectNext and ReadClaimableBudgetedWork; verify that claim rather than
   accept it.

3. The other direction. With a fenced claim no longer counted, one machine
   may lawfully hold a fenced goal and a live goal at once. Read resume in
   metasystem/internal/goal/verbs.go and the stop-batch verification beside
   it in metasystem/internal/goal/stop.go: when the human resumes the fenced
   goal while the seat holds another live claim, does the re-bound claim
   now trip the quota and refuse the resume? If so, say whether that is a
   clear refusal the human can act on (release the other goal first) or a
   second wedge, and name the exact text they would see. This is the
   question the orchestrator most wants answered.

4. The validator change is fleet-wide. A tree with one fenced and one live
   claim on the same machine is now valid; find any other validator rule,
   accounting projection or arc computation in validate.go, project.go or
   budget code that assumed at most one claimed goal per machine, and say
   whether it still holds.

5. Display coverage. fencedClaimDisplay is appended in the "ok" and
   "queued-only" branches of decide. Say whether a seat whose root declares
   goal-free, or whose ledger reads degraded, loses the FENCED line and
   therefore the visibility of a human-owed resume; grade it.

6. The fixtures prove DONE and are deterministic. convertedGoalFacts
   iterates a map; say whether any new test depends on iteration order,
   and whether the three tests would catch a regression at each of the
   four sites (name the site each assertion guards).

# Constraints

Read-only: you edit nothing, you run only tests and reads. Return per the
code-critic schema with every finding carrying a file and line, a
severity, and the artifact it would change. Wall-clock budget: 45
minutes. Plain English in every human-visible field.

# Orchestrator runs

Outside the sandbox on the reviewed tree: the three new fixtures and the
existing fence test passed together in 19 seconds; the goal-cli fixture
bed is running now (the two scenarios the sandbox could not prove,
brain-human-word-refuses and wrong-terminal, are the ones to watch). The
landing receipt runs go-gate --fast and the beds again. Do not repeat the
full goal package (nine minutes) unless a finding needs it.

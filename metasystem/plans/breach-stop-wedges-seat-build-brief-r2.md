Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, follow-up under goal breach-stop-wedges-seat, round 2 of job bsws-build1b-20260909)
Date: 2026-09-09

# Goal

Same goal, same contract: metasystem/plans/goals/breach-stop-wedges-seat.md.
Your round-1 return stopped on the gap the round-1 brief's D5 anticipated:
the one-claim quota is not the only place a fenced claim owns the machine.
You named two more, in metasystem/internal/goal/project.go (the goal-next
frontier puts every same-machine claim in Claimed, and SelectNext continues
on it) and metasystem/internal/goal/turnverdict.go (convertedGoalFacts makes
the first same-machine claim the Current goal). The orchestrator has read
both and a third in the same file: the idle branch that prints "selected
this machine's held goal ... without another claim attempt" hands the fenced
goal to the steward as a continuation target. All of them are in scope now.
The decision is one predicate applied everywhere the machine's live work is
chosen.

# Facts

- Everything you found is in package metasystem/internal/goal. Nothing
  outside it changes in this round.
- The fence and its law do not change: clearClaimBinding in
  metasystem/internal/goal/verbs.go still refuses every claim-clearing verb
  while StopFence is set, and resume stays the one human act that clears it.
- The live specimen this week, with every refusal verbatim:
  metasystem/records/misc/account-provenance-resume-is-wedged.md. The seat
  that hit it saw, in this order: the quota refusal on claim, the verdict
  naming the fenced goal as current work and quoting its stale next step ten
  turns running, and five steward continuation intents minted at it, none
  runnable.
- metasystem/internal/goal/stop_test.go,
  TestBreachStopFenceAndHumanResumeAreOneWayTransactions, fixtures a
  breach-stopped claim end to end; its helpers are the ones to reuse.

# Decisions (the orchestrator's; decided, not open)

D1. The predicate: a claim under a standing StopFence is not this
machine's live work. Add one small method on GoalFile in
metasystem/internal/goal/file.go that says exactly that (State claimed,
Claimed set, StopFence set), and use it at every site below rather than
repeating the three-field test.

D2. Sites, all in metasystem/internal/goal:
  (a) validate.go, the quota block: a fenced claim is not collected into
      the per-machine claims. The claim binding, StopCapability and
      StopFence stay on the goal untouched.
  (b) project.go, Next: a fenced same-machine claim goes into a new
      `Fenced []string` field on NextVerdict, in backlog order, instead of
      into Claimed. SelectNext ignores Fenced, so a machine whose only claim
      is fenced gets NextSelectionReady on the queue head.
  (c) turnverdict.go, convertedGoalFacts: a fenced claim is never the
      Current goal. A machine whose only same-machine claim is fenced reads
      as it would with no claim at all (queued-only, or goal-free when the
      root declares it).
  (d) turnverdict.go, the idle branch: the held-goal candidate must not be
      a fenced claim. With no unfenced held goal, take the existing path
      that selects the queue head and defers its claim to the steward tick,
      exactly as for a machine holding nothing.
  (e) turnverdict.go, display: where the verdict prints the current goal or
      the "no current goal" line, add one line per fenced same-machine
      claim, in plain words: FENCED <id>: breach-stopped by <stopId>
      (<reason>); only goal resume, a human act, clears it; the queue is
      open. Nothing else in the display changes.
  (f) Leave brainSummary alone: it counts held claims for a human-facing
      count, and a fenced claim is still held. Leave legacyGoalFacts alone:
      it serves the legacy single-file world.

D3. Fixtures, in metasystem/internal/goal beside the existing tests:
  (i)   validate: open and claim goal A on mac-a, breach-stop it through
        CloseStop; open goal B and claim it on mac-a: confirmed, tree
        validates. Then park and done on A still refuse with "only goal
        resume may clear its launch fence". Then a claim of a third goal C
        on mac-a refuses with the quota text.
  (ii)  project: with A fenced and B approved and ready on mac-a, Next
        reports A under Fenced and not under Claimed, and SelectNext
        returns Ready B.
  (iii) turnverdict: with A fenced as the only same-machine claim,
        convertedGoalFacts returns no Current goal, and the verdict display
        carries the FENCED line for A. If driving the idle branch (d) from a
        test in this package needs machinery the package's existing verdict
        tests do not have, cover (c) and (e) and report the idle-branch
        fixture as a gap with the seam named; do not build a harness.
  Give each test a name that says what it proves in plain words.

D4. Boundary: metasystem/internal/goal/validate.go, project.go,
turnverdict.go, file.go, and test files in that package. A consumer
outside the package that switches on NextVerdict or GoalFacts and now
needs the Fenced list (a report, the steward, a CLI printer) is not edited
in this round: report it as a gap with the file named.

D5. Source comments describe the application, never this round, a
critique, or a finding id. The one sentence to carry at each site: a
breach-stopped goal is waiting on a human and must not keep the machine
from taking the next item.

# Verification

Required, run from the worktree and reported at evidence level ran:
go test ./internal/goal/ (the whole package; the existing fence test
unchanged and green), then metasystem/scripts/agents/go-gate.sh --fast,
then metasystem/scripts/agents/goal-cli-fixtures.sh. State which of them
fail against the tree before your change. The orchestrator proves the
live seat behaviour after landing; do not claim it from the sandbox.

# Constraints

Wall-clock budget: 90 minutes. Return per the implementer schema with the
diff boundary listed. Gap rule: stop and report a gap; never fill it
silently. Never weaken or delete an existing test to pass.

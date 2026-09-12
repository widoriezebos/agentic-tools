Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-message-truth)
Date: 2026-09-10

# Review brief: the Stop message's judgement of open work (chain stop-truth-build1-20260910)

FINDING IDS: chain-unique, SMT-01, SMT-02, ... never F-n. Report `round`
as 1 in your return: it is this job's own round. One focused round.

Why: the goal record metasystem/plans/goals/stop-message-truth.md
traces the defects; the build brief (in the plans directory under this
goal's name) binds three: a terminal run is never hung and a hung
warning respects acknowledgement (internal/report/scan.go,
internal/goal/turnverdict.go); a held claim or a live proof attempt is
not idle, so the three-strike IDLE WITH BACKLOG counter stops firing
during a seat's own landings (turnverdict.go); the brain summary lines
are capped in width so the display stays within its 4,000-rune bound
(internal/goal/verdictrender.go).

Threat model: a run still hung after its status went terminal by any
path; an acknowledged hung run still printing; prune treating
hung-flagged terminals differently; the idle rule reading a proof
attempt as live after its deadline or terminal block; a held claim
silencing a real idle forever (the steward must still get its turn when
the claim's goal has nothing in flight for the bound the record
names); the counter no longer resetting on a backlog change; the brain
cap dropping names from the verdict FILE rather than the display; any
change to the printed shape beyond the cap; anything outside the
boundary.

Scope: the computed diff of the implementer job. Contract: the goal
record and the build brief.

# Mandate

1. The three rules hold as written with tests that would fail on the
   2026-09-09 shapes (27 green hung runs; refusals counted during a
   landing; forty-claim brain seat).
2. Nothing outside the boundary changed; the printed shape is
   unchanged except the brain cap.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
the reviewedTree from the review record beside the computed diff (the
conformance validator refuses from a sandbox; say so). The orchestrator
runs the packages, the goal-cli bed and the hook suite on the seat.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.

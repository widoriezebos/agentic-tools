Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal metasystem-stop-verb)
Date: 2026-09-08

# Fold brief: round 26 of chain stopverb-build1, the sixth code read

The sixth code read (job stopverb-crit10, register
metasystem/records/misc/metasystem-stop-critique-r9.md) returned five
material findings on the round-25 tree. All five are accepted. This
round folds them and nothing else.

Three of the five are in code the last two folds added, and two of those
are regressions of the rule the fold was written to satisfy. So the
standing instruction for this round is stricter than usual: every
correction below lands with a test that fails before it and passes
after, in the package that owns the behaviour. A fold that adds a
promise adds its check in the same round.

The specification is metasystem/plans/metasystem-stop-verb-design.md.
Sections 16, 17 and 18 are the newest and win where they contradict
earlier text. The goal record
metasystem/plans/goals/metasystem-stop-verb.md carries Wido's durability
requirement in his own words. The previous reads and their dispositions
are metasystem/records/misc/metasystem-stop-critique-r4.md through
-r9.md. Decisions D1 to D107 are the orchestrator's from the earlier
briefs and stand unless one of them is wrong.

# The corrections

## D108 - SVC10-01 - the suite phrase belongs only to a stopped suite's host

The run line may carry the words "(suite stopped)" only for a run that
the suite-stop join actually names, and a run line carries exactly one
final verdict. Today the helper appends the phrase on any terminal
outcome status, including the ordinary path, which then appends its own
verdict directly after it with no separator.

Fix both halves: link the phrase to the record of suites this stop
actually stopped, and make the composition produce one final line per
identity as section 18.2 requires and section 6's grammar spells out.

The test is the critic's own probe, in
internal/stoptransition: a run whose record became terminal
before the run step reached it, with an empty suite-stop join, produces
one line, with one verdict, and no suite phrase. Add the paired case
too: a run that did host a stopped suite still gets the phrase.

## D109 - SVC10-02 - the printed verdict is the verdict that was written

The line for a wrapped run that stop ended with a signal must name the
status the conclusion actually wrote, not the literal words "concluded
ended-unknown". When the workload's exit sidecar survived and the
conclusion terminalised the record red or green, the report says so.
Read the record after the conclusion, or carry the concluded status back
in the outcome; either is acceptable, the constraint is that the printed
status and the durable record agree, which is section 15.1 in both
directions.

Test it in metasystem/internal/run or
internal/stoptransition, whichever owns the seam you choose.
If the run family's construction of its own store blocks a test, that
is a seam worth opening: say so in the return if you cannot.

## D110 - SVC10-03 - arm classifies its caller unconditionally

This one touches Wido's requirement directly, so it is the most
important correction in the round. The arm verb currently skips the
human-terminal classifier whenever the caller passes a temporary human
word with a review date, and then opens the fence and starts every ring.
Any process that can run the engine binary in a stopped checkout can
therefore clear a human stop by inventing a word and a date.

The classifier gate becomes unconditional: arm classifies its caller
first, whatever flags follow, exactly as the steward arm verb already
refuses under a closed fence before it reaches its own temporary-word
branch. The temporary word may still change what arming does afterwards;
it may never be the reason the fence opens.

Test in metasystem/cmd/metasystem: arm with a temporary word and a
review date, from a caller the classifier does not accept, refuses, and
the fence record is unchanged after the refusal.

## D111 - SVC10-04 - a held bookkeeping failure is never dropped

The supervision family holds the shutdown's bookkeeping failures and
attaches them to one anchored component outcome. Two earlier returns,
taken when the anchored component has no matching outcome in the
shutdown report, return before the attachment and before the flag that
records the failures as reported is set.

Make the rule absolute as section 17.5 states it: on every exit from
that function, a held failure is printed, recorded in the survivor
entry, and able to set exit 1. Restructure rather than patch each return
if that is cleaner.

Test the missing-anchored-outcome path in
internal/stoptransition: a shutdown that stops its processes
and fails its registry append, whose anchored outcome is absent, still
prints the failure, records it, and exits 1.

## D112 - SVC10-05 - up describes the phase, as health does

Under a closed fence the up path reports the component as stopped with a
detail that carries only the time of the change, whatever the phase.
Section 18.3 requires the description itself to distinguish the phases:
stop incomplete with the recorded unresolved count and the stop remedy,
or stop unfinished with the stop remedy. The health path already does
this. Make up match it, and change
internal/up/stopfence_test.go, which currently pins the
deviation, to assert the phase-sensitive description instead.

## D113 - the mission loop's fence reads stay as they are

The read noted, without raising it, that the mission loop reads the
fence at start and once after publishing but not on every iteration.
That is what section 13.1's handshake asks for and it is not a defect.
Record it in the return as a gap so the next reader does not spend time
on it. Do not change the mission loop.

## D114 - the declared gaps are restated

The return names, as gaps, every unbuilt slice-1 acceptance scenario
(proof-run-stop, remote-job, slow-owner, crash-recovery,
ignored-signal), the slice-2 fleet scenario, and the dispatch creation
claim that can be orphaned if the dispatch process dies between
replacing its cleanup trap and closing the claim explicitly. None of
them is fixed in this round.

# Scope

Nothing outside these five corrections and their tests. No test is
weakened or deleted to make a correction pass. No new design text: every
correction above is the design as written, applied to code that drifted
from it.

# Verification

Run, and report each result in the return:

- `scripts/agents/go-gate.sh --fast`
- `go test -count=1 -timeout 40m ./internal/stoptransition/ ./internal/run/ ./internal/up/ ./internal/supervise/ ./cmd/metasystem/`

Note that `cmd/metasystem` has one test failing on main, unrelated to
this chain: TestGoalTemporaryAuthorityRefusesPastAndBeyondHorizon fails
because the temporary-authority horizon passed on 2026-09-06. Goal
fixture-review-by-date-rolls-over owns it. Report it as pre-existing and
do not touch it.

The orchestrator runs the supervision, dispatch, goal-cli, mission and
suite-progress beds outside your sandbox. Do not attempt them, and do
not treat a sandbox failure as evidence about the diff.

Gap rule: stop and report a gap; never fill it silently.

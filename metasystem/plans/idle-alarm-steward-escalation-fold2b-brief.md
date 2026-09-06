Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, follow-up round under goal idle-with-backlog-alarm)
Date: 2026-09-06

# Correction round 2 for chain idle-escalate-build1 (supersedes the round-2 brief landed at 1a380e62, which cited a file this worktree does not yet carry)

The code critic (job idle-escalate-crit1, register round 1) returned
three material findings (IGE-01 the escalation gated on the session-stop
marker, IGE-02 the escalation swallowing another branch's block, IGE-03
network work inside the four-second Stop child) and four notes (IGE-04
crash window, IGE-05 the one-claim-per-machine quota, IGE-06 the
unreadable-ledger branch never counting, IGE-07 seat-idle episodes never
clearing). The orchestrator accepted all three material findings and
folds the notes as decided below. The build brief
metasystem/plans/idle-alarm-steward-escalation-build-brief.md stays the
contract except where the decisions below amend it.

# Step zero: merge main

Main moved on this chain's own files while round 1 ran: landing c1525b90
("The Stop hook's refusal carries the turn verdict", another seat's
chain) changed metasystem/scripts/agents/supervision-hook.sh (about two
hundred lines), metasystem/internal/goal/turnverdict.go,
metasystem/internal/report/stopblock.go and its test, and
metasystem/cmd/metasystem/report.go. Before any correction: commit the
round-1 state on this worktree's branch as one work-in-progress commit,
merge main (`git merge main`), and resolve every conflict so that BOTH
behaviours survive: that landing's refusal-carries-verdict contract and
this chain's bounded escalation. Run that landing's tests and this
chain's after the merge. Report the merge commit and each conflicted
file in the return.

One conduct point first: the round-1 return said no contract
contradiction was found, yet the one-claim-per-machine quota
contradicts D2's claim whenever the seat already holds a claim, and the
test scenario holding a claim was removed instead of reported. The gap
rule is: stop and report. This round restores that scenario as a
positive case (D9).

# Decisions (the orchestrator's; decided, not open)

D7. The escalation never depends on the session-stop marker. A stale,
mismatched or expired marker remains display-only as before the change;
escalationAllowed and every "separate SESSION STOP refusal" text go
away. On the third unchanged stop the escalation clears ONLY the idle
block: if another branch (open work, unwatched work, queue change)
blocked on this same stop, that block stands and its text is shown, and
the escalation's local records are still written.

D8. No network work in the verdict. The third stop does only local,
bounded writes: it mints the steward continuation intent (PrepareIntent
path, reason seatIdle), records the incident, saves the verdict state,
and returns success. It never calls the goal Claim verb. The steward's
tick, when it consumes a seatIdle intent, performs the claim when one is
needed (D9) as the seat's actor recorded on the intent (machine plus
lineage plus claim epoch, resolved at the hook as today), then
launches the continuation; a refused claim ends in the human alarm the
steward already raises, with the refusal text in the alert.

D9. Which goal the intent names. If this machine already holds a claim,
the intent names that held goal and the steward attempts no claim: the
continuation continues the seat's own claim, which is what the
continuation role exists for. If the machine holds no claim, the intent
names the first claimable goal and carries a `claimNeeded` mark; the
steward claims it first per D8. The intent record gains the fields it
needs for this (goal, claim needed, seat actor, claim epoch) with
comments stating the invariant each protects.

D10. The unreadable-ledger branch counts too: it stores the sentinel
digest "ledger-unreadable" and, under stop_hook_active, counts repeats
like any other; at three it records the incident and raises the human
alarm (no intent can name a goal it could not read) and returns
success. The hook's comment states the bound and the escalation
truthfully; it never claims that a provider cap is not involved in any
case.

D11. Seat-idle alert episodes: a repeat for the same session and
backlog updates the episode's incident details and refusal count; the
continuation's reap clears the seat-idle episode whose intent it
closed. Nothing else in the alert lifecycle changes.

D12. Pins, added to D6's: verdict tests for D7 (a stale marker still
escalates; an open-work block on the third stop stands while the intent
and incident are written), D8 (the verdict makes no claim and no
network call: inject a Claim seam that fails the test if called), D9
(held claim named without a claim attempt; no held claim marks
claimNeeded), D10 (three unreadable stops escalate to incident and
alarm). Steward tests: a seatIdle intent with claimNeeded claims as the
recorded actor and then launches; a refused claim raises the alarm and
launches nothing; reap clears the episode. One command-layer test that
drives `report turn-verdict` through the real closures in a fixture
repository for the held-claim path (the critic's named gap). Hook bed:
the third stop's success path with an intent on disk.

# Gate

As the build brief's gate, plus `go test ./cmd/metasystem/ -count=1`.
Report each with its evidence level.

# Constraints

Wall-clock budget: 60 minutes. Only the changes above and nothing that
weakens a round-1 pin. Declare the boundary as every file that differs
from main. Gap rule: stop and report a gap with your proposed contract
written out; never fill it silently, and never remove a test scenario
to make a contradiction disappear.

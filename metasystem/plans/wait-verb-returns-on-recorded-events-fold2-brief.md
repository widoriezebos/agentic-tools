Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Round 3 of the build of member 1: close the three gaps round 2 named and
fold the Opus read of round 2 (its findings are listed below when this
brief is dispatched). The goal, the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md and the
return schema are those of round 1's brief, which the dispatcher already
gave you; the workspace is round 2's widened one. Build on the worktree
as round 2 left it; do not commit.

# The three gaps

1. Every Git read a goal wait makes obeys the remaining wait deadline:
   endpoint resolution, the acceptance-gate validation, commit validation
   and change inspection in metasystem/internal/goal/attention.go and the
   helpers it calls run under the wait's context, so a hung Git process
   is cancelled when the deadline or the ten-second fetch ceiling passes.
   TestWaitGoalFetchDeadline proves a hang in each of those stages returns
   65 within the bound, with an injected Git runner.
2. TestWaitGoalLandingAndHumanAct and TestWaitChannelAnswer run the full
   two-clone sequence through the installed waiter: a landing and an
   answer recorded between the cursor capture and the wait's
   registration are found at entry; the wrong-question answer leaves the
   wait pending and the right one ends it; local-only commits, wrong
   authority, lost history and a rewind are rejected with the named
   source failure.
3. TestWaitRestartRecoveryReplay drives all three WAITING routes end to
   end: the session-start hook's `start` event (through the engine's
   `session start` verb), `goal next`, and `report turn-verdict`, each
   printing the same line for a pending row and none for a cleared one.

# The proof's one red

The seat's proof of round 2 (the whole battery on your worktree) was red
in one place: the brain seam-coverage scenario of
metasystem/scripts/agents/brain-fixtures.sh keeps an allow-list of every
`goal.Actor{` and `classifyVerbCaller(` site in the engine, and the new
site in wait_verb.go (`view, err := classifyVerbCaller(stateRoot,
waitCallerPID())`) is not in it. The site is right (registration
classifies its caller); add its line to the allow-list, in LC_ALL=C sort
order (after the run.go lines), exactly as the scenario prints it.
Everything else in the battery passed.

# The Opus read of round 2: ten material findings, each with the seat's decision

WVB-01 (high). A human-act wait returns 6 on its first observation
whenever any other goal is claimable, because the claimable frontier is
read as a level. Decision: the trigger is a CHANGE. Registration saves
the set of claimable goals as it stands (their ids and revisions) beside
the open-work signature; the observation loop returns 6 only when a goal
becomes claimable that was not in the saved set (or a saved one changes
revision). A backlog that already existed at registration is the seat's
business at its next turn end, where the stop gate's idle rule applies as
today; it never ends the wait. The seat's own held claim is never in the
set.

WVB-02 (high). Recovery refuses after a lease takeover because the
WAITING enumeration and --resume require an exact owner-lineage match,
and a new session mints a fresh lineage. Decision: succession is proved
through the checkout lease records: the new session may enumerate and
resume rows whose owner lineage is the one its lease took over from (the
lease record's previous holder and the takeover epoch), on the same
machine; anything else stays refused. Test the takeover case in
TestWaitRestartRecoveryReplay with a real lease takeover in the bed.

WVB-03 (medium). A pending row whose by-id pointer is missing can never be
resumed while session start still advertises it. Decision: implement the
page's fallback: --resume looks the row up by its key when the pointer
is missing (the WAITING line carries kind, target and owner digest for
that), rewrites the pointer under the lock, and reports the repair in
its result.

WVB-04 (medium). The adapter wait-delivery call at registration is
bounded only by the whole wait deadline and runs before any durable row
exists. Decision: bound it by ten seconds (the same ceiling as a fetch);
a hung or failed adapter is 65 with the adapter named; keep the call
before pending is published, as the page says, but write the row's
identity first in state `registering` so a crash mid-call leaves a
visible row that the next registration replaces.

WVB-05 (medium). Transient infrastructure failures and an interrupt race
are reported as 6. Decision: 65 for infrastructure (storage, transport,
adapter), 130 for an interrupt observed at any point, 6 only for the
change rule of WVB-01 or an ownership change; add a test per exit.

WVB-06 (medium). One unreadable waiter row makes WaitingLines return no
lines and both callers swallow the error. Decision: print every readable
row's line, then one line per unreadable row naming the file and the
read error; the callers never swallow it.

WVB-07 (medium). TestWaitAttemptAfterDrain observes the same record
twice. Decision: it must end a worker (or fake its death through the
injected reader), leave the terminal unpublished, observe the wait
pending, then publish a valid Terminal with EndedAt equal to Terminal.At
and observe the return.

WVB-08 (medium). The five bed legs only re-run the unit tests. Decision:
wait-restart kills the waiter process for real and resumes from the rows;
wait-bounds runs a local Git that hangs and observes 65 within the bound;
wait-job-run and wait-proof drive the installed verb (bin/metasystem in
the bed) against a real job record and a real attempt record; wait-ledger
drives the installed verb against the two-clone bed. Go still owns the
assertions; the shell only launches.

WVB-09 (medium). The answer predicate demands a Goal-Transaction trailer
join the contract does not state, and a missing trailer ends the wait
with 4. Decision: the answer predicate is the accepted answer History row
naming the question, nothing more; a missing or unparsable trailer on an
unrelated commit leaves the wait pending.

WVB-10 (low). runCaller now records OwnerLineage instead of MainId on
every run record. Decision: revert that; the wait verb reads what it
needs without changing run-ownership state.

Not material, for your awareness only: WVB-11 (older engines reject the
new `question=` History token until they rebuild: accepted, every seat
rebuilds after the landing), WVB-12 (`channel wait` drops the answer
text: print it in the result), WVB-13 (dead code around the unbuilt
accelerator: remove it), WVB-14 (landing detection walks first parents
only: note it on the page; member two's hint covers the merge case).

# Constraints, Expected Return, Acceptance Criteria, Gap Rule

As in round 1's brief. Wall clock: 60 minutes; a partial round returns
with its tests green for what exists and names what is left.

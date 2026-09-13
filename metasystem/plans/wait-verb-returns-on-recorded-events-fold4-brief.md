Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Round 5, the last fold round of member 1 before the seat's own landing:
fold the Opus read of round 4 (eight material findings) and the three
specification gaps round 4 named, each decided below. The goal, the
design page metasystem/plans/coordinator-wakes-on-events-not-polls-design.md
and the return schema are those of round 1's brief; the workspace is
round 2's widened one plus the shipped channel fixture bed named under
WVB-26. Build on the worktree as round 4 left it; do not commit.

# The read's findings and the seat's decisions

WVB-26 (high). `channel wait` changed its flags: `--timeout` became a Go
duration (it was an integer count of minutes) and `--poll-seconds` was
removed, so the shipped channel fixture bed's three assertions exit 2.
Decision: keep the public surface exactly as it was: `--timeout` is
minutes, `--poll-seconds` is the provider poll cadence inside the wait
loop (bounded by the ten-second observation cycle); the wait's deadline
is derived from the minutes. The channel fixture bed must pass unchanged.

WVB-21 (high). TestWaitChannelAnswer fails outside this machine: it
needs the machine nickname (`git config metasystem.goal.machine`) that
the seat's checkout has and a fresh proof bed does not. Decision: the
test sets the nickname in its own fixture repository, as the other cmd
tests do; it must pass in a clean clone.

WVB-27 (high). A human-act observation projects the whole ledger tree
once per intervening accepted change from the ORIGINAL cursor, so the
cost grows with the wait's age and a 24-hour wait stops completing
observations after about six hours. Decision: the page already allows
it: keep the original cursor as the match floor, but each observation
inspects only the accepted changes since the last checked tip (saved in
the row), never re-projecting states already inspected. Test: an
observation after N new accepted changes costs O(N) reads, and the same
observation repeated with no new changes reads nothing but the tip.

WVB-22 (medium, not resolved in round 4). Decision unchanged: test the
exit-6 route through the installed verb (register with a saved claimable
set, make a new goal claimable, observe 6; no change, observe pending).

WVB-29 (medium). The channel provider poll runs inside the source
observation, so a transient provider failure ends the wait with 65.
Decision: the poll is not a source: its failure is recorded in the row
(last poll error and time) and the ledger observation proceeds; a
provider outage never ends a wait; only the ledger and the deadline do.
Test a poll failure that leaves the wait pending and a later answer that
ends it.

WVB-28 (medium). The hanging-Git fixture leaves its busy-spin shell
orphaned on its own failure path. Decision: the fixture starts the
spinner in its own process group and kills the group on every exit path
(deferred), and asserts nothing of the spinner survives.

WVB-30 (medium). The actionable check has no freshness grace and reads a
tip whose fetch ref the observation already deleted. Decision: the check
reads the same fetched tip inside the observation, before cleanup, and
shares the observation's 30-second freshness grace; a failed frontier
read is tolerated like a failed fetch.

WVB-31 (low). The session-start hook reports "this session is not the
checkout holder" as a failure to read the wait rows. Decision: a
non-holder session has no rows to advertise and prints no notice.

# The three specification gaps, decided

1. A pending-setup job's incarnation: pin the job id and the
   reservation's opid (present from pending-setup on); round and
   startedAt are adopted from the record at the first observation that
   finds it running, once; an opid change is a replaced source (4).
2. TestWaitChannelAnswer's installed two-clone coverage is provided by
   TestWaitGoalLandingAndHumanAct; say so in the test's comment and in
   the page's fixture row (the seat edits the page).
3. A channel wait's row carries `poll: channel` in its selector; the
   WAITING line for such a row prints `metasystem channel wait --resume
   WAIT-ID`; plain `metasystem wait --resume` on it refuses, naming that
   command. Recovery therefore restores the provider poll.

# Constraints, Expected Return, Acceptance Criteria, Gap Rule

As in round 1's brief. Wall clock: 60 minutes; a partial round returns
with its tests green for what exists and names what is left.

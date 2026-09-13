Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Round 4 of the build of member 1: fold the Opus read of round 3 (eleven
material findings, each with the seat's decision below) and close the
five fixture-depth gaps round 3 named. The goal, the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md and the
return schema are those of round 1's brief; the workspace is round 2's
widened one. Build on the worktree as round 3 left it (the seat's proof
of round 3 was green across the whole battery); do not commit.

# The read's findings and the seat's decisions

WVB-15 (high). Every goal observation reads the ledger tree with one
`git show` per file, at least twice per observation: 8 to 10 seconds on
the real ledger against the 15-second ceiling, so a goal wait cannot
complete its registration observation. Decision: no per-file process.
Read the ledger projection the way `goal next` and `report turn-verdict`
already read it (the accepted-projection loader in
metasystem/internal/goal/project.go, under the wait context), or one
`git cat-file --batch` for the files you need; one or two processes per
observation, never one per file. Add a test that counts the Git
invocations of one observation.

WVB-16 (high). A cancelled or failed observation leaks its private fetch
ref (refs/metasystem/goals/fetch/read-*). Decision: the cleanup runs on
every path with its own short context (five seconds), independent of the
expired wait context; a test cancels mid-observation and asserts no ref
remains.

WVB-17 (medium). ObserveJob answers a `pending-setup` job record with
exit 4. Decision: pending-setup is a pending observation, as the switch
below it already says; test it.

WVB-18 (medium). PendingWaitersForLineages returns every row of every
owner when its allow-list is empty. Decision: an empty allow-list returns
no rows; test it.

WVB-19 (medium). Succession is keyed on MainId while rows are keyed on
OwnerLineage. Decision: the row stores both the owner lineage and the
announced mainId at registration; succession walks the lease takeover
chain by mainId and admits a row whose stored mainId is the one taken
over from, then compares the lineage the announcement record gives for
that mainId; a predecessor with an explicit owner lineage is succeeded
the same way. Test both shapes.

WVB-20 (medium). Two new Go files are owned by no testing.json surface.
Decision: add them to the surface that owns their package's tests.

WVB-21 (medium). `channel wait` no longer drives the provider poll that
produces the answer act. Decision: `channel wait` keeps its own provider
poll inside the wait's ten-second cycle (the read that turns a channel
message into an accepted answer act), bounded by the same deadline; the
plain `metasystem wait` does not poll providers. If the poll's provider
is not configured, `channel wait` refuses at entry with a named reason.

WVB-06 (low, the caller half). The callers of WaitingLines and the
session-start hook still swallow the remaining error path. Decision:
every caller prints the error as one line; the hook prints it in its
notice.

WVB-22 (low). The exit-6 route has no test. Decision: test it through
the installed verb: register with a saved set, make a new goal
claimable, observe 6; make no change, observe pending.

WVB-23 (low). A deadline-expired actionable context reports 65, and the
check's ceiling is 15 seconds. Decision: 124 when the wait deadline
passed during the check; the check's own ceiling is the ten-second
observation ceiling.

WVB-24 (low). --resume repairs the pointer of a row it has not proved it
owns. Decision: the ownership check comes first; repair only an owned or
succeeded row.

# The five depth gaps of round 3

1. wait-ledger launches the installed binary against the two-clone bed.
2. wait-restart kills an active installed waiter process (the real
   `metasystem wait` blocking on a job) and resumes from the rows.
3. wait-bounds launches a deliberately hanging local Git executable on
   the PATH of the wait process and observes 65 within the bound.
4. The session-start hook's `start` event is driven in the bed and its
   WAITING line observed.
5. TestWaitAttemptAfterDrain ends a worker (or proves worker death
   through the injected reader) before the terminal is published;
   TestWaitGoalFetchDeadline's injected runner blocks until the supplied
   context is cancelled, not before. `channel wait` prints the accepted
   answer text in its result.

# Constraints, Expected Return, Acceptance Criteria, Gap Rule

As in round 1's brief. Wall clock: 75 minutes; a partial round returns
with its tests green for what exists and names what is left.

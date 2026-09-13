Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal wait-verb-returns-on-recorded-events)
Date: 2026-09-13

# Goal

Round 7 of the build of member 1: fix the one material finding of the
latest rostered read, WVB-55. The goal, the design page
metasystem/plans/coordinator-wakes-on-events-not-polls-design.md and the
return schema are those of round 1's brief; the workspace is round 2's
widened one, and in practice this round needs only
metasystem/internal/run/waiter.go and its tests. Build on the worktree as
it stands: round 6 plus the seat's renewal-floor changes in waiter.go and
waiter_test.go (a renewal reads from the row's last checked tip; a new
row whose first observation matched keeps the cursor from before the
event; the failure exits keep that floor). Keep those behaviours and
their three subtests. Do not commit.

# The finding, as the reader wrote it

WVB-55 (medium). Replaying a saved result still reads from the wait's
original cursor. A plain `metasystem wait --resume WAIT-ID`, and a resume
with `--timeout` of a saved source result (exits 0 to 4), first checks
the saved evidence by observing the ledger again from
`row.Selector.After` (`replaySavedWait`). The ledger observer walks every
accepted change from that floor until it reaches the matched event,
about 100 ms per change, inside a ten-second context with about five
seconds of Git budget. When the human acted long after the wait began,
the check runs out of time and the replay answers exit 4 "the saved
result no longer matches its source evidence" instead of the recorded
result. The reader reproduced it on a local-mode clone of this
repository: a wait registered 45 changes before a real act found it and
published exit 0; a plain resume returned exit 4 after 5.02 s; with the
cursor 5 changes before the act both resume forms returned 0 in 1.1 s,
with 45 or 60 changes both returned 4 after 5.0 s. Replay is the
recovery path section 2 of the page promises after a restart ("first
replay a saved terminal result after checking its source evidence"), so a
restarted seat is told a real, recorded act is invalid. Note from the
reader: a published match saves the event's own commit as its last
checked tip, so that tip is not a floor below the event.

Two related observations the reader made outside its mandate, in the
same function; check both and fix them if they hold:

- A failed or timed-out evidence read is reported as exit 4 (invalid
  source) rather than as a transport or storage failure.
- A saved exit 6 from a human-act wait records a pending observation's
  tip as its evidence, and replay treats any pending observation as
  invalid evidence, so a plain resume of a saved 6 may answer 4 even with
  a young cursor.

# What the fix must satisfy

- Replaying a saved result checks its source evidence with a read whose
  cost does not grow with the number of accepted changes between the
  original cursor and the matched event, and is bounded as the page's
  other reads are.
- A saved result whose evidence still holds is replayed with its saved
  exit, whatever the cursor's age. A saved result whose evidence no
  longer holds (the matched act or landing is gone from the accepted
  ledger, or the job, run or attempt record was replaced) still answers
  4. A read that fails or times out answers the page's failure exit for
  that case, not 4.
- Rows written by earlier rounds, which lack anything this fix adds,
  still replay correctly. If the fix needs a new field in the version-2
  row, keep `schemaVersion` 2, make the field optional, and name it in
  riskiestPart.
- Tests in metasystem/internal/run/waiter_test.go: a replay whose matched
  act lies many accepted changes after the original cursor returns the
  saved exit with an observer that counts reads (the count does not grow
  with the distance); a replay whose evidence is gone returns 4; a replay
  whose evidence read fails returns the failure exit; a saved 6 replays
  as the page says. Keep every existing wait test green.

# Constraints

As in round 1's brief. Wall clock: 45 minutes; a partial round returns
with its tests green for what exists and names what is left.

# Expected Return

As in round 1's brief: evidence (1) `git -C metasystem status --short`,
(2) `( cd metasystem && go test ./internal/run/ -run 'TestWait' -count=1 )`
and `( cd metasystem && go test ./cmd/metasystem/ -run 'Wait|Channel' -count=1 )`
with their pass lines, (3) `( cd metasystem && scripts/agents/go-gate.sh --fast )`
with its last line. whatWasDone says how the replay now checks evidence,
and what it does for each of the two related observations.

# Gap Rule

stop and report a gap; never fill it silently.

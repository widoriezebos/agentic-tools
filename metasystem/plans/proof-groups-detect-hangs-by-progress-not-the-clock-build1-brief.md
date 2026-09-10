Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-10

# Build brief, slice 1: a group is bounded by what it consumes, never by the clock

Goal proof-groups-detect-hangs-by-progress-not-the-clock. The design is
metasystem/plans/proof-groups-detect-hangs-by-progress-not-the-clock-design.md, revision 3
(the revision-3 decision table governs where it amends Decisions 1, 2
and 4; inventory rows 1 and 2; proof rows 1 to 9 and 12 to 14). Wido's words on the goal record: a test that fails under load
is not a test; remove the timeout entirely and replace it with something
not load dependent.

## Mandate

1. `metasystem/testing.json` and internal/testpolicy: groups may carry an
   optional integer `cpuBudgetSeconds`; no group gets one in this slice
   (the fleet's measurements set them later). The field is protected:
   in `testpolicy.ProtectedContract` a group present in both base and
   candidate keeps the base's value, a new group takes the candidate's;
   a test proves a raised candidate value is ignored (revision 3,
   PHD-BUDGET-PROTECTION).
2. `internal/proofrun/test_go.go`: every go group's argv carries
   `-timeout 0` right after `-count=1`; no other `-timeout` literal in
   internal/proofrun (row 9). Native discovery runs under the supervisor
   with a budget of sixty CPU seconds and no window, in place of the
   thirty-minute context.
3. `internal/proofrun/test_build.go`: the group child runs under the
   supervisor of Decision 2 instead of `command.Run()` with a buffered
   output: same bytes to the same buffer and log the parsers read today;
   descendant tree by parent links rebuilt every ten seconds; CPU counter
   = self plus reaped-children CPU per live member (`/proc/<pid>/stat`
   fields 14 to 17 on Linux, `ps -S -o cputime= -p` on Darwin), high-water,
   never decreasing; a section's stage-results file growth counts as
   output; verdict per sample: at or above `cpuBudgetSeconds` (when set) is
   `runaway`; when the counter rose by less than one second and no output
   arrived for `zeroConsumptionWindow` (thirty minutes, one unexported
   constant): if any member is stopped (state T on either platform) the
   group is `stopped`, if any member is in uninterruptible wait (U on
   Darwin, D on Linux) it is `waiting on the host`, both named on the
   record and left running; otherwise it is `dead`; otherwise wait without
   bound. Units: Linux ticks over sysconf(_SC_CLK_TCK); Darwin cputime
   parsed as [[dd-]hh:]mm:ss. A member that vanishes between the tree scan
   and its read is a partial sample that keeps the high-water counter;
   three outright reader failures in a row end the group `invalid`. Stop ladder: the tee drains
   output continuously; SIGQUIT to the tree, then wait for the process to
   exit; if the tree's counter still rises at the next sample it ignored
   the dump request and is killed at once (`dump: not produced, the
   process kept computing`); a quiet tree falls back to the
   zero-consumption rule only, then SIGKILL; the record says `dump:
   complete` or `dump: killed while writing`.
4. The group record carries `progressRule`, `cpuSeconds`,
   `longestSilentSeconds`, `longestZeroCpuSeconds`; `runaway` and `dead`
   are failed deliveries with their reason on the TEST-GROUP line (row 12).
5. The one test hook is the unexported `supervisorLimitsForTest` whose
   zero value is the production pair; a test asserts that pair.
6. Tests per rows 1 to 9 and 12 to 14 of Decision 4 as amended by
   revision 3: rows 1 and 4 start two busy loops per online processor,
   measure the test's own scheduling share (consumed CPU over wall time)
   and assert it below one half before asserting the outcome, skipping by
   name where the share cannot be pushed down; row 13 becomes the
   protected-field test; new rows for `stopped` (a child SIGSTOPped for
   longer than a shortened window is reported stopped, not killed) and
   for a SIGQUIT-ignoring busy child killed at once. The Linux half of
   row 7 is a build-tagged test that the VM seat's cadence runs.
7. Nothing else: not the attempt deadline (slice 2), not the shell gate,
   coverage run or fixture beds (slice 3), not the watchdog.

## Proof

go build, vet, gofmt; go test on internal/proofrun and internal/testpolicy
with -count=1. Report the round as your own.

## Constraints

Wall-clock budget: 40 minutes. Return per the implementer schema. Stop at
a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.

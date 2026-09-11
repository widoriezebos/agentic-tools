Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Review brief: a group is bounded by what it consumes, never by the clock (chain phd-build1-20260910)

FINDING IDS: chain-unique, continue from PHD-15 (PHD-01 to PHD-14 were the
first two critiques', phd-build1-crit2-20260910 on round 6 and
phd-build1-crit3b-20260910 on round 10; their dispositions land with the
chain and every decision is on the design page under "Build decisions");
never F-n. Report `round` as 1 in your return: it is this job's own
round. One focused round: verify the fold of the seven findings first,
then verify the fold of PHD-08 to PHD-14 in round 11 (D-R11-1 to
D-R11-6 on the design page), then attack what rounds 7 to 12 changed (round 12 added the
unexported OnVerdict fixture seam on supervisorOptions; check it is nil
in every production caller and fires once) (round 8: real-reader rows carry
no window, D-R8-1; samples carry Members on both platforms, D-R8-2; round 10: gated
fixture callbacks keep the largest member count, D-R9-1 restated).

Why: Wido's ruling on the goal record
metasystem/plans/goals/proof-groups-detect-hangs-by-progress-not-the-clock.md:
a test that fails under load is not a test; the timeout goes and
something not load dependent replaces it. The design is
metasystem/plans/proof-groups-detect-hangs-by-progress-not-the-clock-design.md
(revision 3; its decision table amends Decisions 1, 2 and 4). The build
brief is
metasystem/plans/proof-groups-detect-hangs-by-progress-not-the-clock-build1-brief.md.
Slice 1 replaces the wall-clock timeout of every go test group with a
supervisor that judges the group by what it consumes: CPU seconds of the
descendant tree, output bytes, task state.

Three gaps the implementer found in revision 3 were decided by the
designer in the round-2 follow-up brief and bind this review: Linux
clock ticks come from AT_CLKTCK in /proc/self/auxv (100 with
`ticks/assumed-100` on the progress rule when unreadable); `stopped` and
`waiting on the host` are current readings, never terminal, and the
supervisor waits without bound until the tree exits or the attempt's
existing cancellation takes custody, with the zero-consumption window
restarting when the tree leaves that state; `cpuBudgetSeconds` is a
nullable integer, strictly positive when present, absent means the dead
rule only. Round 3 corrected the Darwin cputime shape:
BSD ps prints `minutes:seconds.hundredths` with unbounded minutes and
no hours field, and the reader parses `[dd-][hh:]mm:ss[.ff]`; a reader
that is unavailable on the fleet's own Macs would end every Darwin
group `invalid`, so check the parser against real ps output, not the
fixture's shapes alone. Round 4 made the rise that counts as
consumption a fraction of the window, max(10 ms, window / 1800), one
second at the production window, the same threshold in the dump ladder;
check that production behaviour is unchanged and that no test window
smuggles a wall bound back in. Round 5 made the contention row's
competitors hand-shake (`spinning` on stdout) before the share is
measured; check that the row's outcome is decided by consumption and the
measured share, and that its skip names the share. Round 6 lets the row double its
competitors up to eight per processor before it skips.

Threat model: any wall-clock deadline left in internal/proofrun that can
end a group under load (a `-timeout` literal other than 0, a context
deadline around the child, a sleep-then-kill); the zero-consumption
window measured in wall time being confused with a timeout (it is not:
it fires only when consumption is zero, so it is not load dependent, but
a window that also fires when the counter rises slowly IS); a CPU
counter that can decrease (children reaped, members vanishing, tick
rounding) and so declares dead a tree that is alive; a `stopped` or
`waiting on the host` tree being killed; a runaway verdict with no
budget set; the budget protection letting a candidate raise its own
budget; the SIGQUIT ladder losing the dump on a tree that stops
computing (killed while writing must be named); the tee blocking a
dumping child on a full pipe; a partial sample counted as a reader
failure; the stopped and SIGQUIT-ignoring tests depending on scheduling
(the busy-loop precondition must be measured and the row skipped by
name, never passed, where the share cannot be pushed down); a test that
takes the production thirty-minute window instead of the shortened
test pair; a group record missing progressRule, cpuSeconds,
longestSilentSeconds or longestZeroCpuSeconds; runaway or dead not
failing the delivery on the TEST-GROUP line; any test whose pass
depends on the host's load (Wido's rule: that is not a test); anything
outside the slice (the attempt deadline, the shell gate, coverage run,
fixture beds and the watchdog are later slices).

Scope: the computed diff of the implementer job. Contract: the design
at revision 3, the build brief and the goal record.

# Mandate

1. Every go group runs with -timeout 0 and no other clock ends it; the
   supervisor's verdicts (runaway, dead, stopped, waiting on the host,
   invalid) follow the design's rules with the units the design names,
   and the tests for rows 1 to 9 and 12 to 14 would fail on the
   2026-09-10 code (the ten-minute default killing goal-full-coverage
   under load).
2. No test in the diff is load dependent: each outcome is decided by
   consumption, state or bytes, and each scheduling precondition is
   measured before it is relied on.
3. Nothing outside the boundary changed; the parsers read the same
   bytes from the same buffer and log as before.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from the review record beside the computed diff (the
conformance validator refuses from a sandbox; say so). The orchestrator
runs internal/proofrun and internal/testpolicy on the seat.

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.

Working Mode: design
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock, tier 3 DESIGN-BEARING)
Date: 2026-09-10

# Critique brief: hang detection by progress, revision 1

FINDING IDS: PHD-<NAME>, chain-unique, never F-n. You are the design
critic of revision 1; report `round` as 1. One focused round. Write
your record to the one path the declared outputs manifest names, a new
file in the records directory under misc, named for this design and
critique round one.

The design is metasystem/plans/proof-groups-progress-hang-design.md,
revision 1, written by the m1 seat on 2026-09-10 against main at
99f2a9633; the digest line the dispatcher stamps into your prompt is
the digest of the outputs manifest, not of the design.

Why: Wido's words of 2026-09-10 13:45Z are on the goal record: a test
that fails under load is not a test; remove the timeout entirely and
replace it with something not load dependent. The design replaces every
wall-clock bound of the proof run with one rule: hung means no progress
event and no CPU time over a fifteen-minute window.

Attack, in order: (1) the rule itself, the AND of no events and no CPU:
find a hung child it never catches, and a live child it wrongly kills
(a test blocked on the network with no CPU, a test waiting on a
subprocess in another process group, a test whose only work is I/O
wait); (2) CPU accounting of a process group on Darwin and Linux with
`ps -o cputime= -g` (rollover, exited children, threads, a child that
reparents); (3) the constant window versus a key: whether fifteen minutes
is defensible from this fleet's slowest legitimate gap and whether the
`-timeout 0` plus window pair can starve a receipt forever (residual
(a)); (4) the streaming tee against the buffered output the parsers read
today (`parseGoJSON`, coverage checks, the log digest) so that nothing
downstream changes shape; (5) the section adapter's progress events and
the fixture-budget caps left standing in slice 1; (6) the attempt
deadline as a reservation horizon only (Decision 3) against dispatch
admission's proof reservation accounting in internal/dispatch; (7) the
proof table: every row provable in the named home, and the window hook
not becoming a second bound. Read metasystem/internal/proofrun/test_build.go,
test_go.go, watchdog.go, attempt.go and
metasystem/scripts/agents/fixture-budget.sh before writing.

Sort findings by materiality; a finding is material when the built
product would violate the goal's DONE sentence or the design's reject
condition. Say plainly when nothing material remains.

# Constraints

Wall-clock budget: 25 minutes. Read-only: you write only your record.

# Gap Rule

Stop and report a gap; never fill it silently.

Working Mode: design
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock, tier 3 DESIGN-BEARING)
Date: 2026-09-10

# Critique brief: hang detection by consumption, revision 2

FINDING IDS: PHD-<NAME>, chain-unique, new names only; the nine of round
one are folded and closed in the dispositions record beside this brief.
Report `round` as 1 in your return (this job's own round). One focused
round. Write your record to the one path the declared outputs manifest
names, a new file in the records directory under misc, named for this
design and critique round two.

The design is metasystem/plans/proof-groups-progress-hang-design.md,
revision 2. Its round-one critique found the "no output and no CPU" rule
neither necessary nor sufficient and inventoried the clocks revision 1
missed; revision 2 replaces the rule with two: a CPU budget per group
(`cpuBudgetSeconds` in metasystem/testing.json) for runaway compute, and
an exactly-zero-consumption window of thirty minutes for deadlock; it
inventories every wall-clock bound with its slice; it reads the
descendant tree and self-plus-reaped-children CPU per platform; it waits
for the dump to complete; and it records every group's consumption and
longest intervals.

Attack, in order: (1) the two grounds: can host load make a runnable
tree consume exactly zero CPU for thirty minutes (memory pressure, swap,
a stopped job, SIGSTOP from a fixture, a container throttle), and can
the same work consume materially different CPU on a quiet and a loaded
host (cache effects, spin waits, GC pacing), which would make the
budget load dependent after all; (2) the counter: reaped-children
folding on both platforms, zombies, threads, a member that execs, a
tree rebuilt every ten seconds missing a process that lived less than
that and was reaped by a parent that then exited; (3) the seeds: four
times the wall target as CPU seconds on a machine with many cores
where a group's CPU exceeds its wall time; (4) the section adapter's
stage-results growth as output; (5) the dump wait: a dumping process
that blocks on a full pipe; (6) slice 2's five deadline sites and what
else reads the attempt deadline; (7) the proof matrix against the paths
above, and whether row 1's four competitors starve a test on this
Mac's cores at all. Read metasystem/internal/proofrun/test_build.go,
test_go.go, attempt.go, watchdog.go, metasystem/cmd/metasystem/test.go
and metasystem/testing.json before writing.

Sort findings by materiality; a finding is material when the built
product would violate the goal's DONE sentence or the design's reject
condition. Say plainly when nothing material remains.

# Constraints

Wall-clock budget: 25 minutes. Read-only: you write only your record.

# Gap Rule

Stop and report a gap; never fill it silently.

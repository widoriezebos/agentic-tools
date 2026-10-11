# Design brief: lane housekeeping 2 (items measured on 2026-10-11)

Goal (to open): lane-housekeeping-2, tier 2. Follow-up of lane-first-run-fixes (declared whole at u7 on 2026-10-11). Each unit at most 250 production lines, one at a time, rehearsed before landing (metasystem landing rehearse). Findings in /Users/wido/agentic-tools-evidence/lane-test-20261009/findings.md.

## Unit 1 (from finding 104): every check runs detached from any terminal

A person's `landing prove --wait` and `--classify` ran their checks inside the person's process lineage; tests that assert "no terminal fact" flipped, main was recorded red seven times over, and a trunk-red exception was offered for a green main. The person's verb starts the same detached run the keeper would start and waits on its result; no engine path runs a proof command in the caller's terminal. Real-shape test: a classify started from a fake enrolled terminal lineage runs its replay in a child that does not carry the terminal fact.

## Unit 2 (from finding 103): the replay runs the failed tests by name

Attributing a red replayed every group of the failed unit's package (54 units, ~35 min for five tests that take under 3 min). The replay passes the failed tests' names with the unit (LANDING_ONLY plus the tests) and the reporter runs only those plus the package's compile; a unit without test names keeps today's whole-unit replay.

## Unit 3 (from finding 117): the engine's own commands run under its PATH

`work commit`'s cheap check (and every other place the engine runs a declared shell command) prepends the running engine's directory to PATH the way the proof runner does, so a declared `metasystem …` command never fails with "command not found" because of the caller's shell.

## Unit 4 (from findings 103, 118): the commit-time check of a unit runs the unit's impact plan only

The committed cheap check of `work commit` ran 56 minutes of named command tests for an eight-file fix; it runs the unit's own impact selection (named tests of the touched symbols) and leaves package-wide depth to the batch proof.

## Unit 5 (from finding 107): one ledger transaction per trunk result

Clearing eight incidents moved main eight times; a trunk result writes one record.

## Unit 6 (from findings 120-122): the hand-in accepts a tip read that names the units it covered

A hand-built goal cannot get per-unit machinery reads (`--commit` and `--work` refuse once a later read exists; the oldest tree fails today's static gate); the whole-goal check accepts a read of the branch tip whose record names every unit it covered, and the person's exception computes its candidate from the branch.

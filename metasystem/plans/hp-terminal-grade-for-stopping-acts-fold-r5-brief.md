Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal hp-terminal-grade-for-stopping-acts)
Date: 2026-09-09

# Fold round five: the proof-grade scenario has never run; three fixes

Follow-up round on chain hp-terminal-build1-20260909; your worktree
carries round four. The seat gate on round four fails proof-grades, and
the seat's own experiments show the scenario has never got past its
first cap on any round. Three defects, all in
scripts/agents/goal-cli-fixtures.sh, all proven on the seat.

## Defect A: the bed never initializes its fixture budget

    goal-cli fixture scenario started: proof-grades
    fixture cap scale is not initialized

`harness_fixture_cap` (scripts/agents/fixture-budget.sh) needs
METASYSTEM_FIXTURE_CAP_SCALE_MILLI, which only
`harness_fixture_budget_init <root>` sets and exports. Every other bed
that calls a cap initializes first; scripts/agents/supervision-hook-fixtures.sh
does it on the line right after it sources fixture-budget.sh, before
the parent/child split, so the parent calibrates once and each scenario
child inherits the exported scale. goal-cli-fixtures.sh never called
the init because before this chain it never used a cap; the
proof-grades scenario and its cleanup now call `harness_fixture_cap
mission-process-wait` four times.

## Defect B: copied platform binaries are killed at exec on macOS

The scenario copies /bin/sleep (the holder command and the input
keeper) and /bin/bash (the agent shell). On this Mac a platform binary
copied out of its sealed location is killed by the kernel the moment it
is executed: `cp /bin/sleep "$tmp/k" && "$tmp/k" 0` prints "Killed: 9"
and exits 137, with the seat's sandbox on or off. So the holder, the
keeper and the agent shell have never run; the ^D in the holder log is
script(1) seeing the dead keeper's pipe close.

Shape proven on the seat under the round-four engine
(/tmp/hp-terminal-gate-engine-r4's `proc probe`):

    # $tmp/metasystem-fake-agent, the holder command
    #!/bin/bash
    exec -a metasystem-fake-agent /bin/sleep "$1"

started through the existing holder script under `script -q /dev/null`
and fed from a keeper that runs `exec -a proof-grade-input-keeper
/bin/sleep 600` after writing its pid: the holder is the real /bin/sleep
with argv[0] carrying the fake-runtime name; the probe reports
argv ["metasystem-fake-agent","300"], sessionLeaderPid equal to its own
pid, terminalKnown true, a terminalId, liveness alive; it survives the
keeper's death (the ^D reaches a sleep that never reads) and dies on its
own TERM, after which the script session exits. `pgrep -fl
metasystem-fake-agent` counts the holder and not the keeper.

For the agent shell use the same device: a script named
metasystem-fake-agent that runs `exec -a metasystem-fake-agent /bin/bash
"$@"`, so the assertion's ancestor carries the signature in argv[0].
Read internal/humanauthority/authority.go (and the classifier it
calls) to confirm the agent-signature match reads argv[0] or the
command line rather than the executable path; if it reads the path,
stop and report that gap with your proposed resolution.

`exec -a` is in /bin/bash 3.2.

## Defect C: the cleanup treats "never proven" as "still alive"

`proof_grade_holder_keeper_start=$("$ms" proc started-at --pid ...)` is
unguarded: when the keeper is already dead the substitution fails,
set -e aborts the scenario without a word, and the cleanup then
compares an empty recorded start with an empty current start, finds
them equal, spins the whole mission-process-wait cap and reports
"proof-grade input keeper did not exit within 10s after termination":
a false story about a process that never ran. The holder block has the
same comparison.

Guard the keeper-start read the way the holder's readiness loop is
guarded, and fail with a clear message and the holder log when the
keeper is not alive and proven. In the cleanup, an empty recorded start
means nothing was ever proven: skip the kill and the wait for that
process; only a non-empty recorded start earns the ownership check, the
TERM and the bounded wait.

## Mandate

1. `harness_fixture_budget_init "$fixture_bed_root"` immediately after
   the first `source` of fixture-budget.sh in goal-cli-fixtures.sh; the
   later `harness_fixture_warn_if_engine_stale "$root"` becomes
   redundant (the init warns), remove it.
2. The holder command, the keeper and the agent shell become the
   `exec -a` scripts above; no copied binaries remain in the scenario.
3. The keeper-start read is guarded and the two cleanup blocks treat an
   empty recorded start as never proven.
4. Nothing else changes. Rounds one to four stand.

## Proof

Your sandbox cannot run the bed; run `bash -n` and say so. The
orchestrator runs the goal-cli bed and the hook suite on the seat.
Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.

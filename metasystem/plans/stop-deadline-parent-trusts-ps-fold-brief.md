Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-deadline-parent-trusts-ps)
Date: 2026-09-06

# Follow-up on chain sdp-build1-20260906: the gap is decided

Your round one stopped on a real gap: with ps printing nothing the
signal gate refuses TERM and KILL, and the parent's unconditional wait
on the live worker then loses the deadline again. The orchestrator
chooses your first proposal. The contract below replaces the
lifecycle part of
metasystem/plans/stop-deadline-parent-trusts-ps-build-brief.md; the
rest of that brief stands (the goal record
metasystem/plans/goals/stop-deadline-parent-trusts-ps.md is the
contract).

# The contract

1. Liveness: deadline_running in
   metasystem/scripts/agents/supervision-hook.sh is a kill -0 test on
   the worker pid, no ps. Both loops that wait on the worker use it.
2. Signal gate: unchanged. The parent signals the worker or the
   resolver only after ps shows a command line it recognises.
3. After expiry, the parent WAITS ONLY FOR A WORKER IT SIGNALLED. When
   the gate refused (no verifiable command line) and the worker is
   still alive after the ten short attempts, the parent skips the wait,
   writes one stderr line
   "stop deadline: worker <pid> left running, command line unverifiable"
   and goes on to emit the blocking deadline response exactly as today
   (hook-expire recorded, outcome=deadline-expired-block logged). The
   deadline directory and its files stay in place for the worker that
   still writes to them (the worker exits on its own bound; the
   harness tmp sweep reaps the directory later); the parent removes
   them only on the paths where it did wait, as today. When the worker
   exits by itself during the attempts, the parent waits and reads its
   result as today (a finished worker is a verdict, never a refusal).
4. Fixture: the ps-shim scenario in
   metasystem/scripts/agents/supervision-hook-fixtures.sh (a ps first
   on PATH printing nothing, exit 0; the same delaying engine) asserts:
   the blocking response with "deadline expired before a safe turn
   verdict" arrives in under sixty seconds; the log names
   outcome=deadline-expired-block; stderr carries the "left running"
   line with a pid; the worker was not signalled (the delaying engine's
   own ceiling message appears in the worker's stderr file after the
   parent returned, or an equivalent proof you name); and the scenario
   itself is bounded (wait for the orphaned worker with a ceiling of
   seventy seconds before the suite's cleanup, so the suite never leaks
   it). The existing deadline scenario stays green.
5. Only the two files change.

# Gate

As in the build brief: bash -n on both scripts; shellcheck if present;
`bash scripts/agents/supervision-hook-fixtures.sh` green, or the exact
scenarios the sandbox cannot run named in the return (the seat replays
them outside).

# Constraints

Wall-clock budget: 45 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 3, shell only). Declare the
boundary as every file that differs from main. Gap rule: a further gap
stops with the proposed contract written out.

Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-deadline-parent-trusts-ps)
Date: 2026-09-06

# Goal

Goal stop-deadline-parent-trusts-ps (tier 3, approved by Wido on
2026-09-06). Its record,
metasystem/plans/goals/stop-deadline-parent-trusts-ps.md, is the
contract; in short: the Stop hook's deadline parent in
metasystem/scripts/agents/supervision-hook.sh decides whether its worker
is still running by asking ps for the worker's state (the function
deadline_running); when ps prints nothing for a live process (sandboxes
and restricted hosts deny process inspection) the parent concludes the
worker has finished, leaves its deadline loop and waits for the worker
with no deadline at all, so a genuine hang is never refused. The code
reviewer of goal stop-hook-budget-is-ours saw exactly this in a sandbox:
the delaying-engine deadline scenario ran 65 seconds and returned a
non-blocking response instead of the expiry.

DONE means: the parent's liveness test does not depend on ps output; an
empty or failing ps answer counts as "still running" for the deadline
(the wait keeps its deadline); the kill path still refuses to signal a
process whose command line it cannot verify; and a fixture with a ps
that prints nothing proves the parent still expires at its deadline.

# The change

1. metasystem/scripts/agents/supervision-hook.sh: split "is the worker
   alive" from "is it my worker". deadline_running becomes a kill -0
   test on the worker pid (a zombie that kill -0 still sees is handled
   by the wait that follows, as today), with no ps in it. The ps
   command-line check stays only where the parent signals the worker or
   the resolver (kill -TERM, kill -KILL): a process whose command line
   ps cannot show is never signalled, as today. The main deadline loop
   and the ten-attempt stop loop both use the new test. Nothing else in
   the hook moves; the log lines and the JSON responses keep their
   words.
2. metasystem/scripts/agents/supervision-hook-fixtures.sh: one new
   scenario beside the delaying-engine deadline scenario (line 532
   onward). It puts a ps shim first on PATH that prints nothing and
   exits 0 for every invocation, runs the same delaying engine, and
   asserts the blocking response with the words "deadline expired
   before a safe turn verdict" arrives in under sixty seconds and the
   log line names outcome=deadline-expired-block. A second assertion in
   the same scenario: with the shim in place the parent did not signal
   the worker before the deadline (the worker's delay ceiling message
   never appears in stderr before the expiry line, or an equivalent
   proof you name). Keep the scenario's own bound (the existing 61s
   ceiling) so the suite never hangs.
3. Run the whole supervision-hook fixture suite and say which scenarios
   the sandbox could not run (the seat replays them outside).

# Gate

`bash -n scripts/agents/supervision-hook.sh scripts/agents/supervision-hook-fixtures.sh`;
`shellcheck` if present (say so if not);
`bash scripts/agents/supervision-hook-fixtures.sh` green, or the exact
scenarios the sandbox cannot run named in the return.

# Constraints

Wall-clock budget: 45 minutes; return before it ends even if something is
red, naming it. MECHANICAL reach (tier 3, shell only, no Go). Declare
the boundary as every file that differs from main; only the two files
above are expected. Gap rule: stop and report a gap with your proposed
contract written out; never fill it silently. Cite the goal id
stop-deadline-parent-trusts-ps in the return, never a round or finding
number in code comments.

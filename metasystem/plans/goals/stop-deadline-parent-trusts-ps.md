# stop-deadline-parent-trusts-ps

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a hung Stop is allowed to hang the turn end instead of being refused, nothing unsafe is permitted; novelty 1: one liveness test swapped for the one bash already offers; exposure 3: every seat's every turn end, though only when ps misbehaves (sandboxes, restricted hosts); accumulation 1: it does not compound"
- Tier: 3
- Intent: The Stop hook's deadline parent decides whether its worker is still running by asking ps for the worker's state (deadline_running in scripts/agents/supervision-hook.sh); when ps prints nothing for a live process the parent concludes the worker has finished, leaves its deadline loop and waits for the worker with no deadline at all, so a genuine hang is never refused. Found 2026-09-06 by the code reviewer of goal stop-hook-budget-is-ours (finding SHB-02) after the builder's sandbox, which denies process inspection, showed exactly this: the delaying-engine deadline scenario ran 65 seconds and returned a non-blocking response instead of the expiry. Predates the budget change. DONE means the parent's liveness test does not depend on ps output: an empty or failing ps answer is treated as 'still running' for the purpose of the deadline (the wait keeps its deadline), the kill path still refuses to signal a process whose command line it cannot verify, and a fixture with a ps that prints nothing proves the parent still expires at its deadline.
- Origin: main
- Next step: MECHANICAL, one chain: in the deadline parent, split 'is the worker alive' (kill -0, which needs no ps) from 'is it my worker' (the ps command-line check used only before signalling); the deadline loop uses the first; a fixture puts a ps shim on PATH that prints nothing and asserts the deadline block still arrives under sixty seconds. Receipt: supervision-hook-fixtures.sh.
- OpenedAt: 2026-09-06T09:38:27Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T09:38:27Z FDAPZ18VF4J2FX061WP8XCFQW8-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=stop-deadline-parent-trusts-ps
Integrity: sha256=70aa1497b6dd146f39c1d09093e52798d69188e9e6dfa01d3631bde63fd564e4

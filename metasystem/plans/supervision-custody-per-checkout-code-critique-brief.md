Working Mode: review
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout)
Date: 2026-09-05

# Review brief: chain scc-build3 (carrying the work of chains scc-build1 and scc-build2 with the second critic's two corrections)

Round budget: 3 focused rounds, agreed at goal approval (tier 3); exhaustion follows the critique skills' budget rules, never a silent round 4.

Threat model: one user on one machine running several checkouts and fixture roots at once; accidents and stale identities are in scope; hostile inputs are not. A TRUE finding outside this model closes as out-of-scope, citing this section.

Scope: the goal's DONE, verbatim from the ledger: (1) a test in internal/supervise or internal/registry that arms two real checkouts and one temporary root on one machine, under the same and under different main identities, and asserts that no shutdown, takeover, relaunch or sweep of one ever terminates or retires another's owner, watcher or runner; (2) the code path the test exposes fixed so every victim selection is by canonical checkout path; (3) the fixture rule written in docs/orchestration.md and enforced by the supervision suite's self-check: a scenario brings up supervision only with its own registry home and main identity. The files under review are the implementer's declared boundary for this chain. OUT: the stop-hook scenario's own legs (another goal), and any rewrite of the registry contract beyond what the fix needs; a contract change must be named as such.

Return format: numbered findings from SCC-31 upward (SCC-31, SCC-32, …; never an id used by the earlier critic rounds of this goal, SCC-01 to SCC-05 in chains scc-build2-cc1 and scc-build2-cc2, because ids are unique across every critic round of one subject), most severe first, each with file, rule, and the concrete failure it causes; or AGREE with observations that do not gate. A finding is material only if it changes what must be built and names the artifact.

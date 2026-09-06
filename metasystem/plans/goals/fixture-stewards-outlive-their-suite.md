# fixture-stewards-outlive-their-suite

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=3 basis="severity 2: leaked runners load the host and poison later suites, nothing unsafe; novelty 2: a runner bounding its own life on its enrollment kind is new behaviour in the steward; exposure 3: every host that runs any suite, three seats on this one; accumulation 3: fifty orphans in one day, one more per killed suite"
- Tier: 3
- Intent: Fixture steward runners outlive the suite that armed them. On 2026-09-06 around 19:50 CEST the m1d seat found and reaped by hand fifty orphaned steward runners on the m1 host, reparented to init, in fixture beds under the user temp folder (dispatch, health, supervision-hook and supervision suite beds: agent-fixture runner, steward, selftest and budget-dispatch repositories, operator repositories, nested roots), aged 39 minutes to 8 hours 15 minutes, their beds still on disk. Each suite's exit trap shuts the beds' supervision down and kills strays whose argv names the bed, so a suite that exits runs clean; the orphans come from suites that never reached their trap: a delegate sandbox killed at its cap, a seat's alarm or task stop (SIGALRM and SIGKILL fire no trap), a nested validate terminated under load. The runner is a daemon detached from the suite, so no process-group custody of the suite reaches it, and it ticks forever in a bed nobody reads, adding load to every later suite on the host (the dispatch suite's recollection leg already fails under contention, goal dispatch-fixture-recollection-pass-budget). The steward identity now carries its enrollment kind (fixture-stewards-reach-the-desktop, landed 2026-09-06), so a runner knows it is a fixture. DONE means a fixture-enrolled runner bounds its own life: it stops itself when the suite process that armed it is gone (the bed records the suite's pid and start time at arm) or when it is older than a named bound (a config key with a validated default well above any suite's runtime), never a human-enrolled runner; a fixture proves both exits and proves a human enrollment is untouched; and a steward health role or the host-health role counts fixture runners on the host so the next leak is seen, not found by hand.
- Origin: main
- Next step: Unclaimed; awaits approval. Design-bearing: brief the self-bounding rule (the bed records the arming suite pid and start time; an age bound as a validated config key), a design round only if the code critic asks for one, build behind fixtures, land with the full battery receipt.
- OpenedAt: 2026-09-06T18:05:32Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T18:05:32Z PWE3X0NPM8HHYJJH3DFT6BKRGY-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=fixture-stewards-outlive-their-suite
Integrity: sha256=dc4112b2182e1d2294d60722e42deea186c7a90616525f5da75e656661930a19

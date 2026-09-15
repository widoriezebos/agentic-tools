# dispatch-mission-timeout-scenario-holds-under-load

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a false red that costs each seat a classification per proof and may hide a real exit-mapping defect; novelty 1: one scenario, one assertion; exposure 3: every seat's selected sections include this section; accumulation 2: it recurs on every loaded run until fixed"
- Tier: 2
- Intent: Scenario dispatch-e in section dispatcher-adapter-and-mission-runner-fixtures (scripts/agents/dispatch-fixtures.sh, the mission-timeout leg near line 4592) reds intermittently on every seat's proof: 'mission job timeout did not map to exit 4 (got 5)', with the job recorded as status timeout, error budget-cap, phase supervision. On 2026-09-15 it failed on candidates of all three seats (m1b at 09:20 and on its unit 1b run, m1c at 09:38, m1e at 09:56 on brief-declares-the-round-boundary unit 3a-i), and on 2026-09-14 it passed and failed on the same tree (memory/backlog-notes.md). Every seat spends a classification on it per proof. The fixture backdates capDeadline and reaps; under load the supervision budget-cap path appears to record the timeout first. DONE: the race diagnosed from preserved evidence and code; the scenario, or the engine if the product maps one timeout to exit 4 or 5 depending on order, fixed so the result does not depend on load, using an injectable clock and never a longer wait or a retry (Wido's rule); proven by the scenario passing under a loaded sweep and the section green on trunk apart from reds other goals own.
- Origin: human
- Next step: Diagnose first from the preserved evidence (m1e artifacts/agents/suite-failures/20260915T075301Z-dispatch-69190, and the m1b and m1c copies from their runs): which path records budget-cap before the reap's timeout, and whether the product maps one timeout to two exits depending on order (a product defect) or only the fixture's backdated capDeadline races the supervision tick (a test defect). Then fix with an injectable clock. Suggested owner: m1b, which owns the dispatcher bed goals.
- OpenedAt: 2026-09-15T08:13:14Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-15T08:13:14Z 4GVK75JY2B3AMFCFYWEMPDR4SD-m1e-c6925449 open actor=human:Wido targets=dispatch-mission-timeout-scenario-holds-under-load
Integrity: sha256=c7214eaa40d43d4d1ba1ec415b864161821ae3bc8ca2897e6df585adac64db73

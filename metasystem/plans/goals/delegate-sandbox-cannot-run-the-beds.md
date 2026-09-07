# delegate-sandbox-cannot-run-the-beds

- State: queued
- Risk: severity=3 novelty=3 exposure=3 accumulation=3 basis="severity 3: headless, no seat exists to run the beds, so a node either lands a process-owning change unverified or stalls forever; novelty 3: giving a delegate detached process starts, process-table enumeration and scratch repositories is a real authority and custody question, not a flag; exposure 3: every goal whose proof needs live processes, and every headless node by definition; accumulation 3: it recurs on every such goal and today it cost one seat thirteen hours in a single day"
- Tier: 3
- Intent: A delegate cannot run any of the five process-owning fixture beds (supervision, mission, dispatch, goal-cli, suite-progress) nor five of eleven Go packages in the stop-verb boundary: its sandbox denies detached process starts, Darwin process enumeration and process-group proof, and its scratch git repositories inherit the worktree's object quarantine. So a builder cannot see the failure it must fix, and every defect a bed reveals costs a full round trip through the orchestrator. Measured on goal metasystem-stop-verb 2026-09-06/07: of twenty rounds, nine fixed one to three lines each (an unresolved path four times, a manifest prefix, an unbound variable, a wait on a process that never exits, a caller class, a missing field) at roughly ninety minutes per defect, because only the orchestrator's session could run the bed. THE HEADLESS CONSEQUENCE IS THE REAL ONE (Wido's question 2026-09-07): with a session in the driver seat this is a slowdown; with a node running the mission runner and no seat on top, it is a wall, because nothing in the headless loop can run a bed at all. DONE means a dispatched job can run the process-owning beds and the whole package matrix within its own custody, safely (its children are its own, reaped with it, and it cannot signal what it does not own), proven by a delegate round that runs one bed green and by a fixture that proves the custody
- Origin: main
- Next step: decide the shape first, it is an authority question rather than a plumbing one: either the delegate sandbox gains the capabilities under the existing custody rules (process starts confined to its own group, enumeration scoped to its own tree), or verification becomes its own dispatched role (a verify job the orchestrator or the node launches, whose brief names the beds and whose return carries their outcomes). The second is smaller and headless-safe; the first is what makes builders self-sufficient. Read scripts/agents/dispatch.sh's sandbox construction and the effective-permissions record a round writes; the denials are recorded there per round. Sibling: rounds-prove-with-the-smallest-run, which is the discipline half
- OpenedAt: 2026-09-07T20:09:50Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T20:09:50Z 1NKAEF8G3ZD0RQG53W96A8T1CK-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=delegate-sandbox-cannot-run-the-beds
Integrity: sha256=4b24e38f9f10c6f23dc696a1fb8fa1acea603f47aaeaf59f37d92a8665d360eb

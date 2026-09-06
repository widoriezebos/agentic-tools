# metasystem-stop-verb

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: without an orderly stop a human reaches for kill and leaves locks, leases and half-written records behind, which the custody family then has to clean up; novelty 1: the shutdown path exists in arm-supervision.sh, the verb wraps it; exposure 3: every operator on every machine; accumulation 1: nothing compounds, the gap is the same every day"
- Tier: 3
- Intent: Wido's word 2026-09-06: 'For a human it should be a simple command to stop the meta system. And not only simple but also intuitive.' Today there is none: metasystem delegate --cancel stops one job, steward restart restarts and never stops, and the only shutdown path is scripts/agents/arm-supervision.sh --shutdown, an internal script the fixtures call. DONE means: 'metasystem stop --repo .' (accepted from an agent-free terminal, a human act like arm and restart) stops everything the metasystem runs for that checkout in order - running delegate jobs cancelled through the cancel path, then the steward runner, repo watcher, narrator and supervision components through the existing shutdown path - and prints one line per thing it stopped and one line saying how to start again (steward arm). Idempotent: a second stop says nothing is running. 'metasystem status --repo .' prints the same list without stopping anything. A fleet form (--all for every enrolled checkout on this host) is welcome if cheap; per checkout is the requirement. No kill -9 unless the orderly path fails, and then it says so.
- Origin: main
- Next step: MECHANICAL, one chain: the verbs in cmd/metasystem beside steward arm and restart, reusing the shutdown path of scripts/agents/arm-supervision.sh and the cancel path of delegate; a fixture arms a steward in a scratch repository, dispatches a fake job, runs stop, and asserts every process is gone, the records are terminal and the second stop is a no-op; docs: one paragraph in the operator documentation naming stop, status and arm as the three human verbs. Any free seat; a few hours.
- OpenedAt: 2026-09-06T12:24:49Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:25:28Z revision=2 opid=ESANH6C2BR6VYW74A9Q5NQEWW2-m1-7cd0bd60 authority=proven digest=1c0acb89a08769fa2813f7e1b5f6d986a61a106fa9aa15c8cf9f56cb9ab4cc0a

History:
- 2026-09-06T12:24:49Z ZKBER0FQHEW28XV1PJ2X0K49PP-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=metasystem-stop-verb
- 2026-09-06T12:25:28Z ESANH6C2BR6VYW74A9Q5NQEWW2-m1-7cd0bd60 approve actor=human:Wido targets=metasystem-stop-verb
Integrity: sha256=ebbb72863281195f9548f317cb1d58dcd5003b926c1d0ada9e81a0eb1a7bc4c4

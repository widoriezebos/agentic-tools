# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY (Wido 2026-08-27): ordered ahead of actionable-metrics slices two/three — slice two's watches consume job records this arc makes honest. Design brief first (2-3 critique rounds, sol/xhigh); absorb plans/goals-drafts/agent-liveness-contract.md. The arc's four requirements: (1) one custodial launch channel for every delegate run — companion, codex exec, future runtimes — through the adapter seam, agent-agnostic, minting a job record with process identity (pid + start time) under a cap; (2) IDEMPOTENT launch: a retry binds to the standing task, a busy session refuses loudly at the seam, a duplicate can never be minted (makes third-party wrapper double-fires harmless); (3) liveness as system machinery: the steward sweep judges by work-product mtimes + process probe + timed verdict, marking stalled-suspected on the registry and notifying the dispatching goal; (4) terminal states are reaper-stamped, never the runtime's self-report. Composes with suite-dispatch-exclusion (m2's execution guard) and closes the metrics jobs-gap (design fact F2). Likely two build slices after design; appetite proposal at design close.
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 3
- Claimed: machine=m1 lineage=coordinator at=2026-08-27T14:09:04Z

History:
- 2026-08-27T06:12:18Z E8F1PERWGM23AZ2B7JPCWP1BS6-m1-bf243850 open actor=human:wido targets=delegate-job-liveness
- 2026-08-27T11:38:28Z MMQB7VDR1FJ4DT5MRQGQ9RAQ3F-m1-bf243850 edit actor=human:wido targets=delegate-job-liveness
- 2026-08-27T14:09:04Z KSDFQVTR5V1MCPHZQPQNT87V3K-m1-bf243850 claim actor=m1+coordinator targets=delegate-job-liveness
Integrity: sha256=88d260fae53d31f70680da14df7db3ac5c75d15e6d6c3275662130d822c47022

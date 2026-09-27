# pluggable-proof-runner

- State: approved
- Risk: severity=2 novelty=3 exposure=2 accumulation=1 basis="Proof evidence produced off-host must be trusted exactly as much as a host run; a wrong contract could accept a proof of the wrong tree; opt-in, off by default."
- Tier: 3
- Intent: A proof can run somewhere other than the landing host through a user-extendable runner: an extension point (a script or executable the adopter provides) that receives a tree snapshot and a test selection and returns the proof result, so a VM or a second machine can take full suites and batch proofs while the landing host keeps the final platform check. The metasystem never depends on a particular VM; with no runner configured, everything runs on the host as today.
- Origin: human
- Next step: Write the design (the runner contract: inputs, outputs, trust, identity of the proof environment, how a remote result counts as evidence, fallback) from the 2026-09-27 manual VM suite recipe (evidence verbs-object-action-20260927/vm-suite.md) and have Codex Astra critique it.
- OpenedAt: 2026-09-27T18:19:51Z
- Revision: 2
- Labels: efficiency, extension, landing
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-27T18:20:26Z revision=2 opid=K72PV9EQVEB11663MWTTDJ9KQ2-m1e-c6925449 authority=proven digest=95a901d98a7c64af4cadd46af1062885ffaaf86a8b9c9886f14a6de66c8f83e2 episode=2

History:
- 2026-09-27T18:19:51Z Y17CHMTNHH50TFKTC8E94AHZ6X-m1e-c6925449 open actor=human:Wido targets=pluggable-proof-runner
- 2026-09-27T18:20:26Z K72PV9EQVEB11663MWTTDJ9KQ2-m1e-c6925449 approve actor=human:Wido targets=batch-lane-fewer-warmer-proofs,pluggable-proof-runner
Integrity: sha256=119da346409ee6ffb5616b9c48d578558cf574999bb12232f205bd5289c2570e

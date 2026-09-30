# pluggable-proof-runner

- State: queued
- Risk: severity=2 novelty=3 exposure=2 accumulation=1 basis="Proof evidence produced off-host must be trusted exactly as much as a host run; a wrong contract could accept a proof of the wrong tree; opt-in, off by default."
- Tier: 3
- Intent: A proof can run somewhere other than the landing host through a user-extendable runner: an extension point (a script or executable the adopter provides) that receives a tree snapshot and a test selection and returns the proof result, so a VM or a second machine can take full suites and batch proofs while the landing host keeps the final platform check. The metasystem never depends on a particular VM; with no runner configured, everything runs on the host as today.
- Origin: human
- Next step: Write the design (the runner contract: inputs, outputs, trust, identity of the proof environment, how a remote result counts as evidence, fallback) from the 2026-09-27 manual VM suite recipe (evidence verbs-object-action-20260927/vm-suite.md) and have Codex Astra critique it.
- OpenedAt: 2026-09-27T18:19:51Z
- Revision: 3
- Labels: efficiency, extension, landing
- BudgetExceptions: 0

History:
- 2026-09-27T18:19:51Z Y17CHMTNHH50TFKTC8E94AHZ6X-m1e-c6925449 open actor=human:Wido targets=pluggable-proof-runner
- 2026-09-27T18:20:26Z K72PV9EQVEB11663MWTTDJ9KQ2-m1e-c6925449 approve actor=human:Wido targets=batch-lane-fewer-warmer-proofs,pluggable-proof-runner
- 2026-09-30T19:04:37Z AP1JAB2Q9ZTG741MJ9G2JPPFJB-m1e-b6a4eb0a unapprove actor=human:wido targets=pluggable-proof-runner reason=Held: it would change the landing lane while the lane is being redesigned; revisit under landing-lane-runtime-redesign (backlog sync 2026-09-30)
Integrity: sha256=73e0be84fd63e75356a54df849eeb1ec9bc57b9156fa40c614b1aa255d8c9290

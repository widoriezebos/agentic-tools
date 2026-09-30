# pluggable-proof-runner

- State: queued
- Risk: severity=2 novelty=3 exposure=2 accumulation=1 basis="Proof evidence produced off-host must be trusted exactly as much as a host run; a wrong contract could accept a proof of the wrong tree; opt-in, off by default."
- Tier: 3
- Intent: What: Let a proof run somewhere other than the landing computer, such as a VM or a second machine, through a runner the adopter plugs in. With no runner set, everything runs on the host as today. Why: Full test suites already go to the VM by hand, and the lane has a place for a runner but only ever builds the host one. It changes the same lane code that is being redesigned now, so it waits for landing-lane-runtime-redesign. Pros: Heavy suites leave the landing computer; works for any language and any VM. Cons: A result from another machine must be trusted as evidence, which needs a clear contract; the lane gets more moving parts.
- Origin: human
- Next step: Next: After landing-lane-runtime-redesign has landed, write the runner contract (inputs, outputs, trust, identity of the proof environment, fallback) from the manual VM recipe in verbs-object-action-20260927/vm-suite.md and give it one Astra critique round. Done when: a batch proof runs through a configured runner and counts as evidence, and with no runner it runs on the host unchanged.
- OpenedAt: 2026-09-27T18:19:51Z
- Revision: 4
- Labels: efficiency, extension, landing
- BudgetExceptions: 0

History:
- 2026-09-27T18:19:51Z Y17CHMTNHH50TFKTC8E94AHZ6X-m1e-c6925449 open actor=human:Wido targets=pluggable-proof-runner
- 2026-09-27T18:20:26Z K72PV9EQVEB11663MWTTDJ9KQ2-m1e-c6925449 approve actor=human:Wido targets=batch-lane-fewer-warmer-proofs,pluggable-proof-runner
- 2026-09-30T19:04:37Z AP1JAB2Q9ZTG741MJ9G2JPPFJB-m1e-b6a4eb0a unapprove actor=human:wido targets=pluggable-proof-runner reason=Held: it would change the landing lane while the lane is being redesigned; revisit under landing-lane-runtime-redesign (backlog sync 2026-09-30)
- 2026-09-30T19:04:50Z ART4BQAAR30KWS80NCAKGMA9B8-m1e-b6a4eb0a edit actor=human:wido targets=pluggable-proof-runner
Integrity: sha256=8e7a2fa56e959502f56d7b1541f34bf3c5ca9a5bc793cbd75b91a53e8c0854ec

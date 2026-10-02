# pluggable-proof-runner

- State: abandoned
- Risk: severity=2 novelty=3 exposure=2 accumulation=1 basis="Proof evidence produced off-host must be trusted exactly as much as a host run; a wrong contract could accept a proof of the wrong tree; opt-in, off by default."
- Tier: 3
- Intent: What: Let a proof run somewhere other than the landing computer, such as a VM or a second machine, through a runner the adopter plugs in. With no runner set, everything runs on the host as today. Why: Full test suites already go to the VM by hand, and the lane has a place for a runner but only ever builds the host one. It changes the same lane code that is being redesigned now, so it waits for landing-lane-runtime-redesign. Pros: Heavy suites leave the landing computer; works for any language and any VM. Cons: A result from another machine must be trusted as evidence, which needs a clear contract; the lane gets more moving parts.
- Origin: human
- Next step: Next: After landing-lane-runtime-redesign has landed, write the runner contract (inputs, outputs, trust, identity of the proof environment, fallback) from the manual VM recipe in verbs-object-action-20260927/vm-suite.md and give it one Astra critique round. Done when: a batch proof runs through a configured runner and counts as evidence, and with no runner it runs on the host unchanged.
- OpenedAt: 2026-09-27T18:19:51Z
- Revision: 5
- Labels: efficiency, extension, landing
- BudgetExceptions: 0
- Abandoned: by=human:Wido at=2026-10-02T21:01:00Z revision=5 opid=2VC1XHA4VPX656MWK1DP6QTHPX-m1e-9c612d71 because=Superseded (audit 2026-10-02): the plain lane runs whatever landing.prove.command is configured

History:
- 2026-09-27T18:19:51Z Y17CHMTNHH50TFKTC8E94AHZ6X-m1e-c6925449 open actor=human:Wido targets=pluggable-proof-runner
- 2026-09-27T18:20:26Z K72PV9EQVEB11663MWTTDJ9KQ2-m1e-c6925449 approve actor=human:Wido targets=batch-lane-fewer-warmer-proofs,pluggable-proof-runner
- 2026-09-30T19:04:37Z AP1JAB2Q9ZTG741MJ9G2JPPFJB-m1e-b6a4eb0a unapprove actor=human:wido targets=pluggable-proof-runner reason=Held: it would change the landing lane while the lane is being redesigned; revisit under landing-lane-runtime-redesign (backlog sync 2026-09-30)
- 2026-09-30T19:04:50Z ART4BQAAR30KWS80NCAKGMA9B8-m1e-b6a4eb0a edit actor=human:wido targets=pluggable-proof-runner
- 2026-10-02T21:01:00Z 2VC1XHA4VPX656MWK1DP6QTHPX-m1e-9c612d71 abandon actor=human:Wido targets=pluggable-proof-runner reason=Superseded (audit 2026-10-02): the plain lane runs whatever landing.prove.command is configured
Integrity: sha256=16726035d302d2c23632191cb1701e0bff38159b5d0bc8a4ada86a85bb238b00

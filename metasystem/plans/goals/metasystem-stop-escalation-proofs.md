# metasystem-stop-escalation-proofs

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: an unproven escalation path means a stop that reports killed without evidence it killed; novelty 2: a held fake host and its readiness handshake do not exist yet; exposure 2: every seat that stops a checkout with a mission, a suite or an ignored signal in flight; accumulation 1: the scenarios are written once"
- Tier: 2
- Intent: Slice 1b of the stop verb: the escalation and per-family fixture scenarios that slice 1 deliberately left out, plus the fixture-only held fake host they need. Slice 1 (chain stopverb-build1, goal metasystem-stop-verb) builds and proves the human-visible contract with five scenarios; section 10 of plans/metasystem-stop-verb-design.md names the rest: arm-refuses-survivor, mission-stop with its ignore-TERM and host-ignores-TERM variants and its dead-runner live-turn case, proof-run-stop with its dead-watchdog case, slow-owner, crash-recovery, remote-job, ignored-signal and wrong-terminal. DONE means every one of those scenarios exists, passes outside a delegate sandbox, and would fail against the tree before it
- Origin: main
- Next step: Blocked until slice 1 lands. Then: specify the held fake host in scripts/agents/hosts/fake.sh (how a turn selects the held behaviour and how the fixture proves its signal handler is installed before stop runs; round 5 of the slice-1 chain stopped on exactly this and was right to), then write the scenarios in the beds section 10 assigns them, then one code critique. The orchestrator runs the supervision, mission and suite-progress beds outside the sandbox: a delegate sandbox cannot execute a freshly copied fixture engine (status 126) nor authenticate spawned process groups
- OpenedAt: 2026-09-07T07:30:28Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T07:30:28Z 9S47R8KRJBAWTSFQ1TJQDEMD2S-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=metasystem-stop-escalation-proofs
Integrity: sha256=40e6f09224d2bcd954ebf2e1b5d4320a5989d7930a158a8715db55ef45002190

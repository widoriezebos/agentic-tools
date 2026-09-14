# dispatch-bed-preamble-check-survives-a-large-prompt

- State: approved
- Priority: 1
- Sequence: 56
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: a false red in one fixture leg, product behaviour unaffected; novelty 1: the fix pattern is already landed twice; exposure 2: every run of the dispatcher deep section goes red; accumulation 1: one site, and the class gate check in pipelines-never-lose-a-truncated-producer catches the rest later"
- Tier: 1
- Intent: The dispatcher section's mission-runner scenario checks that the host-turn prompt opens with the orchestrator preamble by piping tail -c into head -c into cmp under pipefail, at scripts/agents/dispatch-fixtures.sh lines 4929 to 4931. head exits after its bytes, so whenever more than a pipe buffer of prompt remains past the preamble, tail takes SIGPIPE and the check reports 'host-turn prompt does not open with the orchestrator preamble' even when the bytes match. It is red on trunk 914b61ed in two harness-tracked runs by m1e. Class: pipelines-never-lose-a-truncated-producer. DONE: the check reads the preamble-sized prefix without a producer piped into an early-exiting reader and checks each command's status separately; a leg proves it with a prompt whose remainder exceeds 64 KiB; the mission-runner scenario passes on trunk.
- Origin: human
- Next step: Build, tier 1: extract the prompt's preamble-sized prefix to a file, or read it by byte count, and compare that with cmp against the orchestrator role file, checking each status separately. Add the large-prompt leg. The same pattern landed in 71b96a02 and 9bc768c0. The goal is free; m1b and m1c are busy, so it waits for the next free seat.
- OpenedAt: 2026-09-14T22:02:05Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T22:02:13Z revision=2 opid=3AWAZGMSCCBWKS7NKZRC8HBH3S-m1e-c6925449 authority=proven digest=29a75fd80201ab4654822efb15b6aa3a89b8eae4610079fdfa0ccc62606a6a82

History:
- 2026-09-14T22:02:05Z V5NNW0CNA231NAHKXBTPZQ191B-m1e-c6925449 open actor=human:Wido targets=dispatch-bed-preamble-check-survives-a-large-prompt
- 2026-09-14T22:02:13Z 3AWAZGMSCCBWKS7NKZRC8HBH3S-m1e-c6925449 approve actor=human:Wido targets=dispatch-bed-preamble-check-survives-a-large-prompt
- 2026-09-14T22:02:27Z YXPNR3MDTBG9YX553H61D226S7-m1e-c6925449 set-priority actor=human:Wido targets=dispatch-bed-preamble-check-survives-a-large-prompt reason=priority-order subject=dispatch-bed-preamble-check-survives-a-large-prompt from=unranked to=1:56 requested-sequence=append
Integrity: sha256=c91df040dfa6193a93dce67b30129b20cde960c553843f30d2c485795e4cca0f

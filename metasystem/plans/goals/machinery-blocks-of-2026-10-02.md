# machinery-blocks-of-2026-10-02

- State: queued
- Priority: 2
- Sequence: 50
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="Each block stops an agent but the escape hatch asks the human (severity 1); known defects with traced causes (novelty 1); every seat (exposure 2); recurs on every run (accumulation 2)"
- Tier: 1
- Intent: Fix, by frequency once the unblock log shows them, the eight machinery blocks one seat hit on 2026-10-02 (each one a human question under blocked-agent-asks-the-human until fixed): (1) goal claim without METASYSTEM_OWNER_LINEAGE is read as a person and the refusal names the wrong cause (intent_planning.go:474-540); (2) critique chain rounds are filed under the previous round's job: collect says findings cannot be read, a zero-finding final round cannot close (design-critic-09e95ff70db9b256797eb586); (3) work review G --patch is refused by the seat's own commit guard, and the goal-free read failed without feedback; (4) with a lane registered, work land refuses any non-goal commit; (5) the Stop hook blocks every turn while the agent waits on in-process subagents and costs 15-16 s of a 15 s threshold; (6) 193 open alerts, mostly stale, and health roles with 'no lawful remedy'; (7) stale source copies under artifacts/ break go vet ./... and the static gate; (8) messages whose remedy is wrong ('the goals changed meanwhile; try again' after an approved-intent or not-open refusal). Evidence: agentic-tools-evidence/flaky-20261002/log.md.
- Origin: main
- Next step: Wait for the unblock log after switch-on; fix the most frequent block first, one at a time, by subtraction
- OpenedAt: 2026-10-02T12:29:50Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-02T12:29:50Z 2R9JM2HYAZY2AZW272ZF1ZEV2X-m1e-9c612d71 open actor=human:Wido targets=machinery-blocks-of-2026-10-02
- 2026-10-02T12:29:57Z B8YKA2JDWBD1NZBQ3DB175XF2C-m1e-9c612d71 set-priority actor=human:Wido targets=machinery-blocks-of-2026-10-02 reason=priority-order subject=machinery-blocks-of-2026-10-02 from=unranked to=2:50 requested-sequence=append
Integrity: sha256=63d89b29a30841dee426b9345cfea97c549759defb0e3ba8046a9e0045b7a3d3

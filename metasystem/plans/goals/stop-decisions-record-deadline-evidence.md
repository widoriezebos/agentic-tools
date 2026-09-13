# stop-decisions-record-deadline-evidence

- State: claimed
- Risk: severity=2 novelty=3 exposure=3 accumulation=2 basis="severity 2: a refusal emitted without a durable decision behind it can neither be audited nor drained, and a decision recorded twice per deadline double-counts an incident; novelty 3: a version-2 stop decision record shared by the hook, its deadline parent and the steward is a new durable format with a new identity (turn generation plus deadline end); exposure 3: every stop of every seat; accumulation 2: the record is read by the steward's drain (member stop-incidents-reach-the-steward) and by the seven-day count"
- Tier: 3
- Intent: Member 2 of stop-hook-never-forces-an-empty-turn (its design page, section 7; this member's page plans/stop-decisions-record-deadline-evidence-design.md): each emitted refusal has a prior durable decision with its full up result and the checkout generation, and each incident episode is keyed to the hook's turn generation and deadline end, so the deadline parent and the worker never record the same stop twice and a later drain can tell episodes apart. DONE means: (1) the stop decision record (version 2) is written before any refusal or degraded notice is emitted, carrying the class, cause, component, the full up result as up prints it, the checkout's enrollment generation, the turn generation and the deadline end; (2) an episode is keyed by turn generation and deadline end, and the deadline parent's completion of a worker's decision is single-use: a second completion for the same key changes nothing and says so; (3) an emission whose decision could not be written is itself recorded in the hook log with the write failure and the notice says it; (4) TestArmingDetailSurvivesStop keeps passing: the arming detail reaches the notice byte for byte from the record. Fixtures: TestInfrastructureDeadlineIdentity, TestStopDecisionPersistsBeforeEmission, TestStopDeadlineCompletionIsSingleUse, TestArmingDetailSurvivesStop.
- Origin: main
- Next step: 2026-09-13 09:3xZ m1b: design page LANDED fc86daa97 with its critique brief; the rostered Codex sol design read runs as job member2-design-read-1 (DESIGN-BEARING, SDE-nn findings). On its return: fold, land the fold, close the design chain, then the Codex sol build in a worktree (brief from sections 2, 5, 6), Opus reads, a human commit.
- OpenedAt: 2026-09-13T07:33:02Z
- Revision: 7
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-13T07:33:06Z revision=2 opid=FFGKF0S19KMMHEQGBWCYDSMNY5-m1b-30a7e141 authority=proven digest=f4e2424715bc04c63c301ef8bda578724d344a8a7b8e940dcfbc9aba79d834b7
- Sliced: machine=m1b lineage=main-1789191336-90295-e4b24b revision=3 at=2026-09-13T07:34:48Z
- Claimed: machine=m1b lineage=main-1789191336-90295-e4b24b at=2026-09-13T08:18:07Z revision=7 accountingRevision=3 episodeAt=2026-09-13T07:33:09Z episodeRevision=3 idleSeconds=1591
- StopCapability: generation=7 revision=7 machine=m1b claimEpoch=2 fenceEpoch=0

History:
- 2026-09-13T07:33:02Z 5HG893JMDPRP75G97Y7F9VH7Z8-m1b-30a7e141 open actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence,stop-hook-never-forces-an-empty-turn
- 2026-09-13T07:33:06Z FFGKF0S19KMMHEQGBWCYDSMNY5-m1b-30a7e141 approve actor=human:Wido targets=stop-decisions-record-deadline-evidence
- 2026-09-13T07:33:09Z XQ7QJ39E7D41YY8VYERHA1QJS0-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T07:34:48Z 1V6ZNPPCN7F28F6GZP8K8ZWF1B-m1b-30a7e141 slice-start actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T07:35:10Z DY0P1ET0V12XDX6SGPEKMY5HVC-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T07:51:36Z RBCG3XMYMYQEQJGXBXTXH7NKVW-m1b-30a7e141 release actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
- 2026-09-13T08:18:07Z H2W3Y3JFJHQZCKHY4A67C2DF18-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=stop-decisions-record-deadline-evidence
Integrity: sha256=221070e7debc65313840cd1fe315f69343960374ddb3ff8cdc48ac00245c687b

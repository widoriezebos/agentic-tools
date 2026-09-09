# hp-recovery-grants-no-grade-to-unstamped-intents

- State: approved
- Risk: severity=3 novelty=2 exposure=2 accumulation=1 basis="severity 3: recover today completes a dead session's human-attributed intent without a fresh check; novelty 2: a stamp read on the journal path; exposure 2: every recovery; accumulation 1: nothing compounds"
- Tier: 3
- Intent: goal recover never completes a pre-change journal entry that carries --by but no authority grade: a missing stamp grants no grade and terminalizes toward a fresh human invocation, and only an explicitly human-ratified migration record grandfathers an old entry. Finding HPA-11.
- Origin: main
- Next step: Sol changes internal/goal/recover.go to refuse the stored intent when its authorityGrade is absent and to record the terminalization; a fixture seeds a dead owner's journal with an unstamped foreign release and proves recover does not run it. One Sol round, one Opus review, land. ORDER: after hp-every-by-proves-a-human lands. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed).
- OpenedAt: 2026-09-09T14:35:20Z
- Revision: 2
- Labels: security
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-09T14:36:28Z revision=2 opid=76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 authority=proven digest=734c8c4a6969e71d5375e6a34a71f85eb2b9e656d3d24ec17d6962a5f852058c

History:
- 2026-09-09T14:35:20Z 2QDHB7NZX4DBPPVZ88F5HWMZ9A-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-recovery-grants-no-grade-to-unstamped-intents
- 2026-09-09T14:36:28Z 76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 approve actor=human:Wido targets=fixture-human-authority-mintable-from-conf,hp-enrollment-before-migration,hp-every-by-proves-a-human,hp-grading-matrix-for-widening-acts,hp-recovery-grants-no-grade-to-unstamped-intents,hp-refusals-print-the-one-command,hp-relayed-word-retired,hp-resume-takes-its-budget-from-the-ledger,hp-terminal-grade-for-stopping-acts
Integrity: sha256=b01d8b977c9e365618ca1ab063a60a166bddc70b1a1f31b58ce9a00d9e6d7333

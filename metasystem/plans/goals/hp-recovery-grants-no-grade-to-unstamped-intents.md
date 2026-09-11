# hp-recovery-grants-no-grade-to-unstamped-intents

- State: queued
- Risk: severity=3 novelty=2 exposure=2 accumulation=1 basis="severity 3: recover today completes a dead session's human-attributed intent without a fresh check; novelty 2: a stamp read on the journal path; exposure 2: every recovery; accumulation 1: nothing compounds"
- Tier: 3
- Intent: goal recover never completes a pre-change journal entry that carries --by but no authority grade: a missing stamp grants no grade and terminalizes toward a fresh human invocation, and only an explicitly human-ratified migration record grandfathers an old entry. Finding HPA-11.
- Origin: main
- Next step: Sol changes internal/goal/recover.go to refuse the stored intent when its authorityGrade is absent and to record the terminalization; a fixture seeds a dead owner's journal with an unstamped foreign release and proves recover does not run it. One Sol round, one Opus review, land. ORDER: after hp-every-by-proves-a-human lands. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed).
- OpenedAt: 2026-09-09T14:35:20Z
- Revision: 3
- Labels: security
- BudgetExceptions: 0

History:
- 2026-09-09T14:35:20Z 2QDHB7NZX4DBPPVZ88F5HWMZ9A-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-recovery-grants-no-grade-to-unstamped-intents
- 2026-09-09T14:36:28Z 76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 approve actor=human:Wido targets=fixture-human-authority-mintable-from-conf,hp-enrollment-before-migration,hp-every-by-proves-a-human,hp-grading-matrix-for-widening-acts,hp-recovery-grants-no-grade-to-unstamped-intents,hp-refusals-print-the-one-command,hp-relayed-word-retired,hp-resume-takes-its-budget-from-the-ledger,hp-terminal-grade-for-stopping-acts
- 2026-09-11T07:59:58Z DTBSWBP0TVKJWXMMM7DJGBQ18V-m1-c6925449 unapprove actor=human:Wido targets=hp-recovery-grants-no-grade-to-unstamped-intents reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
Integrity: sha256=0afdc21baa49e8f7cb19965374b10da3be723a47624b11f693287623b78c1071

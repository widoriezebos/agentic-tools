# hp-resume-takes-its-budget-from-the-ledger

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: it reads the approved budget the ledger holds and refuses a differing one, so a bug could resume under the wrong tuple; novelty 1: a default from the record; exposure 2: every resume of a stopped goal; accumulation 1: nothing compounds"
- Tier: 2
- Intent: goal resume reads the standing approved budget from the ledger instead of demanding the five-member tuple retyped on the command line; a human who wants a different budget uses set-budget, and a mismatch refusal prints the standing values. Design revision 2 section 4.
- Origin: main
- Next step: Sol changes runGoalResume to take the goal's Budget when no tuple is given, keeps the explicit tuple as an override that must equal the standing one, and prints the standing values on mismatch; a fixture resumes a breach-stopped goal with no budget flags. One Sol round, one Opus review, land. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed).
- OpenedAt: 2026-09-09T14:35:12Z
- Revision: 3
- Labels: comfort
- BudgetExceptions: 0

History:
- 2026-09-09T14:35:12Z HCFBR9NPS8CRJS3DZFGSJC2TY7-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-resume-takes-its-budget-from-the-ledger
- 2026-09-09T14:36:28Z 76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 approve actor=human:Wido targets=fixture-human-authority-mintable-from-conf,hp-enrollment-before-migration,hp-every-by-proves-a-human,hp-grading-matrix-for-widening-acts,hp-recovery-grants-no-grade-to-unstamped-intents,hp-refusals-print-the-one-command,hp-relayed-word-retired,hp-resume-takes-its-budget-from-the-ledger,hp-terminal-grade-for-stopping-acts
- 2026-09-11T08:00:13Z Y4P6V4DJFC6185XT3TYKX2664P-m1-c6925449 unapprove actor=human:Wido targets=hp-resume-takes-its-budget-from-the-ledger reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
Integrity: sha256=8a72f7158cf9ee9ed65cb2e198f45df227300f6093adba6f9229ffd04541c705

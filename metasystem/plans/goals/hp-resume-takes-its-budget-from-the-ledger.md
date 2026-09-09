# hp-resume-takes-its-budget-from-the-ledger

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: it reads the approved budget the ledger holds and refuses a differing one, so a bug could resume under the wrong tuple; novelty 1: a default from the record; exposure 2: every resume of a stopped goal; accumulation 1: nothing compounds"
- Tier: 2
- Intent: goal resume reads the standing approved budget from the ledger instead of demanding the five-member tuple retyped on the command line; a human who wants a different budget uses set-budget, and a mismatch refusal prints the standing values. Design revision 2 section 4.
- Origin: main
- Next step: Sol changes runGoalResume to take the goal's Budget when no tuple is given, keeps the explicit tuple as an override that must equal the standing one, and prints the standing values on mismatch; a fixture resumes a breach-stopped goal with no budget flags. One Sol round, one Opus review, land. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed).
- OpenedAt: 2026-09-09T14:35:12Z
- Revision: 2
- Labels: comfort
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-09T14:36:28Z revision=2 opid=76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 authority=proven digest=d47c8e1639f140c5a028c8bb173583f9e19d55c1898a2c8b1d60126ed3be7ac9

History:
- 2026-09-09T14:35:12Z HCFBR9NPS8CRJS3DZFGSJC2TY7-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-resume-takes-its-budget-from-the-ledger
- 2026-09-09T14:36:28Z 76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 approve actor=human:Wido targets=fixture-human-authority-mintable-from-conf,hp-enrollment-before-migration,hp-every-by-proves-a-human,hp-grading-matrix-for-widening-acts,hp-recovery-grants-no-grade-to-unstamped-intents,hp-refusals-print-the-one-command,hp-relayed-word-retired,hp-resume-takes-its-budget-from-the-ledger,hp-terminal-grade-for-stopping-acts
Integrity: sha256=4d265c78911df4e69a97cec60d7176ef9c160e475abb78c1ea812b358433581b

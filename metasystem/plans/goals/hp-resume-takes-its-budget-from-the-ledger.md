# hp-resume-takes-its-budget-from-the-ledger

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: it reads the approved budget the ledger holds and refuses a differing one, so a bug could resume under the wrong tuple; novelty 1: a default from the record; exposure 2: every resume of a stopped goal; accumulation 1: nothing compounds"
- Tier: 2
- Intent: goal resume reads the standing approved budget from the ledger instead of demanding the five-member tuple retyped on the command line; a human who wants a different budget uses set-budget, and a mismatch refusal prints the standing values. Design revision 2 section 4.
- Origin: main
- Next step: Sol changes runGoalResume to take the goal's Budget when no tuple is given, keeps the explicit tuple as an override that must equal the standing one, and prints the standing values on mismatch; a fixture resumes a breach-stopped goal with no budget flags. One Sol round, one Opus review, land. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed).
- OpenedAt: 2026-09-09T14:35:12Z
- Revision: 1
- Labels: comfort
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-09T14:35:12Z HCFBR9NPS8CRJS3DZFGSJC2TY7-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-resume-takes-its-budget-from-the-ledger
Integrity: sha256=fa864effa2a1688855cc0741724cda060eeccd4f08f4d5e3922f717bebb69c6b

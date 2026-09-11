# hp-enrollment-before-migration

- State: queued
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="severity 2: bootstrap ordering of enrollment and migration; a wrong order leaves migrate on the weaker grade; novelty 2: splitting enroll into local write and fleet publication; exposure 1: one act per checkout lifetime; accumulation 1: nothing compounds"
- Tier: 2
- Intent: A checkout can enroll a terminal before its backlog is migrated, and the fleet-cutoff publication waits until the backlog exists, so migrate can take the enrolled grade and the bootstrap needs no exception. Finding HPA-05.
- Origin: main
- Next step: Sol splits enroll-terminal into the local write and the fleet publication, defers the publication until the backlog is converted, and grades migrate enrolled; a fixture enrolls on an unconverted checkout, migrates, and sees the cutoff published after. One Sol round, one Opus review, land. ORDER: after hp-every-by-proves-a-human lands. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed).
- OpenedAt: 2026-09-09T14:35:30Z
- Revision: 3
- Labels: security
- BudgetExceptions: 0

History:
- 2026-09-09T14:35:30Z S5HBNF0PRTRKM216MAZA9BB25C-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-enrollment-before-migration
- 2026-09-09T14:36:28Z 76XV3QEKM1Y759NSDZ4GYEH9R3-m1-c6925449 approve actor=human:Wido targets=fixture-human-authority-mintable-from-conf,hp-enrollment-before-migration,hp-every-by-proves-a-human,hp-grading-matrix-for-widening-acts,hp-recovery-grants-no-grade-to-unstamped-intents,hp-refusals-print-the-one-command,hp-relayed-word-retired,hp-resume-takes-its-budget-from-the-ledger,hp-terminal-grade-for-stopping-acts
- 2026-09-11T07:59:46Z 0MXKAV6JW5FSD6MPZMNRJ19DVB-m1-c6925449 unapprove actor=human:Wido targets=hp-enrollment-before-migration reason=Wido 2026-09-11: standing approval withdrawn to regain control of what is built next; re-approve deliberately
Integrity: sha256=bed0bca600a49f0527f2d830e06c6b1263f4b7e6d65044ef37744bba2baa43fb

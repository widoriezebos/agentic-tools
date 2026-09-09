# hp-enrollment-before-migration

- State: queued
- Risk: severity=2 novelty=2 exposure=1 accumulation=1 basis="severity 2: bootstrap ordering of enrollment and migration; a wrong order leaves migrate on the weaker grade; novelty 2: splitting enroll into local write and fleet publication; exposure 1: one act per checkout lifetime; accumulation 1: nothing compounds"
- Tier: 2
- Intent: A checkout can enroll a terminal before its backlog is migrated, and the fleet-cutoff publication waits until the backlog exists, so migrate can take the enrolled grade and the bootstrap needs no exception. Finding HPA-05.
- Origin: main
- Next step: Sol splits enroll-terminal into the local write and the fleet publication, defers the publication until the backlog is converted, and grades migrate enrolled; a fixture enrolls on an unconverted checkout, migrates, and sees the cutoff published after. One Sol round, one Opus review, land. ORDER: after hp-every-by-proves-a-human lands. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed).
- OpenedAt: 2026-09-09T14:35:30Z
- Revision: 1
- Labels: security
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-09T14:35:30Z S5HBNF0PRTRKM216MAZA9BB25C-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-enrollment-before-migration
Integrity: sha256=d4624999d0c0c0c422eca162eb42b5e4154bfe1c5fb4a971a6d2012f6671edde

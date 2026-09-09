# hp-recovery-grants-no-grade-to-unstamped-intents

- State: queued
- Risk: severity=3 novelty=2 exposure=2 accumulation=1 basis="severity 3: recover today completes a dead session's human-attributed intent without a fresh check; novelty 2: a stamp read on the journal path; exposure 2: every recovery; accumulation 1: nothing compounds"
- Tier: 3
- Intent: goal recover never completes a pre-change journal entry that carries --by but no authority grade: a missing stamp grants no grade and terminalizes toward a fresh human invocation, and only an explicitly human-ratified migration record grandfathers an old entry. Finding HPA-11.
- Origin: main
- Next step: Sol changes internal/goal/recover.go to refuse the stored intent when its authorityGrade is absent and to record the terminalization; a fixture seeds a dead owner's journal with an unstamped foreign release and proves recover does not run it. One Sol round, one Opus review, land. ORDER: after hp-every-by-proves-a-human lands. Shared design: plans/human-proof-fits-the-act-design.md revision 2 (the parent human-proof-fits-the-act, concluded as decomposed).
- OpenedAt: 2026-09-09T14:35:20Z
- Revision: 1
- Labels: security
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T14:35:20Z 2QDHB7NZX4DBPPVZ88F5HWMZ9A-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=hp-recovery-grants-no-grade-to-unstamped-intents
Integrity: sha256=d772efaaaf6684190e4b025316ef030758846dc8dbd11c89c22baeb069585b0f

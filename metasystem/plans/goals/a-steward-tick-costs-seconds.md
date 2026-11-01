# a-steward-tick-costs-seconds

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=3 basis="finding 470 (2026-11-01, a1): runner.log ticks 14 to 16 of the landing steward and ps cputime; host load 29 refused a build at the load gate."
- Tier: 2
- Intent: A steward tick costs seconds, not a minute: the tick line times each duty, the tick reads the ledger once, and the claim-quota role sends one note per blocking occurrence; the landing steward no longer burns a core.
- Origin: human
- Next step: Measure first: a design page that names each duty cost from the tick line (finding 470: the landing steward used 67 CPU minutes in 3 hours, duties 60 to 80 s per 120 s tick); then one unit per owner.
- OpenedAt: 2026-11-01T14:42:55Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=24 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=TQNA3XNY4GCTT2VCBTP35S1JFA-m1e-c9ae359b minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-01T14:44:33Z revision=2 opid=TQNA3XNY4GCTT2VCBTP35S1JFA-m1e-c9ae359b authority=proven digest=517c47e8fa7d9f26c57f941d07a400345b9affcc5325930eed303ca79025f1d2 episode=2

History:
- 2026-11-01T14:42:55Z J5MM9BEZ8NRDGWRWRJFNJ3R3D7-m1e-c9ae359b open actor=human:Wido targets=a-steward-tick-costs-seconds
- 2026-11-01T14:44:33Z TQNA3XNY4GCTT2VCBTP35S1JFA-m1e-c9ae359b approve actor=human:Wido targets=a-steward-tick-costs-seconds
Integrity: sha256=47782bfc3a0e72ce0443ed889a5f95ccba44883ce7388424cb2b9dcfd0dacdf9

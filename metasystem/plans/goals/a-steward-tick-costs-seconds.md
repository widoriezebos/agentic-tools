# a-steward-tick-costs-seconds

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=3 basis="finding 470 (2026-11-01, a1): runner.log ticks 14 to 16 of the landing steward and ps cputime; host load 29 refused a build at the load gate."
- Tier: 2
- Intent: A steward tick costs seconds, not a minute: the tick line times each duty, the tick reads the ledger once, and the claim-quota role sends one note per blocking occurrence; the landing steward no longer burns a core.
- Origin: human
- Next step: Measure first: a design page that names each duty cost from the tick line (finding 470: the landing steward used 67 CPU minutes in 3 hours, duties 60 to 80 s per 120 s tick); then one unit per owner.
- OpenedAt: 2026-11-01T14:42:55Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-01T14:42:55Z J5MM9BEZ8NRDGWRWRJFNJ3R3D7-m1e-c9ae359b open actor=human:Wido targets=a-steward-tick-costs-seconds
Integrity: sha256=ebe676674d51e517fd621eca1e8c0e258472128d9396dfcf21beaa30d4309219

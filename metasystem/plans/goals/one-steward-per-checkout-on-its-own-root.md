# one-steward-per-checkout-on-its-own-root

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="severity 3: a shadow steward writes a parallel artifacts tree and can hold locks and leases nobody sees; novelty 2: root validation and duplicate detection are new; exposure 3: every checkout; accumulation 2: steward start path, a health role and the fleet-doctor check"
- Tier: 3
- Intent: On 2026-09-16 a second steward ran six hours on the m1e checkout against the gitignored repo-root artifacts/ instead of metasystem/artifacts/, stuck at generation 1 and sequence 42 while the real steward stood at 931, writing a shadow artifacts tree nobody read, with no supervise-owner and no health role noticing until a housekeeping census found it by process table. DONE: a steward refuses to start on a root that is not an enrolled metasystem root, naming the root it was given and the one it expected; a second steward on the same checkout is refused or reaped with a notice; a health role reports a duplicate or mis-rooted steward within one tick; fleet-doctor-repairs-what-stops-other-seats consumes the check; a fixture reproduces the 2026-09-16 shape; proven on two runtimes.
- Origin: human
- Next step: Find how the second steward was started and why its root check passed (the census report in m1e scratch names its pid and command line); design the root refusal, the duplicate rule and the health role as one small page or a tier-3 build spec; build; land.
- OpenedAt: 2026-09-16T06:44:22Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-16T06:44:22Z JRGD57DA2P07E8JW4HNZMT2G4G-m1e-c6925449 open actor=human:Wido targets=one-steward-per-checkout-on-its-own-root
Integrity: sha256=7f5e4f4897fa4a551b52f04dbb6a49e9d71eb6df3dff9cffb4f305e68b235993

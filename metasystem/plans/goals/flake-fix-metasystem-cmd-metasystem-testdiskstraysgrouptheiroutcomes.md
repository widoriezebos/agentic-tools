# flake-fix-metasystem-cmd-metasystem-testdiskstraysgrouptheiroutcomes

- State: queued
- Priority: 1
- Sequence: 152
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261117T043848.325221000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T043848.325221000Z.log, load 10.4521484375; attempt 20261117T090114.007092000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T090114.007092000Z.log, load 4.0927734375"
- Tier: 3
- Intent: the test TestDiskStraysGroupTheirOutcomes in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261117T043848.325221000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T043848.325221000Z.log, load 10.4521484375; attempt 20261117T090114.007092000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T090114.007092000Z.log, load 4.0927734375); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-11-17T10:12:17Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-17T10:12:17Z 964326EEFE42C622FF32EE7B71-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testdiskstraysgrouptheiroutcomes
- 2026-11-17T12:29:19Z CCTY8MDBZC3AD8PEVAN02F0FXB-m1e-f2b56c95 abandon actor=human:Wido targets=flake-fix-metasystem-cmd-metasystem-testrevivalusestheinjectedprober,flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain,flake-fix-metasystem-cmd-metasystem-testdiskstraysgrouptheiroutcomes reason=priority-order from=1:153 to=1:152
Integrity: sha256=453b6a64f3b1d79fb6789ec6c936554058784b54afed3bd5680cd323eb3d2579

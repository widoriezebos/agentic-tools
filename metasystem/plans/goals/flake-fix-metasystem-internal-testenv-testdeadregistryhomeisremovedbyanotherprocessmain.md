# flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain

- State: queued
- Priority: 1
- Sequence: 63
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261019T212050.742776000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T212050.742776000Z.log, load 5.310546875; attempt 20261020T054219.063899000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T054219.063899000Z.log, load 4.70703125"
- Tier: 3
- Intent: the test TestDeadRegistryHomeIsRemovedByAnotherProcessMain in metasystem/internal/testenv failed 2 times in the landing lane and was green on every named repeat (attempt 20261019T212050.742776000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T212050.742776000Z.log, load 5.310546875; attempt 20261020T054219.063899000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T054219.063899000Z.log, load 4.70703125); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-10-20T06:36:34Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-20T06:36:34Z BF387BA723F7CF28D42249C91F-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain
Integrity: sha256=9aaf24d18693efebfa265ba7fd0fa843c13649bdd3a79be3755024e9c7d53b24

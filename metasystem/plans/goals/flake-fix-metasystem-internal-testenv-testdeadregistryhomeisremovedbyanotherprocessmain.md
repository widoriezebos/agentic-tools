# flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain

- State: queued
- Priority: 1
- Sequence: 63
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261019T212050.742776000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T212050.742776000Z.log, load 5.310546875; attempt 20261020T054219.063899000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T054219.063899000Z.log, load 4.70703125"
- Tier: 3
- Intent: the test TestDeadRegistryHomeIsRemovedByAnotherProcessMain in metasystem/internal/testenv failed 2 times in the landing lane and was green on every named repeat (attempt 20261019T212050.742776000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261019T212050.742776000Z.log, load 5.310546875; attempt 20261020T054219.063899000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T054219.063899000Z.log, load 4.70703125); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in Flaky: metasystem/internal/testenv TestDeadRegistryHomeIsRemovedByAnotherProcessMain; attempt 20261020T072937.156672000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T072937.156672000Z.log; load 4.85986328125; seen 3 times. Flaky: metasystem/internal/testenv TestDeadRegistryHomeIsRemovedByAnotherProcessMain; attempt 20261020T105817.013957000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T105817.013957000Z.log; load 5.53125; seen 4 times.
- OpenedAt: 2026-10-20T06:36:34Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-20T06:36:34Z BF387BA723F7CF28D42249C91F-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain
- 2026-10-20T08:14:59Z 56980E2F695FF9A8875CB7FAF0-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain
- 2026-10-20T11:49:36Z 8506656D7593E4266CF867DBD7-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-testenv-testdeadregistryhomeisremovedbyanotherprocessmain
Integrity: sha256=a78f16ab2792e0dc30df32e868268d61c30f9eed83552daab0216e11ab66ab2e

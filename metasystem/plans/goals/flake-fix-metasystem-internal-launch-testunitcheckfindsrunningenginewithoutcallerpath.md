# flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath

- State: queued
- Priority: 1
- Sequence: 64
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261020T105817.013957000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T105817.013957000Z.log, load 5.53125; attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875"
- Tier: 3
- Intent: the test TestUnitCheckFindsRunningEngineWithoutCallerPath in metasystem/internal/launch failed 2 times in the landing lane and was green on every named repeat (attempt 20261020T105817.013957000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T105817.013957000Z.log, load 5.53125; attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-10-20T14:39:53Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-20T14:39:53Z 1B11C6223C24286256C5A69016-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath
Integrity: sha256=1906b03af3929517f76ae7094f1da743ef53c4e850befc721c047ffc85513995

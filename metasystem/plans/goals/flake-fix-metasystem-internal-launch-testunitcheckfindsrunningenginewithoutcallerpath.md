# flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath

- State: queued
- Priority: 1
- Sequence: 64
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261020T105817.013957000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T105817.013957000Z.log, load 5.53125; attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875"
- Tier: 3
- Intent: the test TestUnitCheckFindsRunningEngineWithoutCallerPath in metasystem/internal/launch failed 2 times in the landing lane and was green on every named repeat (attempt 20261020T105817.013957000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T105817.013957000Z.log, load 5.53125; attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in Flaky: metasystem/internal/launch TestUnitCheckFindsRunningEngineWithoutCallerPath; attempt 20261020T152947.568401000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T152947.568401000Z.log; load 4.55419921875; seen 3 times. Flaky: metasystem/internal/launch TestUnitCheckFindsRunningEngineWithoutCallerPath; attempt 20261020T173709.859025000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T173709.859025000Z.log; load 5.27783203125; seen 4 times. Flaky: metasystem/internal/launch TestUnitCheckFindsRunningEngineWithoutCallerPath; attempt 20261020T185403.290178000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T185403.290178000Z.log; load 5.0556640625; seen 5 times. Flaky: metasystem/internal/launch TestUnitCheckFindsRunningEngineWithoutCallerPath; attempt 20261020T205038.868275000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T205038.868275000Z.log; load 5.90771484375; seen 6 times. Flaky: metasystem/internal/launch TestUnitCheckFindsRunningEngineWithoutCallerPath; attempt 20261020T223413.771623000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T223413.771623000Z.log; load 12.27880859375; seen 7 times. Flaky: metasystem/internal/launch TestUnitCheckFindsRunningEngineWithoutCallerPath; attempt 20261021T011340.812329000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T011340.812329000Z.log; load 4.7041015625; seen 8 times.
- OpenedAt: 2026-10-20T14:39:53Z
- Revision: 7
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-20T14:39:53Z 1B11C6223C24286256C5A69016-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath
- 2026-10-20T16:33:39Z AE02EF6E4394718708A076C911-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath
- 2026-10-20T18:38:00Z 522E9A684C7E42A6A310939CBE-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath
- 2026-10-20T20:13:10Z 2284B9A98210D46CD4DD972591-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath
- 2026-10-20T22:15:55Z 4F0EECEDD1763D351DA42B0B2F-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath
- 2026-10-21T00:12:38Z FB7D63560CE2E6251B54B54D89-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath
- 2026-10-21T02:32:18Z EF82971F530A64A91CD7E0BD24-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath
Integrity: sha256=4480cad60868ffbff30dbbbc425ef3d4b772808d1f4bbdc308ffe7c915e9300f

# flake-fix-metasystem-internal-proofrun-testsupervisorkillsparesthefixturecustodian

- State: queued
- Priority: 1
- Sequence: 65
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261020T114943.702998000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T114943.702998000Z.log, load 5.94677734375; attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875"
- Tier: 3
- Intent: the test TestSupervisorKillSparesTheFixtureCustodian in metasystem/internal/proofrun failed 2 times in the landing lane and was green on every named repeat (attempt 20261020T114943.702998000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T114943.702998000Z.log, load 5.94677734375; attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in Flaky: metasystem/internal/proofrun TestSupervisorKillSparesTheFixtureCustodian; attempt 20261020T152947.568401000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T152947.568401000Z.log; load 4.55419921875; seen 3 times.
- OpenedAt: 2026-10-20T14:39:53Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-20T14:39:53Z 9DCE32C23626A87E17C8800B40-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-proofrun-testsupervisorkillsparesthefixturecustodian
- 2026-10-20T16:33:45Z E340B1199D7C526D871745BFAB-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-proofrun-testsupervisorkillsparesthefixturecustodian
Integrity: sha256=00782b2d063f1eec151482ab37d77add5a58b1a19d0c62904a6c0db88f799886

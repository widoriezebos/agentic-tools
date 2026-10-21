# flake-fix-metasystem-internal-proofrun-testsupervisorkillsparesthefixturecustodian

- State: queued
- Priority: 1
- Sequence: 64
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261020T114943.702998000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T114943.702998000Z.log, load 5.94677734375; attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875"
- Tier: 3
- Intent: the test TestSupervisorKillSparesTheFixtureCustodian in metasystem/internal/proofrun failed 2 times in the landing lane and was green on every named repeat (attempt 20261020T114943.702998000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T114943.702998000Z.log, load 5.94677734375; attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in Flaky: metasystem/internal/proofrun TestSupervisorKillSparesTheFixtureCustodian; attempt 20261020T152947.568401000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T152947.568401000Z.log; load 4.55419921875; seen 3 times. Flaky: metasystem/internal/proofrun TestSupervisorKillSparesTheFixtureCustodian; attempt 20261021T053621.325487000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T053621.325487000Z.log; load 5.04443359375; seen 4 times.
- OpenedAt: 2026-10-20T14:39:53Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-20T14:39:53Z 9DCE32C23626A87E17C8800B40-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-proofrun-testsupervisorkillsparesthefixturecustodian
- 2026-10-20T16:33:45Z E340B1199D7C526D871745BFAB-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-proofrun-testsupervisorkillsparesthefixturecustodian
- 2026-10-21T07:24:40Z 4B613A26C807D47C1C21569DEF-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-proofrun-testsupervisorkillsparesthefixturecustodian
- 2026-10-21T10:10:08Z CRG7YRWGEMBWQGNCQ32F17CGDD-m1e-f2f350ed done actor=human:Wido targets=a-delivered-goal-and-a-needed-act-reach-the-person,a-flake-is-not-a-red,a-proved-main-deploys-itself,flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge,flake-fix-metasystem-internal-launch-testunitcheckfindsrunningenginewithoutcallerpath,flake-fix-metasystem-internal-proofrun-testresourcecustodystreamslivecapnoteandretainsspools,flake-fix-metasystem-internal-proofrun-testsupervisorkillsparesthefixturecustodian,flake-fix-metasystem-internal-proofrun-testtestenvironmentstandardinventorymatchesobserved,flake-fix-metasystem-internal-testenv-testsetupchildisendedwhenthebinaryiskilled,the-fixture-custodian-settles-its-home reason=priority-order from=1:65 to=1:64
Integrity: sha256=d640534bdf3d8e59f32f50c5ee8aa9b0bc2f14834f7dc54b9bdeacf26baf051a

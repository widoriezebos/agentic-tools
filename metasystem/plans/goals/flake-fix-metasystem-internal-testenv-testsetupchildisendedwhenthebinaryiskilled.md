# flake-fix-metasystem-internal-testenv-testsetupchildisendedwhenthebinaryiskilled

- State: queued
- Priority: 1
- Sequence: 67
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875; attempt 20261021T033259.667358000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T033259.667358000Z.log, load 4.861328125"
- Tier: 3
- Intent: the test TestSetupChildIsEndedWhenTheBinaryIsKilled in metasystem/internal/testenv failed 2 times in the landing lane and was green on every named repeat (attempt 20261020T133149.932272000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T133149.932272000Z.log, load 5.86669921875; attempt 20261021T033259.667358000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T033259.667358000Z.log, load 4.861328125); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-10-21T05:15:32Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-21T05:15:32Z 96B422FE723095BB8CF6E92EA5-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-testenv-testsetupchildisendedwhenthebinaryiskilled
Integrity: sha256=183cc4a931a82da4f424a4ad7b84c6c23459d15451caf50da28dcafdefa9ef71

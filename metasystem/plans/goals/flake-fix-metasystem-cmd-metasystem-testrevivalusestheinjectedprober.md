# flake-fix-metasystem-cmd-metasystem-testrevivalusestheinjectedprober

- State: queued
- Priority: 1
- Sequence: 151
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261116T162244.432016000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T162244.432016000Z.log, load 13.826171875; attempt 20261116T185957.375997000Z-replay-1, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T185957.375997000Z-replay-1.log, load 24.998046875"
- Tier: 3
- Intent: the test TestRevivalUsesTheInjectedProber in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261116T162244.432016000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T162244.432016000Z.log, load 13.826171875; attempt 20261116T185957.375997000Z-replay-1, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T185957.375997000Z-replay-1.log, load 24.998046875); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-11-17T08:29:59Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-17T08:29:59Z CD9C4553ACE3D95D4EA66B9B56-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testrevivalusestheinjectedprober
Integrity: sha256=27af23976e1cbb686abfd96dd9320ef54e35899f38e8d7cc708a647d3e7dab1b

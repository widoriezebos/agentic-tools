# flake-fix-metasystem-cmd-metasystem-testproviderrecoveryaskpubliclifecycle

- State: queued
- Priority: 1
- Sequence: 69
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261021T184010.477803000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T184010.477803000Z.log, load 4.82275390625; attempt 20261021T204955.725923000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T204955.725923000Z.log, load 7.40185546875"
- Tier: 3
- Intent: the test TestProviderRecoveryAskPublicLifecycle in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261021T184010.477803000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T184010.477803000Z.log, load 4.82275390625; attempt 20261021T204955.725923000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T204955.725923000Z.log, load 7.40185546875); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-10-21T22:25:31Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-21T22:25:31Z 8C00B3B313CE782250BB1A33F9-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testproviderrecoveryaskpubliclifecycle
Integrity: sha256=ab24f74089091c1406fcd3a0c360e165170cf5c8a6fb51d935c6a7bb163c9a93

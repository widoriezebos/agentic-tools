# flake-fix-metasystem-internal-adopt-testadoptgitintegrationvendoredwritersuseapplicationstate

- State: queued
- Priority: 1
- Sequence: 50
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261013T094836.984885000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261013T094836.984885000Z.log, load 10.6103515625; attempt 20261013T143513.616479000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261013T143513.616479000Z.log, load 7.47900390625; attempt 20261013T181326.521731000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261013T181326.521731000Z.log, load 6.00146484375"
- Tier: 3
- Intent: the test TestAdoptGitIntegrationVendoredWritersUseApplicationState in metasystem/internal/adopt failed 3 times in the landing lane and was green on every named repeat (attempt 20261013T094836.984885000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261013T094836.984885000Z.log, load 10.6103515625; attempt 20261013T143513.616479000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261013T143513.616479000Z.log, load 7.47900390625; attempt 20261013T181326.521731000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261013T181326.521731000Z.log, load 6.00146484375); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in Flaky: metasystem/internal/adopt TestAdoptGitIntegrationVendoredWritersUseApplicationState; attempt 20261013T191907.432337000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261013T191907.432337000Z.log; load 6.2021484375; seen 4 times. Flaky: metasystem/internal/adopt TestAdoptGitIntegrationVendoredWritersUseApplicationState; attempt 20261014T043629.867690000Z; log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261014T043629.867690000Z.log; load 5.60302734375; seen 5 times.
- OpenedAt: 2026-10-13T19:06:39Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-13T19:06:39Z 7B44DF923F7C59A6A51E89F808-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-adopt-testadoptgitintegrationvendoredwritersuseapplicationstate
- 2026-10-13T19:57:47Z BDFFDE23982CBC6E6DABFD951C-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-adopt-testadoptgitintegrationvendoredwritersuseapplicationstate
- 2026-10-14T05:14:04Z 0A1506CFC53DE5EDCD04FD3451-landing-41ad7d6c record-flake actor=landing+landing-agent targets=flake-fix-metasystem-internal-adopt-testadoptgitintegrationvendoredwritersuseapplicationstate
Integrity: sha256=21147e0ae964e9b648941a2cd5eedeabca64a4a21f4cf52a9be18d3fb63ec60e

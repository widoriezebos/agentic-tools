# flake-fix-metasystem-cmd-metasystem-testauthenticatedouterproofkeepsitsnestedworkloadoutofo-6b1c9eda

- State: queued
- Priority: 1
- Sequence: 135
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261106T184337.381193000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261106T184337.381193000Z.log, load 2.435546875; attempt 20261107T154055.637178000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261107T154055.637178000Z.log, load 1.98193359375"
- Tier: 3
- Intent: the test TestAuthenticatedOuterProofKeepsItsNestedWorkloadOutOfOuterProgress in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261106T184337.381193000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261106T184337.381193000Z.log, load 2.435546875; attempt 20261107T154055.637178000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261107T154055.637178000Z.log, load 1.98193359375); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-11-07T22:39:19Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T22:39:19Z 243B27E37FD392006531D14CEA-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testauthenticatedouterproofkeepsitsnestedworkloadoutofo-6b1c9eda
Integrity: sha256=c83e7198f6b3d2d4a9393a9ed3913b161a13e6a89cb6c0fdde01430d1ec926a4

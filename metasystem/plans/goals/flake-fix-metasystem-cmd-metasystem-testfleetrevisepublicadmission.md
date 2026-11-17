# flake-fix-metasystem-cmd-metasystem-testfleetrevisepublicadmission

- State: queued
- Priority: 1
- Sequence: 149
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261116T193240.815881000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T193240.815881000Z.log, load 4.77490234375; attempt 20261116T205103.844778000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T205103.844778000Z.log, load 5.03125"
- Tier: 3
- Intent: the test TestFleetRevisePublicAdmission in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261116T193240.815881000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T193240.815881000Z.log, load 4.77490234375; attempt 20261116T205103.844778000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T205103.844778000Z.log, load 5.03125); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-11-17T08:29:59Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-17T08:29:59Z 5A45A8C63B7632C7DD153D3FDF-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testfleetrevisepublicadmission
Integrity: sha256=291e00f2bcd75b71ada1842561071c1ca66ac608d3257e718420fdc848afbe51

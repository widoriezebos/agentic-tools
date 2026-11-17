# flake-fix-metasystem-cmd-metasystem-testintentreservedgoalnames

- State: queued
- Priority: 1
- Sequence: 146
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261116T193240.815881000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T193240.815881000Z.log, load 4.77490234375; attempt 20261116T205103.844778000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T205103.844778000Z.log, load 5.03125; attempt 20261116T221038.532196000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T221038.532196000Z.log, load 4.78125; attempt 20261116T234231.907872000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T234231.907872000Z.log, load 7.72509765625; attempt 20261117T030623.486674000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T030623.486674000Z.log, load 5.8623046875"
- Tier: 3
- Intent: the test TestIntentReservedGoalNames in metasystem/cmd/metasystem failed 5 times in the landing lane and was green on every named repeat (attempt 20261116T193240.815881000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T193240.815881000Z.log, load 4.77490234375; attempt 20261116T205103.844778000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T205103.844778000Z.log, load 5.03125; attempt 20261116T221038.532196000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T221038.532196000Z.log, load 4.78125; attempt 20261116T234231.907872000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T234231.907872000Z.log, load 7.72509765625; attempt 20261117T030623.486674000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T030623.486674000Z.log, load 5.8623046875); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-11-17T08:29:59Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-17T08:29:59Z DC7F9F2490E4FBBE67A89AA588-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testintentreservedgoalnames
Integrity: sha256=03cf627979bdbb7afc209bcffc9bfab70c98abc47a5924a2934867d30239bd19

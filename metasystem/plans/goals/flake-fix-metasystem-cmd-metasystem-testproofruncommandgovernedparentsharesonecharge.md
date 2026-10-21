# flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge

- State: queued
- Priority: 1
- Sequence: 69
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261020T223413.771623000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T223413.771623000Z.log, load 12.27880859375; attempt 20261021T053621.325487000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T053621.325487000Z.log, load 5.04443359375"
- Tier: 3
- Intent: the test TestProofRunCommandGovernedParentSharesOneCharge in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261020T223413.771623000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T223413.771623000Z.log, load 12.27880859375; attempt 20261021T053621.325487000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T053621.325487000Z.log, load 5.04443359375); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-10-21T07:44:17Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-21T07:44:17Z 4D454710DCDC8F20695D31CB70-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testproofruncommandgovernedparentsharesonecharge
Integrity: sha256=d0aeb757e70455574886a52d7d36a83e75ebb8ae54af5da7c5b875a93d38cab0

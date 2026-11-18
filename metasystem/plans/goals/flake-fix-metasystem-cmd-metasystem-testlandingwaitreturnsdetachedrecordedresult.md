# flake-fix-metasystem-cmd-metasystem-testlandingwaitreturnsdetachedrecordedresult

- State: queued
- Priority: 1
- Sequence: 143
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261117T070701.594918000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T070701.594918000Z.log, load 15.1064453125; attempt 20261118T021716.254297000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261118T021716.254297000Z.log, load 11.85205078125"
- Tier: 3
- Intent: the test TestLandingWaitReturnsDetachedRecordedResult in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261117T070701.594918000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261117T070701.594918000Z.log, load 15.1064453125; attempt 20261118T021716.254297000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261118T021716.254297000Z.log, load 11.85205078125); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-11-18T05:16:05Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-18T05:16:05Z 980FF21B9EA4D0BAEF3F971BE3-landing-m1f-41ad7d6c open actor=landing-m1f+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testlandingwaitreturnsdetachedrecordedresult
Integrity: sha256=ba8256521e31363c3c452f61242bf491d1a55675158c71bc293edddf9574ef62

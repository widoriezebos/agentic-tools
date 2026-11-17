# flake-fix-metasystem-cmd-metasystem-testfrozenpublicversiononecorpusrunsallsixcasesthroughf-970ab07d

- State: queued
- Priority: 1
- Sequence: 147
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261107T154055.637178000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261107T154055.637178000Z.log, load 1.98193359375; attempt 20261116T205103.844778000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T205103.844778000Z.log, load 5.03125"
- Tier: 3
- Intent: the test TestFrozenPublicVersionOneCorpusRunsAllSixCasesThroughFirstTransitionWorker in metasystem/cmd/metasystem failed 2 times in the landing lane and was green on every named repeat (attempt 20261107T154055.637178000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261107T154055.637178000Z.log, load 1.98193359375; attempt 20261116T205103.844778000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261116T205103.844778000Z.log, load 5.03125); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-11-17T08:29:59Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-17T08:29:59Z E7BBF31FF8A6F597721B19710D-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-cmd-metasystem-testfrozenpublicversiononecorpusrunsallsixcasesthroughf-970ab07d
Integrity: sha256=0105cd647f5eb6b265e5d6998ef1f4e6c6bb4969513b45f1f95bb95ba7ab85d6

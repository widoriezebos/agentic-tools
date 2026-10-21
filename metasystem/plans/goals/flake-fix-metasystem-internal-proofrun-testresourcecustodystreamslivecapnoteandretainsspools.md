# flake-fix-metasystem-internal-proofrun-testresourcecustodystreamslivecapnoteandretainsspools

- State: queued
- Priority: 1
- Sequence: 68
- Risk: severity=3 novelty=1 exposure=3 accumulation=3 basis="attempt 20261020T223413.771623000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T223413.771623000Z.log, load 12.27880859375; attempt 20261021T033259.667358000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T033259.667358000Z.log, load 4.861328125"
- Tier: 3
- Intent: the test TestResourceCustodyStreamsLiveCapNoteAndRetainsSpools in metasystem/internal/proofrun failed 2 times in the landing lane and was green on every named repeat (attempt 20261020T223413.771623000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261020T223413.771623000Z.log, load 12.27880859375; attempt 20261021T033259.667358000Z, log /Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261021T033259.667358000Z.log, load 4.861328125); make it hold under load without a wall clock (R-104: artificial clocks, never load-fragile tests)
- Origin: main
- Next step: fix the test, run it 20 times under -p 4 with the package, hand it in
- OpenedAt: 2026-10-21T05:15:32Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-21T05:15:32Z B0918E1D6B47C1D9B03FB8D501-landing-41ad7d6c open actor=landing+landing-agent targets=flake-fix-metasystem-internal-proofrun-testresourcecustodystreamslivecapnoteandretainsspools
Integrity: sha256=66636158c60ab8d76c0df55fffad1f4f729e7684cdde6b931e1fbb7d664b3420

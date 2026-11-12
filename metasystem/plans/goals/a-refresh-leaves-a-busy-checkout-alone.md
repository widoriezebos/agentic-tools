# a-refresh-leaves-a-busy-checkout-alone

- State: approved
- Risk: severity=3 novelty=1 exposure=2 accumulation=1 basis="The m1e checkout's index and worktree were left at a pre-merge tree under a moved HEAD after the refresh raced another writer (607)."
- Tier: 3
- Intent: The seat steward's engine refresh never restores an index it did not change: when it cannot take the index lock it stops before changing anything and retries; on a later failure it undoes only its own steps (finding 607, analysis cause K1).
- Origin: main
- Next step: Build one unit (45, --last) from a brief cut and Astra-checked before the build; program design /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/the-route-holds-when-something-deviates.md decision D12.
- OpenedAt: 2026-11-12T16:59:02Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=2d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=G10AMSJ4WWJZHD1R26JFPTR5S9-m1e-5a087708 minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-12T17:01:22Z revision=2 opid=G10AMSJ4WWJZHD1R26JFPTR5S9-m1e-5a087708 authority=proven digest=3c043773e1d60ef0975208735141aad90000228efd78cda094609105c9ff9259 episode=2

History:
- 2026-11-12T16:59:02Z V73J5CJC9DQDGWPD9B8349E03D-m1e-5a087708 open actor=human:Wido targets=a-refresh-leaves-a-busy-checkout-alone
- 2026-11-12T17:01:22Z G10AMSJ4WWJZHD1R26JFPTR5S9-m1e-5a087708 approve actor=human:Wido targets=a-refresh-leaves-a-busy-checkout-alone
- 2026-11-12T17:02:30Z 4693STA7ZGCH4R3WMYP54RV1S0-m1e-5a087708 set-pin actor=human:Wido targets=a-refresh-leaves-a-busy-checkout-alone
Integrity: sha256=2ce65897c100922ead7353dfaee53e6a361fd173c35da41c5ed282f483b4913d

# a-proved-main-deploys-itself-2

- State: approved
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="the keeper starts deploys of main by itself; a wrong launch deploys a commit twice or misses one, and a person sees it in deploy status"
- Tier: 2
- Intent: The split follow-up of a-proved-main-deploys-itself (Wido's decision 27, 2026-10-22): the lane's keeper deploys a pushed main it observes, once per published push, so a missed or failed deploy is caught up without a person: D7 Published on the push record, D8 the keeper's reconcileDeploy, D9 landing push stops launching, D11 the deploy owners; its design text and round-3 open findings are in the evidence folder (briefs/a-proved-main-deploys-itself-U2-brief.md 'Moved to the follow-up goal' and briefs/a-proved-main-deploys-itself-U2-brief-revision-3-full.md), about 167 production lines; Astra under the rare-case rule.
- Origin: human
- Next step: Re-cut the brief from the evidence folder text under the rare-case rule, fold the round-3 open findings, one Astra round, then build after an-outside-read-is-a-read-record lands
- OpenedAt: 2026-10-23T06:31:41Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-23T06:31:52Z revision=2 opid=VPVQNX5T9WM0S6C1EC8WY6FARJ-m1e-c951e3fd authority=proven digest=5189e967eeff84bb872297a0d5d8bed1b49095b56f23835a62a35395da8a8d10 episode=2

History:
- 2026-10-23T06:31:41Z FH98JB9CREB195AWRW7BVJ17MQ-m1e-c951e3fd open actor=human:Wido targets=a-proved-main-deploys-itself-2
- 2026-10-23T06:31:52Z VPVQNX5T9WM0S6C1EC8WY6FARJ-m1e-c951e3fd approve actor=human:Wido targets=a-proved-main-deploys-itself-2
Integrity: sha256=2110a6df0d3ccd772c3b2f52ec7529a3935d18c5d5618c80169e5bfb191c270a

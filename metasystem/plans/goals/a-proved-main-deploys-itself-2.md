# a-proved-main-deploys-itself-2

- State: queued
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="the keeper starts deploys of main by itself; a wrong launch deploys a commit twice or misses one, and a person sees it in deploy status"
- Tier: 2
- Intent: The split follow-up of a-proved-main-deploys-itself (Wido's decision 27, 2026-10-22): the lane's keeper deploys a pushed main it observes, once per published push, so a missed or failed deploy is caught up without a person: D7 Published on the push record, D8 the keeper's reconcileDeploy, D9 landing push stops launching, D11 the deploy owners; its design text and round-3 open findings are in the evidence folder (briefs/a-proved-main-deploys-itself-U2-brief.md 'Moved to the follow-up goal' and briefs/a-proved-main-deploys-itself-U2-brief-revision-3-full.md), about 167 production lines; Astra under the rare-case rule.
- Origin: human
- Next step: Re-cut the brief from the evidence folder text under the rare-case rule, fold the round-3 open findings, one Astra round, then build after an-outside-read-is-a-read-record lands
- OpenedAt: 2026-10-23T06:31:41Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-23T06:31:41Z FH98JB9CREB195AWRW7BVJ17MQ-m1e-c951e3fd open actor=human:Wido targets=a-proved-main-deploys-itself-2
Integrity: sha256=013b65920beaabe51aab0b25f7feae84a2de82e1679f5024a3fe99096b086f2e

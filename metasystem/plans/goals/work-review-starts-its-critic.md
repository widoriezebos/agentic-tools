# work-review-starts-its-critic

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Review dispatch reliability; gates every tier 2/3 landing"
- Tier: 2
- Intent: What: work review --commit SHA --goal G reliably starts its independent critic and reports its findings. On 2026-10-01 a ui seat's run left a half-made request: 'whether the critic started isn't known yet', no launch record, no critic process, and a rerun treated it as possibly in flight. Why: tier 2 and 3 goals (most of the backlog) need this read to land; machinery seats can't get past it. Pros: tier 2/3 work can land unattended. Cons: the review dispatch path is old and wide; scope to making one request start or fail cleanly.
- Origin: human
- Next step: Reproduce on current main with a tier-2 goal branch; find where the request is recorded before the critic launches; make a failed launch fail cleanly so a repeat starts it
- OpenedAt: 2026-10-01T15:55:55Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T15:56:09Z revision=2 opid=JTF90THEXAEBZKTT7TJMMM3JK7-m1e-9c612d71 authority=proven digest=4d6c69ba579b5ab2396d8efcb800241ba587804b9961e47434247b36ed8d1dcf episode=2

History:
- 2026-10-01T15:55:55Z CM77Z74SFD4T16WKGNBN0NK4RM-m1e-9c612d71 open actor=human:Wido targets=work-review-starts-its-critic
- 2026-10-01T15:56:09Z JTF90THEXAEBZKTT7TJMMM3JK7-m1e-9c612d71 approve actor=human:Wido targets=work-review-starts-its-critic
Integrity: sha256=9404dcffbc6647a75a7decce4e4c619109cff0229496b90ed2cb05cf175b7914

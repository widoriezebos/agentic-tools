# work-review-starts-its-critic

- State: approved
- Priority: 1
- Sequence: 3
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Review dispatch reliability; gates every tier 2/3 landing"
- Tier: 2
- Intent: What: work review --commit SHA --goal G reliably starts its independent critic and reports its findings. On 2026-10-01 a ui seat's run left a half-made request: 'whether the critic started isn't known yet', no launch record, no critic process, and a rerun treated it as possibly in flight. Why: tier 2 and 3 goals (most of the backlog) need this read to land; machinery seats can't get past it. Pros: tier 2/3 work can land unattended. Cons: the review dispatch path is old and wide; scope to making one request start or fail cleanly.
- Origin: human
- Next step: Reproduce on current main with a tier-2 goal branch; find where the request is recorded before the critic launches; make a failed launch fail cleanly so a repeat starts it
- OpenedAt: 2026-10-01T15:55:55Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-01T15:56:09Z revision=2 opid=JTF90THEXAEBZKTT7TJMMM3JK7-m1e-9c612d71 authority=proven digest=4d6c69ba579b5ab2396d8efcb800241ba587804b9961e47434247b36ed8d1dcf episode=2

History:
- 2026-10-01T15:55:55Z CM77Z74SFD4T16WKGNBN0NK4RM-m1e-9c612d71 open actor=human:Wido targets=work-review-starts-its-critic
- 2026-10-01T15:56:09Z JTF90THEXAEBZKTT7TJMMM3JK7-m1e-9c612d71 approve actor=human:Wido targets=work-review-starts-its-critic
- 2026-10-01T15:56:47Z PC9CMTV9BS2CRVRDFRZVV9WQHX-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,host-setup-from-scratch,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,reviewers-check-the-rulings,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,steward-acts-on-behaviour-patterns,stop-hook-never-forces-an-empty-turn,system-start-launches-your-agent,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,work-review-starts-its-critic reason=priority-order subject=work-review-starts-its-critic from=unranked to=1:3 requested-sequence=3
Integrity: sha256=87610117d81affb5a7fc5494f9546466bda8143386c60b72a844ac3378c7351b

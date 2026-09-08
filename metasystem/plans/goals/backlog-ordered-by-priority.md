# backlog-ordered-by-priority

- State: claimed
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: without an order the fleet works whatever a seat happens to read first, and the human's priorities reach the machines only by hand through pins and prompts; novelty 2: a new field on the goal record, a human verb, an ordered listing and a next verb, all on the ledger's existing grammar; exposure 3: every seat's choice of work on every machine; accumulation 2: every day without it the human re-sorts the backlog in conversation"
- Tier: 3
- Intent: Wido's word 2026-09-06: 'Backlog items need to be sorted in order of priority and then a sequence number. And this should be able to update after the fact so that we can change priority of the backlog.' Today nothing orders the ledger: a goal record has no rank, goal list sorts by nothing, and a seat with a free claim picks by reading the approved unclaimed records; the only order that exists is a pin, the order in a seat's kickoff prompt, and prose in a few records. DONE means: every goal record carries a priority (a small ordered scale, e.g. 1 highest) and a sequence number (its place within that priority, unique among open goals); the pair is set and changed after the fact by a human verb at the enrolled terminal (goal set-priority --by <human> --id <goal> --priority <n> [--sequence <n>], with re-sequencing of the others in that priority when a number is inserted) and recorded as a ledger event like set-pin; goal list prints open goals in that order, priority then sequence, with state and pin; goal next --machine <nick> returns the first approved, unclaimed goal in that order that is unpinned or pinned to that machine, and seats take work through it instead of reading; a goal with no priority yet sorts last, so approval of the grandfathered backlog needs no bulk edit; and the channel's status report shows the top of the order so the human sees what the fleet will do next.
- Origin: main
- Next step: One chain, DESIGN-BEARING at the ledger grammar (a design revision first, small): the record fields and their event grammar, the human verb with its re-sequencing rule, the ordered listing, the next verb and its tie to the claim rule (one claim per machine, pins respected), the status report line; fixtures: set-priority reorders and re-sequences; next returns the expected goal for a pinned and an unpinned machine; a goal without priority sorts last; the sweep-approved backlog stays valid. Wido sets the first priorities himself once the verb exists. Any free seat.
- OpenedAt: 2026-09-06T12:34:05Z
- Revision: 4
- Pinned: m1b
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T12:34:28Z revision=2 opid=YRABK927PRM5N2ASNMKZVEYKWH-m1-7cd0bd60 authority=proven digest=57ae2782caf8a6b6be34f6216d0244fa1b0923b0cd41b028e58e3eaf293f436b
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-08T06:10:06Z revision=4 accountingRevision=4
- StopCapability: generation=4 revision=4 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T12:34:05Z TDE1XRY1CH2KR2VBCXFHJHW82P-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=backlog-ordered-by-priority
- 2026-09-06T12:34:28Z YRABK927PRM5N2ASNMKZVEYKWH-m1-7cd0bd60 approve actor=human:Wido targets=backlog-ordered-by-priority
- 2026-09-06T15:43:18Z DC782RYCANRSNPZP6ZSEBP01VZ-m1-7cd0bd60 set-pin actor=human:Wido targets=backlog-ordered-by-priority
- 2026-09-08T06:10:06Z 1TZYJK3Q3DMC2HM5MJX3R9B3MR-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=backlog-ordered-by-priority
Integrity: sha256=617667d2c2c05c7e1d5c2fb239875aa1d748b258c322baa7c49f6538e12d9a22
